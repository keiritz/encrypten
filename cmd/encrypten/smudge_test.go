package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/keiritz/encrypten/internal/crypto"
	"github.com/keiritz/encrypten/internal/keyfile"
)

// setupSmudgeTest creates a temp git repo with a key file and returns cleanup func.
func setupSmudgeTest(t *testing.T, k *keyfile.Key) {
	t.Helper()

	tmp := t.TempDir()

	// Create .git/git-crypt/keys/default
	keyDir := filepath.Join(tmp, ".git", "git-crypt", "keys")
	if err := os.MkdirAll(keyDir, 0o700); err != nil {
		t.Fatal(err)
	}
	keyPath := filepath.Join(keyDir, "default")
	f, err := os.Create(keyPath) //nolint:gosec // test file
	if err != nil {
		t.Fatal(err)
	}
	if err := keyfile.Write(f, k); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	_ = f.Close()

	// git init so that git rev-parse works
	// We need a real git repo for gitutil.KeyDir to work
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(origDir)
	})

	// Initialize git repo
	cmd := exec.Command("git", "init") // #nosec G204
	cmd.Dir = tmp
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
}

// runSmudge calls cmdSmudge with the given input on stdin, returning stdout output.
func runSmudge(t *testing.T, input []byte) []byte {
	t.Helper()

	// Create pipe for stdin
	stdinR, stdinW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}

	// Create pipe for stdout
	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}

	// Replace os.Stdin/os.Stdout
	origStdin := os.Stdin
	origStdout := os.Stdout
	os.Stdin = stdinR
	os.Stdout = stdoutW
	t.Cleanup(func() {
		os.Stdin = origStdin
		os.Stdout = origStdout
	})

	// Write input and close writer
	go func() {
		_, _ = stdinW.Write(input)
		_ = stdinW.Close()
	}()

	// Run smudge
	smudgeErr := cmdSmudge(nil)

	// Close stdout writer so reader can get EOF
	_ = stdoutW.Close()

	// Read output
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(stdoutR); err != nil {
		t.Fatal(err)
	}

	if smudgeErr != nil {
		t.Fatalf("cmdSmudge: %v", smudgeErr)
	}

	return buf.Bytes()
}

func TestSmudgeFilter(t *testing.T) {
	k, err := keyfile.Generate()
	if err != nil {
		t.Fatal(err)
	}

	setupSmudgeTest(t, k)

	plaintext := []byte("Hello, encrypten!\nThis is secret data.\n")
	encrypted, err := crypto.Encrypt(plaintext, k)
	if err != nil {
		t.Fatal(err)
	}

	got := runSmudge(t, encrypted)

	if !bytes.Equal(got, plaintext) {
		t.Errorf("decrypted output mismatch\ngot:  %q\nwant: %q", got, plaintext)
	}
}

func TestSmudgeFilterPlaintext(t *testing.T) {
	k, err := keyfile.Generate()
	if err != nil {
		t.Fatal(err)
	}

	setupSmudgeTest(t, k)

	plaintext := []byte("This is not encrypted at all.\n")

	got := runSmudge(t, plaintext)

	if !bytes.Equal(got, plaintext) {
		t.Errorf("passthrough output mismatch\ngot:  %q\nwant: %q", got, plaintext)
	}
}

func TestSmudgeFilterEmpty(t *testing.T) {
	k, err := keyfile.Generate()
	if err != nil {
		t.Fatal(err)
	}

	setupSmudgeTest(t, k)

	got := runSmudge(t, nil)

	if len(got) != 0 {
		t.Errorf("expected empty output, got: %q", got)
	}
}
