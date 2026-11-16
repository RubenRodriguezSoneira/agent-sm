package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func useProfile(t *testing.T, p Profile) {
	t.Helper()
	old := settings
	settings = p
	t.Cleanup(func() { settings = old })
}

func writeFile(t *testing.T, path, body string, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), mode); err != nil {
		t.Fatal(err)
	}
}

func TestResultValidation(t *testing.T) {
	for _, raw := range []string{
		`{}`, `{"verdict":"PASS"}`, `{"verdict":"UNKNOWN","reasons":[]}`,
		`{"verdict":"PASS","reasons":null}`, `{"verdict":"PASS","reasons":[1]}`,
		`{"verdict":"PASS","reasons":[],"extra":true}`, `{"verdict":"PASS","verdict":"REVISE","reasons":[]}`,
		`{"verdict":"PASS","reasons":[]} {}`, "```json\n{}\n```", `null`, `{"verdict":[],"reasons":[]}`,
	} {
		var out struct {
			Verdict string
			Reasons []string
		}
		if err := validateResult([]byte(raw), steps[Review].Schema, &out); err == nil {
			t.Errorf("accepted invalid result %s", raw)
		}
	}
	var out struct {
		Verdict string
		Reasons []string
	}
	if err := validateResult([]byte(`{"verdict":"PASS","reasons":["verified"]}`), steps[Review].Schema, &out); err != nil {
		t.Fatal(err)
	}
	if err := validateResult([]byte(`{"status":"plan","objective":"x","points":[],"points_derived":true,"files":[],"verification":"default","needs_investigation":true,"fits_one_pr":true,"units":[{"id":"a","title":"b"}]}`),
		steps[Plan].Schema, new(any)); err == nil {
		t.Fatal("accepted incomplete nested unit")
	}
}

func TestProfileResolution(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "profile.json")
	if err := os.Mkdir(filepath.Join(dir, "notes"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, path, `{"checks":["go test ./..."],"notes_dir":"notes","models":{"execute":"example-model"}}`, 0o600)
	t.Setenv("SM_MODEL_EXECUTE", "override-model")
	p, err := loadProfile(path)
	if err != nil {
		t.Fatal(err)
	}
	if p.Provider != "copilot" || p.NotesDir != filepath.Join(dir, "notes") || p.Models["execute"] != "override-model" {
		t.Fatalf("wrong resolution: %+v", p)
	}
	for _, invalid := range []string{
		`{"unknown":true}`, `{"checks":[]}`, `{"checks":[" "]}`, `{"checks":["true"],"session_limit":0}`,
		`{"checks":["true"],"models":{"check":"auto"}}`, `{"checks":["true"],"commands":{"execute":["git push"]}}`,
		`{"checks":["true"],"commands":{"execute":["rtk proxy git push"]}}`,
		`{"checks":["true"],"commands":{"execute":["dotnet test; curl evil"]}}`,
		`{"checks":["true"],"required_test_reports":["../old.trx"]}`,
		`{"checks":["true"],"claude_allow":{"execute":"Bash"}}`,
	} {
		writeFile(t, path, invalid, 0o600)
		if _, err := loadProfile(path); err == nil {
			t.Errorf("accepted invalid profile %s", invalid)
		}
	}
	writeFile(t, path, `{"checks":["true"]}`, 0o600)
	t.Setenv("SM_BUDGET", "2")
	if _, err := loadProfile(path); err == nil {
		t.Fatal("silently interpreted dollars as Copilot credits")
	}
}

func TestQueuedProfile(t *testing.T) {
	p := defaultProfile("copilot")
	p.Checks = []string{"true"}
	useProfile(t, p)
	got, err := queuedProfile(runRequest{Provider: "copilot", Budget: "3.5", Models: map[string]string{"execute": "chosen-model"}})
	if err != nil || got.Models["execute"] != "chosen-model" || got.SessionLimit != 3.5 {
		t.Fatalf("queued overrides: %+v %v", got, err)
	}
	if len(settings.Models) != 0 || settings.SessionLimit != 2 {
		t.Fatal("queue mutated the server's profile")
	}
	for _, req := range []runRequest{
		{Provider: "claude"}, {Audit: true}, {Budget: "0"}, {Budget: "NaN"},
		{Models: map[string]string{"check": "model"}},
	} {
		if _, err := queuedProfile(req); err == nil {
			t.Errorf("accepted invalid queued override %+v", req)
		}
	}
}

func TestClaudeLegacyEnvironment(t *testing.T) {
	t.Setenv("SM_PROVIDER", "claude")
	t.Setenv("SM_CHECKS", "true")
	t.Setenv("SM_EXTRA_ALLOW", "Bash(custom check:*)")
	t.Setenv("SM_AUDIT", "1")
	p, err := loadProfile("")
	if err != nil || p.ClaudeExtra != "Bash(custom check:*)" || !p.Audit {
		t.Fatalf("legacy overrides lost: %+v %v", p, err)
	}
}

