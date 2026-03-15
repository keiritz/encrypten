package gitutil

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Line types for configLine.
const (
	lineOther    = iota // comment, blank, or unparsed
	lineSection         // [section] or [section "subsection"]
	lineKeyValue        // key = value
)

// configLine represents a single line in a git config file.
type configLine struct {
	raw        string // original text (preserved for comments/blanks)
	lineType   int
	section    string // e.g. "filter"
	subsection string // e.g. "git-crypt"
	key        string // e.g. "smudge"
	value      string // e.g. "encrypten smudge"
}

// gitConfigFile is an in-memory representation of a git config file.
type gitConfigFile struct {
	lines []configLine
}

// parseGitConfigFile parses raw git config bytes into a gitConfigFile.
func parseGitConfigFile(data []byte) *gitConfigFile {
	f := &gitConfigFile{}
	var curSection, curSubsection string

	for rawLine := range strings.SplitSeq(string(data), "\n") {
		trimmed := strings.TrimSpace(rawLine)

		// Blank or comment line.
		if trimmed == "" || trimmed[0] == '#' || trimmed[0] == ';' {
			f.lines = append(f.lines, configLine{raw: rawLine, lineType: lineOther})
			continue
		}

		// Section header: [section] or [section "subsection"]
		if trimmed[0] == '[' {
			cl := configLine{raw: rawLine, lineType: lineSection}
			end := strings.IndexByte(trimmed, ']')
			if end > 0 {
				inside := trimmed[1:end]
				if qIdx := strings.IndexByte(inside, '"'); qIdx >= 0 {
					cl.section = strings.TrimSpace(strings.ToLower(inside[:qIdx]))
					// Extract subsection between quotes.
					rest := inside[qIdx+1:]
					if endQ := strings.IndexByte(rest, '"'); endQ >= 0 {
						cl.subsection = rest[:endQ]
					}
				} else {
					cl.section = strings.TrimSpace(strings.ToLower(inside))
				}
			}
			curSection = cl.section
			curSubsection = cl.subsection
			f.lines = append(f.lines, cl)
			continue
		}

		// Key-value line.
		cl := configLine{raw: rawLine, lineType: lineKeyValue, section: curSection, subsection: curSubsection}
		if eqIdx := strings.IndexByte(trimmed, '='); eqIdx >= 0 {
			cl.key = strings.TrimSpace(strings.ToLower(trimmed[:eqIdx]))
			cl.value = strings.TrimSpace(trimmed[eqIdx+1:])
		} else {
			// Boolean key with no value (e.g., "bare").
			cl.key = strings.TrimSpace(strings.ToLower(trimmed))
			cl.value = "true"
		}
		f.lines = append(f.lines, cl)
	}

	return f
}

// Get returns the value for the given section/subsection/key, or ("", false).
func (f *gitConfigFile) Get(section, subsection, key string) (string, bool) {
	section = strings.ToLower(section)
	key = strings.ToLower(key)
	// Last match wins (git behavior).
	var result string
	var found bool
	for _, cl := range f.lines {
		if cl.lineType == lineKeyValue && cl.section == section && cl.subsection == subsection && cl.key == key {
			result = cl.value
			found = true
		}
	}
	return result, found
}

// Set sets or updates a key in the given section/subsection.
// If the key already exists, the last occurrence is updated.
// If the section exists but the key does not, the key is appended after the last line of the section.
// If the section does not exist, it is appended at the end of the file.
func (f *gitConfigFile) Set(section, subsection, key, value string) {
	sectionLower := strings.ToLower(section)
	keyLower := strings.ToLower(key)

	// Find last matching key-value line.
	lastKVIdx := -1
	for i, cl := range f.lines {
		if cl.lineType == lineKeyValue && cl.section == sectionLower && cl.subsection == subsection && cl.key == keyLower {
			lastKVIdx = i
		}
	}
	if lastKVIdx >= 0 {
		f.lines[lastKVIdx].value = value
		f.lines[lastKVIdx].raw = "\t" + keyLower + " = " + value
		return
	}

	// Find last line belonging to the section.
	lastSectionIdx := -1
	for i, cl := range f.lines {
		if cl.section == sectionLower && cl.subsection == subsection && (cl.lineType == lineSection || cl.lineType == lineKeyValue) {
			lastSectionIdx = i
		}
	}
	if lastSectionIdx >= 0 {
		newLine := configLine{
			raw:        "\t" + keyLower + " = " + value,
			lineType:   lineKeyValue,
			section:    sectionLower,
			subsection: subsection,
			key:        keyLower,
			value:      value,
		}
		// Insert after lastSectionIdx.
		f.lines = append(f.lines[:lastSectionIdx+1], append([]configLine{newLine}, f.lines[lastSectionIdx+1:]...)...)
		return
	}

	// Section does not exist — create it.
	var header string
	if subsection != "" {
		header = fmt.Sprintf("[%s %q]", sectionLower, subsection)
	} else {
		header = fmt.Sprintf("[%s]", sectionLower)
	}
	f.lines = append(f.lines, configLine{
		raw:        header,
		lineType:   lineSection,
		section:    sectionLower,
		subsection: subsection,
	})
	f.lines = append(f.lines, configLine{
		raw:        "\t" + keyLower + " = " + value,
		lineType:   lineKeyValue,
		section:    sectionLower,
		subsection: subsection,
		key:        keyLower,
		value:      value,
	})
}

// RemoveSection removes all lines belonging to the given section/subsection.
// Returns true if any lines were removed.
func (f *gitConfigFile) RemoveSection(section, subsection string) bool {
	sectionLower := strings.ToLower(section)
	var kept []configLine
	removed := false
	for _, cl := range f.lines {
		if (cl.lineType == lineSection || cl.lineType == lineKeyValue) && cl.section == sectionLower && cl.subsection == subsection {
			removed = true
			continue
		}
		kept = append(kept, cl)
	}
	if removed {
		f.lines = kept
	}
	return removed
}

// Bytes serializes the config file back to bytes, preserving original formatting.
// The output always ends with a trailing newline to match git's convention.
func (f *gitConfigFile) Bytes() []byte {
	var buf bytes.Buffer
	for i, cl := range f.lines {
		if i > 0 {
			buf.WriteByte('\n')
		}
		buf.WriteString(cl.raw)
	}
	// Ensure trailing newline — git always writes config files with one.
	if buf.Len() > 0 && buf.Bytes()[buf.Len()-1] != '\n' {
		buf.WriteByte('\n')
	}
	return buf.Bytes()
}

// readGitConfigFile reads and parses a git config file.
// If the file does not exist, an empty config is returned.
func readGitConfigFile(path string) (*gitConfigFile, error) {
	data, err := os.ReadFile(path) //nolint:gosec // git config file path
	if err != nil {
		if os.IsNotExist(err) {
			return &gitConfigFile{}, nil
		}
		return nil, err
	}
	return parseGitConfigFile(data), nil
}

// writeGitConfigFile atomically writes a git config file using temp+rename.
func writeGitConfigFile(path string, f *gitConfigFile) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil { //nolint:gosec // git dir permissions
		return err
	}
	tmp, err := os.CreateTemp(dir, ".gitconfig-tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(f.Bytes()); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, path)
}
