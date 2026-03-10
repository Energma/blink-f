package app

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/Energma/blink-f/internal/agent"
	"github.com/Energma/blink-f/internal/app/components"
	"github.com/Energma/blink-f/internal/app/screen"
	"github.com/Energma/blink-f/internal/app/views"
	"github.com/Energma/blink-f/internal/config"
	"github.com/Energma/blink-f/internal/git"
	"github.com/Energma/blink-f/internal/models"
	"github.com/Energma/blink-f/internal/theme"
	"github.com/Energma/blink-f/internal/tmux"
)

// PaneID identifies which pane has focus.
type PaneID int

const (
	PaneWorktrees PaneID = iota
	PaneFileTree
)

// Model is the root BubbleTea model.
type Model struct {
	// Services
	git    *git.Service
	tmux   *tmux.Service
	agents *agent.Registry
	cfg    *config.Config
	theme  *theme.Theme
	styles *Styles

	// Data
	repos     []models.Repo
	worktrees []models.Worktree

	// Selection state
	activeRepo      int
	cursor          int
	filteredIndices []int

	// Filter
	filterText string
	filterMode bool

	// Modal states
	screenMgr *screen.Manager

	// Create form
	createBranch string
	createBase   string

	// Commit form
	commitType    int
	commitScope   string
	commitMsg     string
	commitField   int // 0=type, 1=scope, 2=msg
	commitFiles   []string

	// Agent select
	agentCursor    int
	agentProviders []string
	agentAvailable []string

	// Repo select
	repoCursor int

	// Session manager
	sessionInfos  []views.SessionInfo
	sessionCursor int

	// Confirm dialog
	confirmTitle   string
	confirmMessage string
	confirmAction  func() tea.Cmd

	// Detail view
	detailFiles    []string
	detailDiffStat string

	// UI state
	width         int
	height        int
	ready         bool
	switching     bool // true while waiting for tmux switch/attach
	tmuxAvailable bool
	tmuxSessions  []string

	// Agent tracking: map worktree branch -> agent session name
	agentSessions map[string]string

	// Dual pane state
	activePane      PaneID
	treeRoot        *components.TreeNode
	treeFlatNodes   []*components.TreeNode
	treeCursor      int
	treeFilterMode  bool
	treeFilterText  string

	// Status
	statusText string
	errText    string
}

// New creates a new TUI Model.
func New(cfg *config.Config) *Model {
	t := theme.Get(cfg.UI.Theme)
	s := NewStyles(t)

	repos := make([]models.Repo, 0, len(cfg.Repos)+1)
	gitSvc := git.NewService()

	// Auto-detect current directory as a repo
	cwd, _ := os.Getwd()
	if gitSvc.IsGitRepo(context.Background(), cwd) {
		root, _ := gitSvc.RepoRoot(context.Background(), cwd)
		if root != "" {
			repos = append(repos, models.Repo{
				Path:          root,
				DefaultBranch: gitSvc.DefaultBranch(context.Background(), root),
				Name:          filepath.Base(root),
			})
		}
	}

	// Add configured repos (skip duplicates)
	seen := make(map[string]bool)
	for _, r := range repos {
		seen[r.Path] = true
	}
	for _, rc := range cfg.Repos {
		if !seen[rc.Path] {
			name := rc.Name
			if name == "" {
				name = filepath.Base(rc.Path)
			}
			repos = append(repos, models.Repo{
				Path:          rc.Path,
				DefaultBranch: rc.DefaultBranch,
				Name:          name,
			})
			seen[rc.Path] = true
		}
	}

	// File tree root: parent of cwd, or home dir
	treeRootPath := filepath.Dir(cwd)
	if treeRootPath == "" {
		home, _ := os.UserHomeDir()
		treeRootPath = home
	}
	treeRoot := components.NewTreeRoot(treeRootPath)

	return &Model{
		git:           gitSvc,
		tmux:          tmux.NewService(cfg),
		agents:        agent.NewRegistry(cfg),
		cfg:           cfg,
		theme:         t,
		styles:        s,
		repos:         repos,
		screenMgr:     screen.NewManager(),
		agentSessions: make(map[string]string),
		treeRoot:      treeRoot,
	}
}

