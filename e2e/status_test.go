package e2e_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func setupStatusRepo(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()

	runGit(t, tmp, "init")
	runGit(t, tmp, "config", "user.email", "test@test.com")
	runGit(t, tmp, "config", "user.name", "Test")

	return tmp
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...) //nolint:gosec // test helper
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}

func TestStatusShowsEncrypted(t *testing.T) {
	bin := buildBinary(t)
	repo := setupStatusRepo(t)

	// Write .gitattributes with filter=git-crypt
	if err := os.WriteFile( //nolint:gosec // test setup
		filepath.Join(repo, ".gitattributes"),
		[]byte("secret.txt filter=git-crypt diff=git-crypt\n"),
		0644,
	); err != nil {
		t.Fatal(err)
	}

	// Copy encrypted fixture as secret.txt
	fixture, err := os.ReadFile(filepath.Join("..", "testdata", "fixtures", "encrypted.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "secret.txt"), fixture, 0644); err != nil { //nolint:gosec // test setup
		t.Fatal(err)
	}

	// Commit so git ls-files can find them
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "init")

	cmd := exec.Command(bin, "status") //nolint:gosec // test binary
	cmd.Dir = repo
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("status failed: %v\n%s", err, out)
	}

	want := "    encrypted: secret.txt"
	if !strings.Contains(string(out), want) {
		t.Errorf("output %q does not contain %q", string(out), want)
	}
}

func TestStatusShowsDecrypted(t *testing.T) {
	bin := buildBinary(t)
	repo := setupStatusRepo(t)

	// Write .gitattributes with filter=git-crypt
	if err := os.WriteFile( //nolint:gosec // test setup
		filepath.Join(repo, ".gitattributes"),
		[]byte("secret.txt filter=git-crypt diff=git-crypt\n"),
		0644,
	); err != nil {
		t.Fatal(err)
	}

	// Copy plain fixture as secret.txt (simulating decrypted state)
	fixture, err := os.ReadFile(filepath.Join("..", "testdata", "fixtures", "plain.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "secret.txt"), fixture, 0644); err != nil { //nolint:gosec // test setup
		t.Fatal(err)
	}

	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "init")

	cmd := exec.Command(bin, "status") //nolint:gosec // test binary
	cmd.Dir = repo
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("status failed: %v\n%s", err, out)
	}

	want := "not encrypted: secret.txt"
	if !strings.Contains(string(out), want) {
		t.Errorf("output %q does not contain %q", string(out), want)
	}
}

func TestStatusNoEncryptedFiles(t *testing.T) {
	bin := buildBinary(t)
	repo := setupStatusRepo(t)

	// No .gitattributes — commit a plain file so the repo has at least one commit
	if err := os.WriteFile(filepath.Join(repo, "readme.txt"), []byte("hello\n"), 0644); err != nil { //nolint:gosec // test setup
		t.Fatal(err)
	}

	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "init")

	cmd := exec.Command(bin, "status") //nolint:gosec // test binary
	cmd.Dir = repo
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("status failed: %v\n%s", err, out)
	}

	if len(strings.TrimSpace(string(out))) != 0 {
		t.Errorf("expected empty output, got %q", string(out))
	}
}
