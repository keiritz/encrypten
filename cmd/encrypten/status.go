package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/keiritz/encrypten/internal/fileformat"
	"github.com/keiritz/encrypten/internal/gitutil"
)

func cmdStatus(args []string) error {
	_ = args // reserved for future flags

	root, err := gitutil.RepoRoot("")
	if err != nil {
		return fmt.Errorf("failed to find repository root: %w", err)
	}

	entries, err := gitutil.ListEncryptedFiles(root)
	if err != nil {
		return fmt.Errorf("failed to list encrypted files: %w", err)
	}

	for _, e := range entries {
		data, err := os.ReadFile(filepath.Join(root, e.Path)) //nolint:gosec // path from git ls-files
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", e.Path, err)
		}

		if fileformat.IsEncrypted(data) {
			fmt.Printf("    encrypted: %s\n", e.Path)
		} else {
			fmt.Printf("not encrypted: %s\n", e.Path)
		}
	}

	return nil
}
