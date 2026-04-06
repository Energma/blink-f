package app

import (
	"os/exec"

	"github.com/Energma/blink-f/internal/app/components"
	"github.com/Energma/blink-f/internal/models"
)

// --- Git operation results ---

type worktreesLoadedMsg struct {
	worktrees []models.Worktree
	err       error
}

type worktreeCreatedMsg struct {
	worktree *models.Worktree
	err      error
}

type worktreeDeletedMsg struct {
	name string
	err  error
}

type gitStatusMsg struct {
	index  int
	status models.WorktreeStatus
	err    error
}

type commitResultMsg struct {
	output string
	err    error
}

type pushResultMsg struct {
	err error
}

type pullResultMsg struct {
	err error
}

type stashResultMsg struct {
	err error
}

type commitFilesLoadedMsg struct {
	files []string
	err   error
}

// --- tmux operations ---

type tmuxAvailableMsg struct {
	available bool
	sessions  []string
}

type tmuxSessionCreatedMsg struct {
	sessionName string
	err         error
}

type tmuxSessionKilledMsg struct {
	name string
	err  error
}

// --- Agent operations ---

type agentLaunchedMsg struct {
	provider  string
	worktree  string
	sessionID string
	directCmd *exec.Cmd
	err       error
}

// agentSessionReturnedMsg is sent when the user returns from an agent tmux session.
type agentSessionReturnedMsg struct{}

// tmuxSessionReturnedMsg is sent when the user returns from an attached tmux session.
type tmuxSessionReturnedMsg struct{}

// --- Detail loading ---

type detailLoadedMsg struct {
	files    []string
	diffStat string
}

// --- General ---

type statusMsg string

type errMsg struct {
	err error
}

func (e errMsg) Error() string {
	if e.err != nil {
		return e.err.Error()
	}
	return ""
}

type tickMsg struct{}

type clearStatusMsg struct{}

// --- Branch operations ---

type branchesLoadedMsg struct {
	branches []string
	err      error
}

type branchCheckedOutMsg struct {
	branch string
	err    error
}

// --- File tree ---

type dirListedMsg struct {
	parentPath string
	entries    []components.TreeNode
	err        error
}
