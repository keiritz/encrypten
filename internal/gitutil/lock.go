package gitutil

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// LockFile represents an advisory lock for a worktree.
type LockFile struct {
	path string
}

const (
	lockFileName    = "encrypten.lock"
	maxLockAttempts = 10
	baseLockDelay   = 50 * time.Millisecond
)

// AcquireLock acquires an advisory lock for the worktree at dir.
// Each worktree has its own $GIT_DIR, so locks in different worktrees
// do not interfere with each other.
// It retries GitDir resolution to tolerate transient git index locks
// from concurrent operations.
func AcquireLock(dir string) (*LockFile, error) {
	var gitDir string
	var err error
	for i := range 5 {
		gitDir, err = GitDir(dir)
		if err == nil {
			break
		}
		if i < 4 {
			time.Sleep(baseLockDelay * (1 << i))
		}
	}
	if err != nil {
		return nil, fmt.Errorf("determining git dir: %w", err)
	}

	lockPath := filepath.Join(gitDir, lockFileName)

	for attempt := range maxLockAttempts {
		f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600) //nolint:gosec // lock file in git dir
		if err == nil {
			// Write PID for stale detection.
			_, _ = fmt.Fprintf(f, "%d\n", os.Getpid())
			_ = f.Close()
			return &LockFile{path: lockPath}, nil
		}

		if !os.IsExist(err) {
			return nil, fmt.Errorf("creating lock file: %w", err)
		}

		// Lock file exists — check if stale.
		if checkStaleLock(lockPath) {
			_ = os.Remove(lockPath)
			continue
		}

		// Not stale — exponential backoff.
		if attempt < maxLockAttempts-1 {
			time.Sleep(baseLockDelay * (1 << attempt))
		}
	}

	return nil, fmt.Errorf("another encrypten operation is in progress on this worktree")
}

// Release removes the lock file. It is idempotent — calling Release
// on an already-released lock returns nil.
func (l *LockFile) Release() error {
	if err := os.Remove(l.path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// checkStaleLock returns true if the lock file at path is stale
// (the process that created it is no longer running).
func checkStaleLock(path string) bool {
	data, err := os.ReadFile(path) //nolint:gosec // lock file in git dir
	if err != nil {
		return true // cannot read → treat as stale
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return true // invalid PID → treat as stale
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		return true
	}

	// Signal 0 checks if the process exists without sending a signal.
	if err := proc.Signal(syscall.Signal(0)); err != nil {
		return true // process not running → stale
	}

	return false
}
