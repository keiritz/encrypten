package e2e_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/keiritz/encrypten/internal/gitutil"
)

// setupWorktreeTest creates a repo with encrypten init, commits an encrypted
// file, and adds a secondary worktree. Returns (mainDir, wtDir, binPath).
func setupWorktreeTest(t *testing.T) (string, string, string) {
	t.Helper()
	bin := buildBinary(t)
	binDir := filepath.Dir(bin)
	mainDir := initRepo(t)

	// Run encrypten init.
	initCmd := exec.Command(bin, "init") //nolint:gosec // test binary
	initCmd.Dir = mainDir
	if out, err := initCmd.CombinedOutput(); err != nil {
		t.Fatalf("init failed: %v\n%s", err, out)
	}

	// Set up .gitattributes for encryption.
	if err := os.WriteFile(
		filepath.Join(mainDir, ".gitattributes"),
		[]byte("secret.txt filter=git-crypt diff=git-crypt\n"),
		0600,
	); err != nil {
		t.Fatal(err)
	}

	// Create a secret file.
	if err := os.WriteFile(
		filepath.Join(mainDir, "secret.txt"),
		[]byte("super secret content\n"),
		0600,
	); err != nil {
		t.Fatal(err)
	}

	// Git add and commit (clean filter encrypts the file).
	addCmd := exec.Command("git", "add", ".") //nolint:gosec // test args
	addCmd.Dir = mainDir
	addCmd.Env = envWithBinDir(binDir)
	if out, err := addCmd.CombinedOutput(); err != nil {
		t.Fatalf("git add failed: %v\n%s", err, out)
	}

	commitCmd := exec.Command("git", "commit", "-m", "add secret") //nolint:gosec // test args
	commitCmd.Dir = mainDir
	commitCmd.Env = envWithBinDir(binDir)
	if out, err := commitCmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit failed: %v\n%s", err, out)
	}

	// Create a secondary worktree.
	wtDir := filepath.Join(t.TempDir(), "worktree")
	wtCmd := exec.Command("git", "worktree", "add", wtDir, "-b", "wt-branch") //nolint:gosec // test args
	wtCmd.Dir = mainDir
	wtCmd.Env = envWithBinDir(binDir)
	if out, err := wtCmd.CombinedOutput(); err != nil {
		t.Fatalf("git worktree add failed: %v\n%s", err, out)
	}

	return mainDir, wtDir, bin
}

func TestWorktreeAutoDecrypt(t *testing.T) {
	_, wtDir, _ := setupWorktreeTest(t)

	// The worktree should have the file decrypted (smudge filter ran during checkout).
	content, err := os.ReadFile(filepath.Join(wtDir, "secret.txt")) //nolint:gosec // test path
	if err != nil {
		t.Fatalf("reading secret.txt in worktree: %v", err)
	}
	if string(content) != "super secret content\n" {
		if bytes.HasPrefix(content, []byte("\x00GITCRYPT\x00")) {
			t.Error("secret.txt in worktree is still encrypted — smudge filter did not run")
		} else {
			t.Errorf("secret.txt content = %q, want %q", string(content), "super secret content\n")
		}
	}
}

func TestWorktreeSharesKey(t *testing.T) {
	mainDir, wtDir, _ := setupWorktreeTest(t)

	mainKeyDir, err := gitutil.KeyDir(mainDir)
	if err != nil {
		t.Fatalf("KeyDir(main): %v", err)
	}
	wtKeyDir, err := gitutil.KeyDir(wtDir)
	if err != nil {
		t.Fatalf("KeyDir(worktree): %v", err)
	}

	// Resolve symlinks for reliable comparison (macOS /tmp → /private/tmp).
	mainKeyDirReal, err := filepath.EvalSymlinks(mainKeyDir)
	if err != nil {
		t.Fatalf("EvalSymlinks(main): %v", err)
	}
	wtKeyDirReal, err := filepath.EvalSymlinks(wtKeyDir)
	if err != nil {
		t.Fatalf("EvalSymlinks(worktree): %v", err)
	}

	if mainKeyDirReal != wtKeyDirReal {
		t.Errorf("key directories differ: main=%s, worktree=%s", mainKeyDirReal, wtKeyDirReal)
	}
}

