package keyfile_test

import (
	"bytes"
	"encoding/binary"
	"errors"
	"os"
	"reflect"
	"testing"

	"github.com/keiritz/encrypten/internal/keyfile"
)

func writeUint32(t *testing.T, buf *bytes.Buffer, v uint32) {
	t.Helper()
	if err := binary.Write(buf, binary.BigEndian, v); err != nil {
		t.Fatalf("binary.Write: %v", err)
	}
}

func TestReadGitCryptKey(t *testing.T) {
	f, err := os.Open("../../testdata/fixtures/key_default")
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer func() { _ = f.Close() }()

	k, err := keyfile.Read(f)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	if k.Version != 0 {
		t.Errorf("Version = %d, want 0", k.Version)
	}

	// Verify AES key matches fixture.
	wantAES := [32]byte{
		0xa9, 0x38, 0xee, 0x10, 0xdd, 0xac, 0xab, 0xef,
		0x00, 0x46, 0x09, 0x39, 0x8f, 0xd3, 0x54, 0x26,
		0x8e, 0x9c, 0x35, 0xd5, 0x1e, 0x34, 0xfa, 0x48,
		0xc5, 0x7a, 0xb6, 0x2d, 0x4e, 0xe3, 0x83, 0x0d,
	}
	if k.AESKey != wantAES {
		t.Errorf("AESKey mismatch")
	}

	// Verify HMAC key matches fixture.
	wantHMAC := [64]byte{
		0x52, 0x6f, 0x17, 0x23, 0x4f, 0x7f, 0x25, 0xf2,
		0xcd, 0xa1, 0x12, 0xd3, 0x9d, 0xc2, 0x03, 0x8d,
		0x43, 0xfb, 0xc1, 0xb4, 0xdb, 0x67, 0x33, 0x94,
		0x1d, 0x57, 0x18, 0x5d, 0x79, 0xe3, 0xce, 0x57,
		0x94, 0x5d, 0x37, 0xf5, 0x98, 0x37, 0x69, 0xdc,
		0x7f, 0xd2, 0x28, 0xd4, 0xd7, 0xc3, 0x85, 0xf1,
		0xfc, 0x64, 0x37, 0xa3, 0xab, 0x13, 0x63, 0x5a,
		0x40, 0x12, 0xac, 0xfc, 0x47, 0xbc, 0x42, 0xbc,
	}
	if k.HMACKey != wantHMAC {
		t.Errorf("HMACKey mismatch")
	}
}

func TestReadKeyFields(t *testing.T) {
	// Build a buffer with key fields only (no magic/version/header).
	var buf bytes.Buffer

	// KEY_FIELD_VERSION: tag=1, len=4, value=0
	writeUint32(t, &buf, 1)
	writeUint32(t, &buf, 4)
	writeUint32(t, &buf, 0)

	// KEY_FIELD_AES_KEY: tag=3, len=32
	writeUint32(t, &buf, 3)
	writeUint32(t, &buf, 32)
	aes := [32]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16,
		17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32}
	buf.Write(aes[:])

	// KEY_FIELD_HMAC_KEY: tag=5, len=64
	writeUint32(t, &buf, 5)
	writeUint32(t, &buf, 64)
	var hmac [64]byte
	for i := range hmac {
		hmac[i] = byte(i)
	}
	buf.Write(hmac[:])

	// KEY_FIELD_END: tag=0
	writeUint32(t, &buf, 0)

	// Parse fields individually using ReadField.
	r := bytes.NewReader(buf.Bytes())

	// Field 1: version
	tag, data, err := keyfile.ReadField(r)
	if err != nil {
		t.Fatalf("ReadField version: %v", err)
	}
	if tag != 1 {
		t.Errorf("tag = %d, want 1", tag)
	}
	if len(data) != 4 {
		t.Errorf("data len = %d, want 4", len(data))
	}

	// Field 2: AES key
	tag, data, err = keyfile.ReadField(r)
	if err != nil {
		t.Fatalf("ReadField AES: %v", err)
	}
	if tag != 3 {
		t.Errorf("tag = %d, want 3", tag)
	}
	if !bytes.Equal(data, aes[:]) {
		t.Errorf("AES data mismatch")
	}

	// Field 3: HMAC key
	tag, data, err = keyfile.ReadField(r)
	if err != nil {
		t.Fatalf("ReadField HMAC: %v", err)
	}
	if tag != 5 {
		t.Errorf("tag = %d, want 5", tag)
	}
	if !bytes.Equal(data, hmac[:]) {
		t.Errorf("HMAC data mismatch")
	}

	// Field 4: end
	tag, data, err = keyfile.ReadField(r)
	if err != nil {
		t.Fatalf("ReadField end: %v", err)
	}
	if tag != 0 {
		t.Errorf("tag = %d, want 0", tag)
	}
	if data != nil {
		t.Errorf("end field data should be nil")
	}
}

