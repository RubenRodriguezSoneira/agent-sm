package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Job is one issue to process, started from the web page.
type Job struct {
	Issue      int               `json:"issue"`
	Publish    string            `json:"publish"` // "" (local only) or "pr"; merge is CLI-only
	Audit      bool              `json:"audit"`
	Budget     string            `json:"budget"`
	Discard    bool              `json:"discard"`          // remove an earlier run's worktree and branch first
	Models     map[string]string `json:"models,omitempty"` // state -> model chosen on the page
	Started    string            `json:"started,omitempty"`
	Worktree   string            `json:"worktree,omitempty"`
	Exit       string            `json:"exit,omitempty"`
	Err        string            `json:"err,omitempty"`
	Provider   string            `json:"provider,omitempty"`
	Base       string            `json:"base,omitempty"`
	Profile    *Profile          `json:"-"`
	Management string            `json:"management,omitempty"`
}

// Runner runs queued jobs one at a time, each as `agent-sm <worktree> <issue>` in its own
// process group so Stop can end the whole run (claude, probes and all).
type Runner struct {
	repo, self, stats string
	mu                sync.Mutex
	queue             []Job
	cur               *Job
	cmd               *exec.Cmd
	history           []Job
	working, stopping bool
}

var (
	budgetRe = regexp.MustCompile(`^\d+(\.\d+)?$`)
	modelRe  = regexp.MustCompile(`^[a-z0-9][a-z0-9.\-]{0,63}$`)
)

// agentModels is each agent state's current default model, for the page's pickers.
func agentModels() map[string]string {
	m := map[string]string{}
	for _, st := range steps {
		if st.Role != "" {
			m[st.Name] = stepModel(st)
		}
	}
	return m
}

func (r *Runner) Enqueue(jobs []Job) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.queue = append(r.queue, jobs...)
	if !r.working {
		r.working = true
		go r.loop()
	}
}

func (r *Runner) loop() {
	for {
		r.mu.Lock()
		if len(r.queue) == 0 {
			r.working, r.cur = false, nil
			r.mu.Unlock()
			return
		}
		j := r.queue[0]
		r.queue = r.queue[1:]
		j.Started = time.Now().Format(time.RFC3339)
		r.cur = &j
		r.mu.Unlock()

		r.runOne(&j)

		r.mu.Lock()
		r.history = append(r.history, j)
		if len(r.history) > 30 {
			r.history = r.history[1:]
		}
		r.cur, r.cmd, r.stopping = nil, nil, false
		r.mu.Unlock()
	}
}

func (r *Runner) set(f func()) { r.mu.Lock(); f(); r.mu.Unlock() }

func (r *Runner) worktreeRoot() string {
	return filepath.Join(filepath.Dir(r.repo), filepath.Base(r.repo)+"-worktrees")
}

// leftovers lists the issues whose worktree from an earlier run is still there.
func (r *Runner) leftovers() []int {
	out := []int{}
	dirs, _ := filepath.Glob(filepath.Join(r.worktreeRoot(), "*-sm"))
	for _, d := range dirs {
		if n, err := strconv.Atoi(strings.TrimSuffix(filepath.Base(d), "-sm")); err == nil {
			out = append(out, n)
		}
	}
	return out
}

// worktree makes a fresh detached worktree of the base branch for the issue. An earlier run's
// worktree holds that run's result (its staged changes), so it is only removed, with its local
// branch, when discard says so.
func (r *Runner) worktree(n int, discard bool) (string, error) {
	return r.worktreeAt(n, discard, baseBranch())
}

func (r *Runner) worktreeAt(n int, discard bool, base string) (string, error) {
	dir := filepath.Join(r.worktreeRoot(), fmt.Sprintf("%d-sm", n))
	if _, err := os.Stat(dir); err == nil {
		if !discard {
			return "", fmt.Errorf("worktree %s already exists from an earlier run: select the issue again and use \"Discard old work and run\"", dir)
		}
		if out, err := sh(r.repo, "git", "worktree", "remove", "--force", dir); err != nil {
			return "", fmt.Errorf("cannot discard %s: %s", dir, tail(out, 300))
		}
	}
	if discard {
		sh(r.repo, "git", "branch", "-D", "sm/issue-"+strconv.Itoa(n)) // absent is fine
	}
	if strings.HasPrefix(base, "origin/") {
		if out, err := sh(r.repo, "git", "fetch", "--", "origin", strings.TrimPrefix(base, "origin/")); err != nil {
			return "", fmt.Errorf("fetch base: %s", tail(out, 300))
		}
	}
	if out, err := sh(r.repo, "git", "worktree", "add", "--detach", dir, base); err != nil {
		return "", fmt.Errorf("git worktree add: %s", tail(out, 300))
	}
	for _, f := range strings.Split(os.Getenv("SM_LINKS"), ",") { // gitignored files the run needs, e.g. the ROM dump
		if f = strings.TrimSpace(f); f != "" {
			os.Symlink(filepath.Join(r.repo, f), filepath.Join(dir, f))
		}
	}
	return dir, nil
}

