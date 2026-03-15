package gitutil

import (
	"bufio"
	"bytes"
	"fmt"
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

// ResolvedPaths holds the three paths returned by a single git rev-parse call.
type ResolvedPaths struct {
	GitDir    string
	CommonDir string
	RepoRoot  string
}

// ResolveAll runs a single "git rev-parse --git-dir --git-common-dir --show-toplevel"
// and returns all three paths. Relative paths are resolved relative to dir.
func ResolveAll(dir string) (ResolvedPaths, error) {
	cmdArgs := []string{"rev-parse", "--git-dir", "--git-common-dir", "--show-toplevel"}
	cmd := exec.Command("git", cmdArgs...) // #nosec G204
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.Output()
	if err != nil {
		return ResolvedPaths{}, err
	}

	lines := strings.SplitN(strings.TrimRight(string(out), "\n"), "\n", 3)
	if len(lines) != 3 {
		return ResolvedPaths{}, fmt.Errorf("git rev-parse: expected 3 lines, got %d", len(lines))
	}

	resolve := func(p string) string {
		p = filepath.FromSlash(p)
		if !filepath.IsAbs(p) {
			p = filepath.Join(dir, p)
		}
		return filepath.Clean(p)
	}

	return ResolvedPaths{
		GitDir:    resolve(lines[0]),
		CommonDir: resolve(lines[1]),
		RepoRoot:  resolve(lines[2]),
	}, nil
}

// IsWorktree returns true if dir is inside a secondary worktree
// (i.e., --git-dir differs from --git-common-dir).
func IsWorktree(dir string) (bool, error) {
	paths, err := ResolveAll(dir)
	if err != nil {
		return false, err
	}
	// Resolve symlinks for reliable comparison (macOS /tmp → /private/tmp).
	gitDirReal, err := filepath.EvalSymlinks(paths.GitDir)
	if err != nil {
		return false, err
	}
	commonDirReal, err := filepath.EvalSymlinks(paths.CommonDir)
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
