package e2e_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// envWithBinDir returns a copy of os.Environ() with binDir prepended to PATH,
// replacing any existing PATH entry to avoid duplicate keys.
func envWithBinDir(binDir string) []string {
	newPath := binDir + string(os.PathListSeparator) + os.Getenv("PATH")
	env := os.Environ()
	result := make([]string, 0, len(env))
	for _, e := range env {
		if strings.HasPrefix(e, "PATH=") {
			continue
		}
		result = append(result, e)
	}
	return append(result, "PATH="+newPath)
}

// setupUnlockTest creates a repo with encrypten init, commits an encrypted file,
// exports the key, then removes the key and filter to simulate a locked state.
// Returns (repoDir, exportedKeyPath, binPath).
func setupUnlockTest(t *testing.T) (string, string, string) {
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

	// Export the key.
	exportedKey := filepath.Join(t.TempDir(), "exported_key")
	exportCmd := exec.Command(bin, "export-key", exportedKey) //nolint:gosec // test binary
	exportCmd.Dir = repoDir
	if out, err := exportCmd.CombinedOutput(); err != nil {
		t.Fatalf("export-key failed: %v\n%s", err, out)
	}

	// Remove key and unset filter to simulate locked state.
	keyPath := filepath.Join(repoDir, ".git", "git-crypt", "keys", "default")
	_ = os.Remove(keyPath)
	_ = os.RemoveAll(filepath.Join(repoDir, ".git", "git-crypt"))

	// Unset filter config — remove both git-crypt and encrypten sections
	// since encrypten init registers both filter names.
	for _, section := range []string{"filter.git-crypt", "diff.git-crypt", "filter.encrypten", "diff.encrypten"} {
		unsetCmd := exec.Command("git", "config", "--remove-section", section) //nolint:gosec // test args
		unsetCmd.Dir = repoDir
		_ = unsetCmd.Run() // ignore if section doesn't exist
	}

	// Remove the file before checkout to avoid racy-git: if the commit
	// and checkout happen within the same filesystem timestamp granularity,
	// git may consider the working-tree file "clean" based on cached stat
	// info and skip overwriting it, leaving plaintext instead of the
	// encrypted blob content.
	_ = os.Remove(filepath.Join(repoDir, "secret.txt"))

	// Force checkout without filter to get raw encrypted content in working tree.
	checkoutCmd := exec.Command("git", "checkout", "-f") //nolint:gosec // test args
	checkoutCmd.Dir = repoDir
	if out, err := checkoutCmd.CombinedOutput(); err != nil {
		t.Fatalf("git checkout -f (lock simulation) failed: %v\n%s", err, out)
	}

	return repoDir, exportedKey, bin
}

func TestUnlockWithGitCryptKey(t *testing.T) {
	repoDir, keyPath, bin := setupUnlockTest(t)
	binDir := filepath.Dir(bin)

	cmd := exec.Command(bin, "unlock", keyPath) //nolint:gosec // test binary
	cmd.Dir = repoDir
	cmd.Env = envWithBinDir(binDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("unlock failed: %v\n%s", err, out)
	}

	// Verify key file was installed.
	installedKey := filepath.Join(repoDir, ".git", "git-crypt", "keys", "default")
	if _, err := os.Stat(installedKey); err != nil {
		t.Fatalf("key file not installed: %v", err)
	}
}

func TestUnlockDecryptsFiles(t *testing.T) {
	repoDir, keyPath, bin := setupUnlockTest(t)
	binDir := filepath.Dir(bin)

	cmd := exec.Command(bin, "unlock", keyPath) //nolint:gosec // test binary
	cmd.Dir = repoDir
	cmd.Env = envWithBinDir(binDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("unlock failed: %v\n%s", err, out)
	}

	// Verify the file is decrypted (plaintext).
	content, err := os.ReadFile(filepath.Join(repoDir, "secret.txt")) //nolint:gosec // test path
	if err != nil {
		t.Fatalf("reading secret.txt: %v", err)
	}
	_ = out
	if string(content) != "super secret content\n" {
		// Check it's not still encrypted (starts with \x00GITCRYPT\x00).
		if bytes.HasPrefix(content, []byte("\x00GITCRYPT\x00")) {
			t.Error("secret.txt is still encrypted after unlock")
		} else {
			t.Errorf("secret.txt content = %q, want %q", string(content), "super secret content\n")
		}
	}
}

func TestUnlockSetsFilter(t *testing.T) {
	repoDir, keyPath, bin := setupUnlockTest(t)
	binDir := filepath.Dir(bin)

	cmd := exec.Command(bin, "unlock", keyPath) //nolint:gosec // test binary
	cmd.Dir = repoDir
	cmd.Env = envWithBinDir(binDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("unlock failed: %v\n%s", err, out)
	}
	_ = out

	checks := []struct {
		key  string
		want string
	}{
		{"filter.git-crypt.smudge", "encrypten smudge"},
		{"filter.git-crypt.clean", "encrypten clean"},
		{"filter.git-crypt.required", "true"},
		{"diff.git-crypt.textconv", "encrypten diff"},
	}

	for _, c := range checks {
		gitCmd := exec.Command("git", "config", "--get", c.key) //nolint:gosec // test args
		gitCmd.Dir = repoDir
		got, err := gitCmd.Output()
		if err != nil {
			t.Errorf("git config --get %s failed: %v", c.key, err)
			continue
		}
		gotStr := strings.TrimSpace(string(got))
		if gotStr != c.want {
			t.Errorf("git config %s = %q, want %q", c.key, gotStr, c.want)
		}
	}
}