func TestWorktreeCleanFilter(t *testing.T) {
	_, wtDir, bin := setupWorktreeTest(t)
	binDir := filepath.Dir(bin)

	// Create a new secret file in the worktree.
	if err := os.WriteFile(
		filepath.Join(wtDir, "secret2.txt"),
		[]byte("another secret\n"),
		0600,
	); err != nil {
		t.Fatal(err)
	}

	// Update .gitattributes to cover the new file.
	if err := os.WriteFile(
		filepath.Join(wtDir, ".gitattributes"),
		[]byte("secret.txt filter=git-crypt diff=git-crypt\nsecret2.txt filter=git-crypt diff=git-crypt\n"),
		0600,
	); err != nil {
		t.Fatal(err)
	}

	// Git add.
	addCmd := exec.Command("git", "add", "secret2.txt", ".gitattributes") //nolint:gosec // test args
	addCmd.Dir = wtDir
	addCmd.Env = envWithBinDir(binDir)
	if out, err := addCmd.CombinedOutput(); err != nil {
		t.Fatalf("git add failed: %v\n%s", err, out)
	}

	// Check that git show :secret2.txt returns encrypted content.
	showCmd := exec.Command("git", "show", ":secret2.txt") //nolint:gosec // test args
	showCmd.Dir = wtDir
	showOut, err := showCmd.Output()
	if err != nil {
		t.Fatalf("git show :secret2.txt failed: %v", err)
	}
	if !bytes.HasPrefix(showOut, []byte("\x00GITCRYPT\x00")) {
		t.Errorf("staged secret2.txt should be encrypted, got prefix %q", showOut[:min(len(showOut), 20)])
	}
}

func TestWorktreeSmudgeFilter(t *testing.T) {
	_, wtDir, _ := setupWorktreeTest(t)

	// The smudge filter should have decrypted the file during worktree creation.
	content, err := os.ReadFile(filepath.Join(wtDir, "secret.txt")) //nolint:gosec // test path
	if err != nil {
		t.Fatalf("reading secret.txt: %v", err)
	}
	if string(content) != "super secret content\n" {
		t.Errorf("smudge filter not working: secret.txt = %q", string(content))
	}
}

func TestWorktreeLockIndependent(t *testing.T) {
	mainDir, wtDir, bin := setupWorktreeTest(t)
	binDir := filepath.Dir(bin)

	// Lock the worktree.
	lockCmd := exec.Command(bin, "lock") //nolint:gosec // test binary
	lockCmd.Dir = wtDir
	lockCmd.Env = envWithBinDir(binDir)
	if out, err := lockCmd.CombinedOutput(); err != nil {
		t.Fatalf("lock in worktree failed: %v\n%s", err, out)
	}

	// Worktree secret.txt should be encrypted.
	wtContent, err := os.ReadFile(filepath.Join(wtDir, "secret.txt")) //nolint:gosec // test path
	if err != nil {
		t.Fatalf("reading worktree secret.txt: %v", err)
	}
	if !bytes.HasPrefix(wtContent, []byte("\x00GITCRYPT\x00")) {
		t.Errorf("worktree secret.txt should be encrypted after lock, got prefix %q", wtContent[:min(len(wtContent), 20)])
	}

	// Main worktree secret.txt should still be plaintext.
	mainContent, err := os.ReadFile(filepath.Join(mainDir, "secret.txt")) //nolint:gosec // test path
	if err != nil {
		t.Fatalf("reading main secret.txt: %v", err)
	}
	if string(mainContent) != "super secret content\n" {
		t.Errorf("main secret.txt should still be plaintext, got %q", string(mainContent))
	}
}

func TestWorktreeMainUnaffected(t *testing.T) {
	mainDir, wtDir, bin := setupWorktreeTest(t)
	binDir := filepath.Dir(bin)

	// Lock the worktree.
	lockCmd := exec.Command(bin, "lock") //nolint:gosec // test binary
	lockCmd.Dir = wtDir
	lockCmd.Env = envWithBinDir(binDir)
	if out, err := lockCmd.CombinedOutput(); err != nil {
		t.Fatalf("lock in worktree failed: %v\n%s", err, out)
	}

	// Main worktree filter config should still be set.
	for _, key := range []string{
		"filter.git-crypt.smudge",
		"filter.git-crypt.clean",
		"filter.git-crypt.required",
		"diff.git-crypt.textconv",
	} {
		gitCmd := exec.Command("git", "config", "--get", key) //nolint:gosec // test args
		gitCmd.Dir = mainDir
		if _, err := gitCmd.Output(); err != nil {
			t.Errorf("main worktree git config %s should exist after worktree lock", key)
		}
	}

	// Shared key should still exist.
	keyDir, err := gitutil.KeyDir(mainDir)
	if err != nil {
		t.Fatalf("KeyDir: %v", err)
	}
	keyPath := filepath.Join(keyDir, "default")
	if _, err := os.Stat(keyPath); err != nil {
		t.Errorf("shared key should still exist after worktree lock: %v", err)
	}
}

