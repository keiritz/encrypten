package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/keiritz/encrypten/internal/crypto"
	"github.com/keiritz/encrypten/internal/fileformat"
	"github.com/keiritz/encrypten/internal/gitutil"
	"github.com/keiritz/encrypten/internal/keyfile"
)

func cmdClean() error {
	keyDir, err := gitutil.KeyDir("")
	if err != nil {
		return err
	}
	keyPath := filepath.Join(keyDir, "default")
	return cleanFilter(os.Stdin, os.Stdout, keyPath)
}

func cleanFilter(r io.Reader, w io.Writer, keyPath string) error {
	f, err := os.Open(keyPath) //nolint:gosec // path is constructed internally
	if err != nil {
		return fmt.Errorf("failed to open key file: %w", err)
	}
	defer func() { _ = f.Close() }()

	key, err := keyfile.Read(f)
	if err != nil {
		return fmt.Errorf("failed to read key file: %w", err)
	}

	input, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("failed to read input: %w", err)
	}

	if fileformat.IsEncrypted(input) {
		_, err = w.Write(input)
		return err
	}

	encrypted, err := crypto.Encrypt(input, key)
	if err != nil {
		return fmt.Errorf("failed to encrypt: %w", err)
	}

	_, err = w.Write(encrypted)
	return err
}