func (m *Model) Init() tea.Cmd {
	cmds := []tea.Cmd{
		m.checkTmuxCmd(),
		listDirCmd(m.treeRoot.Path, 1),
	}
	if len(m.repos) > 0 {
		cmds = append(cmds, m.loadWorktreesCmd())
	}
	return tea.Batch(cmds...)
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		return m, nil

	case worktreesLoadedMsg:
		if msg.err != nil {
			m.errText = msg.err.Error()
			return m, m.clearStatusCmd()
		}
		m.worktrees = msg.worktrees
		m.updateFilteredIndices()
		m.enrichTmuxStatus()
		m.enrichAgentStatus()
		return m, m.loadAllStatusesCmd()

	case gitStatusMsg:
		if msg.err == nil && msg.index >= 0 && msg.index < len(m.worktrees) {
			msg.status.TmuxSession = m.worktrees[msg.index].Status.TmuxSession
			msg.status.AgentRunning = m.worktrees[msg.index].Status.AgentRunning
			m.worktrees[msg.index].Status = msg.status
		}
		return m, nil

	case worktreeCreatedMsg:
		if msg.err != nil {
			m.errText = msg.err.Error()
			return m, m.clearStatusCmd()
		}
		m.statusText = "Created worktree: " + msg.worktree.Branch
		m.screenMgr.Pop()
		m.createBranch = ""
		m.createBase = ""
		return m, tea.Batch(m.loadWorktreesCmd(), m.checkTmuxCmd(), m.clearStatusCmd())

	case worktreeDeletedMsg:
		if msg.err != nil {
			m.errText = msg.err.Error()
			return m, m.clearStatusCmd()
		}
		m.statusText = "Deleted worktree: " + msg.name
		m.screenMgr.Pop()
		if m.cursor >= len(m.filteredIndices)-1 {
			m.cursor = max(0, len(m.filteredIndices)-2)
		}
		return m, tea.Batch(m.loadWorktreesCmd(), m.checkTmuxCmd(), m.clearStatusCmd())

	case commitResultMsg:
		m.screenMgr.Pop()
		m.commitMsg = ""
		m.commitScope = ""
		m.commitType = 0
		m.commitField = 0
		m.commitFiles = nil
		if msg.err != nil {
			m.errText = msg.err.Error()
		} else {
			m.statusText = "Committed successfully"
		}
		return m, tea.Batch(m.loadWorktreesCmd(), m.clearStatusCmd())

	case commitFilesLoadedMsg:
		m.commitFiles = msg.files
		return m, nil

	case pushResultMsg:
		if msg.err != nil {
			m.errText = msg.err.Error()
		} else {
			m.statusText = "Pushed successfully"
		}
		return m, tea.Batch(m.loadWorktreesCmd(), m.clearStatusCmd())

	case pullResultMsg:
		if msg.err != nil {
			m.errText = msg.err.Error()
		} else {
			m.statusText = "Pulled successfully"
		}
		return m, tea.Batch(m.loadWorktreesCmd(), m.clearStatusCmd())

	case stashResultMsg:
		if msg.err != nil {
			m.errText = msg.err.Error()
		} else {
			m.statusText = "Stashed changes"
		}
		return m, tea.Batch(m.loadWorktreesCmd(), m.clearStatusCmd())

	case agentLaunchedMsg:
		m.switching = false
		if msg.err != nil {
			m.errText = msg.err.Error()
			m.screenMgr.Pop()
			return m, tea.Batch(m.checkTmuxCmd(), m.clearStatusCmd())
		}
		if msg.provider != "" {
			m.statusText = "Launched " + msg.provider + " in " + msg.worktree
			if msg.worktree != "" && msg.sessionID != "" {
				m.agentSessions[msg.worktree] = msg.sessionID
			}
		}
		m.screenMgr.Pop()
		// Direct launch (no tmux): suspend TUI, run agent full-screen.
		if msg.directCmd != nil {
			return m, tea.ExecProcess(msg.directCmd, func(err error) tea.Msg {
				return statusMsg("Agent exited")
			})
		}
		// Switch to agent tmux session.
		if msg.sessionID != "" {
			if tmux.InsideTmux() {
				return m, m.switchTmuxClientCmd(msg.sessionID)
			}
			attachCmd := exec.Command("tmux", "attach-session", "-t", msg.sessionID)
			return m, tea.ExecProcess(attachCmd, func(err error) tea.Msg {
				return agentSessionReturnedMsg{}
			})
		}
		return m, tea.Batch(m.checkTmuxCmd(), m.clearStatusCmd())

	case agentSessionReturnedMsg:
		m.switching = false
		return m, tea.Batch(m.checkTmuxCmd(), m.loadWorktreesCmd(), m.clearStatusCmd())

	case tmuxAvailableMsg:
		m.tmuxAvailable = msg.available
		m.tmuxSessions = msg.sessions
		// Re-enrich worktree status with new session info
		m.enrichTmuxStatus()
		m.enrichAgentStatus()
		return m, nil

	case tmuxSessionCreatedMsg:
		if msg.err != nil {
			m.switching = false
			m.errText = msg.err.Error()
			return m, m.clearStatusCmd()
		}
		if tmux.InsideTmux() {
			// Non-blocking: tell tmux server to switch client
			return m, m.switchTmuxClientCmd(msg.sessionName)
		}
		// Outside tmux: suspend TUI and attach to the session
		attachCmd := exec.Command("tmux", "attach-session", "-t", msg.sessionName)
		return m, tea.ExecProcess(attachCmd, func(err error) tea.Msg {
			return tmuxSessionReturnedMsg{}
		})

	case tmuxSessionReturnedMsg:
		m.switching = false
		return m, tea.Batch(m.checkTmuxCmd(), m.loadWorktreesCmd())

	case tmuxSessionKilledMsg:
		if msg.err != nil {
			m.errText = msg.err.Error()
		} else {
			m.statusText = "Killed session: " + msg.name
			// Remove from agent tracking
			for branch, sid := range m.agentSessions {
				if sid == msg.name {
					delete(m.agentSessions, branch)
				}
			}
		}
		// Rebuild session list
		m.buildSessionInfos()
		if m.sessionCursor >= len(m.sessionInfos) {
			m.sessionCursor = max(0, len(m.sessionInfos)-1)
		}
		return m, tea.Batch(m.checkTmuxCmd(), m.clearStatusCmd())

	case detailLoadedMsg:
		m.detailFiles = msg.files
		m.detailDiffStat = msg.diffStat
		return m, nil

	case statusMsg:
		m.statusText = string(msg)
		return m, m.clearStatusCmd()

	case tickMsg:
		return m, tea.Batch(m.loadWorktreesCmd(), m.checkTmuxCmd(), m.tickCmd())

	case clearStatusMsg:
		m.statusText = ""
		m.errText = ""
		return m, nil

	case errMsg:
		m.switching = false
		if msg.err != nil {
			m.errText = msg.err.Error()
			return m, m.clearStatusCmd()
		}
		return m, nil

	case tea.FocusMsg:
		// Terminal regained focus (e.g. user switched back from another tmux session).
		// Reset switching guard and refresh data so the TUI is never stuck.
		m.switching = false
		return m, tea.Batch(m.checkTmuxCmd(), m.loadWorktreesCmd())

	case dirListedMsg:
		if msg.err != nil {
			m.errText = "tree: " + msg.err.Error()
			return m, m.clearStatusCmd()
		}
		node := components.FindNode(m.treeRoot, msg.parentPath)
		if node != nil {
			node.Children = msg.entries
			node.Loaded = true
			node.Expanded = true
			m.treeFlatNodes = components.FlattenVisible(m.treeRoot)
		}
		return m, nil

	case tea.KeyPressMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

func (m *Model) View() tea.View {
	if !m.ready {
		v := tea.NewView("Loading...")
		v.AltScreen = true
		return v
	}

	// Repo name for status bar
	repoName := ""
	if m.activeRepo >= 0 && m.activeRepo < len(m.repos) {
		repoName = m.repos[m.activeRepo].DisplayName()
	}

	// Stats
	stats := components.ComputeStats(m.worktrees)

	// Key hints
	hints := components.KeyHints(m.theme, m.screenMgr.Active(), components.PaneID(m.activePane))

	// Status bar
	statusBar := components.StatusBar(
		m.width, m.theme, repoName,
		stats, m.tmuxAvailable,
		m.screenMgr.Active(), m.statusText, m.errText,
	)

	// Calculate pane heights
	// Reserve lines for hints + statusbar + padding
	hintsHeight := 1
	statusHeight := 1
	padding := 2
	reservedHeight := hintsHeight + statusHeight + padding
	availableHeight := m.height - reservedHeight
	if availableHeight < 10 {
		availableHeight = 10
	}

	// Hide bottom pane when a modal overlay is active
	hasOverlay := m.screenMgr.Active() != screen.None

	// 60/40 split — upper pane keeps its size even with overlay
	upperHeight := availableHeight * 60 / 100
	lowerHeight := availableHeight - upperHeight

	// Content height inside bordered panes (border takes 2 lines)
	upperContentHeight := upperHeight - 2
	lowerContentHeight := lowerHeight - 2
	if upperContentHeight < 3 {
		upperContentHeight = 3
	}
	if lowerContentHeight < 3 {
		lowerContentHeight = 3
	}

	contentWidth := m.width - 4 // padding from App style + border
	if contentWidth < 20 {
		contentWidth = 20
	}

	// Upper pane: worktree dashboard
	upperContent := views.Dashboard(
		m.worktrees,
		m.filteredIndices,
		m.cursor,
		m.filterText,
		m.filterMode,
		m.theme,
		contentWidth,
		upperContentHeight,
	)

	// Lower pane: file tree
	var treeMatchIndices []int
	if m.treeFilterText != "" {
		treeMatchIndices = components.FilterFlatNodes(m.treeFlatNodes, m.treeFilterText)
	}
	lowerContent := views.FileTree(
		m.treeFlatNodes,
		m.treeCursor,
		m.activePane == PaneFileTree,
		m.treeFilterMode,
		m.treeFilterText,
		treeMatchIndices,
		m.theme,
		contentWidth,
		lowerContentHeight,
	)

	// Border colors based on focus
	upperBorderColor := m.theme.Border
	lowerBorderColor := m.theme.Border
	upperTitle := " Worktrees "
	lowerTitle := " Browse: " + m.treeRoot.Path + " "
	if m.activePane == PaneWorktrees {
		upperBorderColor = m.theme.Primary
	} else {
		lowerBorderColor = m.theme.Primary
	}

	upperPane := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(upperBorderColor).
		BorderTop(true).
		BorderBottom(true).
		BorderLeft(true).
		BorderRight(true).
		Width(m.width - 2).
		Height(upperContentHeight).
		Render(upperContent)

	// Inject title into border
	upperPane = injectBorderTitle(upperPane, upperTitle, m.theme, m.activePane == PaneWorktrees)

	lowerPane := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lowerBorderColor).
		BorderTop(true).
		BorderBottom(true).
		BorderLeft(true).
		BorderRight(true).
		Width(m.width - 2).
		Height(lowerContentHeight).
		Render(lowerContent)

	lowerPane = injectBorderTitle(lowerPane, lowerTitle, m.theme, m.activePane == PaneFileTree)

	var content string
	if hasOverlay {
		content = lipgloss.JoinVertical(lipgloss.Left, upperPane, hints)
	} else {
		content = lipgloss.JoinVertical(lipgloss.Left, upperPane, hints, lowerPane)
	}

	// Overlay modal if active
	overlay := ""
	switch m.screenMgr.Active() {
	case screen.Help:
		overlay = views.Help(m.theme, m.width, m.height)
	case screen.WorktreeCreate:
		overlay = views.Create(m.createBranch, m.createBase, 0, m.theme, m.width)
	case screen.Commit:
		overlay = views.Commit(m.commitType, m.commitScope, m.commitMsg, m.commitField, m.cfg.Git.ConventionalCommits, m.commitFiles, m.theme, m.width)
	case screen.AgentSelect:
		overlay = views.AgentSelect(m.agentProviders, m.agentAvailable, m.agentCursor, m.theme, m.width)
	case screen.Confirm:
		overlay = views.Confirm(m.confirmTitle, m.confirmMessage, m.theme, m.width)
	case screen.RepoSelect:
		overlay = views.RepoSelect(m.repos, m.activeRepo, m.repoCursor, m.theme, m.width)
	case screen.Sessions:
		overlay = views.Sessions(m.sessionInfos, m.sessionCursor, m.theme, m.width, m.height)
	case screen.WorktreeDetail:
		wt := m.selectedWorktree()
		if wt != nil {
			overlay = views.Detail(*wt, m.detailFiles, m.detailDiffStat, m.theme, m.width)
		}
	}

	// Compose
	if overlay != "" {
		content = content + "\n\n" + overlay
	}

	rendered := m.styles.App.
		Width(m.width).
		Height(m.height).
		Render(content + "\n" + statusBar)

	v := tea.NewView(rendered)
	v.AltScreen = true
	v.ReportFocus = true
	return v
}

