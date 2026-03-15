package e2e_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// setupLockTest creates an unlocked repo with an encrypted file committed.
// Returns (repoDir, binPath).
func setupLockTest(t *testing.T) (string, string) {
	t.Helper()
	bin := buildBinary(t)
	binDir := filepath.Dir(bin)
	repoDir := initRepo(t)

	// Run encrypten init.
	cmd := exec.Command(bin, "init") //nolint:gosec // test binary
	cmd.Dir = repoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("init failed: %v\n%s", err, out)
	}

	// Set up .gitattributes for encryption.
	if err := os.WriteFile(
		filepath.Join(repoDir, ".gitattributes"),
		[]byte("secret.txt filter=git-crypt diff=git-crypt\n"),
		0600,
	); err != nil {
		t.Fatal(err)
	}

	// Create a secret file.
	if err := os.WriteFile(
		filepath.Join(repoDir, "secret.txt"),
		[]byte("super secret content\n"),
		0600,
	); err != nil {
		t.Fatal(err)
	}

	// Git add and commit (clean filter encrypts the file).
	addCmd := exec.Command("git", "add", ".") //nolint:gosec // test args
	addCmd.Dir = repoDir
	addCmd.Env = envWithBinDir(binDir)
	if out, err := addCmd.CombinedOutput(); err != nil {
		t.Fatalf("git add failed: %v\n%s", err, out)
	}

	commitCmd := exec.Command("git", "commit", "-m", "add secret") //nolint:gosec // test args
	commitCmd.Dir = repoDir
	commitCmd.Env = envWithBinDir(binDir)
	if out, err := commitCmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit failed: %v\n%s", err, out)
	}

	return repoDir, bin
}

func TestLockEncryptsFiles(t *testing.T) {
	repoDir, bin := setupLockTest(t)

	cmd := exec.Command(bin, "lock") //nolint:gosec // test binary
	cmd.Dir = repoDir
	cmd.Env = envWithBinDir(filepath.Dir(bin))
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("lock failed: %v\n%s", err, out)
	}

	// Verify secret.txt starts with the GITCRYPT header.
	content, err := os.ReadFile(filepath.Join(repoDir, "secret.txt")) //nolint:gosec // test path
	if err != nil {
		t.Fatalf("reading secret.txt: %v", err)
	}
	if !bytes.HasPrefix(content, []byte("\x00GITCRYPT\x00")) {
		t.Errorf("secret.txt should be encrypted (start with \\x00GITCRYPT\\x00), got prefix %q", content[:min(len(content), 20)])
	}
}

func TestLockPreservesKeyAndFilter(t *testing.T) {
	repoDir, bin := setupLockTest(t)

	cmd := exec.Command(bin, "lock") //nolint:gosec // test binary
	cmd.Dir = repoDir
	cmd.Env = envWithBinDir(filepath.Dir(bin))
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("lock failed: %v\n%s", err, out)
	}
	_ = out

	// Key file and shared filter config are preserved after lock to avoid
	// concurrent races with other worktrees' smudge filters.
	keyPath := filepath.Join(repoDir, ".git", "git-crypt", "keys", "default")
	if _, err := os.Stat(keyPath); err != nil {
		t.Error("key file should be preserved after lock")
	}

	for _, key := range []string{
		"filter.git-crypt.smudge",
		"filter.git-crypt.clean",
		"filter.git-crypt.required",
		"diff.git-crypt.textconv",
	} {
		gitCmd := exec.Command("git", "config", "--get", key) //nolint:gosec // test args
		gitCmd.Dir = repoDir
		if _, err := gitCmd.Output(); err != nil {
			t.Errorf("git config %s should be preserved after lock", key)
		}
	}
}

func TestLockRejectsDirtyWorkingTree(t *testing.T) {
	repoDir, bin := setupLockTest(t)

	// Make the working tree dirty.
	if err := os.WriteFile(filepath.Join(repoDir, "secret.txt"), []byte("modified\n"), 0600); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(bin, "lock") //nolint:gosec // test binary
	cmd.Dir = repoDir
	cmd.Env = envWithBinDir(filepath.Dir(bin))
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("lock should have failed with dirty working tree")
	}
	if !strings.Contains(string(out), "not clean") {
		t.Errorf("expected 'not clean' in output, got: %s", out)
	}
}

func TestLockForceBypassesDirtyCheck(t *testing.T) {
	repoDir, bin := setupLockTest(t)

	// Make the working tree dirty with a non-encrypted file.
	if err := os.WriteFile(filepath.Join(repoDir, "README.md"), []byte("dirty\n"), 0600); err != nil {
		t.Fatal(err)
	}
	addCmd := exec.Command("git", "add", "README.md") //nolint:gosec // test args
	addCmd.Dir = repoDir
	addCmd.Env = envWithBinDir(filepath.Dir(bin))
	if out, err := addCmd.CombinedOutput(); err != nil {
		t.Fatalf("git add failed: %v\n%s", err, out)
	}

	cmd := exec.Command(bin, "lock", "--force") //nolint:gosec // test binary
	cmd.Dir = repoDir
	cmd.Env = envWithBinDir(filepath.Dir(bin))
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("lock --force should succeed with dirty working tree: %v\n%s", err, out)
	}

	// Verify secret.txt is encrypted.
	content, err := os.ReadFile(filepath.Join(repoDir, "secret.txt")) //nolint:gosec // test path
	if err != nil {
		t.Fatalf("reading secret.txt: %v", err)
	}
	if !bytes.HasPrefix(content, []byte("\x00GITCRYPT\x00")) {
		t.Error("secret.txt should be encrypted after lock --force")
	}
}

func TestLockAlreadyLocked(t *testing.T) {
	repoDir, bin := setupLockTest(t)

	// First lock.
	cmd := exec.Command(bin, "lock") //nolint:gosec // test binary
	cmd.Dir = repoDir
	cmd.Env = envWithBinDir(filepath.Dir(bin))
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("first lock failed: %v\n%s", err, out)
	}

	// Second lock should fail.
	cmd2 := exec.Command(bin, "lock") //nolint:gosec // test binary
	cmd2.Dir = repoDir
	out2, err := cmd2.CombinedOutput()
	if err == nil {
		t.Fatal("second lock should have failed for already-locked repo")
	}

	if !strings.Contains(string(out2), "already locked") {
		t.Errorf("expected 'already locked' in output, got: %s", out2)
	}
}
