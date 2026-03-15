package gitutil

import (
	"fmt"
	"os/exec"
	"path/filepath"
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

// filterSections lists the filter name prefixes to configure.
// Both git-crypt (for backward compatibility) and encrypten are registered.
var filterSections = []string{"git-crypt", "encrypten"}

// localConfigPath returns the path to the shared git config file.
func localConfigPath(dir string) (string, error) {
	common, err := GitCommonDir(dir)
	if err != nil {
		return "", err
	}
	return filepath.Join(common, "config"), nil
}

// worktreeConfigPath returns the path to the worktree-specific config file.
func worktreeConfigPath(dir string) (string, error) {
	gitDir, err := GitDir(dir)
	if err != nil {
		return "", err
	}
	return filepath.Join(gitDir, "config.worktree"), nil
}

// SetFilter configures the git-crypt and encrypten filters (smudge, clean, required).
// It is idempotent — values that already match are skipped.
func SetFilter(dir string) error {
	cfgPath, err := localConfigPath(dir)
	if err != nil {
		return err
	}
	f, err := readGitConfigFile(cfgPath)
	if err != nil {
		return err
	}

	changed := false
	for _, section := range filterSections {
		pairs := []struct{ key, val string }{
			{"smudge", filterSmudge},
			{"clean", filterClean},
			{"required", filterReqd},
		}
		for _, p := range pairs {
			got, ok := f.Get("filter", section, p.key)
			if ok && got == p.val {
				continue
			}
			f.Set("filter", section, p.key, p.val)
			changed = true
		}
	}
	if !changed {
		return nil
	}
	return writeGitConfigFile(cfgPath, f)
}

// SetDiffTextconv configures the git-crypt and encrypten diff textconv drivers.
// It is idempotent — values that already match are skipped.
func SetDiffTextconv(dir string) error {
	cfgPath, err := localConfigPath(dir)
	if err != nil {
		return err
	}
	f, err := readGitConfigFile(cfgPath)
	if err != nil {
		return err
	}

	changed := false
	for _, section := range filterSections {
		got, ok := f.Get("diff", section, "textconv")
		if ok && got == diffTextconv {
			continue
		}
		f.Set("diff", section, "textconv", diffTextconv)
		changed = true
	}
	if !changed {
		return nil
	}
	return writeGitConfigFile(cfgPath, f)
}

// UnsetFilter removes filter and diff sections for both git-crypt and encrypten.
// It is idempotent — missing sections are silently ignored.
func UnsetFilter(dir string) error {
	cfgPath, err := localConfigPath(dir)
	if err != nil {
		return err
	}
	f, err := readGitConfigFile(cfgPath)
	if err != nil {
		return err
	}

	changed := false
	for _, name := range filterSections {
		for _, kind := range []string{"filter", "diff"} {
			if f.RemoveSection(kind, name) {
				changed = true
			}
		}
	}
	if !changed {
		return nil
	}
	return writeGitConfigFile(cfgPath, f)
}

// EnableWorktreeConfig enables extensions.worktreeConfig so that
// per-worktree config overrides are supported.
func EnableWorktreeConfig(dir string) error {
	cfgPath, err := localConfigPath(dir)
	if err != nil {
		return err
	}
	f, err := readGitConfigFile(cfgPath)
	if err != nil {
		return err
	}

	got, ok := f.Get("extensions", "", "worktreeconfig")
	if ok && got == "true" {
		return nil
	}
	f.Set("extensions", "", "worktreeConfig", "true")
	return writeGitConfigFile(cfgPath, f)
}

// SetFilterWorktree sets filter and diff config for both git-crypt and encrypten
// using worktree-specific config.
func SetFilterWorktree(dir, smudge, clean, required string) error {
	if err := EnableWorktreeConfig(dir); err != nil {
		return fmt.Errorf("enabling worktree config: %w", err)
	}

	wtPath, err := worktreeConfigPath(dir)
	if err != nil {
		return err
	}
	f, err := readGitConfigFile(wtPath)
	if err != nil {
		return err
	}

	for _, section := range filterSections {
		f.Set("filter", section, "smudge", smudge)
		f.Set("filter", section, "clean", clean)
		f.Set("filter", section, "required", required)
		f.Set("diff", section, "textconv", "cat")
	}
	return writeGitConfigFile(wtPath, f)
}

// UnsetFilterWorktree removes filter and diff sections for both git-crypt and
// encrypten from the worktree-specific config.
func UnsetFilterWorktree(dir string) error {
	if err := EnableWorktreeConfig(dir); err != nil {
		return fmt.Errorf("enabling worktree config: %w", err)
	}

	wtPath, err := worktreeConfigPath(dir)
	if err != nil {
		return err
	}
	f, err := readGitConfigFile(wtPath)
	if err != nil {
		return err
	}

	for _, name := range filterSections {
		for _, kind := range []string{"filter", "diff"} {
			f.RemoveSection(kind, name)
		}
	}
	return writeGitConfigFile(wtPath, f)
}

// FilterConfigured checks whether all filter/diff config values for both
// git-crypt and encrypten sections match the expected encrypten values.
func FilterConfigured(dir string) (bool, error) {
	cfgPath, err := localConfigPath(dir)
	if err != nil {
		return false, err
	}
	f, err := readGitConfigFile(cfgPath)
	if err != nil {
		return false, err
	}

	for _, section := range filterSections {
		checks := []struct {
			key  string
			want string
		}{
			{"smudge", filterSmudge},
			{"clean", filterClean},
			{"required", filterReqd},
			{"textconv", diffTextconv},
		}
		kinds := []string{"filter", "filter", "filter", "diff"}
		for i, c := range checks {
			got, ok := f.Get(kinds[i], section, c.key)
			if !ok || got != c.want {
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
