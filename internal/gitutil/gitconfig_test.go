package gitutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseEmpty(t *testing.T) {
	f := parseGitConfigFile([]byte{})
	if len(f.lines) != 1 {
		t.Fatalf("expected 1 line (empty), got %d", len(f.lines))
	}
}

func TestParseAndGet(t *testing.T) {
	input := `[core]
	repositoryformatversion = 0
	bare = false
[filter "git-crypt"]
	smudge = encrypten smudge
	clean = encrypten clean
	required = true
# a comment
[diff "git-crypt"]
	textconv = encrypten diff
`
	f := parseGitConfigFile([]byte(input))

	tests := []struct {
		section, subsection, key string
		want                     string
		wantOK                   bool
	}{
		{"core", "", "repositoryformatversion", "0", true},
		{"core", "", "bare", "false", true},
		{"filter", "git-crypt", "smudge", "encrypten smudge", true},
		{"filter", "git-crypt", "clean", "encrypten clean", true},
		{"filter", "git-crypt", "required", "true", true},
		{"diff", "git-crypt", "textconv", "encrypten diff", true},
		{"core", "", "nonexistent", "", false},
		{"filter", "encrypten", "smudge", "", false},
	}
	for _, tt := range tests {
		got, ok := f.Get(tt.section, tt.subsection, tt.key)
		if ok != tt.wantOK || got != tt.want {
			t.Errorf("Get(%q, %q, %q) = (%q, %v), want (%q, %v)",
				tt.section, tt.subsection, tt.key, got, ok, tt.want, tt.wantOK)
		}
	}
}

func TestSetExistingKey(t *testing.T) {
	input := `[filter "git-crypt"]
	smudge = old value
`
	f := parseGitConfigFile([]byte(input))
	f.Set("filter", "git-crypt", "smudge", "new value")

	got, ok := f.Get("filter", "git-crypt", "smudge")
	if !ok || got != "new value" {
		t.Errorf("after Set, Get = (%q, %v), want (%q, true)", got, ok, "new value")
	}
}

func TestSetNewKeyExistingSection(t *testing.T) {
	input := `[filter "git-crypt"]
	smudge = encrypten smudge
`
	f := parseGitConfigFile([]byte(input))
	f.Set("filter", "git-crypt", "clean", "encrypten clean")

	got, ok := f.Get("filter", "git-crypt", "clean")
	if !ok || got != "encrypten clean" {
		t.Errorf("after Set, Get = (%q, %v), want (%q, true)", got, ok, "encrypten clean")
	}

	// Verify smudge is still there.
	got, ok = f.Get("filter", "git-crypt", "smudge")
	if !ok || got != "encrypten smudge" {
		t.Errorf("smudge was lost: (%q, %v)", got, ok)
	}
}

func TestSetNewSection(t *testing.T) {
	input := `[core]
	bare = false
`
	f := parseGitConfigFile([]byte(input))
	f.Set("filter", "encrypten", "smudge", "encrypten smudge")

	got, ok := f.Get("filter", "encrypten", "smudge")
	if !ok || got != "encrypten smudge" {
		t.Errorf("after Set, Get = (%q, %v), want (%q, true)", got, ok, "encrypten smudge")
	}

	// Verify serialization contains the new section.
	out := string(f.Bytes())
	if !strings.Contains(out, `[filter "encrypten"]`) {
		t.Errorf("output missing section header:\n%s", out)
	}
}

func TestSetNoSubsection(t *testing.T) {
	f := parseGitConfigFile([]byte{})
	f.Set("extensions", "", "worktreeConfig", "true")

	got, ok := f.Get("extensions", "", "worktreeconfig")
	if !ok || got != "true" {
		t.Errorf("Get = (%q, %v), want (%q, true)", got, ok, "true")
	}

	out := string(f.Bytes())
	if !strings.Contains(out, "[extensions]") {
		t.Errorf("output missing section header:\n%s", out)
	}
}

func TestRemoveSection(t *testing.T) {
	input := `[core]
	bare = false
[filter "git-crypt"]
	smudge = encrypten smudge
	clean = encrypten clean
[diff "git-crypt"]
	textconv = encrypten diff
`
	f := parseGitConfigFile([]byte(input))

	removed := f.RemoveSection("filter", "git-crypt")
	if !removed {
		t.Error("RemoveSection returned false, expected true")
	}

	_, ok := f.Get("filter", "git-crypt", "smudge")
	if ok {
		t.Error("filter.git-crypt.smudge still exists after RemoveSection")
	}

	// diff.git-crypt should still be there.
	got, ok := f.Get("diff", "git-crypt", "textconv")
	if !ok || got != "encrypten diff" {
		t.Errorf("diff.git-crypt.textconv = (%q, %v), want (%q, true)", got, ok, "encrypten diff")
	}
}

func TestRemoveSectionNonexistent(t *testing.T) {
	f := parseGitConfigFile([]byte(`[core]
	bare = false
`))
	removed := f.RemoveSection("filter", "git-crypt")
	if removed {
		t.Error("RemoveSection returned true for nonexistent section")
	}
}

func TestBytesPreservesComments(t *testing.T) {
	input := `# top comment
[core]
	bare = false
; another comment

[filter "git-crypt"]
	smudge = encrypten smudge`
	f := parseGitConfigFile([]byte(input))
	out := string(f.Bytes())
	if out != input {
		t.Errorf("Bytes() changed content.\nGot:\n%s\nWant:\n%s", out, input)
	}
}

func TestReadWriteRoundtrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config")

	// Read nonexistent file returns empty.
	f, err := readGitConfigFile(path)
	if err != nil {
		t.Fatal(err)
	}

	f.Set("filter", "git-crypt", "smudge", "encrypten smudge")
	f.Set("filter", "git-crypt", "clean", "encrypten clean")

	if err := writeGitConfigFile(path, f); err != nil {
		t.Fatal(err)
	}

	// Read back.
	f2, err := readGitConfigFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := f2.Get("filter", "git-crypt", "smudge")
	if !ok || got != "encrypten smudge" {
		t.Errorf("after roundtrip, Get = (%q, %v)", got, ok)
	}
}

func TestWriteAtomicNoPartial(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config")

	// Write initial content.
	f := parseGitConfigFile([]byte("[core]\n\tbare = false\n"))
	if err := writeGitConfigFile(path, f); err != nil {
		t.Fatal(err)
	}

	// Verify no temp files are left.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".gitconfig-tmp-") {
			t.Errorf("temp file left behind: %s", e.Name())
		}
	}
}
