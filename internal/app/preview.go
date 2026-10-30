package app

import (
	"context"
	"path/filepath"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/Energma/blink-f/internal/app/screen"
	"github.com/Energma/blink-f/internal/models"
	"github.com/Energma/blink-f/internal/tmux"
)

// previewInterval is how often the visible session is captured again; often
// enough to follow an agent, rare enough to cost nothing.
const previewInterval = time.Second

// previewVisible reports whether the preview pane is on screen. Modals take
// the full width, so the preview steps aside while one is open.
func (m *Model) previewVisible() bool {
	return m.previewOn && m.width >= previewMinWidth && m.screenMgr.Active() == screen.None
}

// previewTarget identifies the worktree the preview shows, "" for none.
func (m *Model) previewTarget() string {
	if wt := m.selectedWorktree(); wt != nil {
		return wt.Path
	}
	return ""
}

// previewSessionName picks the session to preview for wt: the agent session
// when one runs, since watching agents is what the preview is for, otherwise
// the plain session. "" when the worktree has neither.
func previewSessionName(wt models.Worktree) string {
	if wt.Status.AgentRunning {
		return tmux.AgentSessionNameForWorktree(filepath.Base(wt.RepoDir), wt.Branch)
	}
	return wt.Status.TmuxSession
}

// previewCmd captures right away when the selection changed or the refresh
// loop is starting, and keeps exactly one tick in flight while the preview is
// visible. before is the previewTarget from before the message was handled.
func (m *Model) previewCmd(before string) tea.Cmd {
	if !m.previewVisible() || !m.tmuxAvailable {
		return nil
	}
	var cmds []tea.Cmd
	if m.previewTarget() != before || !m.previewTicking {
		cmds = append(cmds, m.capturePreviewCmd())
	}
	if !m.previewTicking {
		m.previewTicking = true
		cmds = append(cmds, tea.Tick(previewInterval, func(time.Time) tea.Msg {
			return previewTickMsg{}
		}))
	}
	return tea.Batch(cmds...)
}

func (m *Model) capturePreviewCmd() tea.Cmd {
	wt := m.selectedWorktree()
	if wt == nil {
		return nil
	}
	path := wt.Path
	session := previewSessionName(*wt)
	if session == "" {
		return nil
	}
	svc := m.tmux
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		content, err := svc.CapturePane(ctx, session)
		return previewCapturedMsg{path: path, session: session, content: content, err: err}
	}
}

// previewTitle names the pane after the selected worktree.
func (m *Model) previewTitle() string {
	wt := m.selectedWorktree()
	if wt == nil {
		return " Preview "
	}
	return " Preview: " + filepath.Base(wt.RepoDir) + "/" + wt.Branch + " "
}

// previewBody renders the preview content for a w x h content area.
func (m *Model) previewBody(w, h int) string {
	dim := lipgloss.NewStyle().Foreground(m.theme.TextDim)
	wt := m.selectedWorktree()
	if wt == nil {
		return dim.Render("no worktree selected")
	}
	if !m.tmuxAvailable {
		return dim.Render(m.tmux.Backend() + " not available")
	}
	session := previewSessionName(*wt)
	switch {
	case session == "":
		return dim.Render("no session — press enter to start")
	case m.previewPath != wt.Path || m.previewSession != session:
		return dim.Render("capturing " + session + "...")
	case m.previewErr != nil:
		return dim.Render(m.previewErr.Error())
	}
	return previewLines(m.previewContent, w, h)
}
