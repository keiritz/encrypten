package gitutil_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/keiritz/encrypten/internal/gitutil"
)

// writeFile creates a file with the given content in dir.
func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestListEncryptedFiles(t *testing.T) {
	dir := initTestRepo(t)

	writeFile(t, dir, ".gitattributes", "secret.txt filter=git-crypt diff=git-crypt\n")
	writeFile(t, dir, "secret.txt", "secret data")
	writeFile(t, dir, "plain.txt", "plain data")

	run(t, dir, "git", "add", ".")
	run(t, dir, "git", "commit", "-m", "add files")

	entries, err := gitutil.ListEncryptedFiles(dir)
	if err != nil {
		t.Fatal(err)
	}

	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	if entries[0].Path != "secret.txt" {
		t.Errorf("Path = %q, want %q", entries[0].Path, "secret.txt")
	}
	if entries[0].KeyName != "default" {
		t.Errorf("KeyName = %q, want %q", entries[0].KeyName, "default")
	}
}

func TestListEncryptedFilesNested(t *testing.T) {
	dir := initTestRepo(t)

	writeFile(t, dir, "sub/.gitattributes", "nested.txt filter=git-crypt diff=git-crypt\n")
	writeFile(t, dir, "sub/nested.txt", "nested secret")
	writeFile(t, dir, "sub/plain.txt", "plain")

	run(t, dir, "git", "add", ".")
	run(t, dir, "git", "commit", "-m", "add nested files")

	entries, err := gitutil.ListEncryptedFiles(dir)
	if err != nil {
		t.Fatal(err)
	}

	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	if entries[0].Path != "sub/nested.txt" {
		t.Errorf("Path = %q, want %q", entries[0].Path, "sub/nested.txt")
	}
	if entries[0].KeyName != "default" {
		t.Errorf("KeyName = %q, want %q", entries[0].KeyName, "default")
	}
}

func TestListEncryptedFilesGlob(t *testing.T) {
	dir := initTestRepo(t)

	writeFile(t, dir, ".gitattributes", "*.key filter=git-crypt diff=git-crypt\n")
	writeFile(t, dir, "a.key", "key a")
	writeFile(t, dir, "b.key", "key b")
	writeFile(t, dir, "c.txt", "not a key")

	run(t, dir, "git", "add", ".")
	run(t, dir, "git", "commit", "-m", "add glob files")

	entries, err := gitutil.ListEncryptedFiles(dir)
	if err != nil {
		t.Fatal(err)
	}

	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}

	paths := map[string]bool{}
	for _, e := range entries {
		paths[e.Path] = true
		if e.KeyName != "default" {
			t.Errorf("KeyName for %q = %q, want %q", e.Path, e.KeyName, "default")
		}
	}
	for _, want := range []string{"a.key", "b.key"} {
		if !paths[want] {
			t.Errorf("missing expected path %q", want)
		}
	}
}

func TestListEncryptedFilesEncryptenFilter(t *testing.T) {
	dir := initTestRepo(t)

	writeFile(t, dir, ".gitattributes", "secret.txt filter=encrypten diff=encrypten\n")
	writeFile(t, dir, "secret.txt", "secret data")
	writeFile(t, dir, "plain.txt", "plain data")

	run(t, dir, "git", "add", ".")
	run(t, dir, "git", "commit", "-m", "add files")

	entries, err := gitutil.ListEncryptedFiles(dir)
	if err != nil {
		t.Fatal(err)
	}

	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	if entries[0].Path != "secret.txt" {
		t.Errorf("Path = %q, want %q", entries[0].Path, "secret.txt")
	}
	if entries[0].KeyName != "default" {
		t.Errorf("KeyName = %q, want %q", entries[0].KeyName, "default")
	}
}

func TestListEncryptedFilesEncryptenNamedKey(t *testing.T) {
	dir := initTestRepo(t)

	writeFile(t, dir, ".gitattributes", "secret.txt filter=encrypten-mykey diff=encrypten-mykey\n")
	writeFile(t, dir, "secret.txt", "named key secret")

	run(t, dir, "git", "add", ".")
	run(t, dir, "git", "commit", "-m", "add named key file")

	entries, err := gitutil.ListEncryptedFiles(dir)
	if err != nil {
		t.Fatal(err)
	}

	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	if entries[0].Path != "secret.txt" {
		t.Errorf("Path = %q, want %q", entries[0].Path, "secret.txt")
	}
	if entries[0].KeyName != "mykey" {
		t.Errorf("KeyName = %q, want %q", entries[0].KeyName, "mykey")
	}
}

func TestListEncryptedFilesNamedKey(t *testing.T) {
	dir := initTestRepo(t)

	writeFile(t, dir, ".gitattributes", "secret.txt filter=git-crypt-mykey diff=git-crypt-mykey\n")
	writeFile(t, dir, "secret.txt", "named key secret")

	run(t, dir, "git", "add", ".")
	run(t, dir, "git", "commit", "-m", "add named key file")

	entries, err := gitutil.ListEncryptedFiles(dir)
	if err != nil {
		t.Fatal(err)
	}

	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	if entries[0].Path != "secret.txt" {
		t.Errorf("Path = %q, want %q", entries[0].Path, "secret.txt")
	}
	if entries[0].KeyName != "mykey" {
		t.Errorf("KeyName = %q, want %q", entries[0].KeyName, "mykey")
	}
}
