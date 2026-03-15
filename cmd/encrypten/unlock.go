package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/keiritz/encrypten/internal/crypto"
	"github.com/keiritz/encrypten/internal/fileformat"
	"github.com/keiritz/encrypten/internal/gitutil"
	"github.com/keiritz/encrypten/internal/keyfile"
)

func cmdUnlock(args []string) error {
	force := false
	var filtered []string
	for _, a := range args {
		if a == "--force" || a == "-f" {
			force = true
		} else {
			filtered = append(filtered, a)
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

	// Check for uncommitted changes.
	if !force {
		if err := ensureClean(dir); err != nil {
			return err
		}
	}

	isWt, err := gitutil.IsWorktree(dir)
	if err != nil {
		return fmt.Errorf("checking worktree status: %w", err)
	}

	// Secondary worktree: key already exists in shared dir → unlock without key arg.
	if isWt {
		return unlockWorktree(dir, filtered)
	}

	// Main worktree: require key file argument.
	if len(filtered) != 1 {
		return fmt.Errorf("usage: encrypten unlock <KEY_FILE>")
	}
	return unlockMain(dir, filtered[0])
}

// unlockMain performs the original unlock behavior for the main worktree.
func unlockMain(dir, keyFilePath string) error {
	// Check if already unlocked: files must be decrypted AND config+key in place.
	state, err := gitutil.DetectEncryptionState(dir)
	if err != nil {
		return fmt.Errorf("detecting encryption state: %w", err)
	}
	if state == gitutil.StateFullyDecrypted || state == gitutil.StateNoFiles {
		match, err := gitutil.FilterConfigMatchesGitCrypt(dir)
		if err != nil {
			return fmt.Errorf("checking filter config: %w", err)
		}
		if match {
			keyDir, err := gitutil.KeyDir(dir)
			if err != nil {
				return fmt.Errorf("determining key directory: %w", err)
			}
			if _, err := os.Stat(filepath.Join(keyDir, "default")); err == nil {
				return fmt.Errorf("this repository is already unlocked")
			}
		}
	}

	// Read and validate the key file.
	f, err := os.Open(keyFilePath) //nolint:gosec // user-provided CLI argument
	if err != nil {
		return fmt.Errorf("opening key file: %w", err)
	}
	key, err := keyfile.Read(f)
	_ = f.Close()
	if err != nil {
		return fmt.Errorf("reading key file: %w", err)
	}

	// Install key.
	keyDir, err := gitutil.KeyDir(dir)
	if err != nil {
		return fmt.Errorf("determining key directory: %w", err)
	}
	if err := os.MkdirAll(keyDir, 0700); err != nil {
		return fmt.Errorf("creating key directory: %w", err)
	}

	keyPath := filepath.Join(keyDir, "default")

	// Write to a temp file then rename for atomicity — prevents concurrent
	// smudge filter reads from seeing a truncated/partial key file.
	// Use PID in temp name to avoid collisions between concurrent processes.
	tmpPath := fmt.Sprintf("%s.tmp.%d", keyPath, os.Getpid())
	kf, err := os.OpenFile(tmpPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600) //nolint:gosec // path derived from git dir
	if err != nil {
		return fmt.Errorf("creating key file: %w", err)
	}
	writeErr := keyfile.Write(kf, key)
	closeErr := kf.Close()
	if writeErr != nil {
		_ = os.Remove(tmpPath) //nolint:gosec // cleanup
		return fmt.Errorf("writing key: %w", writeErr)
	}
	if closeErr != nil {
		_ = os.Remove(tmpPath) //nolint:gosec // cleanup
		return fmt.Errorf("closing key file: %w", closeErr)
	}
	if err := os.Rename(tmpPath, keyPath); err != nil {
		_ = os.Remove(tmpPath) //nolint:gosec // cleanup
		return fmt.Errorf("installing key file: %w", err)
	}

	// Warn if encrypten is not in $PATH.
	if err := gitutil.CheckInPath(); err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: %v\n", err)
	}

	// Set filter and diff configuration (idempotent).
	if err := gitutil.SetFilter(dir); err != nil {
		return fmt.Errorf("setting filter config: %w", err)
	}
	if err := gitutil.SetDiffTextconv(dir); err != nil {
		return fmt.Errorf("setting diff textconv config: %w", err)
	}

	// Remove per-WT override if present (from previous lockMain).
	_ = gitutil.UnsetFilterWorktree(dir)

	// Decrypt files in-place.
	// No rollback of key/filter on failure — removing shared state
	// during concurrent operations would break other worktrees.
	if err := transformDecrypt(dir); err != nil {
		return err
	}

	return nil
}

// unlockWorktree unlocks a secondary worktree. The shared key must already
// exist; we just remove the worktree filter override and re-checkout.
func unlockWorktree(dir string, args []string) error {
	// Accept optional key arg for compatibility, but it's not required.
	if len(args) > 1 {
		return fmt.Errorf("usage: encrypten unlock [KEY_FILE]")
	}

	// If a key file is provided, install it (same as main).
	if len(args) == 1 {
		return unlockMain(dir, args[0])
	}

	// Verify shared key exists.
	keyDir, err := gitutil.KeyDir(dir)
	if err != nil {
		return fmt.Errorf("determining key directory: %w", err)
	}
	keyPath := filepath.Join(keyDir, "default")
	if _, err := os.Stat(keyPath); err != nil {
		return fmt.Errorf("no shared key found — provide a key file: encrypten unlock <KEY_FILE>")
	}

	// Ensure shared filter config exists (may have been cleaned up when all WTs were locked).
	if err := gitutil.SetFilter(dir); err != nil {
		return fmt.Errorf("ensuring shared filter config: %w", err)
	}
	if err := gitutil.SetDiffTextconv(dir); err != nil {
		return fmt.Errorf("ensuring shared diff config: %w", err)
	}

	// Remove worktree filter override.
	if err := gitutil.UnsetFilterWorktree(dir); err != nil {
		return fmt.Errorf("removing worktree filter override: %w", err)
	}

	// Decrypt files in-place.
	if err := transformDecrypt(dir); err != nil {
		return err
	}

	return nil
}

// transformDecrypt decrypts tracked files in-place.
func transformDecrypt(dir string) error {
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
		if !fileformat.IsEncrypted(data) {
			continue // already decrypted
		}
		dec, err := crypto.Decrypt(data, key)
		if err != nil {
			return fmt.Errorf("decrypting %s: %w", e.Path, err)
		}
		if err := os.WriteFile(p, dec, info.Mode()); err != nil { //nolint:gosec // path from git-tracked file list
			return fmt.Errorf("writing %s: %w", e.Path, err)
		}
	}

	// Refresh the git index so the working tree appears clean after
	// in-place transformation.
	return refreshIndex(dir, entries)
}
