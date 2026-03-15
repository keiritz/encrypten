package gitutil

import (
	"bytes"
	"os/exec"
	"strings"
)

// EncryptedFileEntry represents a file with a git-crypt filter attribute.
type EncryptedFileEntry struct {
	Path    string
	KeyName string
}

// ListEncryptedFiles returns files that have filter=git-crypt set in .gitattributes.
// It uses git check-attr to let git handle attribute resolution (including
// nested .gitattributes and glob patterns).
func ListEncryptedFiles(dir string) ([]EncryptedFileEntry, error) {
	// Step 1: Get tracked files.
	lsFiles := exec.Command("git", "ls-files", "-z") // #nosec G204
	lsFiles.Dir = dir
	lsOut, err := lsFiles.Output()
	if err != nil {
		return nil, err
	}

	// No tracked files → nothing to check.
	if len(lsOut) == 0 {
		return nil, nil
	}

	// Step 2: Check filter attribute for each file.
	checkAttr := exec.Command("git", "check-attr", "filter", "-z", "--stdin") // #nosec G204
	checkAttr.Dir = dir
	checkAttr.Stdin = bytes.NewReader(lsOut)
	attrOut, err := checkAttr.Output()
	if err != nil {
		return nil, err
	}

	// Step 3: Parse NUL-delimited output.
	// Format with -z: path NUL attr NUL value NUL (repeating triplets).
	return parseCheckAttrOutput(attrOut), nil
}

// parseCheckAttrOutput parses the NUL-delimited output of git check-attr -z.
// Each record is a triplet: path \0 attribute \0 value \0.
func parseCheckAttrOutput(data []byte) []EncryptedFileEntry {
	fields := strings.Split(string(data), "\x00")

	var entries []EncryptedFileEntry
	// Process triplets; ignore trailing incomplete group.
	for i := 0; i+2 < len(fields); i += 3 {
		path := fields[i]
		value := fields[i+2]

		entry, ok := parseFilterValue(path, value)
		if ok {
			entries = append(entries, entry)
		}
	}
	return entries
}

// parseFilterValue checks if a filter attribute value indicates git-crypt
// encryption and returns the corresponding entry.
func parseFilterValue(path, value string) (EncryptedFileEntry, bool) {
	if value == "git-crypt" {
		return EncryptedFileEntry{Path: path, KeyName: "default"}, true
	}
	if keyName, ok := strings.CutPrefix(value, "git-crypt-"); ok {
		return EncryptedFileEntry{Path: path, KeyName: keyName}, true
	}
	return EncryptedFileEntry{}, false
}
