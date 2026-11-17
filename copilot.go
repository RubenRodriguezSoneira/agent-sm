package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	copilot "github.com/github/copilot-sdk/go"
	"github.com/github/copilot-sdk/go/rpc"
)

// copilotRequest is one Copilot SDK session. Decide answers every tool call before it runs;
// Event sees every session event, in order.
type copilotRequest struct {
	Repo      string                                     `json:"cwd"`
	Home      string                                     `json:"-"`
	SessionID string                                     `json:"session_id,omitempty"`
	Model     string                                     `json:"model"`
	System    string                                     `json:"system_message"`
	Prompt    string                                     `json:"prompt"`
	Schema    map[string]any                             `json:"response_schema"`
	Tools     []string                                   `json:"available_tools"`
	AddDirs   []string                                   `json:"additional_directories,omitempty"`
	Limit     float64                                    `json:"max_ai_credits"`
	Decide    func(tool string, args any) (bool, string) `json:"-"`
	Event     func(copilot.SessionEvent)                 `json:"-"`
}

type copilotReply struct {
	Content    string                     // the final assistant message
	Usage      *rpc.UsageGetMetricsResult // nil when the runtime could not report it
	OverBudget bool                       // the session hit its AI-credit limit
}

// copilotSession runs one prompt in a fresh Copilot CLI process; tests replace it.
var copilotSession = runCopilotSession

func runCopilotSession(ctx context.Context, req copilotRequest) (reply copilotReply, err error) {
	path, err := exec.LookPath("copilot")
	if err != nil {
		return reply, err
	}
	// ModeEmpty and a private COPILOT_HOME: the operator's MCP servers, skills, custom agents,
	// instructions and session store are never loaded.
	client := copilot.NewClient(&copilot.ClientOptions{Mode: copilot.ModeEmpty, BaseDirectory: req.Home,
		WorkingDirectory: req.Repo, Env: cleanAgentEnv(), Connection: copilot.StdioConnection{Path: path}})
	if err := client.Start(ctx); err != nil {
		return reply, fmt.Errorf("start Copilot CLI: %w", err)
	}
	defer client.Stop()
	var mu sync.Mutex
	var session *copilot.Session
	limit := req.Limit
	cfg := &copilot.SessionConfig{Model: req.Model, WorkingDirectory: req.Repo, SessionID: req.SessionID,
		AvailableTools: req.Tools, AdditionalDirectories: req.AddDirs,
		SessionLimits: &rpc.SessionLimitsConfig{MaxAiCredits: &limit},
		SystemMessage: &copilot.SystemMessageConfig{Mode: "customize", Content: req.System,
			Sections: map[string]copilot.SectionOverride{"environment_context": {Action: copilot.SectionActionPreserve}}},
		Hooks: &copilot.SessionHooks{OnPreToolUse: func(in copilot.PreToolUseHookInput, _ copilot.HookInvocation) (*copilot.PreToolUseHookOutput, error) {
			if ok, why := req.Decide(in.ToolName, in.ToolArgs); !ok {
				return &copilot.PreToolUseHookOutput{PermissionDecision: "deny", PermissionDecisionReason: why}, nil
			}
			return &copilot.PreToolUseHookOutput{PermissionDecision: "allow"}, nil
		}},
		// Every call the hook allowed skips the runtime prompt; anything that still asks is unexpected.
		OnPermissionRequest: func(copilot.PermissionRequest, copilot.PermissionInvocation) (rpc.PermissionDecision, error) {
			why := "agent-sm answers tool permissions in its preToolUse hook only"
			return &rpc.PermissionDecisionReject{Feedback: &why}, nil
		},
		OnEvent: func(e copilot.SessionEvent) {
			if d, ok := e.Data.(*copilot.SessionLimitsExhaustedRequestedData); ok {
				mu.Lock()
				reply.OverBudget = true
				s := session
				mu.Unlock()
				if s != nil {
					go s.RPC.UI.HandlePendingSessionLimitsExhausted(context.WithoutCancel(ctx), &rpc.UIHandlePendingSessionLimitsExhaustedRequest{
						RequestID: d.RequestID, Response: rpc.UISessionLimitsExhaustedResponse{Action: rpc.UISessionLimitsExhaustedResponseActionCancel}})
				}
			}
			req.Event(e)
		}}
	s, err := client.CreateSession(ctx, cfg)
	if err != nil {
		return reply, fmt.Errorf("create Copilot session: %w", err)
	}
	mu.Lock()
	session = s
	mu.Unlock()
	final, err := s.SendAndWait(ctx, copilot.MessageOptions{Prompt: req.Prompt, ResponseSchema: req.Schema})
	// A timed-out or stopped turn is aborted, then usage is read: credits are spent either way.
	after, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	if ctx.Err() != nil {
		s.Abort(after)
	}
	usage, usageErr := s.RPC.Usage.GetMetrics(after)
	if usageErr != nil {
		logEvent(map[string]any{"event": "usage-error", "err": usageErr.Error()})
	}
	s.Disconnect()
	mu.Lock()
	defer mu.Unlock()
	reply.Usage = usage
	if final != nil {
		if d, ok := final.Data.(*copilot.AssistantMessageData); ok {
			reply.Content = d.Content
		}
	}
	return reply, err
}

