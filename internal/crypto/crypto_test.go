package crypto_test

import (
	"bytes"
	"crypto/rand"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/keiritz/encrypten/internal/crypto"
	"github.com/keiritz/encrypten/internal/keyfile"
)

const fixtureDir = "../../testdata/fixtures"

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(fixtureDir, name)) //nolint:gosec // test fixture path is not user input
	if err != nil {
		t.Fatalf("failed to read fixture %s: %v", name, err)
	}
	return data
}

func loadKey(t *testing.T) *keyfile.Key {
	t.Helper()
	f, err := os.Open(filepath.Join(fixtureDir, "key_default"))
	if err != nil {
		t.Fatalf("failed to open key file: %v", err)
	}
	defer func() { _ = f.Close() }()

	key, err := keyfile.Read(f)
	if err != nil {
		t.Fatalf("failed to read key: %v", err)
	}
	return key
}

func TestDecryptGitCryptFile(t *testing.T) {
	key := loadKey(t)
	encrypted := readFixture(t, "encrypted.bin")
	expected := readFixture(t, "plain.txt")

	plaintext, err := crypto.Decrypt(encrypted, key)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}
	if !bytes.Equal(plaintext, expected) {
		t.Errorf("decrypted content mismatch\ngot:  %q\nwant: %q", plaintext, expected)
	}
}

func TestEncryptMatchesGitCrypt(t *testing.T) {
	key := loadKey(t)
	plaintext := readFixture(t, "plain.txt")
	expected := readFixture(t, "encrypted.bin")

	ciphertext, err := crypto.Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}
	if !bytes.Equal(ciphertext, expected) {
		t.Errorf("encrypted output does not match git-crypt output\ngot len=%d, want len=%d", len(ciphertext), len(expected))
	}
}

func TestEncryptDecryptRoundtrip(t *testing.T) {
	key, err := keyfile.Generate()
	if err != nil {
		t.Fatalf("Generate key failed: %v", err)
	}

	original := make([]byte, 1024)
	if _, err := rand.Read(original); err != nil {
		t.Fatalf("rand.Read failed: %v", err)
	}

	encrypted, err := crypto.Encrypt(original, key)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	decrypted, err := crypto.Decrypt(encrypted, key)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}

	if !bytes.Equal(decrypted, original) {
		t.Error("roundtrip failed: decrypted data does not match original")
	}
}

func TestDecryptEmptyFile(t *testing.T) {
	key := loadKey(t)
	encrypted := readFixture(t, "encrypted_empty.bin")

	plaintext, err := crypto.Decrypt(encrypted, key)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}
	if len(plaintext) != 0 {
		t.Errorf("expected empty plaintext, got %d bytes", len(plaintext))
	}
}

func TestEncryptEmptyFile(t *testing.T) {
	key := loadKey(t)
	expected := readFixture(t, "encrypted_empty.bin")

	ciphertext, err := crypto.Encrypt([]byte{}, key)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}
	if !bytes.Equal(ciphertext, expected) {
		t.Errorf("encrypted empty file does not match fixture\ngot len=%d, want len=%d", len(ciphertext), len(expected))
	}
}

func TestDecryptLargeFile(t *testing.T) {
	key := loadKey(t)
	encrypted := readFixture(t, "encrypted_large.bin")
	expected := readFixture(t, "plain_large.bin")

	plaintext, err := crypto.Decrypt(encrypted, key)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}
	if !bytes.Equal(plaintext, expected) {
		t.Errorf("decrypted large file mismatch: got %d bytes, want %d bytes", len(plaintext), len(expected))
	}
}

func TestEncryptLargeFile(t *testing.T) {
	key := loadKey(t)
	plaintext := readFixture(t, "plain_large.bin")
	expected := readFixture(t, "encrypted_large.bin")

	ciphertext, err := crypto.Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}
	if !bytes.Equal(ciphertext, expected) {
		t.Errorf("encrypted large file does not match fixture: got %d bytes, want %d bytes", len(ciphertext), len(expected))
	}
}

func TestHMACNonceGeneration(t *testing.T) {
	key := loadKey(t)
	plaintext := readFixture(t, "plain.txt")
	encrypted := readFixture(t, "encrypted.bin")

	output, err := crypto.Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	// Nonce is at bytes 10..22 (after 10-byte magic header).
	gotNonce := output[10:22]
	wantNonce := encrypted[10:22]
	if !bytes.Equal(gotNonce, wantNonce) {
		t.Errorf("HMAC nonce mismatch\ngot:  %x\nwant: %x", gotNonce, wantNonce)
	}
}

func TestCTRCounterCompatibility(t *testing.T) {
	key := loadKey(t)
	plaintext := readFixture(t, "plain.txt")
	expected := readFixture(t, "encrypted.bin")

	output, err := crypto.Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	if !bytes.Equal(output, expected) {
		t.Errorf("CTR counter compatibility failed: output does not match git-crypt encrypted file")
	}
}

func TestDecryptCorruptedFile(t *testing.T) {
	key := loadKey(t)
	encrypted := readFixture(t, "encrypted.bin")

	// Corrupt the ciphertext portion (after header + nonce).
	corrupted := make([]byte, len(encrypted))
	copy(corrupted, encrypted)
	if len(corrupted) > 22 {
		corrupted[22] ^= 0xFF
	}

	_, err := crypto.Decrypt(corrupted, key)
	if !errors.Is(err, crypto.ErrHMACMismatch) {
		t.Errorf("expected ErrHMACMismatch, got: %v", err)
	}
}

func TestDecryptWrongKey(t *testing.T) {
	wrongKey, err := keyfile.Generate()
	if err != nil {
		t.Fatalf("Generate key failed: %v", err)
	}

	encrypted := readFixture(t, "encrypted.bin")

	_, err = crypto.Decrypt(encrypted, wrongKey)
	if !errors.Is(err, crypto.ErrHMACMismatch) {
		t.Errorf("expected ErrHMACMismatch, got: %v", err)
	}
}
