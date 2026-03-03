package app

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"

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
	tmuxAvailable bool
	tmuxSessions  []string

	// Agent tracking: map worktree branch -> agent session name
	agentSessions map[string]string

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
	}
}

func (m *Model) Init() tea.Cmd {
	cmds := []tea.Cmd{
		m.checkTmuxCmd(),
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
		// Outside tmux: attach to the split session.
		if msg.shouldAttach && msg.sessionID != "" {
			attachCmd := exec.Command("tmux", "attach-session", "-t", msg.sessionID)
			return m, tea.ExecProcess(attachCmd, func(err error) tea.Msg {
				// Session may have ended normally; ignore the error.
				return agentSessionReturnedMsg{}
			})
		}
		return m, tea.Batch(m.checkTmuxCmd(), m.clearStatusCmd())

	case agentSessionReturnedMsg:
		// Returned from an agent tmux session (detached or session ended).
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
			m.errText = msg.err.Error()
			return m, m.clearStatusCmd()
		}
		return m, m.checkTmuxCmd()

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
		if msg.err != nil {
			m.errText = msg.err.Error()
			return m, m.clearStatusCmd()
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

	// Main content = dashboard
	content := views.Dashboard(
		m.worktrees,
		m.filteredIndices,
		m.cursor,
		m.filterText,
		m.filterMode,
		m.theme,
		m.width,
		m.height,
	)

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

	// Key hints
	hints := components.KeyHints(m.theme, m.screenMgr.Active())

	// Status bar
	statusBar := components.StatusBar(
		m.width, m.theme, repoName,
		stats, m.tmuxAvailable,
		m.screenMgr.Active(), m.statusText, m.errText,
	)

	// Compose
	if overlay != "" {
		content = content + "\n\n" + overlay
	}

	rendered := m.styles.App.
		Width(m.width).
		Height(m.height).
		Render(content + "\n\n" + hints + "\n" + statusBar)

	v := tea.NewView(rendered)
	v.AltScreen = true
	return v
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
