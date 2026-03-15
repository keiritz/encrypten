package fileformat

import (
	"errors"
	"io"
	"os"
)

const (
	// HeaderSize is the length of the magic bytes prefix.
	HeaderSize = 10
	// NonceSize is the length of the HMAC-SHA1 derived nonce.
	NonceSize = 12
)

// Magic is the git-crypt file header: "\x00GITCRYPT\x00".
var Magic = [HeaderSize]byte{0x00, 'G', 'I', 'T', 'C', 'R', 'Y', 'P', 'T', 0x00}

var (
	// ErrNotEncrypted indicates the data does not start with the git-crypt magic.
	ErrNotEncrypted = errors.New("fileformat: not a git-crypt encrypted file")
	// ErrTooShort indicates the data is too short to contain a valid header.
	ErrTooShort = errors.New("fileformat: data too short for header")
)

// Header represents a parsed git-crypt file header.
type Header struct {
	Nonce      [NonceSize]byte
	Ciphertext []byte // slice of input (no copy)
}

// IsEncryptedFile reports whether the file at path begins with the git-crypt
// magic bytes, reading only the first HeaderSize bytes.
func IsEncryptedFile(path string) (bool, error) {
	f, err := os.Open(path) //nolint:gosec // caller-controlled path
	if err != nil {
		return false, err
	}
	defer f.Close() //nolint:errcheck // read-only file
	var buf [HeaderSize]byte
	_, err = io.ReadFull(f, buf[:])
	if err != nil {
		return false, nil // short file = not encrypted
	}
	return IsEncrypted(buf[:]), nil
}

// IsEncrypted reports whether data begins with the git-crypt magic bytes.
func IsEncrypted(data []byte) bool {
	if len(data) < HeaderSize {
		return false
	}
	for i := range HeaderSize {
		if data[i] != Magic[i] {
			return false
		}
	}
	return true
}

// ParseHeader parses a git-crypt encrypted file, returning the nonce and
// a ciphertext slice referencing the original data (no copy).
func ParseHeader(data []byte) (*Header, error) {
	if len(data) < HeaderSize+NonceSize {
		if len(data) >= HeaderSize && IsEncrypted(data) {
			return nil, ErrTooShort
		}
		if len(data) < HeaderSize {
			return nil, ErrTooShort
		}
		return nil, ErrNotEncrypted
	}
	if !IsEncrypted(data) {
		return nil, ErrNotEncrypted
	}
	h := &Header{
		Ciphertext: data[HeaderSize+NonceSize:],
	}
	copy(h.Nonce[:], data[HeaderSize:HeaderSize+NonceSize])
	return h, nil
}
