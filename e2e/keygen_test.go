package e2e_test

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/keiritz/encrypten/internal/keyfile"
)

func TestKeygenCreatesValidKey(t *testing.T) {
	bin := buildBinary(t)
	keyPath := filepath.Join(t.TempDir(), "test.key")

	cmd := exec.Command(bin, "keygen", keyPath) //nolint:gosec // bin is built by test
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("keygen failed: %v\n%s", err, out)
	}

	// Check file exists and size is 148 bytes.
	info, err := os.Stat(keyPath)
	if err != nil {
		t.Fatalf("stat key file: %v", err)
	}
	if info.Size() != 148 {
		t.Errorf("key file size = %d, want 148", info.Size())
	}

	// Check file permissions are 0600 (Unix only; Windows doesn't support POSIX permissions).
	if runtime.GOOS != "windows" {
		perm := info.Mode().Perm()
		if perm != fs.FileMode(0600) {
			t.Errorf("key file permissions = %o, want 0600", perm)
		}
	}

	// Read back and validate key contents.
	f, err := os.Open(keyPath) //nolint:gosec // path is test-controlled
	if err != nil {
		t.Fatalf("open key file: %v", err)
	}
	defer func() { _ = f.Close() }()

	key, err := keyfile.Read(f)
	if err != nil {
		t.Fatalf("keyfile.Read: %v", err)
	}

	// AES key must be non-zero.
	allZero := true
	for _, b := range key.AESKey {
		if b != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		t.Error("AES key is all zeros")
	}

	// HMAC key must be non-zero.
	allZero = true
	for _, b := range key.HMACKey {
		if b != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		t.Error("HMAC key is all zeros")
	}
}

func TestKeygenReadableByGitCrypt(t *testing.T) {
	if _, err := exec.LookPath("git-crypt"); err != nil {
		t.Skip("git-crypt not found in PATH")
	}

	bin := buildBinary(t)
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "test.key")

	// Generate key file.
	cmd := exec.Command(bin, "keygen", keyPath) //nolint:gosec // bin is built by test
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("keygen failed: %v\n%s", err, out)
	}

	// Create a temporary git repository.
	repoDir := filepath.Join(dir, "repo")
	for _, args := range [][]string{
		{"init", repoDir},
		{"-C", repoDir, "commit", "--allow-empty", "-m", "init"},
	} {
		gitCmd := exec.Command("git", args...) //nolint:gosec // test-controlled args
		if out, err := gitCmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}

	// Initialize git-crypt in the repo.
	initCmd := exec.Command("git-crypt", "init")
	initCmd.Dir = repoDir
	if out, err := initCmd.CombinedOutput(); err != nil {
		t.Fatalf("git-crypt init failed: %v\n%s", err, out)
	}

	// Lock the repo so we can test unlock.
	lockCmd := exec.Command("git-crypt", "lock")
	lockCmd.Dir = repoDir
	if out, err := lockCmd.CombinedOutput(); err != nil {
		t.Fatalf("git-crypt lock failed: %v\n%s", err, out)
	}

	// Attempt to unlock with our generated key.
	unlockCmd := exec.Command("git-crypt", "unlock", keyPath) //nolint:gosec // test-controlled path
	unlockCmd.Dir = repoDir
	if out, err := unlockCmd.CombinedOutput(); err != nil {
		t.Fatalf("git-crypt unlock with generated key failed: %v\n%s", err, out)
	}
}