func copilotAgent(c *Ctx, state State, prompt string, out any, transcript *agentTranscript) (res Out) {
	limit := settings.SessionLimit
	if c.RunBudget > 0 {
		limit = min(limit, c.RunBudget-c.Cost)
		if c.CostUnknown > 0 {
			return OverBudget
		}
	}
	if limit < minCopilotCredits {
		logEvent(map[string]any{"event": "run-budget-reached",
			"err": fmt.Sprintf("Copilot requires at least %d AI credits for a step; effective limit is %.2f", minCopilotCredits, limit)})
		return OverBudget
	}
	ctx, cancel := context.WithTimeout(runCtx, time.Duration(settings.TimeoutMinutes*float64(time.Minute)))
	defer cancel()
	repo, err := filepath.Abs(c.Repo)
	if err != nil {
		logEvent(map[string]any{"event": "agent-error", "err": err.Error()})
		return Invalid
	}
	if err := os.MkdirAll(c.Scratch, 0o755); err != nil {
		logEvent(map[string]any{"event": "agent-error", "err": err.Error()})
		return Invalid
	}
	home, err := os.MkdirTemp(c.Scratch, "copilot-home-")
	if err != nil {
		logEvent(map[string]any{"event": "agent-error", "err": err.Error()})
		return Invalid
	}
	defer os.RemoveAll(home) // exact per-session directory, never a caller-provided root
	var schema map[string]any
	if err := json.Unmarshal([]byte(steps[state].Schema), &schema); err != nil {
		panic(fmt.Sprintf("schema for %s: %v", state, err))
	}
	tools := []string{"view", "grep", "glob", "bash"}
	if steps[state].Edit {
		tools = append(tools, "edit", "create")
	}
	req := copilotRequest{Repo: repo, Home: home, Model: stepModel(steps[state]), System: rolePrompt(state), Prompt: prompt,
		Schema: strictSchema(schema), Tools: tools, AddDirs: settings.AddDirs, Limit: limit}

	st := &Stats{Provider: "copilot", Issue: c.Issue, State: state.String(), UsageUnknown: true,
		ToolCalls: map[string]int{}, BashCmds: map[string]int{}, DeniedTools: map[string]int{}}
	if state == Investigate && len(c.Units) > 0 {
		st.Unit = c.Units[c.UnitIdx].ID
	}
	start := time.Now()
	prog := &progress{c: c, state: state, unit: st.Unit, start: start}
	// Hook and event callbacks run on SDK goroutines.
	var mu sync.Mutex
	req.Decide = func(tool string, args any) (bool, string) {
		ok, why := copilotDecision(state, repo, tool, args)
		if !ok {
			mu.Lock()
			defer mu.Unlock()
			st.Denials++
			st.DeniedTools[tool]++
			b, _ := json.Marshal(args)
			if len(st.DeniedSample) < 5 {
				st.DeniedSample = append(st.DeniedSample, tool+" "+tail(string(b), 200))
			}
			prog.add("error", "denied "+tool+": "+why)
		}
		return ok, why
	}
	req.Event = func(e copilot.SessionEvent) {
		mu.Lock()
		defer mu.Unlock()
		if transcript != nil {
			if b, ok := transcriptEvent(e); ok {
				transcript.stdout.Write(append(b, '\n'))
			}
		}
		switch d := e.Data.(type) {
		case *copilot.SessionStartData:
			st.SessionID = d.SessionID
		case *copilot.AssistantTurnStartData:
			st.Turns++
		case *copilot.ToolExecutionStartData:
			st.ToolCalls[d.ToolName]++
			var in toolInput
			b, _ := json.Marshal(d.Arguments)
			json.Unmarshal(b, &in)
			if d.ToolName == "bash" {
				for _, w := range cmdWords(in.Command) {
					st.BashCmds[w]++
				}
			}
			prog.tool(d.ToolName, in)
		case *copilot.ToolExecutionCompleteData:
			if !d.Success && (d.Error == nil || d.Error.Code == nil || *d.Error.Code != "denied") {
				prog.add("error", "the last tool call returned an error")
			}
		case *copilot.AssistantMessageData:
			if d.ParentToolCallID == nil {
				prog.add("say", d.Content)
			}
		case *copilot.AssistantUsageData:
			st.Model = d.Model
			st.InputTokens += int(deref(d.InputTokens))
			st.OutputTokens += int(deref(d.OutputTokens))
			st.CacheCreation += int(deref(d.CacheWriteTokens))
			st.CacheRead += int(deref(d.CacheReadTokens))
		case *copilot.SessionErrorData:
			prog.add("error", d.Message)
			logEvent(map[string]any{"event": "agent-error", "err": d.ErrorType + ": " + d.Message})
		}
	}
	if transcript != nil {
		req.SessionID = transcript.sessionID
		if err := transcript.request(req, state, cancel); err != nil {
			logEvent(map[string]any{"event": "transcript-error", "err": err.Error()})
			return Invalid
		}
	}
	prog.add("say", "Copilot session started")
	reply, err := copilotSession(ctx, req)
	mu.Lock()
	defer mu.Unlock()
	st.DurationMs = int(time.Since(start).Milliseconds())
	st.Killed = ctx.Err() != nil
	if u := reply.Usage; u != nil && u.TotalNanoAiu != nil && finiteNonnegative(*u.TotalNanoAiu) {
		credits := *u.TotalNanoAiu / 1e9
		st.Credits = &credits
		c.Cost += credits // a partial total is still a floor
		st.UsageUnknown = u.AiCreditsStatus != nil && *u.AiCreditsStatus != rpc.AiCreditsStatusComplete
		if st.Model == "" && len(u.ModelMetrics) == 1 {
			for m := range u.ModelMetrics {
				st.Model = m
			}
		}
	}
	if st.UsageUnknown {
		c.CostUnknown++
		logEvent(map[string]any{"event": "usage-error", "err": "Copilot reported no complete AI-credit total"})
	}
	defer func() {
		st.Outcome, st.OK = res.String(), res == Ok
		if transcript != nil {
			b, _ := json.Marshal(map[string]any{"type": "result", "exitCode": map[bool]int{true: 0, false: 1}[res == Ok]})
			transcript.stdout.Write(append(b, '\n'))
		}
		writeStats(st)
		logEvent(map[string]any{"event": "agent", "stats": st})
		prog.add("say", "Copilot session ended: "+res.String())
		prog.flush(true)
	}()
	switch {
	case reply.OverBudget || st.Credits != nil && *st.Credits >= limit:
		return OverBudget
	case ctx.Err() == context.DeadlineExceeded:
		return Timeout
	case err != nil:
		logEvent(map[string]any{"event": "agent-error", "err": err.Error()})
		return Invalid
	}
	result := dropNulls([]byte(reply.Content))
	if err := validateResult(result, steps[state].Schema, out); err != nil {
		logEvent(map[string]any{"event": "invalid-result", "err": err.Error()})
		return Invalid
	}
	st.Result = tail(string(result), 1500)
	st.Summary = resultSummary(state.String(), result)
	return Ok
}