// injectBorderTitle replaces the start of the top border line with a title.
func injectBorderTitle(rendered, title string, t *theme.Theme, focused bool) string {
	lines := strings.SplitN(rendered, "\n", 2)
	if len(lines) < 2 {
		return rendered
	}

	titleColor := t.TextDim
	if focused {
		titleColor = t.Primary
	}
	styledTitle := lipgloss.NewStyle().Foreground(titleColor).Bold(true).Render(title)

	// Replace characters after the first border rune
	topLine := lines[0]
	runes := []rune(topLine)
	if len(runes) > 2 {
		// Keep first rune (corner), insert title, continue border
		titleWidth := len([]rune(title))
		if titleWidth+2 < len(runes) {
			newTop := string(runes[0:1]) + styledTitle + string(runes[1+titleWidth:])
			return newTop + "\n" + lines[1]
		}
	}

	return rendered
}

// --- Helper methods ---

func (m *Model) updateFilteredIndices() {
	m.filteredIndices = components.FilterWorktrees(m.worktrees, m.filterText)
	if m.cursor >= len(m.filteredIndices) {
		m.cursor = max(0, len(m.filteredIndices)-1)
	}
}

func (m *Model) enrichTmuxStatus() {
	sessionSet := make(map[string]bool)
	for _, s := range m.tmuxSessions {
		sessionSet[s] = true
	}
	for i, wt := range m.worktrees {
		repoName := filepath.Base(wt.RepoDir)
		name := tmux.SessionNameForWorktree(repoName, wt.Branch)
		if sessionSet[name] {
			m.worktrees[i].Status.TmuxSession = name
		} else {
			m.worktrees[i].Status.TmuxSession = ""
		}
	}
}

