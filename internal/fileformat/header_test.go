package fileformat_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/keiritz/encrypten/internal/fileformat"
)

const fixtureDir = "../../testdata/fixtures"

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(fixtureDir, name)) //nolint:gosec // test fixture path
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return data
}

func TestIsEncrypted(t *testing.T) {
	for _, name := range []string{"encrypted.bin", "encrypted_empty.bin", "encrypted_large.bin"} {
		t.Run(name, func(t *testing.T) {
			data := readFixture(t, name)
			if !fileformat.IsEncrypted(data) {
				t.Errorf("IsEncrypted(%s) = false, want true", name)
			}
		})
	}
}

func TestIsNotEncrypted(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{"plain.txt", readFixture(t, "plain.txt")},
		{"plain_empty.txt", readFixture(t, "plain_empty.txt")},
		{"nil", nil},
		{"empty", []byte{}},
		{"short", []byte{0x00, 'G', 'I'}},
		{"partial_magic", []byte{0x00, 'G', 'I', 'T', 'C', 'R', 'Y', 'P', 'T', 0x01}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if fileformat.IsEncrypted(tt.data) {
				t.Errorf("IsEncrypted(%s) = true, want false", tt.name)
			}
		})
	}
}

func TestParseHeader(t *testing.T) {
	t.Run("encrypted.bin", func(t *testing.T) {
		data := readFixture(t, "encrypted.bin")
		h, err := fileformat.ParseHeader(data)
		if err != nil {
			t.Fatalf("ParseHeader: %v", err)
		}
		if len(h.Nonce) != fileformat.NonceSize {
			t.Errorf("nonce length = %d, want %d", len(h.Nonce), fileformat.NonceSize)
		}
		if len(h.Ciphertext) != 18 {
			t.Errorf("ciphertext length = %d, want 18", len(h.Ciphertext))
		}
	})

	t.Run("encrypted_empty.bin", func(t *testing.T) {
		data := readFixture(t, "encrypted_empty.bin")
		h, err := fileformat.ParseHeader(data)
		if err != nil {
			t.Fatalf("ParseHeader: %v", err)
		}
		if len(h.Ciphertext) != 0 {
			t.Errorf("ciphertext length = %d, want 0", len(h.Ciphertext))
		}
	})

	t.Run("nil_data", func(t *testing.T) {
		_, err := fileformat.ParseHeader(nil)
		if err != fileformat.ErrTooShort {
			t.Errorf("ParseHeader(nil) error = %v, want ErrTooShort", err)
		}
	})

	t.Run("plaintext", func(t *testing.T) {
		data := readFixture(t, "plain.txt")
		_, err := fileformat.ParseHeader(data)
		if err != fileformat.ErrNotEncrypted {
			t.Errorf("ParseHeader(plain) error = %v, want ErrNotEncrypted", err)
		}
	})

	t.Run("truncated_after_magic", func(t *testing.T) {
		// 10 bytes magic + only 5 bytes of nonce (need 12)
		data := make([]byte, fileformat.HeaderSize+5)
		copy(data, fileformat.Magic[:])
		_, err := fileformat.ParseHeader(data)
		if err != fileformat.ErrTooShort {
			t.Errorf("ParseHeader(truncated) error = %v, want ErrTooShort", err)
		}
	})

	t.Run("ciphertext_is_slice_of_input", func(t *testing.T) {
		data := readFixture(t, "encrypted.bin")
		h, err := fileformat.ParseHeader(data)
		if err != nil {
			t.Fatalf("ParseHeader: %v", err)
		}
		// Verify ciphertext references the original backing array (no copy).
		if &h.Ciphertext[0] != &data[fileformat.HeaderSize+fileformat.NonceSize] {
			t.Error("ciphertext is not a slice of input data")
		}
	})
}

func TestHeaderSize(t *testing.T) {
	if fileformat.HeaderSize != 10 {
		t.Errorf("HeaderSize = %d, want 10", fileformat.HeaderSize)
	}
}
