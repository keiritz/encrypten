package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

const fixtureDir = "../../testdata/fixtures"

func loadFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(fixtureDir, name)) //nolint:gosec // test fixture path
	if err != nil {
		t.Fatalf("failed to read fixture %s: %v", name, err)
	}
	return data
}

func TestCleanFilter(t *testing.T) {
	keyPath := filepath.Join(fixtureDir, "key_default")
	plaintext := loadFixture(t, "plain.txt")
	expected := loadFixture(t, "encrypted.bin")

	var out bytes.Buffer
	if err := cleanFilter(bytes.NewReader(plaintext), &out, keyPath); err != nil {
		t.Fatalf("cleanFilter: %v", err)
	}

	if !bytes.Equal(out.Bytes(), expected) {
		t.Errorf("output mismatch: got %d bytes, want %d bytes", out.Len(), len(expected))
	}
}

func TestCleanFilterAlreadyEncrypted(t *testing.T) {
	keyPath := filepath.Join(fixtureDir, "key_default")
	encrypted := loadFixture(t, "encrypted.bin")

	var out bytes.Buffer
	if err := cleanFilter(bytes.NewReader(encrypted), &out, keyPath); err != nil {
		t.Fatalf("cleanFilter: %v", err)
	}

	if !bytes.Equal(out.Bytes(), encrypted) {
		t.Errorf("already-encrypted data should pass through unchanged")
	}
}

func TestCleanFilterEmpty(t *testing.T) {
	keyPath := filepath.Join(fixtureDir, "key_default")
	plaintext := loadFixture(t, "plain_empty.txt")
	expected := loadFixture(t, "encrypted_empty.bin")

	var out bytes.Buffer
	if err := cleanFilter(bytes.NewReader(plaintext), &out, keyPath); err != nil {
		t.Fatalf("cleanFilter: %v", err)
	}

	if !bytes.Equal(out.Bytes(), expected) {
		t.Errorf("output mismatch: got %d bytes, want %d bytes", out.Len(), len(expected))
	}
}
