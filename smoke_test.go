package main

import (
	"os"
	"path/filepath"
	"testing"
)

// Live check of agent(): one real, cheap call that reads a file, so the activity feed has
// something to show. Run with SM_SMOKE=1 go test -run Smoke -v
func TestSmokeAgent(t *testing.T) {
	if os.Getenv("SM_SMOKE") == "" {
		t.Skip("set SM_SMOKE=1 to spend a few cents on a real agent call")
	}
	t.Setenv("SM_STATS", t.TempDir()+"/stats.jsonl")
	var r struct{ Verdict string }
	c := &Ctx{Repo: t.TempDir(), Issue: 1, Scratch: t.TempDir()}
	ok := agent(c, Review, "Read /etc/hostname with the Read tool, then answer: verdict PASS.",
		`{"type":"object","properties":{"verdict":{"enum":["PASS","FAIL"]}},"required":["verdict"],"additionalProperties":false}`,
		"Read", false, &r)
	st, _ := os.ReadFile(filepath.Join(c.Scratch, "status.json"))
	t.Logf("ok=%v verdict=%q\nstatus.json:\n%s", ok, r.Verdict, st)
	if !ok {
		t.Fatal("agent returned invalid")
	}
}
