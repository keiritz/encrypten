package e2e_test

import (
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestConcurrentLockSameWorktree(t *testing.T) {
	_, _, bin := setupWorktreeTest(t)
	binDir := filepath.Dir(bin)

	// Set up a second worktree that is unlocked and create two concurrent lock attempts.
	mainDir, wtDir, bin2 := setupWorktreeTest(t)
	_ = mainDir
	_ = bin2

	var wg sync.WaitGroup
	wg.Add(2)

	results := make([]error, 2)
	outputs := make([]string, 2)

	for i := range 2 {
		go func(idx int) {
			defer wg.Done()
			cmd := exec.Command(bin, "lock") //nolint:gosec // test binary
			cmd.Dir = wtDir
			cmd.Env = envWithBinDir(binDir)
			out, err := cmd.CombinedOutput()
			results[idx] = err
			outputs[idx] = string(out)
		}(i)
	}

	wg.Wait()

	successCount := 0
	inProgressCount := 0
	for i, err := range results {
		if err == nil {
			successCount++
		} else if strings.Contains(outputs[i], "another encrypten operation is in progress") ||
			strings.Contains(outputs[i], "already locked") {
			inProgressCount++
		}
	}

	// At least one should succeed (or report already locked); the other may fail with "in progress".
	if successCount == 0 && inProgressCount == 0 {
		t.Errorf("expected at least one success or known error, got: [%v] [%v], outputs: [%s] [%s]",
			results[0], results[1], outputs[0], outputs[1])
	}
}

func TestConcurrentLockDifferentWorktrees(t *testing.T) {
	mainDir, wtDir, bin := setupWorktreeTest(t)
	binDir := filepath.Dir(bin)

	// Create a second worktree.
	wtDir2 := filepath.Join(t.TempDir(), "worktree2")
	wtCmd := exec.Command("git", "worktree", "add", wtDir2, "-b", "wt-branch2") //nolint:gosec // test args
	wtCmd.Dir = mainDir
	wtCmd.Env = envWithBinDir(binDir)
	if out, err := wtCmd.CombinedOutput(); err != nil {
		t.Fatalf("git worktree add (second) failed: %v\n%s", err, out)
	}

	// Lock both worktrees concurrently — should both succeed since they have different $GIT_DIR.
	var wg sync.WaitGroup
	wg.Add(2)

	dirs := []string{wtDir, wtDir2}
	results := make([]error, 2)
	outputs := make([]string, 2)

	for i, d := range dirs {
		go func(idx int, dir string) {
			defer wg.Done()
			cmd := exec.Command(bin, "lock") //nolint:gosec // test binary
			cmd.Dir = dir
			cmd.Env = envWithBinDir(binDir)
			out, err := cmd.CombinedOutput()
			results[idx] = err
			outputs[idx] = string(out)
		}(i, d)
	}

	wg.Wait()

	for i, err := range results {
		if err != nil {
			t.Errorf("lock in worktree %d (%s) failed: %v\n%s", i, dirs[i], err, outputs[i])
		}
	}
}
