package gitutil_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/keiritz/encrypten/internal/gitutil"
)

func TestAllWorktreesEncrypted(t *testing.T) {
	// A repo with no encrypted files should return true (no decrypted files).
	dir := initTestRepo(t)
	allEnc, err := gitutil.AllWorktreesEncrypted(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !allEnc {
		t.Error("expected true for repo with no encrypted files")
	}
}

func TestAllWorktreesEncryptedMixed(t *testing.T) {
	dir := initTestRepo(t)

	// Set up .gitattributes so secret.txt is tracked as encrypted.
	if err := os.WriteFile(
		filepath.Join(dir, ".gitattributes"),
		[]byte("secret.txt filter=git-crypt diff=git-crypt\n"),
		0600,
	); err != nil {
		t.Fatal(err)
	}

	// Create a plaintext "secret" file (not actually encrypted).
	if err := os.WriteFile(
		filepath.Join(dir, "secret.txt"),
		[]byte("plaintext content\n"),
		0600,
	); err != nil {
		t.Fatal(err)
	}

	run(t, dir, "git", "add", ".")
	run(t, dir, "git", "commit", "-m", "add secret")

	// File is plaintext → should be detected as decrypted → AllWorktreesEncrypted = false.
	allEnc, err := gitutil.AllWorktreesEncrypted(dir)
	if err != nil {
		t.Fatal(err)
	}
	if allEnc {
		t.Error("expected false when worktree has decrypted files")
	}
}
