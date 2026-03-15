package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/keiritz/encrypten/internal/crypto"
	"github.com/keiritz/encrypten/internal/fileformat"
	"github.com/keiritz/encrypten/internal/gitutil"
	"github.com/keiritz/encrypten/internal/keyfile"
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

	// Encrypt files in-place (much faster than re-checkout via filter).
	if err := transformEncrypt(dir); err != nil {
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

// loadDefaultKey reads the encryption key from the key directory.
func loadDefaultKey(dir string) (*keyfile.Key, error) {
	keyDir, err := gitutil.KeyDir(dir)
	if err != nil {
		return nil, fmt.Errorf("determining key directory: %w", err)
	}
	f, err := os.Open(filepath.Join(keyDir, "default")) //nolint:gosec // path derived from git dir
	if err != nil {
		return nil, fmt.Errorf("opening key file: %w", err)
	}
	defer func() { _ = f.Close() }()
	k, err := keyfile.Read(f)
	if err != nil {
		return nil, fmt.Errorf("reading key file: %w", err)
	}
	return k, nil
}

// transformEncrypt encrypts tracked files in-place.
func transformEncrypt(dir string) error {
	entries, err := gitutil.ListEncryptedFiles(dir)
	if err != nil {
		return fmt.Errorf("listing encrypted files: %w", err)
	}
	if len(entries) == 0 {
		return nil
	}

	key, err := loadDefaultKey(dir)
	if err != nil {
		return err
	}

	for _, e := range entries {
		p := filepath.Join(dir, e.Path)
		info, err := os.Stat(p)
		if err != nil {
			return fmt.Errorf("stat %s: %w", e.Path, err)
		}
		data, err := os.ReadFile(p) //nolint:gosec // path from git-tracked file list
		if err != nil {
			return fmt.Errorf("reading %s: %w", e.Path, err)
		}
		if fileformat.IsEncrypted(data) {
			continue // already encrypted
		}
		enc, err := crypto.Encrypt(data, key)
		if err != nil {
			return fmt.Errorf("encrypting %s: %w", e.Path, err)
		}
		if err := os.WriteFile(p, enc, info.Mode()); err != nil { //nolint:gosec // path from git-tracked file list
			return fmt.Errorf("writing %s: %w", e.Path, err)
		}
	}

	// Refresh the git index so the working tree appears clean after
	// in-place transformation.
	return refreshIndex(dir, entries)
}

// refreshIndex updates the git index entries for the given files so that
// git considers the working tree clean after an in-place transformation.
func refreshIndex(dir string, entries []gitutil.EncryptedFileEntry) error {
	args := make([]string, 0, len(entries)+2)
	args = append(args, "update-index", "--")
	for _, e := range entries {
		args = append(args, e.Path)
	}
	cmd := exec.Command("git", args...) // #nosec G204
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("updating index: %w\n%s", err, out)
	}
	return nil
}