func (m *Model) enrichAgentStatus() {
	// Check if agent sessions still exist
	sessionSet := make(map[string]bool)
	for _, s := range m.tmuxSessions {
		sessionSet[s] = true
	}

	for branch, sid := range m.agentSessions {
		if !sessionSet[sid] {
			delete(m.agentSessions, branch)
		}
	}

	for i, wt := range m.worktrees {
		_, hasAgent := m.agentSessions[wt.Branch]
		m.worktrees[i].Status.AgentRunning = hasAgent
	}
}

func (m *Model) buildSessionInfos() {
	// Build a map of worktree sessions
	wtBySession := make(map[string]*models.Worktree)
	for i, wt := range m.worktrees {
		if wt.Status.TmuxSession != "" {
			wtBySession[wt.Status.TmuxSession] = &m.worktrees[i]
		}
	}

	agentSessionSet := make(map[string]bool)
	for _, sid := range m.agentSessions {
		agentSessionSet[sid] = true
	}

	m.sessionInfos = make([]views.SessionInfo, 0, len(m.tmuxSessions))
	for _, name := range m.tmuxSessions {
		info := views.SessionInfo{
			Name:    name,
			IsAgent: agentSessionSet[name] || strings.HasSuffix(name, "-agent"),
		}
		if wt, ok := wtBySession[name]; ok {
			info.Worktree = wt
		}
		m.sessionInfos = append(m.sessionInfos, info)
	}
}

func (m *Model) sortedAgentProviders() ([]string, []string) {
	all := m.agents.List()
	sort.Strings(all)
	avail := m.agents.Available()
	return all, avail
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
