package app

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

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
	launchWorktree  string // worktree blink was started in, selected on first load

	// Filter
	filterText string
	filterMode bool

	// Modal states
	screenMgr *screen.Manager

	// Create form
	createBranch components.TextInput
	createBase   components.TextInput
	createField  int // 0=branch name, 1=base branch

	// Branch select
	branches        []string
	branchMatches   []string // branches filtered by branchFilter, what the list shows
	branchFilter    components.TextInput
	branchCursor    int
	branchCurrent   string // branch checked out in the target worktree, marked in the list
	branchForCreate bool   // true = pick base branch for create, false = checkout

	// Commit form
	commitType  int
	commitScope components.TextInput
	commitMsg   components.TextInput
	commitField int // 0=type, 1=scope, 2=msg
	commitFiles []string

	// Agent select
	agentCursor    int
	agentProviders []string
	agentAvailable []string
	agentShell     bool // picker lists a plain shell first, offered when nothing runs yet

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
	activePane     PaneID
	treeRoot       *components.TreeNode
	treeFlatNodes  []*components.TreeNode
	treeCursor     int
	treeFilterMode bool
	treeFilterText string

	// Help scroll
	helpScroll int

	// Session preview pane. previewOn is the user's toggle; the pane also
	// needs a wide terminal and no modal open (see previewVisible).
	previewOn      bool
	previewTicking bool   // a previewTickMsg is in flight; keeps one refresh loop
	previewPath    string // worktree the content below was captured for
	previewSession string
	previewContent string
	previewErr     error

	// Last left click, to detect a double-click.
	lastClick   clickTarget
	lastClickAt time.Time

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
	var launchWorktree string
	if gitSvc.IsGitRepo(context.Background(), cwd) {
		root, _ := gitSvc.RepoRoot(context.Background(), cwd)
		if root != "" {
			launchWorktree = root
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
		git:            gitSvc,
		tmux:           tmux.NewService(cfg),
		agents:         agent.NewRegistry(cfg),
		cfg:            cfg,
		theme:          t,
		styles:         s,
		repos:          repos,
		screenMgr:      screen.NewManager(),
		agentSessions:  make(map[string]string),
		treeRoot:       treeRoot,
		previewOn:      true,
		launchWorktree: launchWorktree,
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

// Update handles msg, then keeps the session preview in step with whatever
// changed: the selection, the screen, the terminal size or the toggle.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	before := m.previewTarget()
	model, cmd := m.update(msg)
	return model, tea.Batch(cmd, m.previewCmd(before))
}

func (m *Model) update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
		// Only the first load follows the launch dir; later reloads keep the
		// user's own selection.
		if m.launchWorktree != "" {
			m.selectWorktree(m.launchWorktree)
			m.launchWorktree = ""
		}
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
		m.createBranch.Reset()
		m.createBase.Reset()
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
		m.commitMsg.Reset()
		m.commitScope.Reset()
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
		// Switch to agent session.
		if msg.sessionID != "" {
			if m.tmux.Inside() {
				return m, m.switchTmuxClientCmd(msg.sessionID)
			}
			return m, tea.ExecProcess(m.tmux.AttachCmd(msg.sessionID), func(err error) tea.Msg {
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
		if m.tmux.Inside() {
			// Non-blocking: tell the multiplexer to switch client
			return m, m.switchTmuxClientCmd(msg.sessionName)
		}
		// Outside the multiplexer: suspend TUI and attach to the session
		return m, tea.ExecProcess(m.tmux.AttachCmd(msg.sessionName), func(err error) tea.Msg {
			return tmuxSessionReturnedMsg{}
		})

	case sessionPickMsg:
		m.switching = false
		m.statusText = ""
		m.openAgentPicker(msg.shell)
		return m, nil

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

	case branchesLoadedMsg:
		if msg.err != nil {
			m.errText = msg.err.Error()
			return m, m.clearStatusCmd()
		}
		m.branches = msg.branches
		m.applyBranchFilter()
		return m, nil

	case tea.PasteMsg:
		if in := m.focusedInput(); in != nil {
			in.Insert(msg.Content)
			if in == &m.branchFilter {
				m.applyBranchFilter()
			}
		}
		return m, nil

	case branchCheckedOutMsg:
		if msg.err != nil {
			m.errText = msg.err.Error()
			return m, m.clearStatusCmd()
		}
		m.statusText = "Switched to " + msg.branch
		return m, tea.Batch(m.loadWorktreesCmd(), m.clearStatusCmd())

	case statusMsg:
		m.statusText = string(msg)
		return m, m.clearStatusCmd()

	case tickMsg:
		return m, tea.Batch(m.loadWorktreesCmd(), m.checkTmuxCmd(), m.tickCmd())

	case previewTickMsg:
		// Update's previewCmd captures again and re-arms the tick while the
		// preview is still visible; otherwise the loop ends here.
		m.previewTicking = false
		return m, nil

	case previewCapturedMsg:
		if msg.path != m.previewTarget() {
			return m, nil
		}
		m.previewPath = msg.path
		m.previewSession = msg.session
		m.previewContent = msg.content
		m.previewErr = msg.err
		return m, nil

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

	case tea.MouseClickMsg:
		return m.handleMouseClick(tea.Mouse(msg), time.Now())

	case tea.MouseWheelMsg:
		return m.handleMouseWheel(tea.Mouse(msg))
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

	l := m.layout()

	// Navigation hints (between panes, or above the status bar with the preview)
	hints, _ := m.navHints(l.Hints)

	// Status bar
	statusBar := components.StatusBar(
		l.Status.W, m.theme, repoName,
		stats, m.tmuxAvailable,
		m.screenMgr.Active(), m.statusText, m.errText,
	)

	upperContent, _, _ := m.dashboard(l.Worktrees)
	lowerContent, _, _ := m.fileTree(l.FileTree)

	upperPane := m.renderPane(l.Worktrees, " Worktrees ", upperContent, m.activePane == PaneWorktrees)
	lowerPane := m.renderPane(l.FileTree, " Browse: "+m.treeRoot.Path+" ", lowerContent, m.activePane == PaneFileTree)

	// Check for overlay modal — replaces lower pane when active
	overlay := ""
	switch m.screenMgr.Active() {
	case screen.Help:
		overlay = views.Help(m.theme, m.width, m.height, m.helpScroll)
	case screen.WorktreeCreate:
		overlay = views.Create(&m.createBranch, &m.createBase, m.createField, m.theme, m.width)
	case screen.Commit:
		overlay = views.Commit(m.commitType, &m.commitScope, &m.commitMsg, m.commitField, m.cfg.Git.ConventionalCommits, m.commitFiles, m.theme, m.width)
	case screen.AgentSelect:
		overlay = views.AgentSelect(m.agentProviders, m.agentAvailable, m.agentShell, m.agentCursor, m.theme, m.width)
	case screen.Confirm:
		overlay = views.Confirm(m.confirmTitle, m.confirmMessage, m.theme, m.width)
	case screen.RepoSelect:
		overlay = views.RepoSelect(m.repos, m.activeRepo, m.repoCursor, m.theme, m.width)
	case screen.Sessions:
		overlay = views.Sessions(m.sessionInfos, m.sessionCursor, m.theme, m.width, m.height)
	case screen.BranchSelect:
		overlay = views.BranchSelect(m.branchMatches, len(m.branches), &m.branchFilter, m.branchCurrent, m.branchCursor, m.theme, m.width, m.height)
	case screen.WorktreeDetail:
		wt := m.selectedWorktree()
		if wt != nil {
			overlay = views.Detail(*wt, m.detailFiles, m.detailDiffStat, m.theme, m.width)
		}
	}

	var content string
	switch {
	case overlay != "":
		content = lipgloss.JoinVertical(lipgloss.Left, upperPane, hints, overlay)
	case l.Preview.W > 0:
		previewPane := m.renderPane(l.Preview, m.previewTitle(), m.previewBody(l.Preview.W-2, l.Preview.H-2), false)
		left := lipgloss.JoinVertical(lipgloss.Left, upperPane, lowerPane)
		content = lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.JoinHorizontal(lipgloss.Top, left, previewPane), hints)
	default:
		content = lipgloss.JoinVertical(lipgloss.Left, upperPane, hints, lowerPane)
	}

	rendered := m.styles.App.
		Width(m.width).
		Height(m.height).
		Render(content + "\n" + statusBar)

	v := tea.NewView(rendered)
	v.AltScreen = true
	v.ReportFocus = true
	if m.cfg.UI.Mouse {
		v.MouseMode = tea.MouseModeCellMotion
	}
	return v
}

// layout is where View puts each pane; mouse hit-testing reads it too.
func (m *Model) layout() layout {
	return computeLayout(m.width, m.height, m.previewVisible())
}

// navHints renders the nav hints line for r, with where each hint is drawn.
// Hints that don't fit are dropped whole rather than cut mid-word, so every
// hint on screen can be clicked.
func (m *Model) navHints(r rect) (string, []components.HintSpan) {
	all, spans := components.RenderHints(m.theme, r.W, components.NavHints(m.screenMgr.Active(), components.PaneID(m.activePane)))
	line, _, _ := strings.Cut(all, "\n")
	var shown []components.HintSpan
	for _, s := range spans {
		if s.Line == 0 {
			shown = append(shown, s)
		}
	}
	// A single hint wider than the line would still overflow it.
	return ansi.Truncate(line, r.W, ""), shown
}

// dashboard renders the worktree pane's content for r, along with the item
// on each content line and where the pane hints are (see views.Dashboard).
// View and mouse clicks share it so a click lands on what is drawn there.
func (m *Model) dashboard(r rect) (string, []int, []components.HintSpan) {
	return views.Dashboard(
		m.worktrees,
		m.filteredIndices,
		m.cursor,
		m.filterText,
		m.filterMode,
		m.activePane == PaneWorktrees,
		components.PaneHints(components.PaneWorktrees),
		m.theme,
		r.W-2,
		r.H-2,
	)
}

// fileTree renders the file tree pane's content for r, like dashboard.
func (m *Model) fileTree(r rect) (string, []int, []components.HintSpan) {
	var matchIndices []int
	if m.treeFilterText != "" {
		matchIndices = components.FilterFlatNodes(m.treeFlatNodes, m.treeFilterText)
	}
	return views.FileTree(
		m.treeFlatNodes,
		m.treeCursor,
		m.activePane == PaneFileTree,
		m.treeFilterMode,
		m.treeFilterText,
		matchIndices,
		components.PaneHints(components.PaneFileTree),
		m.theme,
		r.W-2,
		r.H-2,
	)
}

// renderPane draws a bordered pane that fills r exactly, clipping content
// that doesn't fit so the rect stays the pane's true on-screen size.
func (m *Model) renderPane(r rect, title, content string, focused bool) string {
	borderColor := m.theme.Border
	if focused {
		borderColor = m.theme.Primary
	}
	pane := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Width(r.W).
		Height(r.H).
		Render(fitLines(content, r.W-2, r.H-2))
	return injectBorderTitle(pane, title, m.theme, focused)
}

// injectBorderTitle replaces the start of the top border line with a title.
// The border line is styled, so it is cut by cell width, never by rune or
// byte, or the colour escapes would be split.
func injectBorderTitle(rendered, title string, t *theme.Theme, focused bool) string {
	lines := strings.SplitN(rendered, "\n", 2)
	if len(lines) < 2 {
		return rendered
	}

	titleColor := t.TextDim
	if focused {
		titleColor = t.Primary
	}

	// Keep the first corner and at least one border cell before the other.
	topLine := lines[0]
	room := ansi.StringWidth(topLine) - 3
	if room <= 0 {
		return rendered
	}
	title = ansi.Truncate(title, room, "")
	styledTitle := lipgloss.NewStyle().Foreground(titleColor).Bold(true).Render(title)
	titleWidth := ansi.StringWidth(title)
	newTop := ansi.Truncate(topLine, 1, "") + styledTitle + ansi.TruncateLeft(topLine, 1+titleWidth, "")
	return newTop + "\n" + lines[1]
}

// --- Helper methods ---

func (m *Model) updateFilteredIndices() {
	m.filteredIndices = components.FilterWorktrees(m.worktrees, m.filterText)
	if m.cursor >= len(m.filteredIndices) {
		m.cursor = max(0, len(m.filteredIndices)-1)
	}
}

// selectWorktree moves the cursor to the listed worktree at path, if any.
func (m *Model) selectWorktree(path string) {
	for i, idx := range m.filteredIndices {
		if m.worktrees[idx].Path == path {
			m.cursor = i
			return
		}
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

// enrichAgentStatus rebuilds agent tracking from the live session list rather
// than from what this process launched, so agents survive a blink restart.
func (m *Model) enrichAgentStatus() {
	sessionSet := make(map[string]bool)
	for _, s := range m.tmuxSessions {
		sessionSet[s] = true
	}

	m.agentSessions = make(map[string]string)
	for i, wt := range m.worktrees {
		name := tmux.AgentSessionNameForWorktree(filepath.Base(wt.RepoDir), wt.Branch)
		running := sessionSet[name]
		if running {
			m.agentSessions[wt.Branch] = name
		}
		m.worktrees[i].Status.AgentRunning = running
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

// openAgentPicker shows the launcher; shell adds a plain-shell entry on top.
func (m *Model) openAgentPicker(shell bool) {
	m.agentProviders, m.agentAvailable = m.sortedAgentProviders()
	m.agentShell = shell
	m.agentCursor = 0
	m.screenMgr.Push(screen.AgentSelect)
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
