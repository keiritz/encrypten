package gitutil_test

import (
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/keiritz/encrypten/internal/gitutil"
)

// run executes a command in dir, failing the test on error.
func run(t *testing.T, dir, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...) //nolint:gosec // test helper with controlled inputs
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v failed: %v\n%s", name, args, err, out)
	}
}

// initTestRepo creates a temporary git repo with an empty commit and returns
// its symlink-resolved path.
func initTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	// Resolve symlinks (macOS /tmp → /private/tmp).
	dir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	run(t, dir, "git", "init")
	run(t, dir, "git", "config", "user.email", "test@test.com")
	run(t, dir, "git", "config", "user.name", "Test")
	run(t, dir, "git", "commit", "--allow-empty", "-m", "init")
	return dir
}

func TestRepoRoot(t *testing.T) {
	dir := initTestRepo(t)

	root, err := gitutil.RepoRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	if root != dir {
		t.Errorf("RepoRoot = %q, want %q", root, dir)
	}
}

func TestGitCommonDir(t *testing.T) {
	dir := initTestRepo(t)

	got, err := gitutil.GitCommonDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, ".git")
	if got != want {
		t.Errorf("GitCommonDir = %q, want %q", got, want)
	}
}

func TestKeyDir(t *testing.T) {
	dir := initTestRepo(t)

	got, err := gitutil.KeyDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, ".git", "git-crypt", "keys")
	if got != want {
		t.Errorf("KeyDir = %q, want %q", got, want)
	}
}

func TestListWorktreesSingleRepo(t *testing.T) {
	dir := initTestRepo(t)

	wts, err := gitutil.ListWorktrees(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(wts) != 1 {
		t.Fatalf("expected 1 worktree, got %d", len(wts))
	}
	if wts[0].Path != dir {
		t.Errorf("worktree path = %q, want %q", wts[0].Path, dir)
	}
}

func TestListWorktrees(t *testing.T) {
	main := initTestRepo(t)
	wt := filepath.Join(t.TempDir(), "wt")

	// Resolve parent symlinks for comparison.
	wtParent, err := filepath.EvalSymlinks(filepath.Dir(wt))
	if err != nil {
		t.Fatal(err)
	}
	wt = filepath.Join(wtParent, filepath.Base(wt))

	run(t, main, "git", "worktree", "add", wt, "-b", "test-wt")

	wts, err := gitutil.ListWorktrees(main)
	if err != nil {
		t.Fatal(err)
	}
	if len(wts) != 2 {
		t.Fatalf("expected 2 worktrees, got %d", len(wts))
	}

	paths := map[string]bool{}
	for _, w := range wts {
		paths[w.Path] = true
	}
	if !paths[main] {
		t.Errorf("main worktree %q not found in list", main)
	}
	if !paths[wt] {
		t.Errorf("secondary worktree %q not found in list", wt)
	}
}

func TestKeyDirWorktree(t *testing.T) {
	main := initTestRepo(t)
	wt := filepath.Join(t.TempDir(), "wt")

	// Resolve parent symlinks for the worktree path comparison.
	wtParent, err := filepath.EvalSymlinks(filepath.Dir(wt))
	if err != nil {
		t.Fatal(err)
	}
	wt = filepath.Join(wtParent, filepath.Base(wt))

	run(t, main, "git", "worktree", "add", wt, "-b", "test-wt")

	mainKey, err := gitutil.KeyDir(main)
	if err != nil {
		t.Fatal(err)
	}
	wtKey, err := gitutil.KeyDir(wt)
	if err != nil {
		t.Fatal(err)
	}
	if wtKey != mainKey {
		t.Errorf("KeyDir(worktree) = %q, want %q (same as main)", wtKey, mainKey)
	}
}
