package app

import (
	"context"

	tea "charm.land/bubbletea/v2"

	"github.com/Energma/blink-f/internal/app/screen"
	"github.com/Energma/blink-f/internal/git"
	"github.com/Energma/blink-f/internal/tmux"
)

func (m *Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Modal-specific handling first
	switch m.screenMgr.Active() {
	case screen.Help:
		return m.handleHelpKey(key)
	case screen.WorktreeCreate:
		return m.handleCreateKey(msg)
	case screen.Commit:
		return m.handleCommitKey(msg)
	case screen.AgentSelect:
		return m.handleAgentKey(key)
	case screen.Confirm:
		return m.handleConfirmKey(key)
	case screen.RepoSelect:
		return m.handleRepoKey(key)
	case screen.WorktreeDetail:
		return m.handleDetailKey(key)
	case screen.Sessions:
		return m.handleSessionsKey(key)
	case screen.Game:
		return m.handleGameKey(key)
	}

	// Filter mode
	if m.filterMode {
		return m.handleFilterKey(msg)
	}

	// Dashboard keys
	switch key {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "j", "down":
		if m.cursor < len(m.filteredIndices)-1 {
			m.cursor++
		}
	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
		}
	case "g":
		m.cursor = 0
	case "G":
		m.cursor = max(0, len(m.filteredIndices)-1)

	case "enter":
		if wt := m.selectedWorktree(); wt != nil {
			if m.tmuxAvailable {
				return m, m.switchWorktreeCmd(*wt)
			}
		}

	case "l", "right":
		if wt := m.selectedWorktree(); wt != nil {
			m.screenMgr.Push(screen.WorktreeDetail)
			return m, m.loadDetailCmd(*wt)
		}

	case "n":
		m.createBranch = ""
		m.createBase = ""
		m.screenMgr.Push(screen.WorktreeCreate)

	case "d":
		if wt := m.selectedWorktree(); wt != nil && !wt.IsMain {
			m.confirmTitle = "Delete Worktree"
			m.confirmMessage = "Delete worktree '" + wt.Branch + "'?"
			capturedWt := *wt
			m.confirmAction = func() tea.Cmd {
				return m.deleteWorktreeCmd(capturedWt, false)
			}
			m.screenMgr.Push(screen.Confirm)
		}

	case "D":
		// Clean merged worktrees
		m.confirmTitle = "Clean Merged"
		m.confirmMessage = "Remove all worktrees whose branches are merged?"
		m.confirmAction = func() tea.Cmd {
			return m.cleanMergedCmd()
		}
		m.screenMgr.Push(screen.Confirm)

	case "a":
		m.agentProviders, m.agentAvailable = m.sortedAgentProviders()
		m.agentCursor = 0
		m.screenMgr.Push(screen.AgentSelect)

	case "Q":
		// Kill agent session for selected worktree
		if wt := m.selectedWorktree(); wt != nil {
			if sid, ok := m.agentSessions[wt.Branch]; ok {
				return m, m.killSessionCmd(sid)
			}
		}

	case "c":
		m.commitType = 0
		m.commitScope = ""
		m.commitMsg = ""
		m.commitField = 0
		m.commitFiles = nil
		if !m.cfg.Git.ConventionalCommits {
			m.commitField = 0
		}
		m.screenMgr.Push(screen.Commit)
		return m, m.loadCommitFilesCmd()

	case "p":
		if wt := m.selectedWorktree(); wt != nil {
			m.statusText = "Pushing..."
			return m, m.pushCmd()
		}

	case "u":
		if wt := m.selectedWorktree(); wt != nil {
			m.statusText = "Pulling..."
			return m, m.pullCmd()
		}

	case "s":
		if wt := m.selectedWorktree(); wt != nil {
			return m, m.stashCmd()
		}

	case "e":
		// Open editor in worktree directory
		if wt := m.selectedWorktree(); wt != nil {
			return m, m.launchEditorCmd(wt.Path)
		}

	case "S":
		// Open sessions management panel
		m.buildSessionInfos()
		m.sessionCursor = 0
		m.screenMgr.Push(screen.Sessions)

	case "B":
		// Easter egg: Blink Run mini-game
		m.game = newGameState()
		m.screenMgr.Push(screen.Game)

	case "/":
		m.filterMode = true
		m.filterText = ""

	case "tab":
		if len(m.repos) > 1 {
			m.repoCursor = m.activeRepo
			m.screenMgr.Push(screen.RepoSelect)
		}

	case "r":
		m.statusText = "Refreshing..."
		return m, tea.Batch(m.loadWorktreesCmd(), m.checkTmuxCmd(), m.clearStatusCmd())

	case "?":
		m.screenMgr.Push(screen.Help)
	}

	return m, nil
}

