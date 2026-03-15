package e2e_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func setupGitRepo(t *testing.T, fixturePath string) string {
	t.Helper()
	tmp := t.TempDir()

	// git init
	cmd := exec.Command("git", "init")
	cmd.Dir = tmp
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v\n%s", err, out)
	}

	// Place key fixture at .git/git-crypt/keys/default
	keyDir := filepath.Join(tmp, ".git", "git-crypt", "keys")
	if err := os.MkdirAll(keyDir, 0700); err != nil {
		t.Fatal(err)
	}

	src, err := os.ReadFile(fixturePath) //nolint:gosec // test fixture path
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(keyDir, "default"), src, 0600); err != nil { //nolint:gosec // test setup
		t.Fatal(err)
	}

	return tmp
}

func TestExportKey(t *testing.T) {
	bin := buildBinary(t)
	fixturePath := filepath.Join("..", "testdata", "fixtures", "key_default")

	repoDir := setupGitRepo(t, fixturePath)
	outputFile := filepath.Join(t.TempDir(), "exported_key")

	cmd := exec.Command(bin, "export-key", outputFile) //nolint:gosec // test binary
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("export-key failed: %v\n%s", err, out)
	}

	// Compare output with fixture
	exported, err := os.ReadFile(outputFile) //nolint:gosec // test output path
	if err != nil {
		t.Fatal(err)
	}
	fixture, err := os.ReadFile(fixturePath) //nolint:gosec // test fixture path
	if err != nil {
		t.Fatal(err)
	}

	if string(exported) != string(fixture) {
		t.Error("exported key does not match fixture")
	}
}

func TestExportKeyMatchesGitCrypt(t *testing.T) {
	bin := buildBinary(t)
	fixturePath := filepath.Join("..", "testdata", "fixtures", "key_default")

	repoDir := setupGitRepo(t, fixturePath)
	outputFile := filepath.Join(t.TempDir(), "exported_key")

	cmd := exec.Command(bin, "export-key", outputFile) //nolint:gosec // test binary
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("export-key failed: %v\n%s", err, out)
	}

	// The exported key must be byte-identical to the git-crypt generated fixture
	exported, err := os.ReadFile(outputFile) //nolint:gosec // test output path
	if err != nil {
		t.Fatal(err)
	}
	fixture, err := os.ReadFile(fixturePath) //nolint:gosec // test fixture path
	if err != nil {
		t.Fatal(err)
	}

	if len(exported) != len(fixture) {
		t.Fatalf("exported key length %d != fixture length %d", len(exported), len(fixture))
	}
	for i := range exported {
		if exported[i] != fixture[i] {
			t.Fatalf("byte mismatch at offset %d: got 0x%02x, want 0x%02x", i, exported[i], fixture[i])
		}
	}
}