func TestExampleProfiles(t *testing.T) {
	paths, err := filepath.Glob("examples/*.json")
	if err != nil || len(paths) == 0 {
		t.Fatal("missing example profiles")
	}
	for _, path := range paths {
		if _, err := loadProfile(path); err != nil {
			t.Errorf("%s: %v", path, err)
		}
	}
}

func TestSnapshotDoesNotReadChangedEnvironment(t *testing.T) {
	p := defaultProfile("copilot")
	p.Checks = []string{"true"}
	p.Models["plan"] = "queued-model"
	b, _ := json.Marshal(p)
	t.Setenv("SM_RESOLVED_CONFIG", string(b))
	t.Setenv("SM_PROVIDER", "claude")
	t.Setenv("SM_CHECKS", "false")
	t.Setenv("SM_MODEL_PLAN", "changed-model")
	got, err := loadProfile("")
	if err != nil || got.Provider != "copilot" || got.Checks[0] != "true" || got.Models["plan"] != "queued-model" {
		t.Fatalf("snapshot changed: %+v %v", got, err)
	}
}

func TestCopilotPolicy(t *testing.T) {
	p := defaultProfile("copilot")
	p.Commands["execute"] = []string{"dotnet test"}
	useProfile(t, p)
	execute := strings.Join(copilotArgs(Execute, "task", "/tmp/usage", 1), "\n")
	review := strings.Join(copilotArgs(Review, "task", "/tmp/usage", 1), "\n")
	if !strings.Contains(execute, "--allow-tool=shell(dotnet test:*)") ||
		strings.Contains(review, "--allow-tool=shell(dotnet test:*)") ||
		!strings.Contains(review, "--deny-tool=write") ||
		!strings.Contains(execute, "--deny-tool=shell(rtk proxy git push:*)") {
		t.Fatal("incorrect role-specific command policy")
	}
	for _, bad := range []string{"--allow-all", "--trust-folder", "--allow-tool=shell(dotnet:*)"} {
		if strings.Contains(execute, bad) {
			t.Errorf("overbroad policy: %s", bad)
		}
	}
	t.Setenv("COPILOT_ALLOW_ALL", "true")
	t.Setenv("JIRA_API_TOKEN", "test-token")
	env := strings.Join(cleanAgentEnv(), "\n")
	if strings.Contains(env, "test-token") || strings.Contains(env, "COPILOT_ALLOW_ALL=true") {
		t.Fatal("inherited bypass or Jira credential reached agent")
	}
}

func TestGenericPrompts(t *testing.T) {
	useProfile(t, defaultProfile("copilot"))
	for _, state := range []State{Plan, Investigate, Execute, Review} {
		prompt := rolePrompt(state) + dispatch(&Ctx{}, steps[state].Role, "")
		for _, legacy := range []string{"rust-mm", "Ghidra", "rescue-re", "cargo fmt", "py-desmume"} {
			if strings.Contains(prompt, legacy) {
				t.Errorf("%v contains %s", state, legacy)
			}
		}
	}
}

func TestCopilotUsage(t *testing.T) {
	credits, model, err := copilotUsage([]byte(`{"totalNanoAiu":250000000,"modelMetrics":{"actual-model":{}}}`))
	if err != nil || credits != .25 || model != "actual-model" {
		t.Fatalf("%v %s %v", credits, model, err)
	}
	for _, raw := range []string{`{}`, `{"totalNanoAiu":null}`, `{"totalNanoAiu":-1}`, `invalid`} {
		if _, _, err := copilotUsage([]byte(raw)); err == nil {
			t.Errorf("accepted unknown usage %s", raw)
		}
	}
}

