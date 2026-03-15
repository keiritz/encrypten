package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha1" //nolint:gosec // HMAC-SHA1 is required for git-crypt compatibility
	"crypto/subtle"
	"errors"
	"fmt"

	"github.com/keiritz/encrypten/internal/fileformat"
	"github.com/keiritz/encrypten/internal/keyfile"
)

// ErrHMACMismatch indicates that the HMAC verification failed during decryption.
var ErrHMACMismatch = errors.New("crypto: HMAC verification failed")

// Encrypt encrypts plaintext using AES-256-CTR with an HMAC-SHA1 derived nonce,
// producing output compatible with git-crypt.
func Encrypt(plaintext []byte, key *keyfile.Key) ([]byte, error) {
	nonce := generateNonce(plaintext, key.HMACKey)
	iv := buildIV(nonce)

	block, err := aes.NewCipher(key.AESKey[:])
	if err != nil {
		return nil, fmt.Errorf("crypto: %w", err)
	}

	stream := cipher.NewCTR(block, iv[:])

	out := make([]byte, fileformat.HeaderSize+fileformat.NonceSize+len(plaintext))
	copy(out, fileformat.Magic[:])
	copy(out[fileformat.HeaderSize:], nonce[:])

	stream.XORKeyStream(out[fileformat.HeaderSize+fileformat.NonceSize:], plaintext)

	return out, nil
}

// Decrypt decrypts a git-crypt encrypted file using AES-256-CTR and verifies
// the HMAC-SHA1 nonce.
func Decrypt(data []byte, key *keyfile.Key) ([]byte, error) {
	header, err := fileformat.ParseHeader(data)
	if err != nil {
		return nil, fmt.Errorf("crypto: %w", err)
	}

	iv := buildIV(header.Nonce)

	block, err := aes.NewCipher(key.AESKey[:])
	if err != nil {
		return nil, fmt.Errorf("crypto: %w", err)
	}

	stream := cipher.NewCTR(block, iv[:])

	plaintext := make([]byte, len(header.Ciphertext))
	stream.XORKeyStream(plaintext, header.Ciphertext)

	expected := generateNonce(plaintext, key.HMACKey)
	if subtle.ConstantTimeCompare(expected[:], header.Nonce[:]) != 1 {
		return nil, ErrHMACMismatch
	}

	return plaintext, nil
}

// generateNonce derives a 12-byte nonce from the plaintext using HMAC-SHA1.
func generateNonce(plaintext []byte, hmacKey [64]byte) [fileformat.NonceSize]byte {
	h := hmac.New(sha1.New, hmacKey[:])
	if _, err := h.Write(plaintext); err != nil {
		panic("crypto: hmac write failed: " + err.Error())
	}
	sum := h.Sum(nil)

	var nonce [fileformat.NonceSize]byte
	copy(nonce[:], sum[:fileformat.NonceSize])
	return nonce
}

// buildIV constructs a 16-byte AES-CTR IV from a 12-byte nonce
// by appending 4 zero bytes as the initial counter.
func buildIV(nonce [fileformat.NonceSize]byte) [aes.BlockSize]byte {
	var iv [aes.BlockSize]byte
	copy(iv[:], nonce[:])
	return iv
}