func TestWriteKeyRoundtrip(t *testing.T) {
	k1, err := keyfile.Generate()
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	var buf bytes.Buffer
	if err := keyfile.Write(&buf, k1); err != nil {
		t.Fatalf("Write: %v", err)
	}

	k2, err := keyfile.Read(&buf)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	if !reflect.DeepEqual(k1, k2) {
		t.Errorf("roundtrip mismatch")
	}
}

func TestWriteKeyFormat(t *testing.T) {
	k := &keyfile.Key{Version: 0}
	for i := range k.AESKey {
		k.AESKey[i] = byte(i)
	}
	for i := range k.HMACKey {
		k.HMACKey[i] = byte(i + 100)
	}

	data, err := keyfile.MarshalKey(k)
	if err != nil {
		t.Fatalf("MarshalKey: %v", err)
	}

	if len(data) != 148 {
		t.Fatalf("len = %d, want 148", len(data))
	}

	// Check magic.
	wantMagic := []byte{0x00, 'G', 'I', 'T', 'C', 'R', 'Y', 'P', 'T', 'K', 'E', 'Y'}
	if !bytes.Equal(data[:12], wantMagic) {
		t.Errorf("magic mismatch")
	}

	// Check format version = 2.
	ver := binary.BigEndian.Uint32(data[12:16])
	if ver != 2 {
		t.Errorf("format version = %d, want 2", ver)
	}

	// Check header field end.
	headerEnd := binary.BigEndian.Uint32(data[16:20])
	if headerEnd != 0 {
		t.Errorf("header end = %d, want 0", headerEnd)
	}

	// Check key field version tag.
	versionTag := binary.BigEndian.Uint32(data[20:24])
	if versionTag != 1 {
		t.Errorf("version tag = %d, want 1", versionTag)
	}

	// Check AES key data at correct offset.
	if !bytes.Equal(data[40:72], k.AESKey[:]) {
		t.Errorf("AES key data mismatch at offset 40:72")
	}

	// Check HMAC key data at correct offset.
	if !bytes.Equal(data[80:144], k.HMACKey[:]) {
		t.Errorf("HMAC key data mismatch at offset 80:144")
	}

	// Check key field end.
	keyEnd := binary.BigEndian.Uint32(data[144:148])
	if keyEnd != 0 {
		t.Errorf("key field end = %d, want 0", keyEnd)
	}
}

func TestReadInvalidMagic(t *testing.T) {
	data := make([]byte, 148)
	copy(data, []byte("BADMAGICBYTE"))

	_, err := keyfile.Read(bytes.NewReader(data))
	if !errors.Is(err, keyfile.ErrInvalidMagic) {
		t.Errorf("err = %v, want ErrInvalidMagic", err)
	}
}

func TestReadTruncated(t *testing.T) {
	// Valid magic but nothing after.
	magic := []byte{0x00, 'G', 'I', 'T', 'C', 'R', 'Y', 'P', 'T', 'K', 'E', 'Y'}

	_, err := keyfile.Read(bytes.NewReader(magic))
	if !errors.Is(err, keyfile.ErrTruncated) {
		t.Errorf("err = %v, want ErrTruncated", err)
	}

	// Even shorter data.
	_, err = keyfile.Read(bytes.NewReader(magic[:5]))
	if !errors.Is(err, keyfile.ErrTruncated) {
		t.Errorf("err = %v, want ErrTruncated", err)
	}
}

func TestReadUnsupportedVersion(t *testing.T) {
	var buf bytes.Buffer
	buf.Write([]byte{0x00, 'G', 'I', 'T', 'C', 'R', 'Y', 'P', 'T', 'K', 'E', 'Y'})
	writeUint32(t, &buf, 99)

	_, err := keyfile.Read(&buf)
	if !errors.Is(err, keyfile.ErrUnsupportedVersion) {
		t.Errorf("err = %v, want ErrUnsupportedVersion", err)
	}
}

func TestKeyGenerate(t *testing.T) {
	k, err := keyfile.Generate()
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	if k.Version != 0 {
		t.Errorf("Version = %d, want 0", k.Version)
	}

	// AES key should not be all zeros.
	var zeroAES [32]byte
	if k.AESKey == zeroAES {
		t.Error("AESKey is all zeros")
	}

	// HMAC key should not be all zeros.
	var zeroHMAC [64]byte
	if k.HMACKey == zeroHMAC {
		t.Error("HMACKey is all zeros")
	}
}