func TestProviderResults(t *testing.T) {
	for _, provider := range []string{"claude", "copilot"} {
		for _, mode := range []string{"valid", "incomplete", "failure", "unknown-usage", "budget"} {
			t.Run(provider+"/"+mode, func(t *testing.T) {
				dir := t.TempDir()
				p := defaultProfile(provider)
				p.SessionLimit = 1
				useProfile(t, p)
				writeFile(t, filepath.Join(dir, provider), `#!/bin/sh
usage=""
while [ "$#" -gt 0 ]; do
  if [ "$1" = "--usage-output-file" ]; then shift; usage="$1"; fi
  shift
done
if [ "$PROVIDER" = "copilot" ]; then
  if [ "$FIXTURE_MODE" != "unknown-usage" ]; then
    amount=100000000
    if [ "$FIXTURE_MODE" = "budget" ]; then amount=1000000000; fi
    printf '{"totalNanoAiu":%s,"modelMetrics":{"actual-model":{}}}' "$amount" > "$usage"
  fi
  if [ "$FIXTURE_MODE" = "incomplete" ]; then echo '{}'; else echo '{"verdict":"PASS","reasons":[]}'; fi
else
  cost=',"total_cost_usd":0.1'
  if [ "$FIXTURE_MODE" = "unknown-usage" ]; then cost=""; fi
  subtype=success
  if [ "$FIXTURE_MODE" = "budget" ]; then subtype=error_max_budget_usd; fi
  result='{"verdict":"PASS","reasons":[]}'
  if [ "$FIXTURE_MODE" = "incomplete" ]; then result='{}'; fi
  printf '{"type":"result","subtype":"%s","is_error":false%s,"structured_output":%s}\n' "$subtype" "$cost" "$result"
fi
if [ "$FIXTURE_MODE" = "failure" ]; then exit 1; fi
`, 0o755)
				t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
				t.Setenv("SM_STATS", filepath.Join(dir, "stats.jsonl"))
				t.Setenv("PROVIDER", provider)
				t.Setenv("FIXTURE_MODE", mode)
				c := &Ctx{Repo: dir, Scratch: dir, RunBudget: 5}
				var r struct {
					Verdict string
					Reasons []string
				}
				got := agent(c, Review, "bounded test", &r)
				want := Ok
				if mode == "incomplete" || mode == "failure" {
					want = Invalid
				}
				if mode == "budget" {
					want = OverBudget
				}
				if got != want {
					t.Fatalf("got %v want %v", got, want)
				}
				if mode == "unknown-usage" && c.CostUnknown != 1 {
					t.Fatal("missing usage treated as free")
				}
			})
		}
	}
}

func TestCheckCommandsAndReports(t *testing.T) {
	dir := t.TempDir()
	p := defaultProfile("copilot")
	p.TimeoutMinutes = .001
	useProfile(t, p)
	start := time.Now()
	if _, err := checkCommand(dir, "sleep 300"); err == nil || time.Since(start) > 3*time.Second {
		t.Fatalf("check timeout failed: %v", err)
	}
	for _, counters := range []string{`total="0" executed="0" passed="0"`, `total="2" executed="1" passed="1"`, `total="2" executed="2" passed="1"`} {
		writeFile(t, filepath.Join(dir, "test.trx"), `<TestRun><ResultSummary><Counters `+counters+`/></ResultSummary></TestRun>`, 0o600)
		if err := verifyTestReports(dir, []string{"*.trx"}); err == nil {
			t.Fatal("accepted empty/skipped/failed required tests")
		}
	}
	writeFile(t, filepath.Join(dir, "test.trx"), `<TestRun><ResultSummary><Counters total="2" executed="2" passed="2"/></ResultSummary></TestRun>`, 0o600)
	if err := verifyTestReports(dir, []string{"*.trx"}); err != nil {
		t.Fatal(err)
	}
	if err := verifyTestReports(t.TempDir(), []string{"*.trx"}); err == nil {
		t.Fatal("accepted stale/missing report")
	}
}

func TestUnknownUsageStopsRun(t *testing.T) {
	fake(t, map[State]func(*Ctx) Out{Plan: func(c *Ctx) Out { c.CostUnknown++; return Direct }})
	if exit := run(&Ctx{RunBudget: 20}); exit != GaveUp {
		t.Fatalf("unknown usage continued: %v", exit)
	}
}