// transcriptEvent keeps what an auditor reads. model.* events are debug records that resend the
// whole conversation on every call, and encrypted reasoning cannot be read; both would make long
// sessions' transcripts outgrow the archive reader.
func transcriptEvent(e copilot.SessionEvent) ([]byte, bool) {
	if strings.HasPrefix(string(e.Type()), "model.") {
		return nil, false
	}
	if m, ok := e.Data.(*copilot.AssistantMessageData); ok {
		trimmed := *m
		trimmed.EncryptedContent, trimmed.ReasoningOpaque, trimmed.ReasoningBlocks = nil, nil, nil
		e.Data = &trimmed
	}
	b, err := json.Marshal(e)
	return b, err == nil
}

// strictSchema rewrites a step schema into the strict form Copilot's response_format demands:
// every property is required, and properties that were optional accept null instead.
func strictSchema(schema map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range schema {
		out[k] = v
	}
	if items, ok := schema["items"].(map[string]any); ok {
		out["items"] = strictSchema(items)
	}
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		return out
	}
	required := map[string]bool{}
	if r, ok := schema["required"].([]any); ok {
		for _, k := range r {
			required[fmt.Sprint(k)] = true
		}
	}
	all, strict := make([]any, 0, len(props)), map[string]any{}
	for k, v := range props {
		p, _ := v.(map[string]any)
		p = strictSchema(p)
		if !required[k] {
			if t, ok := p["type"].(string); ok {
				p["type"] = []any{t, "null"}
			} else {
				p = map[string]any{"anyOf": []any{p, map[string]any{"type": "null"}}}
			}
		}
		all, strict[k] = append(all, k), p
	}
	sort.Slice(all, func(i, j int) bool { return all[i].(string) < all[j].(string) })
	out["properties"], out["required"] = strict, all
	return out
}

// dropNulls undoes strictSchema's encoding of omitted optional fields. Step schemas never
// allow null, so removing null members cannot hide an invalid result: a missing required
// field still fails validation. Unparseable input is returned unchanged for validation to report.
func dropNulls(raw []byte) []byte {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	v, err := uniqueJSON(d)
	if err != nil || !bytes.Contains(raw, []byte("null")) {
		return raw
	}
	var strip func(any) any
	strip = func(v any) any {
		switch t := v.(type) {
		case map[string]any:
			for k, x := range t {
				if x == nil {
					delete(t, k)
				} else {
					t[k] = strip(x)
				}
			}
		case []any:
			for i, x := range t {
				t[i] = strip(x)
			}
		}
		return v
	}
	b, err := json.Marshal(strip(v))
	if err != nil {
		return raw
	}
	return b
}

func deref[T any](p *T) T {
	var zero T
	if p == nil {
		return zero
	}
	return *p
}
