package gitutil_test

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/keiritz/encrypten/internal/gitutil"
)

// configGet is a test helper that reads a git config value.
func configGet(t *testing.T, dir, key string) string {
	t.Helper()
	cmd := exec.Command("git", "config", "--get", key) //nolint:gosec // test helper
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git config --get %s: %v", key, err)
	}
	return strings.TrimSpace(string(out))
}

func TestSetFilter(t *testing.T) {
	dir := initTestRepo(t)

	if err := gitutil.SetFilter(dir); err != nil {
		t.Fatal(err)
	}

	for _, section := range []string{"git-crypt", "encrypten"} {
		if got := configGet(t, dir, "filter."+section+".smudge"); got != "encrypten smudge" {
			t.Errorf("filter.%s.smudge = %q, want %q", section, got, "encrypten smudge")
		}
		if got := configGet(t, dir, "filter."+section+".clean"); got != "encrypten clean" {
			t.Errorf("filter.%s.clean = %q, want %q", section, got, "encrypten clean")
		}
		if got := configGet(t, dir, "filter."+section+".required"); got != "true" {
			t.Errorf("filter.%s.required = %q, want %q", section, got, "true")
		}
	}
}

func TestSetDiffTextconv(t *testing.T) {
	dir := initTestRepo(t)

	if err := gitutil.SetDiffTextconv(dir); err != nil {
		t.Fatal(err)
	}

	for _, section := range []string{"git-crypt", "encrypten"} {
		if got := configGet(t, dir, "diff."+section+".textconv"); got != "encrypten diff" {
			t.Errorf("diff.%s.textconv = %q, want %q", section, got, "encrypten diff")
		}
	}
}

func TestUnsetFilter(t *testing.T) {
	dir := initTestRepo(t)

	// Set then unset.
	if err := gitutil.SetFilter(dir); err != nil {
		t.Fatal(err)
	}
	if err := gitutil.SetDiffTextconv(dir); err != nil {
		t.Fatal(err)
	}
	if err := gitutil.UnsetFilter(dir); err != nil {
		t.Fatal(err)
	}

	// Values should no longer exist.
	for _, key := range []string{
		"filter.git-crypt.smudge",
		"filter.git-crypt.clean",
		"filter.git-crypt.required",
		"diff.git-crypt.textconv",
		"filter.encrypten.smudge",
		"filter.encrypten.clean",
		"filter.encrypten.required",
		"diff.encrypten.textconv",
	} {
		cmd := exec.Command("git", "config", "--get", key) //nolint:gosec // test
		cmd.Dir = dir
		if err := cmd.Run(); err == nil {
			t.Errorf("expected %s to be unset, but it still exists", key)
		}
	}

	// Idempotent: calling UnsetFilter again should not error.
	if err := gitutil.UnsetFilter(dir); err != nil {
		t.Errorf("second UnsetFilter should be idempotent, got: %v", err)
	}
}

func TestSetFilterIdempotent(t *testing.T) {
	dir := initTestRepo(t)

	// First call sets values.
	if err := gitutil.SetFilter(dir); err != nil {
		t.Fatal(err)
	}
	// Second call should succeed without error (idempotent).
	if err := gitutil.SetFilter(dir); err != nil {
		t.Fatal(err)
	}

	// Values should still be correct.
	for _, section := range []string{"git-crypt", "encrypten"} {
		if got := configGet(t, dir, "filter."+section+".smudge"); got != "encrypten smudge" {
			t.Errorf("filter.%s.smudge = %q, want %q", section, got, "encrypten smudge")
		}
	}
}

func TestEnableWorktreeConfigIdempotent(t *testing.T) {
	dir := initTestRepo(t)

	// First call enables.
	if err := gitutil.EnableWorktreeConfig(dir); err != nil {
		t.Fatal(err)
	}
	if got := configGet(t, dir, "extensions.worktreeConfig"); got != "true" {
		t.Errorf("worktreeConfig = %q, want %q", got, "true")
	}

	// Second call should succeed (idempotent, no duplicate writes).
	if err := gitutil.EnableWorktreeConfig(dir); err != nil {
		t.Fatal(err)
	}
	if got := configGet(t, dir, "extensions.worktreeConfig"); got != "true" {
		t.Errorf("worktreeConfig = %q after second call, want %q", got, "true")
	}
}

func TestFilterConfigMatchesGitCrypt(t *testing.T) {
	dir := initTestRepo(t)

	// No config set → false.
	match, err := gitutil.FilterConfigMatchesGitCrypt(dir)
	if err != nil {
		t.Fatal(err)
	}
	if match {
		t.Error("expected false when no config is set")
	}

	// Filter only (no diff) → false.
	if err := gitutil.SetFilter(dir); err != nil {
		t.Fatal(err)
	}
	match, err = gitutil.FilterConfigMatchesGitCrypt(dir)
	if err != nil {
		t.Fatal(err)
	}
	if match {
		t.Error("expected false when only filter is set")
	}

	// All config set → true.
	if err := gitutil.SetDiffTextconv(dir); err != nil {
		t.Fatal(err)
	}
	match, err = gitutil.FilterConfigMatchesGitCrypt(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !match {
		t.Error("expected true when all config is set")
	}

	// Change a value → false.
	run(t, dir, "git", "config", "filter.git-crypt.smudge", "wrong-value")
	match, err = gitutil.FilterConfigMatchesGitCrypt(dir)
	if err != nil {
		t.Fatal(err)
	}
	if match {
		t.Error("expected false when a value is changed")
	}
}
