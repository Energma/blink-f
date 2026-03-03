package models

import "time"

type Worktree struct {
	Path     string         `json:"path"`
	Branch   string         `json:"branch"`
	HEAD     string         `json:"head"`
	RepoDir  string         `json:"repoDir"`
	IsMain   bool           `json:"isMain"`
	Bare     bool           `json:"bare"`
	Detached bool           `json:"detached"`
	Status   WorktreeStatus `json:"status"`
	Last     *CommitInfo    `json:"lastCommit,omitempty"`
}

type WorktreeStatus struct {
	Clean        bool   `json:"clean"`
	Staged       int    `json:"staged"`
	Modified     int    `json:"modified"`
	Untracked    int    `json:"untracked"`
	Ahead        int    `json:"ahead"`
	Behind       int    `json:"behind"`
	Conflicts    int    `json:"conflicts"`
	TmuxSession  string `json:"tmuxSession,omitempty"`
	AgentRunning bool   `json:"agentRunning"`
}

type CommitInfo struct {
	Hash    string    `json:"hash"`
	Subject string    `json:"subject"`
	Author  string    `json:"author"`
	Date    time.Time `json:"date"`
}

// DisplayName returns a short name for the worktree.
func (w *Worktree) DisplayName() string {
	if w.Branch != "" {
		return w.Branch
	}
	if w.Detached {
		if len(w.HEAD) >= 8 {
			return w.HEAD[:8]
		}
		return w.HEAD
	}
	return w.Path
}

// StatusLine returns a compact status string like "+3 ~2 ?1 ↑2 ↓1".
func (s *WorktreeStatus) StatusLine() string {
	if s.Clean {
		return "clean"
	}
	var parts []string
	if s.Staged > 0 {
		parts = append(parts, "+"+itoa(s.Staged))
	}
	if s.Modified > 0 {
		parts = append(parts, "~"+itoa(s.Modified))
	}
	if s.Untracked > 0 {
		parts = append(parts, "?"+itoa(s.Untracked))
	}
	if s.Conflicts > 0 {
		parts = append(parts, "!"+itoa(s.Conflicts))
	}
	if s.Ahead > 0 {
		parts = append(parts, "↑"+itoa(s.Ahead))
	}
	if s.Behind > 0 {
		parts = append(parts, "↓"+itoa(s.Behind))
	}
	if len(parts) == 0 {
		return "clean"
	}
	result := parts[0]
	for _, p := range parts[1:] {
		result += " " + p
	}
	return result
}

func itoa(n int) string {
	if n < 10 {
		return string(rune('0' + n))
	}
	return intToStr(n)
}

func intToStr(n int) string {
	if n == 0 {
		return "0"
	}
	digits := make([]byte, 0, 4)
	for n > 0 {
		digits = append(digits, byte('0'+n%10))
		n /= 10
	}
	for i, j := 0, len(digits)-1; i < j; i, j = i+1, j-1 {
		digits[i], digits[j] = digits[j], digits[i]
	}
	return string(digits)
}
