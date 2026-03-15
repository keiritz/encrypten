package gitutil_test

import (
	"testing"

	"github.com/keiritz/encrypten/internal/gitutil"
)

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
