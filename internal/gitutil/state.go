package gitutil

import (
	"path/filepath"

	"github.com/keiritz/encrypten/internal/fileformat"
)

// RepoEncryptionState represents the encryption state of tracked files.
type RepoEncryptionState int

const (
	// StateFullyEncrypted means all git-crypt files are encrypted.
	StateFullyEncrypted RepoEncryptionState = iota
	// StateFullyDecrypted means all git-crypt files are decrypted.
	StateFullyDecrypted
	// StateMixed means some files are encrypted and some are decrypted.
	StateMixed
	// StateNoFiles means no git-crypt attributed files were found.
	StateNoFiles
)

// DetectEncryptionState inspects the working tree to determine whether
// git-crypt attributed files are currently encrypted or decrypted.
func DetectEncryptionState(dir string) (RepoEncryptionState, error) {
	entries, err := ListEncryptedFiles(dir)
	if err != nil {
		return StateNoFiles, err
	}
	if len(entries) == 0 {
		return StateNoFiles, nil
	}

	var encrypted, decrypted int
	for _, e := range entries {
		enc, err := fileformat.IsEncryptedFile(filepath.Join(dir, e.Path))
		if err != nil {
			// Skip unreadable files (e.g. deleted in worktree).
			continue
		}
		if enc {
			encrypted++
		} else {
			decrypted++
		}
	}

	if encrypted == 0 && decrypted == 0 {
		return StateNoFiles, nil
	}
	if decrypted == 0 {
		return StateFullyEncrypted, nil
	}
	if encrypted == 0 {
		return StateFullyDecrypted, nil
	}
	return StateMixed, nil
}

// AllWorktreesEncrypted returns true if every worktree is either
// fully encrypted or has no encrypted files.
func AllWorktreesEncrypted(dir string) (bool, error) {
	wts, err := ListWorktrees(dir)
	if err != nil {
		return false, err
	}
	for _, wt := range wts {
		state, err := DetectEncryptionState(wt.Path)
		if err != nil {
			return false, err
		}
		if state == StateFullyDecrypted || state == StateMixed {
			return false, nil
		}
	}
	return true, nil
}
