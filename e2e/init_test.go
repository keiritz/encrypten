package e2e_test

import (
	"bytes"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/keiritz/encrypten/internal/keyfile"
)

// initRepo creates a temporary git repository and returns its path.
func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	repoDir := filepath.Join(dir, "repo")
	for _, args := range [][]string{
		{"init", repoDir},
		{"-C", repoDir, "config", "user.email", "test@test.com"},
		{"-C", repoDir, "config", "user.name", "Test"},
		{"-C", repoDir, "commit", "--allow-empty", "-m", "init"},
	} {
		cmd := exec.Command("git", args...) //nolint:gosec // test-controlled args
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}
	return repoDir
}

func TestInitCreatesKey(t *testing.T) {
	bin := buildBinary(t)
	repoDir := initRepo(t)

	cmd := exec.Command(bin, "init") //nolint:gosec // bin is built by test
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("init failed: %v\n%s", err, out)
	}

	keyPath := filepath.Join(repoDir, ".git", "git-crypt", "keys", "default")
	info, err := os.Stat(keyPath)
	if err != nil {
		t.Fatalf("stat key file: %v", err)
	}
	if info.Size() != 148 {
		t.Errorf("key file size = %d, want 148", info.Size())
	}

	// Verify it is a valid key file.
	f, err := os.Open(keyPath) //nolint:gosec // test-controlled path
	if err != nil {
		t.Fatalf("open key file: %v", err)
	}
	defer func() { _ = f.Close() }()

	key, err := keyfile.Read(f)
	if err != nil {
		t.Fatalf("keyfile.Read: %v", err)
	}

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
}

func TestInitSetsFilter(t *testing.T) {
	bin := buildBinary(t)
	repoDir := initRepo(t)

	cmd := exec.Command(bin, "init") //nolint:gosec // bin is built by test
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("init failed: %v\n%s", err, out)
	}

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
		gitCmd := exec.Command("git", "config", "--get", c.key) //nolint:gosec // test-controlled args
		gitCmd.Dir = repoDir
		got, err := gitCmd.Output()
		if err != nil {
			t.Errorf("git config --get %s failed: %v", c.key, err)
			continue
		}
		gotStr := string(bytes.TrimSpace(got))
		if gotStr != c.want {
			t.Errorf("git config %s = %q, want %q", c.key, gotStr, c.want)
		}
	}
}

func TestInitIdempotent(t *testing.T) {
	bin := buildBinary(t)
	repoDir := initRepo(t)

	// First run.
	cmd1 := exec.Command(bin, "init") //nolint:gosec // bin is built by test
	cmd1.Dir = repoDir
	out, err := cmd1.CombinedOutput()
	if err != nil {
		t.Fatalf("first init failed: %v\n%s", err, out)
	}

	keyPath := filepath.Join(repoDir, ".git", "git-crypt", "keys", "default")
	key1, err := os.ReadFile(keyPath) //nolint:gosec // test-controlled path
	if err != nil {
		t.Fatalf("read key after first init: %v", err)
	}

	// Second run.
	cmd2 := exec.Command(bin, "init") //nolint:gosec // bin is built by test
	cmd2.Dir = repoDir
	out, err = cmd2.CombinedOutput()
	if err != nil {
		t.Fatalf("second init failed: %v\n%s", err, out)
	}

	key2, err := os.ReadFile(keyPath) //nolint:gosec // test-controlled path
	if err != nil {
		t.Fatalf("read key after second init: %v", err)
	}

	if !bytes.Equal(key1, key2) {
		t.Error("key changed after second init — expected idempotent behavior")
	}
}

func TestInitKeyPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permissions not supported on Windows")
	}

	bin := buildBinary(t)
	repoDir := initRepo(t)

	cmd := exec.Command(bin, "init") //nolint:gosec // bin is built by test
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("init failed: %v\n%s", err, out)
	}

	keyPath := filepath.Join(repoDir, ".git", "git-crypt", "keys", "default")
	info, err := os.Stat(keyPath)
	if err != nil {
		t.Fatalf("stat key file: %v", err)
	}

	perm := info.Mode().Perm()
	if perm != fs.FileMode(0600) {
		t.Errorf("key file permissions = %o, want 0600", perm)
	}
}