func childEnv(j *Job, stats string) []string {
	skip := map[string]bool{"SM_SCRATCH": true, "SM_EVENTS": true, "SM_PUBLISH": true, "SM_AUDIT": true, "SM_BUDGET": true, "SM_STATS": true, "SM_RESOLVED_CONFIG": true, "SM_WORKTREE_BASE": true}
	var env []string
	for _, e := range os.Environ() {
		if k, _, _ := strings.Cut(e, "="); !skip[k] {
			env = append(env, e)
		}
	}
	env = append(env, "SM_STATS="+stats)
	if j.Publish == levelPR {
		env = append(env, "SM_PUBLISH=pr")
	}
	if j.Audit {
		env = append(env, "SM_AUDIT=1")
	}
	if j.Budget != "" {
		env = append(env, "SM_BUDGET="+j.Budget)
	}
	for state, m := range j.Models { // appended last, so the page's choice wins over the server's environment
		env = append(env, "SM_MODEL_"+strings.ToUpper(state)+"="+m)
	}
	p := settings
	if j.Profile != nil {
		p = *j.Profile
	}
	models := map[string]string{}
	for state, m := range p.Models {
		models[state] = m
	}
	for state, m := range j.Models {
		models[state] = m
	}
	p.Models = models
	b, _ := json.Marshal(p)
	env = append(env, "SM_RESOLVED_CONFIG="+string(b), "SM_WORKTREE_BASE="+j.Base)
	return env
}

func (r *Runner) runOne(j *Job) {
	p := settings
	if j.Profile != nil {
		p = *j.Profile
	}
	out, err := sh(r.repo, "gh", "issue", "view", strconv.Itoa(j.Issue), "--json", "body")
	var issue struct{ Body string }
	if err != nil || json.Unmarshal([]byte(out), &issue) != nil {
		r.set(func() { j.Err = "cannot read issue metadata before creating worktree" })
		return
	}
	meta, err := issueMetadata(issue.Body)
	if err != nil {
		r.set(func() { j.Err = err.Error() })
		return
	}
	base := p.Base
	if meta.Base != "" {
		base = meta.Base
	}
	r.set(func() { j.Base = base })
	wt, err := r.worktreeAt(j.Issue, j.Discard, base)
	if err != nil {
		r.set(func() { j.Err = err.Error() })
		return
	}
	r.set(func() { j.Worktree = wt })
	scratch := filepath.Join(scratchRoot(), strconv.Itoa(j.Issue))
	if err := os.MkdirAll(scratch, 0o755); err != nil {
		r.set(func() { j.Err = err.Error() })
		return
	}
	logf, err := os.Create(filepath.Join(scratch, "run.log"))
	if err != nil {
		r.set(func() { j.Err = err.Error() })
		return
	}
	defer logf.Close()
	cmd := exec.Command(r.self, wt, strconv.Itoa(j.Issue))
	cmd.Env = childEnv(j, r.stats)
	cmd.Stdout, cmd.Stderr = logf, logf
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		r.set(func() { j.Err = err.Error() })
		return
	}
	r.set(func() { r.cmd = cmd })
	cmd.Wait()
	exit := "died" // no exit event: the process ended abnormally
	for _, l := range readLines(filepath.Join(scratch, "events.jsonl"), 1<<20) {
		var e struct {
			Exit, Management string
			ManagementError  string `json:"management_error"`
		}
		if json_unmarshal(l, &e) && e.Exit != "" {
			exit = e.Exit
			r.set(func() {
				j.Management = e.Management
				if e.ManagementError != "" {
					j.Err = "Synchronization incomplete: " + e.ManagementError
				}
			})
		}
	}
	r.mu.Lock()
	if r.stopping {
		exit = "stopped"
	}
	j.Exit = exit
	r.mu.Unlock()
	if p.Management.Mode == "service" && exit != string(Ready) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		m, err := newManager(ctx, r.repo, p)
		if err == nil {
			err = m.deactivate(ctx, j.Issue, exit)
		}
		if err != nil {
			r.set(func() { j.Err = "synchronization deactivation failed: " + err.Error() })
		}
	}
}

// Stop clears the queue and ends the running job's whole process group.
func (r *Runner) Stop() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.queue, r.stopping = nil, true
	if r.cmd != nil && r.cmd.Process != nil {
		syscall.Kill(-r.cmd.Process.Pid, syscall.SIGTERM)
	}
}

func (r *Runner) Snapshot() map[string]any {
	r.mu.Lock()
	defer r.mu.Unlock()
	var cur *Job
	if r.cur != nil {
		c := *r.cur
		cur = &c
	}
	return map[string]any{"can_run": true, "repo": r.repo, "leftover": r.leftovers(), "provider": settings.Provider,
		"models": agentModels(), "session_limit": settings.SessionLimit, "usage_unit": settings.unit(),
		"running": cur, "queued": append([]Job{}, r.queue...), "history": append([]Job{}, r.history...)}
}