func TestUnlockRejectsDirtyWorkingTree(t *testing.T) {
	repoDir, keyPath, bin := setupUnlockTest(t)

	// Make the working tree dirty.
	if err := os.WriteFile(filepath.Join(repoDir, "secret.txt"), []byte("modified\n"), 0600); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(bin, "unlock", keyPath) //nolint:gosec // test binary
	cmd.Dir = repoDir
	cmd.Env = envWithBinDir(filepath.Dir(bin))
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("unlock should have failed with dirty working tree")
	}
	if !strings.Contains(string(out), "not clean") {
		t.Errorf("expected 'not clean' in output, got: %s", out)
	}
}

func TestUnlockForceBypassesDirtyCheck(t *testing.T) {
	repoDir, keyPath, bin := setupUnlockTest(t)
	binDir := filepath.Dir(bin)

	// Make the working tree dirty with a non-encrypted file.
	if err := os.WriteFile(filepath.Join(repoDir, "README.md"), []byte("dirty\n"), 0600); err != nil {
		t.Fatal(err)
	}
	addCmd := exec.Command("git", "add", "README.md") //nolint:gosec // test args
	addCmd.Dir = repoDir
	addCmd.Env = envWithBinDir(binDir)
	if out, err := addCmd.CombinedOutput(); err != nil {
		t.Fatalf("git add failed: %v\n%s", err, out)
	}

	cmd := exec.Command(bin, "unlock", "--force", keyPath) //nolint:gosec // test binary
	cmd.Dir = repoDir
	cmd.Env = envWithBinDir(binDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("unlock --force should succeed with dirty working tree: %v\n%s", err, out)
	}

	// Verify the file is decrypted.
	content, err := os.ReadFile(filepath.Join(repoDir, "secret.txt")) //nolint:gosec // test path
	if err != nil {
		t.Fatalf("reading secret.txt: %v", err)
	}
	if string(content) != "super secret content\n" {
		t.Errorf("secret.txt content = %q, want %q", string(content), "super secret content\n")
	}
}

func TestUnlockAlreadyUnlocked(t *testing.T) {
	bin := buildBinary(t)
	repoDir := initRepo(t)

	// Run encrypten init (sets up key + filter = unlocked state).
	cmd := exec.Command(bin, "init") //nolint:gosec // test binary
	cmd.Dir = repoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("init failed: %v\n%s", err, out)
	}

	// Export key to use as argument.
	exportedKey := filepath.Join(t.TempDir(), "exported_key")
	exportCmd := exec.Command(bin, "export-key", exportedKey) //nolint:gosec // test binary
	exportCmd.Dir = repoDir
	if out, err := exportCmd.CombinedOutput(); err != nil {
		t.Fatalf("export-key failed: %v\n%s", err, out)
	}

	// Try to unlock — should fail because already unlocked.
	unlockCmd := exec.Command(bin, "unlock", exportedKey) //nolint:gosec // test binary
	unlockCmd.Dir = repoDir
	out, err := unlockCmd.CombinedOutput()
	if err == nil {
		t.Fatal("unlock should have failed for already-unlocked repo")
	}

	if !strings.Contains(string(out), "already unlocked") {
		t.Errorf("expected 'already unlocked' in output, got: %s", out)
	}
}

func TestUnlockWrongKey(t *testing.T) {
	repoDir, _, bin := setupUnlockTest(t)
	binDir := filepath.Dir(bin)

	// Create a different key by running init in a separate repo and exporting.
	otherRepo := initRepo(t)
	initCmd := exec.Command(bin, "init") //nolint:gosec // test binary
	initCmd.Dir = otherRepo
	if out, err := initCmd.CombinedOutput(); err != nil {
		t.Fatalf("init other repo failed: %v\n%s", err, out)
	}

	wrongKey := filepath.Join(t.TempDir(), "wrong_key")
	exportCmd := exec.Command(bin, "export-key", wrongKey) //nolint:gosec // test binary
	exportCmd.Dir = otherRepo
	if out, err := exportCmd.CombinedOutput(); err != nil {
		t.Fatalf("export-key from other repo failed: %v\n%s", err, out)
	}

	// Unlock with the wrong key — transformDecrypt should fail with HMAC mismatch.
	unlockCmd := exec.Command(bin, "unlock", wrongKey) //nolint:gosec // test binary
	unlockCmd.Dir = repoDir
	unlockCmd.Env = envWithBinDir(binDir)
	out, err := unlockCmd.CombinedOutput()

	if err != nil {
		// Expected: HMAC validation rejects the wrong key.
		return
	}

	// If unlock succeeded, the file should not be correctly decrypted.
	content, err := os.ReadFile(filepath.Join(repoDir, "secret.txt")) //nolint:gosec // test path
	if err != nil {
		t.Fatalf("reading secret.txt: %v", err)
	}
	_ = out
	if string(content) == "super secret content\n" {
		t.Error("file was correctly decrypted with wrong key — this should not happen")
	}
}
