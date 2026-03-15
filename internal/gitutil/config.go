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

// filterSections lists the filter name prefixes to configure.
// Both git-crypt (for backward compatibility) and encrypten are registered.
var filterSections = []string{"git-crypt", "encrypten"}

// SetFilter configures the git-crypt and encrypten filters (smudge, clean, required).
// It is idempotent — values that already match are skipped.
func SetFilter(dir string) error {
	var pairs []struct{ key, val string }
	for _, section := range filterSections {
		pairs = append(pairs,
			struct{ key, val string }{"filter." + section + ".smudge", filterSmudge},
			struct{ key, val string }{"filter." + section + ".clean", filterClean},
			struct{ key, val string }{"filter." + section + ".required", filterReqd},
		)
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

// SetDiffTextconv configures the git-crypt and encrypten diff textconv drivers.
// It is idempotent — values that already match are skipped.
func SetDiffTextconv(dir string) error {
	for _, section := range filterSections {
		key := "diff." + section + ".textconv"
		got, err := gitConfigGet(dir, key)
		if err == nil && got == diffTextconv {
			continue
		}
		if err := gitConfig(dir, key, diffTextconv); err != nil {
			return err
		}
	}
	return nil
}

// UnsetFilter removes filter and diff sections for both git-crypt and encrypten.
// It is idempotent — missing sections are silently ignored.
func UnsetFilter(dir string) error {
	for _, name := range filterSections {
		for _, kind := range []string{"filter", "diff"} {
			section := kind + "." + name
			if err := gitConfig(dir, "--remove-section", section); err != nil {
				// Exit code 128 means the section doesn't exist; ignore it.
				if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 128 {
					continue
				}
				return err
			}
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

// SetFilterWorktree sets filter and diff config for both git-crypt and encrypten
// using --worktree scope.
func SetFilterWorktree(dir, smudge, clean, required string) error {
	if err := EnableWorktreeConfig(dir); err != nil {
		return fmt.Errorf("enabling worktree config: %w", err)
	}
	for _, section := range filterSections {
		if err := gitConfig(dir, "--worktree", "filter."+section+".smudge", smudge); err != nil {
			return err
		}
		if err := gitConfig(dir, "--worktree", "filter."+section+".clean", clean); err != nil {
			return err
		}
		if err := gitConfig(dir, "--worktree", "filter."+section+".required", required); err != nil {
			return err
		}
		if err := gitConfig(dir, "--worktree", "diff."+section+".textconv", "cat"); err != nil {
			return err
		}
	}
	return nil
}

// UnsetFilterWorktree removes filter and diff sections for both git-crypt and
// encrypten from the worktree-specific config.
func UnsetFilterWorktree(dir string) error {
	if err := EnableWorktreeConfig(dir); err != nil {
		return fmt.Errorf("enabling worktree config: %w", err)
	}
	for _, name := range filterSections {
		for _, kind := range []string{"filter", "diff"} {
			section := kind + "." + name
			if err := gitConfig(dir, "--worktree", "--remove-section", section); err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 128 {
					continue
				}
				return err
			}
		}
	}
	return nil
}

// FilterConfigured checks whether all filter/diff config values for both
// git-crypt and encrypten sections match the expected encrypten values.
func FilterConfigured(dir string) (bool, error) {
	for _, section := range filterSections {
		checks := []struct {
			key  string
			want string
		}{
			{"filter." + section + ".smudge", filterSmudge},
			{"filter." + section + ".clean", filterClean},
			{"filter." + section + ".required", filterReqd},
			{"diff." + section + ".textconv", diffTextconv},
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
	}
	return true, nil
}

// FilterConfigMatchesGitCrypt is an alias for FilterConfigured for backward compatibility.
func FilterConfigMatchesGitCrypt(dir string) (bool, error) {
	return FilterConfigured(dir)
}
