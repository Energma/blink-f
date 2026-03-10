package app

import (
	"context"
	"path/filepath"

	tea "charm.land/bubbletea/v2"

	"github.com/Energma/blink-f/internal/app/components"
	"github.com/Energma/blink-f/internal/app/screen"
	"github.com/Energma/blink-f/internal/git"
	"github.com/Energma/blink-f/internal/models"
)

func (m *Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	// Block all input while a tmux switch/attach is in progress.
	if m.switching {
		return m, nil
	}

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
	}

	// Filter mode
	if m.filterMode {
		return m.handleFilterKey(msg)
	}

	// Global keys (work from any pane)
	switch key {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "?":
		m.screenMgr.Push(screen.Help)
		return m, nil
	case "r":
		m.statusText = "Refreshing..."
		return m, tea.Batch(m.loadWorktreesCmd(), m.checkTmuxCmd(), m.clearStatusCmd())
	case "ctrl+up":
		m.activePane = PaneWorktrees
		return m, nil
	case "ctrl+down":
		m.activePane = PaneFileTree
		return m, nil
	}

	// Tree filter mode
	if m.treeFilterMode {
		return m.handleTreeFilterKey(msg)
	}

	// Pane-specific keys
	if m.activePane == PaneFileTree {
		return m.handleTreeKey(key)
	}

	// Dashboard keys (upper pane)
	switch key {
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
				m.switching = true
				m.statusText = "Switching..."
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

	case "/":
		m.filterMode = true
		m.filterText = ""

	case "tab":
		if len(m.repos) > 1 {
			m.repoCursor = m.activeRepo
			m.screenMgr.Push(screen.RepoSelect)
		}

	}

	return m, nil
}

// --- File tree handler ---

func (m *Model) handleTreeKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "j", "down":
		if m.treeCursor < len(m.treeFlatNodes)-1 {
			m.treeCursor++
		}
		return m, m.treeHoverPreview()
	case "k", "up":
		if m.treeCursor > 0 {
			m.treeCursor--
		}
		return m, m.treeHoverPreview()
	case "g":
		m.treeCursor = 0
		return m, m.treeHoverPreview()
	case "G":
		m.treeCursor = max(0, len(m.treeFlatNodes)-1)
		return m, m.treeHoverPreview()

	case "enter", "l", "right":
		if m.treeCursor >= 0 && m.treeCursor < len(m.treeFlatNodes) {
			node := m.treeFlatNodes[m.treeCursor]
			if node.IsGitRepo {
				// Select this repo and switch focus to worktree pane
				return m.selectRepoFromTree(node.Path, node.Name)
			}
			// Expand/collapse directory
			if !node.Loaded {
				return m, listDirCmd(node.Path, node.Depth+1)
			}
			node.Expanded = !node.Expanded
			m.treeFlatNodes = components.FlattenVisible(m.treeRoot)
		}

	case "h", "left":
		if m.treeCursor >= 0 && m.treeCursor < len(m.treeFlatNodes) {
			node := m.treeFlatNodes[m.treeCursor]
			if node.Expanded {
				// Collapse current node
				node.Expanded = false
				m.treeFlatNodes = components.FlattenVisible(m.treeRoot)
			} else {
				// Move cursor to parent
				parent := components.FindParent(m.treeRoot, node.Path)
				if parent != nil {
					for i, n := range m.treeFlatNodes {
						if n.Path == parent.Path {
							m.treeCursor = i
							break
						}
					}
				}
			}
		}

	case "backspace":
		// Re-root tree to parent directory — navigate up the filesystem
		return m.treeNavigateUp()

	case "f":
		// Enter tree filter mode
		m.treeFilterMode = true
		m.treeFilterText = ""

	case "/":
		// Switch to upper pane and activate worktree filter
		m.activePane = PaneWorktrees
		m.filterMode = true
		m.filterText = ""
	}

	return m, nil
}

