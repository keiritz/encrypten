package main

import (
	"fmt"
	"os"

	"github.com/keiritz/encrypten/internal/keyfile"
)

func runKeygen(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: encrypten keygen KEYFILE")
	}
	path := args[0]

	key, err := keyfile.Generate()
	if err != nil {
		return fmt.Errorf("generating key: %w", err)
	}

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600) //nolint:gosec // path is user-provided CLI argument
	if err != nil {
		return err
	}

	writeErr := keyfile.Write(f, key)
	closeErr := f.Close()

	if writeErr != nil {
		_ = os.Remove(path) //nolint:gosec // cleanup of file we just created
		return fmt.Errorf("writing key: %w", writeErr)
	}
	if closeErr != nil {
		_ = os.Remove(path) //nolint:gosec // cleanup of file we just created
		return fmt.Errorf("closing file: %w", closeErr)
	}

	return nil
}
