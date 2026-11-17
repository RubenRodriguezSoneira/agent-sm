package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Copilot tool calls are answered by agent-sm itself (the SDK preToolUse hook), so the policy
// below is the whole permission model: no CLI flag or runtime default grants anything.

var (
	shellSubstRe = regexp.MustCompile("\\$\\(|`|<\\(|>\\(")
	safeRedirRe  = regexp.MustCompile(`\d*>&\d+|&?\d*>>?\s*/dev/null\b`)
	// Programs that run another command line, which a prefix check cannot see into.
	shellWrappers = map[string]bool{"sh": true, "bash": true, "zsh": true, "dash": true, "eval": true, "exec": true,
		"env": true, "xargs": true, "nohup": true, "timeout": true, "nice": true, "command": true, "builtin": true,
		"source": true, ".": true, "sudo": true, "doas": true, "su": true}
)

// copilotDecision allows or denies one Copilot tool call. A denial's reason is shown to the model.
func copilotDecision(state State, repo, tool string, args any) (bool, string) {
	a, _ := args.(map[string]any)
	path, _ := a["path"].(string)
	switch tool {
	case "view", "grep", "glob":
		if path == "" || settings.Audit && !strings.HasPrefix(path, "~") || insideRoots(repo, path, true) {
			return true, ""
		}
		return false, "path " + path + " is outside the repository and the configured directories"
	case "edit", "create":
		if !steps[state].Edit {
			return false, "the " + state.String() + " step may not change files"
		}
		if path != "" && insideRoots(repo, path, false) {
			return true, ""
		}
		return false, "path " + path + " is outside the repository and the configured directories"
	case "bash":
		command, _ := a["command"].(string)
		return shellDecision(state, command)
	}
	return false, "tool " + tool + " is not available to agent-sm agents"
}

