package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/keiritz/encrypten/internal/crypto"
	"github.com/keiritz/encrypten/internal/keyfile"
)

// runDiff calls cmdDiff with the given args, capturing stdout.
func runDiff(t *testing.T, args []string) []byte {
	t.Helper()

	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}

	origStdout := os.Stdout
	os.Stdout = stdoutW
	t.Cleanup(func() {
		os.Stdout = origStdout
	})

	diffErr := cmdDiff(args)

	_ = stdoutW.Close()

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(stdoutR); err != nil {
		t.Fatal(err)
	}

	if diffErr != nil {
		t.Fatalf("cmdDiff: %v", diffErr)
	}

	return buf.Bytes()
}

func TestDiffTextconv(t *testing.T) {
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

	// Write encrypted data to a temp file
	tmpFile := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(tmpFile, encrypted, 0o600); err != nil {
		t.Fatal(err)
	}

	got := runDiff(t, []string{tmpFile})

	if !bytes.Equal(got, plaintext) {
		t.Errorf("decrypted output mismatch\ngot:  %q\nwant: %q", got, plaintext)
	}
}

func TestDiffTextconvPlaintext(t *testing.T) {
	k, err := keyfile.Generate()
	if err != nil {
		t.Fatal(err)
	}

	setupSmudgeTest(t, k)

	plaintext := []byte("This is not encrypted at all.\n")

	tmpFile := filepath.Join(t.TempDir(), "plain.txt")
	if err := os.WriteFile(tmpFile, plaintext, 0o600); err != nil {
		t.Fatal(err)
	}

	got := runDiff(t, []string{tmpFile})

	if !bytes.Equal(got, plaintext) {
		t.Errorf("passthrough output mismatch\ngot:  %q\nwant: %q", got, plaintext)
	}
}

func TestDiffTextconvEmpty(t *testing.T) {
	k, err := keyfile.Generate()
	if err != nil {
		t.Fatal(err)
	}

	setupSmudgeTest(t, k)

	tmpFile := filepath.Join(t.TempDir(), "empty.txt")
	if err := os.WriteFile(tmpFile, nil, 0o600); err != nil {
		t.Fatal(err)
	}

	got := runDiff(t, []string{tmpFile})

	if len(got) != 0 {
		t.Errorf("expected empty output, got: %q", got)
	}
}