// handleTreeFilterKey handles input while tree filter is active.
func (m *Model) handleTreeFilterKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	switch key {
	case "esc":
		m.treeFilterMode = false
		m.treeFilterText = ""
	case "enter":
		// Jump to first match and exit filter
		if m.treeFilterText != "" {
			matches := components.FilterFlatNodes(m.treeFlatNodes, m.treeFilterText)
			if len(matches) > 0 {
				m.treeCursor = matches[0]
			}
		}
		m.treeFilterMode = false
		return m, m.treeHoverPreview()
	case "tab":
		// Jump to next match
		if m.treeFilterText != "" {
			matches := components.FilterFlatNodes(m.treeFlatNodes, m.treeFilterText)
			if len(matches) > 0 {
				// Find next match after current cursor
				next := matches[0]
				for _, idx := range matches {
					if idx > m.treeCursor {
						next = idx
						break
					}
				}
				m.treeCursor = next
			}
		}
		return m, m.treeHoverPreview()
	case "backspace":
		if len(m.treeFilterText) > 0 {
			m.treeFilterText = m.treeFilterText[:len(m.treeFilterText)-1]
			m.treeFilterJumpToFirst()
		}
	default:
		if len(key) == 1 && key[0] >= ' ' {
			m.treeFilterText += key
			m.treeFilterJumpToFirst()
		}
	}
	return m, nil
}

// treeFilterJumpToFirst moves the cursor to the first matching node during live search.
func (m *Model) treeFilterJumpToFirst() {
	if m.treeFilterText == "" {
		return
	}
	matches := components.FilterFlatNodes(m.treeFlatNodes, m.treeFilterText)
	if len(matches) > 0 {
		m.treeCursor = matches[0]
	}
}

// treeNavigateUp re-roots the tree to the parent directory.
func (m *Model) treeNavigateUp() (tea.Model, tea.Cmd) {
	parentPath := filepath.Dir(m.treeRoot.Path)
	if parentPath == m.treeRoot.Path {
		// Already at filesystem root
		return m, nil
	}
	m.treeRoot = components.NewTreeRoot(parentPath)
	m.treeCursor = 0
	m.treeFlatNodes = components.FlattenVisible(m.treeRoot)
	return m, listDirCmd(parentPath, 1)
}

// treeHoverPreview loads worktrees when cursor lands on a git repo (without switching pane).
func (m *Model) treeHoverPreview() tea.Cmd {
	if m.treeCursor < 0 || m.treeCursor >= len(m.treeFlatNodes) {
		return nil
	}
	node := m.treeFlatNodes[m.treeCursor]
	if !node.IsGitRepo {
		return nil
	}

	// Check if already the active repo
	for _, r := range m.repos {
		if r.Path == node.Path {
			// Already loaded, just switch activeRepo
			for i, rr := range m.repos {
				if rr.Path == node.Path {
					m.activeRepo = i
					m.cursor = 0
					return m.loadWorktreesCmd()
				}
			}
		}
	}

	// Add and preview
	ctx := context.Background()
	m.repos = append(m.repos, models.Repo{
		Path:          node.Path,
		Name:          node.Name,
		DefaultBranch: m.git.DefaultBranch(ctx, node.Path),
	})
	m.activeRepo = len(m.repos) - 1
	m.cursor = 0
	return m.loadWorktreesCmd()
}

// selectRepoFromTree adds a repo from the file tree and loads its worktrees.
func (m *Model) selectRepoFromTree(path, name string) (tea.Model, tea.Cmd) {
	// Check if repo already in list
	for i, r := range m.repos {
		if r.Path == path {
			m.activeRepo = i
			m.cursor = 0
			m.activePane = PaneWorktrees
			return m, m.loadWorktreesCmd()
		}
	}

	// Add new repo
	ctx := context.Background()
	m.repos = append(m.repos, models.Repo{
		Path:          path,
		Name:          name,
		DefaultBranch: m.git.DefaultBranch(ctx, path),
	})
	m.activeRepo = len(m.repos) - 1
	m.cursor = 0
	m.activePane = PaneWorktrees
	m.statusText = "Switched to " + name
	return m, tea.Batch(m.loadWorktreesCmd(), m.clearStatusCmd())
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
			m.switching = true
			m.statusText = "Launching agent..."
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
			m.switching = true
			m.statusText = "Switching..."
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
// The actual switch/attach is handled by the tmuxSessionCreatedMsg handler
// in Update, which uses tea.ExecProcess when outside tmux.
func (m *Model) attachSessionCmd(name string) tea.Cmd {
	return func() tea.Msg {
		return tmuxSessionCreatedMsg{sessionName: name, err: nil}
	}
}
