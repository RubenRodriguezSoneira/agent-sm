package main

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"
)

var commandPrefix = regexp.MustCompile(`^[a-zA-Z0-9_./~-]+(?: [a-zA-Z0-9_./~-]+)*$`)

func agent(c *Ctx, state State, prompt string, out any) (result Out) {
	var transcript *agentTranscript
	if settings.Transcripts {
		var err error
		transcript, err = newTranscript(c, state)
		if err != nil {
			logEvent(map[string]any{"event": "transcript-error", "err": err.Error()})
			return Invalid
		}
		defer func() {
			if err := transcript.finish(result); err != nil {
				logEvent(map[string]any{"event": "transcript-error", "path": transcript.dir, "err": err.Error()})
				result = Invalid
			}
		}()
	}
	if settings.Provider == "claude" {
		return claudeAgent(c, state, prompt, out, transcript)
	}
	return copilotAgent(c, state, prompt, out, transcript)
}

func process(ctx context.Context, dir, name string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	// A descendant holding a pipe must not keep an otherwise exited session alive.
	cmd.WaitDelay = time.Second
	return cmd
}

type boundedOutput struct {
	bytes.Buffer
	limit int
}

func (b *boundedOutput) Write(p []byte) (int, error) {
	n := len(p)
	if b.Len()+n > b.limit {
		remaining := max(0, b.limit-b.Len())
		b.Buffer.Write(p[:remaining])
		return n, fmt.Errorf("output exceeds %d bytes", b.limit)
	}
	return b.Buffer.Write(p)
}

func copilotArgs(state State, prompt, usagePath string, limit float64) []string {
	// Silent text is the documented final-response transport. Validate all of stdout:
	// no JSON extracted from prose, event envelopes, or markdown fences.
	prompt = rolePrompt(state) + "\n\n" + prompt + "\n\nReturn exactly one JSON object, without markdown or other text. Required schema:\n" + steps[state].Schema
	args := []string{"-p", prompt, "--silent", "--stream=off", "--output-format=text",
		"--model", stepModel(steps[state]), "--max-ai-credits", strconv.FormatFloat(limit, 'f', -1, 64),
		"--usage-output-file", usagePath, "--no-ask-user", "--no-auto-update", "--disable-builtin-mcps"}
	// Restrict visibility as well as permissions: inherited MCPs/subagents cannot
	// become available merely because the operator has them installed.
	tools := "view,grep,glob,bash"
	if steps[state].Edit {
		tools += ",edit,create"
		args = append(args, "--allow-tool=write")
	} else {
		args = append(args, "--deny-tool=write")
	}
	args = append(args, "--available-tools="+tools, "--allow-tool=read")
	for _, entry := range strings.Split(configuredAllow(state), ",") {
		if prefix, ok := strings.CutPrefix(entry, "Bash("); ok {
			prefix = strings.TrimSuffix(prefix, ":*)")
			args = append(args, "--allow-tool=shell("+prefix+":*)")
		}
	}
	for _, prefix := range outward {
		args = append(args, "--deny-tool=shell("+prefix+":*)",
			"--deny-tool=shell(rtk "+prefix+":*)", "--deny-tool=shell(rtk proxy "+prefix+":*)")
	}
	if settings.Management.Mode == "service" {
		for _, prefix := range []string{"gh", "curl", "wget"} {
			for _, wrapper := range []string{"", "rtk ", "rtk proxy "} {
				args = append(args, "--deny-tool=shell("+wrapper+prefix+":*)")
			}
		}
	}
	for _, dir := range settings.AddDirs {
		args = append(args, "--add-dir", dir)
	}
	return args
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
	var dir string
	var err error
	if transcript != nil {
		dir = transcript.dir
	} else {
		dir, err = os.MkdirTemp(c.Scratch, "copilot-")
		if err != nil {
			logEvent(map[string]any{"event": "agent-error", "err": err.Error()})
			return Invalid
		}
		defer os.RemoveAll(dir) // exact per-session directory, never a caller-provided root
	}
	usagePath := filepath.Join(dir, "usage.json")
	args := copilotArgs(state, prompt, usagePath, limit)
	if transcript != nil {
		args = append(args, "--session-id", transcript.sessionID, "--sync-session=off")
		if err := transcript.request(args, state, cancel); err != nil {
			logEvent(map[string]any{"event": "transcript-error", "err": err.Error()})
			return Invalid
		}
	}
	cmd := process(ctx, c.Repo, "copilot", args...)
	// Never inherit permission bypasses from a parent agent session.
	cmd.Env = cleanAgentEnv()
	stdout := &boundedOutput{limit: 8 << 20}
	stderr := &boundedOutput{limit: 64 << 10}
	cmd.Stdout, cmd.Stderr = stdout, stderr
	if transcript != nil {
		cmd.Stdout = io.MultiWriter(transcript.stdout, stdout)
		cmd.Stderr = io.MultiWriter(transcript.stderr, stderr)
	}
	st := &Stats{Provider: "copilot", Issue: c.Issue, State: state.String(), UsageUnknown: true}
	if state == Investigate && len(c.Units) > 0 {
		st.Unit = c.Units[c.UnitIdx].ID
	}
	start := time.Now()
	prog := &progress{c: c, state: state, unit: st.Unit, start: start}
	prog.add("say", "Copilot session started (tool-level telemetry unavailable in final-response mode)")
	err = cmd.Run()
	st.DurationMs = int(time.Since(start).Milliseconds())
	st.Killed = ctx.Err() != nil
	if b, readErr := os.ReadFile(usagePath); readErr == nil {
		credits, model, usageErr := copilotUsage(b)
		if usageErr == nil {
			st.Credits, st.Model, st.UsageUnknown = &credits, model, false
			c.Cost += credits
		} else {
			logEvent(map[string]any{"event": "usage-error", "err": usageErr.Error()})
		}
	} else {
		logEvent(map[string]any{"event": "usage-error", "err": readErr.Error()})
	}
	if st.UsageUnknown {
		c.CostUnknown++
	}
	defer func() {
		st.Outcome, st.OK = res.String(), res == Ok
		writeStats(st)
		logEvent(map[string]any{"event": "agent", "stats": st})
		prog.add("say", "Copilot session ended: "+res.String())
		prog.flush(true)
	}()
	if ctx.Err() == context.DeadlineExceeded {
		return Timeout
	}
	if err != nil {
		// Native budget errors have changed between CLI releases; preserve diagnostics
		// and fail rather than treating an arbitrary error string as a completed result.
		logEvent(map[string]any{"event": "agent-error", "err": err.Error(), "stderr": tail(stderr.String(), 1500)})
		if st.Credits != nil && *st.Credits >= limit {
			return OverBudget
		}
		return Invalid
	}
	if st.Credits != nil && *st.Credits >= limit {
		return OverBudget
	}
	if err := validateResult(stdout.Bytes(), steps[state].Schema, out); err != nil {
		logEvent(map[string]any{"event": "invalid-result", "err": err.Error()})
		return Invalid
	}
	st.Result = tail(stdout.String(), 1500)
	return Ok
}