func TestMain(m *testing.M) {
	if os.Getenv("AGENT_SM_TEST_HELPER") == "1" {
		main()
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestQueuedCLI(t *testing.T) {
	dir := t.TempDir()
	repo := filepath.Join(dir, "repo")
	if err := os.Mkdir(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", "-q", "-b", "main"}, {"-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "--allow-empty", "-m", "init"}} {
		if out, err := sh(repo, "git", args...); err != nil {
			t.Fatalf("%s %v", out, err)
		}
	}
	if out, err := sh(repo, "git", "checkout", "-q", "-b", "feature"); err != nil {
		t.Fatalf("%s %v", out, err)
	}
	writeFile(t, filepath.Join(repo, "base-marker.txt"), "feature branch", 0o644)
	for _, args := range [][]string{{"add", "base-marker.txt"}, {"-c", "user.name=t", "-c", "user.email=t@t", "commit", "-qm", "feature"}, {"checkout", "-q", "main"}, {"remote", "add", "origin", repo}} {
		if out, err := sh(repo, "git", args...); err != nil {
			t.Fatalf("%s %v", out, err)
		}
	}
	writeFile(t, filepath.Join(dir, "gh"), `#!/bin/sh
case "$1 $2" in
  "issue view") echo '{"title":"fixture task","body":"Base branch: feature","url":"https://github.com/example/repo/issues/1"}';;
  "pr list") echo 0;;
  *) exit 1;;
esac
`, 0o755)
	writeFile(t, filepath.Join(dir, "copilot"), `#!/bin/sh
prompt=""
usage=""
while [ "$#" -gt 0 ]; do
  case "$1" in
    -p) shift; prompt="$1";;
    --usage-output-file) shift; usage="$1";;
  esac
  shift
done
printf '{"totalNanoAiu":100000000,"modelMetrics":{"fixture-model":{}}}' > "$usage"
case "$prompt" in
  *"You are the planner"*) echo '{"status":"plan","objective":"deliver file","points":["file exists"],"points_derived":false,"files":["outcome.txt"],"verification":"default","needs_investigation":false,"fits_one_pr":true,"units":[]}';;
  *"You are the implementation"*) echo delivered > outcome.txt; echo '{"status":"done","summary":"delivered file","closes_issue":true}';;
  *"You independently review"*) echo '{"verdict":"PASS","reasons":["file verified"]}';;
  *) exit 1;;
esac
`, 0o755)
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
	t.Setenv("AGENT_SM_TEST_HELPER", "1")
	t.Setenv("SM_SCRATCH_ROOT", filepath.Join(dir, "scratch"))
	p := defaultProfile("copilot")
	p.Base = "main"
	p.Checks = []string{"test -f outcome.txt"}
	useProfile(t, p)
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	r := &Runner{repo: repo, self: self, stats: filepath.Join(dir, "stats.jsonl")}
	j := Job{Issue: 1, Provider: "copilot", Profile: &p}
	r.runOne(&j)
	if j.Err != "" || j.Exit != "ready" {
		t.Fatalf("queued run: %+v", j)
	}
	if j.Base != "origin/feature" {
		t.Fatalf("declared base not propagated: %s", j.Base)
	}
	if _, err := os.Stat(filepath.Join(j.Worktree, "base-marker.txt")); err != nil {
		t.Fatal("run started from configured main instead of declared feature base")
	}
	staged, err := sh(j.Worktree, "git", "diff", "--cached", "--name-only")
	if err != nil || strings.TrimSpace(staged) != "outcome.txt" {
		t.Fatalf("staged result: %q %v", staged, err)
	}
	b, err := os.ReadFile(r.stats)
	if err != nil || !strings.Contains(string(b), `"provider":"copilot"`) || strings.Contains(string(b), `"usage_unknown":true`) {
		t.Fatalf("stats: %s %v", b, err)
	}
	// No network-backed GitHub mutation is implemented by this fixture.
}

func TestChecksPreserveCandidate(t *testing.T) {
	for _, tc := range []struct {
		name, command, format string
		want                  Out
	}{
		{"pass with spaces", "true", "", Pass},
		{"failed check", "false", "", Fail},
		{"modified worktree", "printf changed > 'allowed name.txt'", "", Fail},
		{"modified index", "printf outside > outside.txt && git add outside.txt", "", Fail},
		{"formatter failure", "true", "false", Fail},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := t.TempDir()
			for _, args := range [][]string{{"init", "-q", "-b", "main"}, {"-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "--allow-empty", "-m", "init"}} {
				if out, err := sh(repo, "git", args...); err != nil {
					t.Fatalf("%s %v", out, err)
				}
			}
			writeFile(t, filepath.Join(repo, "allowed name.txt"), "candidate", 0o644)
			p := defaultProfile("copilot")
			p.Checks = []string{tc.command}
			p.Format = tc.format
			useProfile(t, p)
			c := &Ctx{Repo: repo, Scratch: t.TempDir(), Files: []string{"allowed name.txt"}}
			if got := check(c); got != tc.want {
				t.Fatalf("got %v want %v: %s", got, tc.want, c.Feedback)
			}
		})
	}
}

func TestCopilotStop(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "copilot"), "#!/bin/sh\nsleep 300\n", 0o755)
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
	t.Setenv("SM_STATS", filepath.Join(dir, "stats.jsonl"))
	p := defaultProfile("copilot")
	p.TimeoutMinutes = .001
	useProfile(t, p)
	got := agent(&Ctx{Repo: dir, Scratch: dir}, Review, "test", new(any))
	if got != Timeout {
		t.Fatalf("got %v", got)
	}
}

func TestUsageReportSeparatesProviders(t *testing.T) {
	got := report([]byte(`{"state":"execute","provider":"copilot","credits":1.5}
{"state":"execute","provider":"copilot","usage_unknown":true}
{"state":"execute","provider":"claude","cost_usd":0.2}`))
	for _, want := range []string{"copilot/execute", "1.500 known credits", "1 unknown-usage", "claude/execute", "$0.200"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %s in %s", want, got)
		}
	}
}
