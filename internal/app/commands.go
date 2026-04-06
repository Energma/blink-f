package app

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/Energma/blink-f/internal/agent"
	"github.com/Energma/blink-f/internal/app/components"
	"github.com/Energma/blink-f/internal/models"
	"github.com/Energma/blink-f/internal/tmux"
)

func (m *Model) loadWorktreesCmd() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		dir := m.currentRepoDir()
		if dir == "" {
			return worktreesLoadedMsg{err: nil}
		}
		wts, err := m.git.ListWorktrees(ctx, dir)
		if err != nil {
			return worktreesLoadedMsg{err: err}
		}
		// Load last commit for each
		for i := range wts {
			ci, _ := m.git.LastCommitInfo(ctx, wts[i].Path)
			wts[i].Last = ci
		}
		return worktreesLoadedMsg{worktrees: wts}
	}
}

func (m *Model) loadStatusCmd(index int, dir string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		st, err := m.git.Status(ctx, dir)
		return gitStatusMsg{index: index, status: st, err: err}
	}
}

func (m *Model) loadAllStatusesCmd() tea.Cmd {
	var cmds []tea.Cmd
	for i, wt := range m.worktrees {
		cmds = append(cmds, m.loadStatusCmd(i, wt.Path))
	}
	return tea.Batch(cmds...)
}

func (m *Model) checkTmuxCmd() tea.Cmd {
	return func() tea.Msg {
		available := m.tmux.IsAvailable()
		var sessions []string
		if available {
			sessions, _ = m.tmux.ListSessions(context.Background())
		}
		return tmuxAvailableMsg{available: available, sessions: sessions}
	}
}

func (m *Model) createWorktreeCmd(branch, baseBranch string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		dir := m.currentRepoDir()
		wt, err := m.git.CreateWorktree(ctx, dir, branch, baseBranch, m.cfg.Worktree.BaseDir)
		if err != nil {
			return worktreeCreatedMsg{err: err}
		}

		// Auto-create tmux session
		if m.cfg.Tmux.AutoSession && m.tmux.IsAvailable() {
			repoName := filepath.Base(dir)
			_, _ = m.tmux.EnsureSession(ctx, repoName, branch, wt.Path)
		}

		// Auto-symlinks
		createSymlinks(dir, wt.Path, m.cfg.Worktree.AutoSymlinks)

		return worktreeCreatedMsg{worktree: wt}
	}
}

func (m *Model) deleteWorktreeCmd(wt models.Worktree, force bool) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()

		// Kill associated tmux session first
		repoName := filepath.Base(wt.RepoDir)
		sessionName := tmux.SessionNameForWorktree(repoName, wt.Branch)
		if m.tmux.SessionExists(sessionName) {
			_ = m.tmux.KillSession(ctx, sessionName)
		}

		err := m.git.RemoveWorktree(ctx, wt.RepoDir, wt.Path, force)
		return worktreeDeletedMsg{name: wt.Branch, err: err}
	}
}

func (m *Model) switchWorktreeCmd(wt models.Worktree) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		if !m.tmux.IsAvailable() {
			return errMsg{err: fmt.Errorf("tmux not available — install with: sudo pacman -S tmux")}
		}
		repoName := filepath.Base(wt.RepoDir)
		name, err := m.tmux.EnsureSession(ctx, repoName, wt.Branch, wt.Path)
		if err != nil {
			return errMsg{err: err}
		}
		// Don't call SwitchSession here — the Update handler will use
		// tea.ExecProcess (outside tmux) or switch-client (inside tmux)
		// so the TUI is properly suspended.
		return tmuxSessionCreatedMsg{sessionName: name, err: nil}
	}
}

// switchTmuxClientCmd runs switch-client inside tmux (non-blocking).
func (m *Model) switchTmuxClientCmd(name string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		err := exec.CommandContext(ctx, "tmux", "switch-client", "-t", name).Run()
		if err != nil {
			return errMsg{err: fmt.Errorf("switch to %s: %w", name, err)}
		}
		return tmuxSessionReturnedMsg{}
	}
}

func (m *Model) commitCmd(message string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		wt := m.selectedWorktree()
		if wt == nil {
			return commitResultMsg{err: nil}
		}
		out, err := m.git.Commit(ctx, wt.Path, message)
		return commitResultMsg{output: out, err: err}
	}
}

func (m *Model) pushCmd() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		wt := m.selectedWorktree()
		if wt == nil {
			return pushResultMsg{}
		}
		err := m.git.Push(ctx, wt.Path)
		return pushResultMsg{err: err}
	}
}

func (m *Model) pullCmd() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		wt := m.selectedWorktree()
		if wt == nil {
			return pullResultMsg{}
		}
		err := m.git.Pull(ctx, wt.Path)
		return pullResultMsg{err: err}
	}
}

func (m *Model) stashCmd() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		wt := m.selectedWorktree()
		if wt == nil {
			return stashResultMsg{}
		}
		err := m.git.Stash(ctx, wt.Path)
		return stashResultMsg{err: err}
	}
}

func (m *Model) launchAgentCmd(providerName string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		wt := m.selectedWorktree()
		if wt == nil {
			return agentLaunchedMsg{err: nil}
		}

		name := providerName
		if name == "" {
			name = m.cfg.Agents.Default
		}

		p, err := m.agents.Get(name)
		if err != nil {
			return agentLaunchedMsg{err: err}
		}

		// No tmux: suspend TUI and run agent directly.
		if !m.tmux.IsAvailable() {
			cmd := agent.LaunchDirect(ctx, p, wt.Path)
			return agentLaunchedMsg{
				provider:  name,
				worktree:  wt.Branch,
				directCmd: cmd,
			}
		}

		sessionID := tmux.SessionNameForWorktree(filepath.Base(wt.RepoDir), wt.Branch) + "-agent"

		if err = agent.LaunchInSplitSession(ctx, m.tmux, p, sessionID, wt.Path); err != nil {
			return agentLaunchedMsg{err: err}
		}

		// Don't call SwitchSession here — the Update handler will use
		// tea.ExecProcess (outside tmux) or switch-client (inside tmux).
		return agentLaunchedMsg{
			provider:  name,
			worktree:  wt.Branch,
			sessionID: sessionID,
		}
	}
}

