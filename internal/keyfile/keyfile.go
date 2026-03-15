package keyfile

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

var magic = [12]byte{0x00, 'G', 'I', 'T', 'C', 'R', 'Y', 'P', 'T', 'K', 'E', 'Y'}

const (
	FormatVersion = 2

	headerFieldEnd  = 0
	keyFieldEnd     = 0
	keyFieldVersion = 1
	keyFieldAESKey  = 3
	keyFieldHMACKey = 5

	aesKeyLen  = 32
	hmacKeyLen = 64
)

var (
	ErrInvalidMagic       = errors.New("keyfile: invalid magic bytes")
	ErrUnsupportedVersion = errors.New("keyfile: unsupported format version")
	ErrTruncated          = errors.New("keyfile: unexpected EOF")
)

// Key holds the symmetric keys used for git-crypt compatible encryption.
type Key struct {
	Version uint32
	AESKey  [32]byte
	HMACKey [64]byte
}

// Read parses a git-crypt key file from r.
func Read(r io.Reader) (*Key, error) {
	// Read and verify magic bytes.
	var m [12]byte
	if err := readFull(r, m[:]); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrTruncated, err)
	}
	if m != magic {
		return nil, ErrInvalidMagic
	}

	// Read and verify format version.
	var version uint32
	if err := readUint32(r, &version); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrTruncated, err)
	}
	if version != FormatVersion {
		return nil, fmt.Errorf("%w: %d", ErrUnsupportedVersion, version)
	}

	// Read header fields until END.
	if err := skipFields(r); err != nil {
		return nil, err
	}

	// Read key fields.
	k := &Key{}
	if err := readKeyFields(r, k); err != nil {
		return nil, err
	}

	return k, nil
}

// Write serializes a Key to w in git-crypt key file format.
func Write(w io.Writer, k *Key) error {
	// Magic.
	if _, err := w.Write(magic[:]); err != nil {
		return err
	}

	// Format version.
	if err := writeUint32(w, FormatVersion); err != nil {
		return err
	}

	// Header field end.
	if err := writeUint32(w, headerFieldEnd); err != nil {
		return err
	}

	// Key version field.
	if err := writeUint32(w, keyFieldVersion); err != nil {
		return err
	}
	if err := writeUint32(w, 4); err != nil {
		return err
	}
	if err := writeUint32(w, k.Version); err != nil {
		return err
	}

	// AES key field.
	if err := writeUint32(w, keyFieldAESKey); err != nil {
		return err
	}
	if err := writeUint32(w, aesKeyLen); err != nil {
		return err
	}
	if _, err := w.Write(k.AESKey[:]); err != nil {
		return err
	}

	// HMAC key field.
	if err := writeUint32(w, keyFieldHMACKey); err != nil {
		return err
	}
	if err := writeUint32(w, hmacKeyLen); err != nil {
		return err
	}
	if _, err := w.Write(k.HMACKey[:]); err != nil {
		return err
	}

	// Key field end.
	if err := writeUint32(w, keyFieldEnd); err != nil {
		return err
	}

	return nil
}

// Generate creates a new Key with cryptographically random AES and HMAC keys.
func Generate() (*Key, error) {
	k := &Key{Version: 0}
	if _, err := rand.Read(k.AESKey[:]); err != nil {
		return nil, err
	}
	if _, err := rand.Read(k.HMACKey[:]); err != nil {
		return nil, err
	}
	return k, nil
}

// readKeyFields reads TLV key fields until a KEY_FIELD_END tag.
func readKeyFields(r io.Reader, k *Key) error {
	for {
		var tag uint32
		if err := readUint32(r, &tag); err != nil {
			return fmt.Errorf("%w: %w", ErrTruncated, err)
		}

		if tag == keyFieldEnd {
			return nil
		}

		var length uint32
		if err := readUint32(r, &length); err != nil {
			return fmt.Errorf("%w: %w", ErrTruncated, err)
		}

		switch tag {
		case keyFieldVersion:
			if length != 4 {
				return fmt.Errorf("%w: unexpected version field length %d", ErrTruncated, length)
			}
			if err := readUint32(r, &k.Version); err != nil {
				return fmt.Errorf("%w: %w", ErrTruncated, err)
			}
		case keyFieldAESKey:
			if length != aesKeyLen {
				return fmt.Errorf("%w: unexpected AES key length %d", ErrTruncated, length)
			}
			if err := readFull(r, k.AESKey[:]); err != nil {
				return fmt.Errorf("%w: %w", ErrTruncated, err)
			}
		case keyFieldHMACKey:
			if length != hmacKeyLen {
				return fmt.Errorf("%w: unexpected HMAC key length %d", ErrTruncated, length)
			}
			if err := readFull(r, k.HMACKey[:]); err != nil {
				return fmt.Errorf("%w: %w", ErrTruncated, err)
			}
		default:
			// Skip unknown fields for forward compatibility.
			if err := skipBytes(r, length); err != nil {
				return fmt.Errorf("%w: %w", ErrTruncated, err)
			}
		}
	}
}

// skipFields reads and discards TLV fields until a FIELD_END tag.
func skipFields(r io.Reader) error {
	for {
		var tag uint32
		if err := readUint32(r, &tag); err != nil {
			return fmt.Errorf("%w: %w", ErrTruncated, err)
		}
		if tag == headerFieldEnd {
			return nil
		}
		var length uint32
		if err := readUint32(r, &length); err != nil {
			return fmt.Errorf("%w: %w", ErrTruncated, err)
		}
		if err := skipBytes(r, length); err != nil {
			return fmt.Errorf("%w: %w", ErrTruncated, err)
		}
	}
}

func readFull(r io.Reader, buf []byte) error {
	_, err := io.ReadFull(r, buf)
	return err
}

func readUint32(r io.Reader, v *uint32) error {
	return binary.Read(r, binary.BigEndian, v)
}

func writeUint32(w io.Writer, v uint32) error {
	return binary.Write(w, binary.BigEndian, v)
}

func skipBytes(r io.Reader, n uint32) error {
	_, err := io.CopyN(io.Discard, r, int64(n))
	return err
}

// ReadField reads a single TLV field from r, returning the tag, data, and any error.
// For end markers (tag=0), data is nil.
func ReadField(r io.Reader) (tag uint32, data []byte, err error) {
	if err := readUint32(r, &tag); err != nil {
		return 0, nil, fmt.Errorf("%w: %w", ErrTruncated, err)
	}
	if tag == 0 {
		return 0, nil, nil
	}
	var length uint32
	if err := readUint32(r, &length); err != nil {
		return 0, nil, fmt.Errorf("%w: %w", ErrTruncated, err)
	}
	data = make([]byte, length)
	if err := readFull(r, data); err != nil {
		return 0, nil, fmt.Errorf("%w: %w", ErrTruncated, err)
	}
	return tag, data, nil
}

// WriteField writes a single TLV field to w.
func WriteField(w io.Writer, tag uint32, data []byte) error {
	if err := writeUint32(w, tag); err != nil {
		return err
	}
	if tag == 0 {
		return nil
	}
	if err := writeUint32(w, uint32(len(data))); err != nil { //nolint:gosec // data length is bounded by key sizes
		return err
	}
	_, err := w.Write(data)
	return err
}

// MarshalKey serializes a Key to bytes.
func MarshalKey(k *Key) ([]byte, error) {
	var buf bytes.Buffer
	if err := Write(&buf, k); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
