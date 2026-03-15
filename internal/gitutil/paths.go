package gitutil

import (
	"bufio"
	"bytes"
	"os/exec"
	"path/filepath"
	"strings"
)

// gitRevParse runs "git rev-parse" with the given args in dir.
// If dir is empty, the current directory is used.
func gitRevParse(dir string, args ...string) (string, error) {
	cmdArgs := append([]string{"rev-parse"}, args...)
	cmd := exec.Command("git", cmdArgs...) // #nosec G204
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return filepath.FromSlash(strings.TrimSpace(string(out))), nil
}

// RepoRoot returns the worktree-specific root directory (--show-toplevel).
func RepoRoot(dir string) (string, error) {
	return gitRevParse(dir, "--show-toplevel")
}

// GitCommonDir returns the shared .git directory (--git-common-dir).
// If git returns a relative path, it is resolved relative to dir.
func GitCommonDir(dir string) (string, error) {
	p, err := gitRevParse(dir, "--git-common-dir")
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(p) {
		p = filepath.Join(dir, p)
	}
	return filepath.Clean(p), nil
}

// GitDir returns the worktree-specific .git directory (--git-dir).
func GitDir(dir string) (string, error) {
	p, err := gitRevParse(dir, "--git-dir")
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(p) {
		p = filepath.Join(dir, p)
	}
	return filepath.Clean(p), nil
}

// IsWorktree returns true if dir is inside a secondary worktree
// (i.e., --git-dir differs from --git-common-dir).
func IsWorktree(dir string) (bool, error) {
	gitDir, err := GitDir(dir)
	if err != nil {
		return false, err
	}
	commonDir, err := GitCommonDir(dir)
	if err != nil {
		return false, err
	}
	// Resolve symlinks for reliable comparison (macOS /tmp → /private/tmp).
	gitDirReal, err := filepath.EvalSymlinks(gitDir)
	if err != nil {
		return false, err
	}
	commonDirReal, err := filepath.EvalSymlinks(commonDir)
	if err != nil {
		return false, err
	}
	return gitDirReal != commonDirReal, nil
}

// KeyDir returns the path to the git-crypt keys directory
// under the common git directory.
func KeyDir(dir string) (string, error) {
	common, err := GitCommonDir(dir)
	if err != nil {
		return "", err
	}
	return filepath.Join(common, "git-crypt", "keys"), nil
}

// WorktreeInfo holds information about a single git worktree.
type WorktreeInfo struct {
	Path string
}

// ListWorktrees returns all worktrees registered in the repository
// by parsing "git worktree list --porcelain".
func ListWorktrees(dir string) ([]WorktreeInfo, error) {
	cmd := exec.Command("git", "worktree", "list", "--porcelain") // #nosec G204
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var wts []WorktreeInfo
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "worktree ") {
			p := filepath.FromSlash(strings.TrimPrefix(line, "worktree "))
			wts = append(wts, WorktreeInfo{Path: p})
		}
	}
	return wts, scanner.Err()
}