func TestWorktreeMultiple(t *testing.T) {
	mainDir, wtDir1, bin := setupWorktreeTest(t)
	binDir := filepath.Dir(bin)

	// Create a second worktree.
	wtDir2 := filepath.Join(t.TempDir(), "worktree2")
	wtCmd := exec.Command("git", "worktree", "add", wtDir2, "-b", "wt-branch2") //nolint:gosec // test args
	wtCmd.Dir = mainDir
	wtCmd.Env = envWithBinDir(binDir)
	if out, err := wtCmd.CombinedOutput(); err != nil {
		t.Fatalf("git worktree add (second) failed: %v\n%s", err, out)
	}

	// Both worktrees should have decrypted files.
	for _, d := range []string{wtDir1, wtDir2} {
		content, err := os.ReadFile(filepath.Join(d, "secret.txt")) //nolint:gosec // test path
		if err != nil {
			t.Fatalf("reading secret.txt in %s: %v", d, err)
		}
		if string(content) != "super secret content\n" {
			t.Errorf("secret.txt in %s = %q, want plaintext", d, string(content))
		}
	}

	// Lock worktree1 — worktree2 should remain decrypted.
	lockCmd := exec.Command(bin, "lock") //nolint:gosec // test binary
	lockCmd.Dir = wtDir1
	lockCmd.Env = envWithBinDir(binDir)
	if out, err := lockCmd.CombinedOutput(); err != nil {
		t.Fatalf("lock in worktree1 failed: %v\n%s", err, out)
	}

	// worktree1 should be encrypted.
	wt1Content, err := os.ReadFile(filepath.Join(wtDir1, "secret.txt")) //nolint:gosec // test path
	if err != nil {
		t.Fatalf("reading worktree1 secret.txt: %v", err)
	}
	if !bytes.HasPrefix(wt1Content, []byte("\x00GITCRYPT\x00")) {
		t.Errorf("worktree1 secret.txt should be encrypted")
	}

	// worktree2 should still be plaintext.
	wt2Content, err := os.ReadFile(filepath.Join(wtDir2, "secret.txt")) //nolint:gosec // test path
	if err != nil {
		t.Fatalf("reading worktree2 secret.txt: %v", err)
	}
	if string(wt2Content) != "super secret content\n" {
		t.Errorf("worktree2 secret.txt should still be plaintext, got %q", string(wt2Content))
	}
}

func TestWorktreeLockMainPreservesWorktree(t *testing.T) {
	mainDir, wtDir, bin := setupWorktreeTest(t)
	binDir := filepath.Dir(bin)

	// Lock main worktree.
	lockCmd := exec.Command(bin, "lock") //nolint:gosec // test binary
	lockCmd.Dir = mainDir
	lockCmd.Env = envWithBinDir(binDir)
	if out, err := lockCmd.CombinedOutput(); err != nil {
		t.Fatalf("lock main failed: %v\n%s", err, out)
	}

	// Main should be encrypted.
	mainContent, err := os.ReadFile(filepath.Join(mainDir, "secret.txt")) //nolint:gosec // test path
	if err != nil {
		t.Fatalf("reading main secret.txt: %v", err)
	}
	if !bytes.HasPrefix(mainContent, []byte("\x00GITCRYPT\x00")) {
		t.Errorf("main secret.txt should be encrypted after lock, got prefix %q", mainContent[:min(len(mainContent), 20)])
	}

	// Worktree should still be decrypted.
	wtContent, err := os.ReadFile(filepath.Join(wtDir, "secret.txt")) //nolint:gosec // test path
	if err != nil {
		t.Fatalf("reading worktree secret.txt: %v", err)
	}
	if string(wtContent) != "super secret content\n" {
		t.Errorf("worktree secret.txt should still be plaintext, got %q", string(wtContent))
	}

	// Key should still exist (worktree needs it).
	keyDir, err := gitutil.KeyDir(mainDir)
	if err != nil {
		t.Fatalf("KeyDir: %v", err)
	}
	keyPath := filepath.Join(keyDir, "default")
	if _, err := os.Stat(keyPath); err != nil {
		t.Errorf("key should still exist when worktree is unlocked: %v", err)
	}
}

