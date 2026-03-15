package gitutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// initTestRepo creates a minimal git repo for lock tests.
func initTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, args := range [][]string{
		{"init", dir},
		{"-C", dir, "config", "user.email", "test@test.com"},
		{"-C", dir, "config", "user.name", "Test"},
		{"-C", dir, "commit", "--allow-empty", "-m", "init"},
	} {
		cmd := exec.Command("git", args...) //nolint:gosec // test args
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}
	return dir
}

func TestAcquireLockCreatesFile(t *testing.T) {
	dir := initTestRepo(t)

	lock, err := AcquireLock(dir)
	if err != nil {
		t.Fatalf("AcquireLock failed: %v", err)
	}
	defer func() { _ = lock.Release() }()

	gitDir, err := GitDir(dir)
	if err != nil {
		t.Fatalf("GitDir: %v", err)
	}

	lockPath := filepath.Join(gitDir, lockFileName)
	data, err := os.ReadFile(lockPath) //nolint:gosec // test path
	if err != nil {
		t.Fatalf("lock file not found: %v", err)
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		t.Fatalf("lock file does not contain valid PID: %q", string(data))
	}
	if pid != os.Getpid() {
		t.Errorf("lock file PID = %d, want %d", pid, os.Getpid())
	}
}

func TestReleaseLockRemovesFile(t *testing.T) {
	dir := initTestRepo(t)

	lock, err := AcquireLock(dir)
	if err != nil {
		t.Fatalf("AcquireLock failed: %v", err)
	}

	lockPath := lock.path
	if err := lock.Release(); err != nil {
		t.Fatalf("Release failed: %v", err)
	}

	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Errorf("lock file should not exist after Release, err = %v", err)
	}
}

func TestAcquireLockBlocksConcurrent(t *testing.T) {
	dir := initTestRepo(t)

	lock1, err := AcquireLock(dir)
	if err != nil {
		t.Fatalf("AcquireLock (first) failed: %v", err)
	}

	// Try to acquire from another goroutine — it should block then succeed
	// after we release.
	var wg sync.WaitGroup
	wg.Add(1)

	var lock2 *LockFile
	var lock2Err error

	go func() {
		defer wg.Done()
		lock2, lock2Err = AcquireLock(dir)
	}()

	// Give the goroutine time to start and hit the backoff.
	// Release the lock so it can acquire.
	if err := lock1.Release(); err != nil {
		t.Fatalf("Release failed: %v", err)
	}

	wg.Wait()

	if lock2Err != nil {
		t.Fatalf("AcquireLock (second) failed: %v", lock2Err)
	}
	defer func() { _ = lock2.Release() }()
}

func TestStaleLockDetection(t *testing.T) {
	dir := initTestRepo(t)

	gitDir, err := GitDir(dir)
	if err != nil {
		t.Fatalf("GitDir: %v", err)
	}

	// Create a lock file with a non-existent PID.
	lockPath := filepath.Join(gitDir, lockFileName)
	if err := os.WriteFile(lockPath, []byte("999999999\n"), 0600); err != nil {
		t.Fatalf("writing stale lock: %v", err)
	}

	// AcquireLock should detect the stale lock and succeed.
	lock, err := AcquireLock(dir)
	if err != nil {
		t.Fatalf("AcquireLock should succeed with stale lock: %v", err)
	}
	defer func() { _ = lock.Release() }()
}

func TestReleaseLockIdempotent(t *testing.T) {
	dir := initTestRepo(t)

	lock, err := AcquireLock(dir)
	if err != nil {
		t.Fatalf("AcquireLock failed: %v", err)
	}

	if err := lock.Release(); err != nil {
		t.Fatalf("Release (first) failed: %v", err)
	}

	if err := lock.Release(); err != nil {
		t.Fatalf("Release (second) should not fail: %v", err)
	}
}
