package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/keiritz/encrypten/internal/gitutil"
)

func cmdLock(args []string) error {
	force := false
	for _, a := range args {
		if a == "--force" || a == "-f" {
			force = true
		}
	}

	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	lock, err := gitutil.AcquireLock(dir)
	if err != nil {
		return fmt.Errorf("acquiring lock: %w", err)
	}
	defer func() { _ = lock.Release() }()

	// Check if already locked by inspecting actual file contents.
	state, err := gitutil.DetectEncryptionState(dir)
	if err != nil {
		return fmt.Errorf("detecting encryption state: %w", err)
	}
	if state == gitutil.StateFullyEncrypted || state == gitutil.StateNoFiles {
		return fmt.Errorf("this repository is already locked")
	}

	// Check for uncommitted changes.
	if !force {
		if err := ensureClean(dir); err != nil {
			return err
		}
	}

	// Lock by setting per-WT filter override to cat (passthrough).
	// This is the same for main and secondary worktrees — shared filter
	// config and key file are never removed to avoid concurrent races.
	if err := gitutil.SetFilterWorktree(dir, "cat", "cat", "false"); err != nil {
		return fmt.Errorf("setting worktree filter override: %w", err)
	}

	// Re-checkout: cat override takes effect, writing encrypted content.
	if err := recheckout(dir); err != nil {
		_ = gitutil.UnsetFilterWorktree(dir)
		return err
	}

	return nil
}

// ensureClean checks that the working tree has no uncommitted changes.
func ensureClean(dir string) error {
	// Refresh stat info to avoid false positives from stale index.
	refresh := exec.Command("git", "update-index", "-q", "--really-refresh") // #nosec G204
	refresh.Dir = dir
	_ = refresh.Run() // best-effort

	check := exec.Command("git", "diff-index", "--quiet", "HEAD", "--") // #nosec G204
	check.Dir = dir
	if err := check.Run(); err != nil {
		return fmt.Errorf("working directory not clean.\nPlease commit your changes or 'git stash' them before running this command")
	}
	return nil
}

// recheckout removes encrypted files and runs git checkout to re-create them.
func recheckout(dir string) error {
	entries, err := gitutil.ListEncryptedFiles(dir)
	if err != nil {
		return fmt.Errorf("listing encrypted files: %w", err)
	}
	for _, e := range entries {
		_ = os.Remove(filepath.Join(dir, e.Path))
	}

	if len(entries) > 0 {
		args := []string{"checkout", "--"}
		for _, e := range entries {
			args = append(args, e.Path)
		}
		cmd := exec.Command("git", args...) // #nosec G204
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git checkout failed: %w\n%s", err, out)
		}
	}
	return nil
}
