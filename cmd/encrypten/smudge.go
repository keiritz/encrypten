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

func cmdSmudge(_ []string) error {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return fmt.Errorf("failed to read stdin: %w", err)
	}

	if len(data) == 0 {
		return nil
	}

	if !fileformat.IsEncrypted(data) {
		_, err := os.Stdout.Write(data)
		return err
	}

	keyDir, err := gitutil.KeyDir("")
	if err != nil {
		return err
	}
	keyPath := filepath.Join(keyDir, "default")

	f, err := os.Open(keyPath) //nolint:gosec // path is constructed internally
	if err != nil {
		return fmt.Errorf("failed to open key file: %w", err)
	}
	defer func() { _ = f.Close() }()

	k, err := keyfile.Read(f)
	if err != nil {
		return fmt.Errorf("failed to read key file: %w", err)
	}

	plaintext, err := crypto.Decrypt(data, k)
	if err != nil {
		return fmt.Errorf("failed to decrypt: %w", err)
	}

	_, err = os.Stdout.Write(plaintext)
	return err
}
