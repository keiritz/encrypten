package main

import (
	"fmt"
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
		enc, err := fileformat.IsEncryptedFile(filepath.Join(root, e.Path))
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", e.Path, err)
		}

		if enc {
			fmt.Printf("    encrypted: %s\n", e.Path)
		} else {
			fmt.Printf("not encrypted: %s\n", e.Path)
		}
	}

	return nil
}
