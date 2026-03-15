package gitutil_test

import (
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/keiritz/encrypten/internal/gitutil"
)

func initBenchRepo(b *testing.B) string {
	b.Helper()
	dir := b.TempDir()
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		b.Fatal(err)
	}
	dir = resolved
	for _, args := range [][]string{
		{"init", dir},
		{"-C", dir, "config", "user.email", "test@test.com"},
		{"-C", dir, "config", "user.name", "Test"},
		{"-C", dir, "commit", "--allow-empty", "-m", "init"},
	} {
		cmd := exec.Command("git", args...) //nolint:gosec // bench helper
		if out, err := cmd.CombinedOutput(); err != nil {
			b.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}
	return dir
}

func BenchmarkSetFilter(b *testing.B) {
	dir := initBenchRepo(b)
	for range b.N {
		if err := gitutil.SetFilter(dir); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSetDiffTextconv(b *testing.B) {
	dir := initBenchRepo(b)
	for range b.N {
		if err := gitutil.SetDiffTextconv(dir); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkUnsetFilter(b *testing.B) {
	dir := initBenchRepo(b)
	if err := gitutil.SetFilter(dir); err != nil {
		b.Fatal(err)
	}
	if err := gitutil.SetDiffTextconv(dir); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for range b.N {
		if err := gitutil.UnsetFilter(dir); err != nil {
			b.Fatal(err)
		}
		if err := gitutil.SetFilter(dir); err != nil {
			b.Fatal(err)
		}
		if err := gitutil.SetDiffTextconv(dir); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkFilterConfigured(b *testing.B) {
	dir := initBenchRepo(b)
	if err := gitutil.SetFilter(dir); err != nil {
		b.Fatal(err)
	}
	if err := gitutil.SetDiffTextconv(dir); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for range b.N {
		ok, err := gitutil.FilterConfigured(dir)
		if err != nil {
			b.Fatal(err)
		}
		if !ok {
			b.Fatal("expected true")
		}
	}
}

func BenchmarkEnableWorktreeConfig(b *testing.B) {
	dir := initBenchRepo(b)
	for range b.N {
		if err := gitutil.EnableWorktreeConfig(dir); err != nil {
			b.Fatal(err)
		}
	}
}