func copilotUsage(b []byte) (float64, string, error) {
	var u struct {
		TotalNanoAIU *float64                   `json:"totalNanoAiu"`
		ModelMetrics map[string]json.RawMessage `json:"modelMetrics"`
	}
	if err := json.Unmarshal(b, &u); err != nil {
		return 0, "", err
	}
	if u.TotalNanoAIU == nil || !finiteNonnegative(*u.TotalNanoAIU) {
		return 0, "", fmt.Errorf("Copilot usage contains no complete AI-credit total")
	}
	model := ""
	if len(u.ModelMetrics) == 1 {
		for m := range u.ModelMetrics {
			model = m
		}
	}
	return *u.TotalNanoAIU / 1e9, model, nil
}

func cleanAgentEnv() []string {
	var env []string
	for _, entry := range os.Environ() {
		k, _, _ := strings.Cut(entry, "=")
		switch k {
		case "COPILOT_ALLOW_ALL", "COPILOT_ASSISTED_APPROVAL", "GITHUB_COPILOT_PROMPT_MODE_EXTENSIONS",
			"SM_RESOLVED_CONFIG", "JIRA_API_TOKEN", "JIRA_USER_EMAIL", "BASH_ENV", "ENV":
			continue
		}
		if settings.Management.Mode == "service" {
			switch k {
			case "GH_TOKEN", "GITHUB_TOKEN", "GH_ENTERPRISE_TOKEN", "GITHUB_ENTERPRISE_TOKEN":
				continue
			}
		}
		env = append(env, entry)
	}
	env = append(env, "COPILOT_ALLOW_ALL=false", "COPILOT_ASSISTED_APPROVAL=false", "GITHUB_COPILOT_PROMPT_MODE_EXTENSIONS=false")
	return env
}

func checkCommand(dir, command string, env ...string) (string, error) {
	ctx, cancel := context.WithTimeout(runCtx, time.Duration(settings.TimeoutMinutes*float64(time.Minute)))
	defer cancel()
	cmd := process(ctx, dir, "sh", "-c", command)
	cmd.Env = append(cleanAgentEnv(), env...)
	output := &boundedOutput{limit: 4 << 20}
	cmd.Stdout, cmd.Stderr = output, output
	err := cmd.Run()
	logEvent(map[string]any{"event": "check-command", "command": command, "ok": err == nil})
	return output.String(), err
}

func verifyTestReports(dir string, patterns []string) error {
	for _, pattern := range patterns {
		files, err := filepath.Glob(filepath.Join(dir, pattern))
		if err != nil || len(files) == 0 {
			return fmt.Errorf("required test report %q missing", pattern)
		}
		for _, path := range files {
			b, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			var report struct {
				XMLName  xml.Name `xml:"TestRun"`
				Counters struct {
					Total    int `xml:"total,attr"`
					Executed int `xml:"executed,attr"`
					Passed   int `xml:"passed,attr"`
				} `xml:"ResultSummary>Counters"`
			}
			if err := xml.Unmarshal(b, &report); err != nil {
				return fmt.Errorf("invalid TRX %s: %w", path, err)
			}
			n := report.Counters
			if n.Executed == 0 || n.Executed != n.Total || n.Passed != n.Executed {
				return fmt.Errorf("required test report %s: total=%d executed=%d passed=%d", path, n.Total, n.Executed, n.Passed)
			}
		}
	}
	return nil
}