func TestWorktreeAllLockedPreservesSharedState(t *testing.T) {
	mainDir, wtDir, bin := setupWorktreeTest(t)
	binDir := filepath.Dir(bin)

	// Lock the worktree first.
	lockWt := exec.Command(bin, "lock") //nolint:gosec // test binary
	lockWt.Dir = wtDir
	lockWt.Env = envWithBinDir(binDir)
	if out, err := lockWt.CombinedOutput(); err != nil {
		t.Fatalf("lock worktree failed: %v\n%s", err, out)
	}

	// Lock main.
	lockMain := exec.Command(bin, "lock") //nolint:gosec // test binary
	lockMain.Dir = mainDir
	lockMain.Env = envWithBinDir(binDir)
	if out, err := lockMain.CombinedOutput(); err != nil {
		t.Fatalf("lock main failed: %v\n%s", err, out)
	}

	// Key file and shared filter config are intentionally preserved
	// to avoid races with concurrent smudge filter invocations.
	keyDir, err := gitutil.KeyDir(mainDir)
	if err != nil {
		t.Fatalf("KeyDir: %v", err)
	}
	keyPath := filepath.Join(keyDir, "default")
	if _, err := os.Stat(keyPath); err != nil {
		t.Error("key file should be preserved after lock")
	}

	// Shared filter config should still exist (never removed).
	for _, key := range []string{
		"filter.git-crypt.smudge",
		"filter.git-crypt.clean",
		"filter.git-crypt.required",
		"diff.git-crypt.textconv",
	} {
		gitCmd := exec.Command("git", "config", "--get", key) //nolint:gosec // test args
		gitCmd.Dir = mainDir
		if _, err := gitCmd.Output(); err != nil {
			t.Errorf("git config %s should still exist after lock (shared state preserved)", key)
		}
	}
}

func TestWorktreeRemoveClean(t *testing.T) {
	mainDir, wtDir, bin := setupWorktreeTest(t)
	binDir := filepath.Dir(bin)

	// Remove the worktree.
	rmCmd := exec.Command("git", "worktree", "remove", wtDir) //nolint:gosec // test args
	rmCmd.Dir = mainDir
	rmCmd.Env = envWithBinDir(binDir)
	if out, err := rmCmd.CombinedOutput(); err != nil {
		t.Fatalf("git worktree remove failed: %v\n%s", err, out)
	}

	// Main worktree key should still exist.
	keyDir, err := gitutil.KeyDir(mainDir)
	if err != nil {
		t.Fatalf("KeyDir: %v", err)
	}
	keyPath := filepath.Join(keyDir, "default")
	if _, err := os.Stat(keyPath); err != nil {
		t.Errorf("key should still exist after worktree remove: %v", err)
	}

	// Main worktree filter config should still work.
	match, err := gitutil.FilterConfigMatchesGitCrypt(mainDir)
	if err != nil {
		t.Fatalf("FilterConfigMatchesGitCrypt: %v", err)
	}
	if !match {
		t.Error("main filter config should still be set after worktree remove")
	}

	// Main worktree file should still be decrypted.
	content, err := os.ReadFile(filepath.Join(mainDir, "secret.txt")) //nolint:gosec // test path
	if err != nil {
		t.Fatalf("reading main secret.txt: %v", err)
	}
	if string(content) != "super secret content\n" {
		t.Errorf("main secret.txt = %q, want plaintext", string(content))
	}

	// Lock should still work on main.
	lockCmd := exec.Command(bin, "lock") //nolint:gosec // test binary
	lockCmd.Dir = mainDir
	lockCmd.Env = envWithBinDir(binDir)
	if out, err := lockCmd.CombinedOutput(); err != nil {
		t.Fatalf("lock on main after worktree remove failed: %v\n%s", err, out)
	}
}
