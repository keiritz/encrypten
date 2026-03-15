package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/keiritz/encrypten/internal/gitutil"
	"github.com/keiritz/encrypten/internal/keyfile"
)

func cmdExportKey(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: encrypten export-key <FILE | ->")
	}
	dest := args[0]

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

	var w *os.File
	if dest == "-" {
		w = os.Stdout
	} else {
		w, err = os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600) //nolint:gosec // dest is user-provided CLI argument
		if err != nil {
			return fmt.Errorf("failed to create output file: %w", err)
		}
		defer func() { _ = w.Close() }()
	}

	if err := keyfile.Write(w, k); err != nil {
		return fmt.Errorf("failed to write key: %w", err)
	}

	return nil
}