// --- Modal handlers ---

func (m *Model) handleHelpKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc", "?", "q":
		m.screenMgr.Pop()
	}
	return m, nil
}

func (m *Model) handleCreateKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	switch key {
	case "esc":
		m.screenMgr.Pop()
	case "enter":
		if m.createBranch != "" {
			return m, m.createWorktreeCmd(m.createBranch, m.createBase)
		}
	case "backspace":
		if len(m.createBranch) > 0 {
			m.createBranch = m.createBranch[:len(m.createBranch)-1]
		}
	default:
		if len(key) == 1 && key[0] >= ' ' {
			m.createBranch += key
		}
	}
	return m, nil
}

func (m *Model) handleCommitKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	maxField := 2
	if !m.cfg.Git.ConventionalCommits {
		maxField = 0
	}

	switch key {
	case "esc":
		m.screenMgr.Pop()
	case "tab":
		m.commitField = (m.commitField + 1) % (maxField + 1)
	case "enter":
		if m.commitMsg != "" {
			var message string
			if m.cfg.Git.ConventionalCommits {
				types := git.ConventionalTypes()
				if m.commitType >= 0 && m.commitType < len(types) {
					message = git.FormatConventionalCommit(types[m.commitType], m.commitScope, m.commitMsg)
				} else {
					message = m.commitMsg
				}
			} else {
				message = m.commitMsg
			}
			return m, m.commitCmd(message)
		}
	case "left":
		if m.commitField == 0 && m.cfg.Git.ConventionalCommits {
			if m.commitType > 0 {
				m.commitType--
			}
		}
	case "right":
		if m.commitField == 0 && m.cfg.Git.ConventionalCommits {
			types := git.ConventionalTypes()
			if m.commitType < len(types)-1 {
				m.commitType++
			}
		}
	case "backspace":
		switch m.commitField {
		case 1:
			if len(m.commitScope) > 0 {
				m.commitScope = m.commitScope[:len(m.commitScope)-1]
			}
		case 2:
			if len(m.commitMsg) > 0 {
				m.commitMsg = m.commitMsg[:len(m.commitMsg)-1]
			}
		case 0:
			if !m.cfg.Git.ConventionalCommits && len(m.commitMsg) > 0 {
				m.commitMsg = m.commitMsg[:len(m.commitMsg)-1]
			}
		}
	default:
		if len(key) == 1 && key[0] >= ' ' {
			switch m.commitField {
			case 1:
				m.commitScope += key
			case 2:
				m.commitMsg += key
			case 0:
				if !m.cfg.Git.ConventionalCommits {
					m.commitMsg += key
				}
			}
		}
	}
	return m, nil
}

func (m *Model) handleAgentKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		m.screenMgr.Pop()
	case "j", "down":
		if m.agentCursor < len(m.agentProviders)-1 {
			m.agentCursor++
		}
	case "k", "up":
		if m.agentCursor > 0 {
			m.agentCursor--
		}
	case "enter":
		if m.agentCursor >= 0 && m.agentCursor < len(m.agentProviders) {
			return m, m.launchAgentCmd(m.agentProviders[m.agentCursor])
		}
	}
	return m, nil
}

func (m *Model) handleConfirmKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "y":
		if m.confirmAction != nil {
			cmd := m.confirmAction()
			m.confirmAction = nil
			return m, cmd
		}
		m.screenMgr.Pop()
	case "n", "esc":
		m.confirmAction = nil
		m.screenMgr.Pop()
	}
	return m, nil
}