// insideRoots reports whether path (relative paths are taken from repo) lies in repo or a
// configured add_dirs entry; reads may also use the temp dir, where large tool output is saved.
func insideRoots(repo, path string, read bool) bool {
	if strings.HasPrefix(path, "~") {
		return false
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(repo, path)
	}
	path = resolvePath(path)
	roots := append([]string{repo}, settings.AddDirs...)
	if read {
		roots = append(roots, os.TempDir())
	}
	for _, r := range roots {
		if r == "" || !filepath.IsAbs(r) {
			continue
		}
		r = resolvePath(r)
		if path == r || strings.HasPrefix(path, r+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

// resolvePath follows symlinks in the longest existing prefix of an absolute path.
func resolvePath(p string) string {
	p, rest := filepath.Clean(p), ""
	for {
		if r, err := filepath.EvalSymlinks(p); err == nil {
			return filepath.Join(r, rest)
		}
		parent := filepath.Dir(p)
		if parent == p {
			return filepath.Join(p, rest)
		}
		rest, p = filepath.Join(filepath.Base(p), rest), parent
	}
}

// shellMask returns a shell line with quoted and escaped text replaced (so it cannot hide
// separators) and heredoc bodies removed. ok is false when the line cannot be read safely.
func shellMask(s string) (string, bool) {
	var b strings.Builder
	var pending []string // heredoc delimiters whose bodies start at the next newline
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '\\':
			if i+1 >= len(s) {
				return "", false
			}
			if s[i+1] != '\n' { // a backslash-newline only continues the line
				b.WriteByte('_')
			}
			i++
		case c == '\'':
			j := strings.IndexByte(s[i+1:], '\'')
			if j < 0 {
				return "", false
			}
			b.WriteString(`""`)
			i += j + 1
		case c == '"':
			j := i + 1
			for ; j < len(s) && s[j] != '"'; j++ {
				if s[j] == '\\' {
					j++
				}
			}
			if j >= len(s) {
				return "", false
			}
			b.WriteString(`""`)
			i = j
		case strings.HasPrefix(s[i:], "<<<"):
			b.WriteString("<<<")
			i += 2
		case strings.HasPrefix(s[i:], "<<"):
			m := heredocRe.FindStringSubmatchIndex(s[i:])
			if m == nil || m[0] != 0 {
				return "", false
			}
			pending = append(pending, s[i+m[2]:i+m[3]])
			b.WriteByte(' ')
			i += m[1] - 1
		case c == '\n' && len(pending) > 0:
			b.WriteByte('\n')
			rest := s[i+1:]
			for _, word := range pending {
				found := false
				for !found && rest != "" {
					line, after, _ := strings.Cut(rest, "\n")
					found, rest = strings.TrimSpace(line) == word, after
				}
				if !found {
					return "", false
				}
			}
			pending = nil
			i = len(s) - len(rest) - 1
		default:
			b.WriteByte(c)
		}
	}
	return b.String(), len(pending) == 0
}

// shellDecision checks every command of a shell line. Normal runs need each one to match a
// configured Bash(prefix:*) entry; audit runs allow any command except the outward ones.
func shellDecision(state State, command string) (bool, string) {
	if strings.TrimSpace(command) == "" {
		return false, "empty command"
	}
	if shellSubstRe.MatchString(command) {
		return false, "command and process substitution are not allowed; run the commands separately"
	}
	masked, ok := shellMask(command)
	if !ok {
		return false, "unterminated quote or heredoc"
	}
	masked = safeRedirRe.ReplaceAllString(masked, " ")
	if strings.Contains(masked, ">") {
		return false, "output redirection is not allowed; use the edit or create tools to write files"
	}
	if strings.Contains(strings.ReplaceAll(masked, "&&", ""), "&") {
		return false, "background jobs are not allowed"
	}
	var prefixes []string
	for _, entry := range strings.Split(configuredAllow(state), ",") {
		if p, ok := strings.CutPrefix(strings.TrimSpace(entry), "Bash("); ok {
			prefixes = append(prefixes, strings.TrimSuffix(strings.TrimSuffix(p, ")"), ":*"))
		}
	}
	for _, seg := range sepRe.Split(masked, -1) {
		f := strings.Fields(seg)
		for len(f) > 0 && (f[0] == "do" || f[0] == "then" || f[0] == "else" || f[0] == "if" || f[0] == "elif" ||
			f[0] == "while" || f[0] == "until" || f[0] == "!") {
			f = f[1:]
		}
		if len(f) == 0 || f[0] == "done" || f[0] == "fi" || f[0] == "for" {
			continue // a for word list runs nothing: substitutions were rejected above
		}
		line := strings.Join(f, " ")
		if ok, why := shellCommandDecision(line, prefixes); !ok {
			return false, why
		}
	}
	return true, ""
}

func shellCommandDecision(line string, prefixes []string) (bool, string) {
	f := strings.Fields(line)
	switch {
	case strings.ContainsAny(f[0], "(){}"):
		return false, "subshells and command groups are not allowed: " + line
	case strings.Contains(f[0], "="):
		return false, "variable assignments are not allowed: " + line
	case f[0] == "case":
		return false, "case statements are not allowed"
	}
	inner := strings.TrimPrefix(strings.TrimPrefix(line, "rtk "), "proxy ")
	for _, o := range outward {
		if inner == o || strings.HasPrefix(inner, o+" ") {
			return false, "agent-sm never lets an agent run " + o
		}
	}
	words := strings.Fields(inner)
	if words[0] == "rm" {
		for _, w := range words[1:] {
			if w == "--recursive" || strings.HasPrefix(w, "-") && !strings.HasPrefix(w, "--") && strings.ContainsAny(w, "rR") {
				return false, "agent-sm never lets an agent delete directory trees"
			}
		}
	}
	if settings.Management.Mode == "service" && (words[0] == "gh" || words[0] == "curl" || words[0] == "wget") {
		return false, "service management runs have no network or GitHub command access"
	}
	if settings.Audit {
		if shellWrappers[words[0]] {
			return false, words[0] + " runs another command line, which agent-sm cannot check"
		}
		return true, ""
	}
	for _, p := range prefixes {
		if line == p || strings.HasPrefix(line, p+" ") {
			return true, ""
		}
	}
	return false, "command not in this step's allow list: " + line
}