// loadDetailCmd loads detail info for a worktree via message.
func (m *Model) loadDetailCmd(wt models.Worktree) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		files, _ := m.git.FilesChanged(ctx, wt.Path)
		stat, _ := m.git.DiffStat(ctx, wt.Path)
		return detailLoadedMsg{files: files, diffStat: stat}
	}
}

// loadCommitFilesCmd loads the list of files that will be committed.
func (m *Model) loadCommitFilesCmd() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		wt := m.selectedWorktree()
		if wt == nil {
			return commitFilesLoadedMsg{}
		}
		files, err := m.git.FilesChanged(ctx, wt.Path)
		return commitFilesLoadedMsg{files: files, err: err}
	}
}

// killSessionCmd kills a tmux session by name.
func (m *Model) killSessionCmd(name string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		err := m.tmux.KillSession(ctx, name)
		return tmuxSessionKilledMsg{name: name, err: err}
	}
}

// launchEditorCmd opens the configured editor in the worktree directory.
func (m *Model) launchEditorCmd(dir string) tea.Cmd {
	return func() tea.Msg {
		cmd := m.cfg.Editor.Command
		args := make([]string, len(m.cfg.Editor.Args))
		copy(args, m.cfg.Editor.Args)

		c := exec.Command(cmd, args...)
		c.Dir = dir
		if err := c.Start(); err != nil {
			return errMsg{err: fmt.Errorf("editor: %w", err)}
		}
		// Don't wait — editor runs independently
		go func() { _ = c.Wait() }()
		return statusMsg("Opened " + cmd + " in " + filepath.Base(dir))
	}
}

// loadBranchesCmd fetches local branch names for the current repo.
func (m *Model) loadBranchesCmd() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		dir := m.currentRepoDir()
		if dir == "" {
			return branchesLoadedMsg{err: fmt.Errorf("no repo selected")}
		}
		branches, err := m.git.BranchList(ctx, dir)
		return branchesLoadedMsg{branches: branches, err: err}
	}
}

// checkoutBranchCmd switches the selected worktree to the given branch.
func (m *Model) checkoutBranchCmd(dir, branch string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		err := m.git.CheckoutBranch(ctx, dir, branch)
		return branchCheckedOutMsg{branch: branch, err: err}
	}
}

// cleanMergedCmd finds and deletes merged worktrees.
func (m *Model) cleanMergedCmd() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		dir := m.currentRepoDir()
		if dir == "" {
			return errMsg{err: fmt.Errorf("no repo selected")}
		}

		defaultBranch := m.git.DefaultBranch(ctx, dir)
		wts, err := m.git.ListWorktrees(ctx, dir)
		if err != nil {
			return errMsg{err: err}
		}

		var removed []string
		repoName := filepath.Base(dir)
		for _, wt := range wts {
			if wt.IsMain || wt.Branch == defaultBranch {
				continue
			}
			if m.git.IsBranchMerged(ctx, dir, wt.Branch, defaultBranch) {
				// Kill session
				sn := tmux.SessionNameForWorktree(repoName, wt.Branch)
				if m.tmux.SessionExists(sn) {
					_ = m.tmux.KillSession(ctx, sn)
				}
				if err := m.git.RemoveWorktree(ctx, dir, wt.Path, false); err == nil {
					removed = append(removed, wt.Branch)
				}
			}
		}

		if len(removed) == 0 {
			return statusMsg("No merged worktrees to clean")
		}
		return statusMsg(fmt.Sprintf("Cleaned %d merged: %s", len(removed), strings.Join(removed, ", ")))
	}
}

func (m *Model) tickCmd() tea.Cmd {
	return tea.Tick(5*time.Second, func(t time.Time) tea.Msg {
		return tickMsg{}
	})
}

func (m *Model) clearStatusCmd() tea.Cmd {
	return tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
		return clearStatusMsg{}
	})
}

// --- Helpers ---

func (m *Model) currentRepoDir() string {
	if m.activeRepo >= 0 && m.activeRepo < len(m.repos) {
		return m.repos[m.activeRepo].Path
	}
	return ""
}

func (m *Model) selectedWorktree() *models.Worktree {
	if len(m.filteredIndices) > 0 && m.cursor >= 0 && m.cursor < len(m.filteredIndices) {
		idx := m.filteredIndices[m.cursor]
		if idx >= 0 && idx < len(m.worktrees) {
			return &m.worktrees[idx]
		}
	}
	return nil
}

func createSymlinks(mainDir, worktreeDir string, symlinks []string) {
	for _, name := range symlinks {
		src := filepath.Join(mainDir, name)
		dst := filepath.Join(worktreeDir, name)
		if fileExists(src) && !fileExists(dst) {
			_ = symlink(src, dst)
		}
	}
}

// listDirCmd reads directory entries asynchronously for the file tree.
func listDirCmd(parentPath string, depth int) tea.Cmd {
	return func() tea.Msg {
		entries, err := components.ReadDirEntries(parentPath, depth)
		return dirListedMsg{parentPath: parentPath, entries: entries, err: err}
	}
}

func fileExists(path string) bool {
	_, err := statFile(path)
	return err == nil
}
