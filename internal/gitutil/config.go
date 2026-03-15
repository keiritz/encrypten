package gitutil

import (
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Filter and diff configuration values for git-crypt compatibility.
const (
	filterSmudge = "encrypten smudge"
	filterClean  = "encrypten clean"
	filterReqd   = "true"
	diffTextconv = "encrypten diff"
)

// CheckInPath verifies that "encrypten" is found in $PATH.
// Returns a user-friendly error if not found.
func CheckInPath() error {
	_, err := exec.LookPath("encrypten")
	if err != nil {
		return fmt.Errorf("\"encrypten\" was not found in $PATH.\n" +
			"Git filters are registered as \"encrypten smudge\" / \"encrypten clean\" and require\n" +
			"the binary to be in $PATH. Git operations on encrypted files will fail until\n" +
			"encrypten is added to $PATH")
	}
	return nil
}

// gitConfig runs "git config" with the given args in dir.
// It retries up to 3 times on exit code 255 (lockfile contention).
func gitConfig(dir string, args ...string) error {
	const maxRetries = 3
	for attempt := 0; ; attempt++ {
		cmdArgs := append([]string{"config"}, args...)
		cmd := exec.Command("git", cmdArgs...) // #nosec G204
		cmd.Dir = dir
		_, err := cmd.Output()
		if err == nil {
			return nil
		}
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 255 && attempt < maxRetries {
			time.Sleep(time.Duration(50<<uint(attempt)) * time.Millisecond)
			continue
		}
		return err
	}
}

// gitConfigGet runs "git config --get" for the given key in dir.
func gitConfigGet(dir string, key string) (string, error) {
	cmd := exec.Command("git", "config", "--get", key) // #nosec G204
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// SetFilter configures the git-crypt filter (smudge, clean, required).
// It is idempotent — values that already match are skipped.
func SetFilter(dir string) error {
	pairs := []struct{ key, val string }{
		{"filter.git-crypt.smudge", filterSmudge},
		{"filter.git-crypt.clean", filterClean},
		{"filter.git-crypt.required", filterReqd},
	}
	for _, p := range pairs {
		got, err := gitConfigGet(dir, p.key)
		if err == nil && got == p.val {
			continue
		}
		if err := gitConfig(dir, p.key, p.val); err != nil {
			return err
		}
	}
	return nil
}

// SetDiffTextconv configures the git-crypt diff textconv driver.
// It is idempotent — the value is skipped if it already matches.
func SetDiffTextconv(dir string) error {
	got, err := gitConfigGet(dir, "diff.git-crypt.textconv")
	if err == nil && got == diffTextconv {
		return nil
	}
	return gitConfig(dir, "diff.git-crypt.textconv", diffTextconv)
}

// UnsetFilter removes both filter.git-crypt and diff.git-crypt sections.
// It is idempotent — missing sections are silently ignored.
func UnsetFilter(dir string) error {
	for _, section := range []string{"filter.git-crypt", "diff.git-crypt"} {
		if err := gitConfig(dir, "--remove-section", section); err != nil {
			// Exit code 128 means the section doesn't exist; ignore it.
			if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 128 {
				continue
			}
			return err
		}
	}
	return nil
}

// EnableWorktreeConfig enables extensions.worktreeConfig so that
// per-worktree config overrides are supported.
func EnableWorktreeConfig(dir string) error {
	val, err := gitConfigGet(dir, "extensions.worktreeConfig")
	if err == nil && val == "true" {
		return nil
	}
	return gitConfig(dir, "extensions.worktreeConfig", "true")
}

// SetFilterWorktree sets filter.git-crypt and diff.git-crypt using --worktree scope.
func SetFilterWorktree(dir, smudge, clean, required string) error {
	if err := EnableWorktreeConfig(dir); err != nil {
		return fmt.Errorf("enabling worktree config: %w", err)
	}
	if err := gitConfig(dir, "--worktree", "filter.git-crypt.smudge", smudge); err != nil {
		return err
	}
	if err := gitConfig(dir, "--worktree", "filter.git-crypt.clean", clean); err != nil {
		return err
	}
	if err := gitConfig(dir, "--worktree", "filter.git-crypt.required", required); err != nil {
		return err
	}
	return gitConfig(dir, "--worktree", "diff.git-crypt.textconv", "cat")
}

// UnsetFilterWorktree removes filter.git-crypt and diff.git-crypt sections
// from the worktree-specific config.
func UnsetFilterWorktree(dir string) error {
	if err := EnableWorktreeConfig(dir); err != nil {
		return fmt.Errorf("enabling worktree config: %w", err)
	}
	for _, section := range []string{"filter.git-crypt", "diff.git-crypt"} {
		if err := gitConfig(dir, "--worktree", "--remove-section", section); err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 128 {
				continue
			}
			return err
		}
	}
	return nil
}

// FilterConfigMatchesGitCrypt checks whether all four git-crypt filter/diff
// config values match the expected encrypten values.
func FilterConfigMatchesGitCrypt(dir string) (bool, error) {
	checks := []struct {
		key  string
		want string
	}{
		{"filter.git-crypt.smudge", filterSmudge},
		{"filter.git-crypt.clean", filterClean},
		{"filter.git-crypt.required", filterReqd},
		{"diff.git-crypt.textconv", diffTextconv},
	}
	for _, c := range checks {
		got, err := gitConfigGet(dir, c.key)
		if err != nil {
			return false, nil //nolint:nilerr // missing key means no match
		}
		if got != c.want {
			return false, nil
		}
	}
	return true, nil
}