func (m *Model) handleRepoKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		m.screenMgr.Pop()
	case "j", "down":
		if m.repoCursor < len(m.repos)-1 {
			m.repoCursor++
		}
	case "k", "up":
		if m.repoCursor > 0 {
			m.repoCursor--
		}
	case "enter":
		m.activeRepo = m.repoCursor
		m.cursor = 0
		m.screenMgr.Pop()
		return m, m.loadWorktreesCmd()
	}
	return m, nil
}

func (m *Model) handleDetailKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc", "h", "left":
		m.screenMgr.Pop()
	case "e":
		// Open editor from detail view
		if wt := m.selectedWorktree(); wt != nil {
			m.screenMgr.Pop()
			return m, m.launchEditorCmd(wt.Path)
		}
	case "a":
		// Launch agent from detail view
		if m.selectedWorktree() != nil {
			m.screenMgr.Pop()
			m.agentProviders, m.agentAvailable = m.sortedAgentProviders()
			m.agentCursor = 0
			m.screenMgr.Push(screen.AgentSelect)
		}
	case "c":
		// Commit from detail view
		if m.selectedWorktree() != nil {
			m.screenMgr.Pop()
			m.commitType = 0
			m.commitScope = ""
			m.commitMsg = ""
			m.commitField = 0
			m.commitFiles = nil
			if !m.cfg.Git.ConventionalCommits {
				m.commitField = 0
			}
			m.screenMgr.Push(screen.Commit)
			return m, m.loadCommitFilesCmd()
		}
	case "p":
		// Push from detail view
		if wt := m.selectedWorktree(); wt != nil {
			m.screenMgr.Pop()
			m.statusText = "Pushing..."
			return m, m.pushCmd()
		}
	}
	return m, nil
}

func (m *Model) handleSessionsKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		m.screenMgr.Pop()

	case "j", "down":
		if m.sessionCursor < len(m.sessionInfos)-1 {
			m.sessionCursor++
		}

	case "k", "up":
		if m.sessionCursor > 0 {
			m.sessionCursor--
		}

	case "enter":
		// Attach to session (switch tmux)
		if m.sessionCursor >= 0 && m.sessionCursor < len(m.sessionInfos) {
			name := m.sessionInfos[m.sessionCursor].Name
			m.screenMgr.Pop()
			return m, m.attachSessionCmd(name)
		}

	case "x":
		// Kill selected session
		if m.sessionCursor >= 0 && m.sessionCursor < len(m.sessionInfos) {
			name := m.sessionInfos[m.sessionCursor].Name
			return m, m.killSessionCmd(name)
		}

	case "X":
		// Kill all orphaned sessions (no linked worktree)
		var cmds []tea.Cmd
		for _, s := range m.sessionInfos {
			if s.Worktree == nil {
				cmds = append(cmds, m.killSessionCmd(s.Name))
			}
		}
		if len(cmds) > 0 {
			return m, tea.Batch(cmds...)
		}
		m.statusText = "No orphaned sessions"
		return m, m.clearStatusCmd()
	}

	return m, nil
}

func (m *Model) handleFilterKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	switch key {
	case "esc":
		m.filterMode = false
		m.filterText = ""
		m.updateFilteredIndices()
	case "enter":
		m.filterMode = false
	case "backspace":
		if len(m.filterText) > 0 {
			m.filterText = m.filterText[:len(m.filterText)-1]
			m.updateFilteredIndices()
		}
	default:
		if len(key) == 1 && key[0] >= ' ' {
			m.filterText += key
			m.updateFilteredIndices()
		}
	}
	return m, nil
}

// attachSessionCmd switches to an existing tmux session by name.
func (m *Model) attachSessionCmd(name string) tea.Cmd {
	return func() tea.Msg {
		if !tmux.InsideTmux() {
			return errMsg{err: nil}
		}
		ctx := context.Background()
		err := m.tmux.SwitchSession(ctx, name)
		return tmuxSessionCreatedMsg{sessionName: name, err: err}
	}
}

func (m *Model) handleGameKey(key string) (tea.Model, tea.Cmd) {
	if key == "esc" {
		m.screenMgr.Pop()
		return m, nil
	}
	// Delegate all other keys to the game itself.
	cmd := m.game.HandleKey(key)
	return m, cmd
}
