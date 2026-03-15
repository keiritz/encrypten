package e2e_test

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
)

var (
	gitCryptOnce sync.Once
	gitCryptBin  string
	gitCryptErr  error
)

// buildGitCrypt clones and builds git-crypt v0.8.0 from source.
// The result is cached via sync.Once so all tests share a single build.
func buildGitCrypt(t *testing.T) string {
	t.Helper()
	gitCryptOnce.Do(func() {
		dir, err := os.MkdirTemp("", "git-crypt-build-*")
		if err != nil {
			gitCryptErr = fmt.Errorf("create temp dir: %w", err)
			return
		}

		cloneCmd := exec.Command("git", "clone", "--depth", "1", "--branch", "0.8.0", //nolint:gosec // constant args
			"https://github.com/AGWA/git-crypt.git", dir)
		if out, err := cloneCmd.CombinedOutput(); err != nil {
			gitCryptErr = fmt.Errorf("git clone failed: %w\n%s", err, out)
			return
		}

		// Detect OpenSSL flags via pkg-config (macOS homebrew).
		cxxflags := ""
		ldflags := ""
		if pkgOut, err := exec.Command("pkg-config", "--cflags", "libcrypto").Output(); err == nil { //nolint:gosec // build flags
			cxxflags = string(bytes.TrimSpace(pkgOut))
		}
		if pkgOut, err := exec.Command("pkg-config", "--libs", "libcrypto").Output(); err == nil { //nolint:gosec // build flags
			ldflags = string(bytes.TrimSpace(pkgOut))
		}

		makeCmd := exec.Command("make") //nolint:gosec // build
		makeCmd.Dir = dir
		makeCmd.Env = append(os.Environ(),
			"CXXFLAGS="+cxxflags,
			"LDFLAGS="+ldflags,
		)
		if out, err := makeCmd.CombinedOutput(); err != nil {
			gitCryptErr = fmt.Errorf("make failed: %w\n%s", err, out)
			return
		}

		gitCryptBin = filepath.Join(dir, "git-crypt")
		if _, err := os.Stat(gitCryptBin); err != nil {
			gitCryptErr = fmt.Errorf("git-crypt binary not found: %w", err)
			return
		}
	})

	if gitCryptErr != nil {
		t.Skipf("git-crypt build failed: %v", gitCryptErr)
	}
	return gitCryptBin
}

// verifyPlaintext checks that filename in repoDir contains the expected plaintext
// and does not start with the GITCRYPT header.
func verifyPlaintext(t *testing.T, repoDir, filename, expected string) {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(repoDir, filename)) //nolint:gosec // test path
	if err != nil {
		t.Fatalf("reading %s: %v", filename, err)
	}
	if bytes.HasPrefix(content, []byte("\x00GITCRYPT\x00")) {
		t.Errorf("%s is still encrypted (has GITCRYPT header)", filename)
		return
	}
	if string(content) != expected {
		t.Errorf("%s content = %q, want %q", filename, string(content), expected)
	}
}

// clearEncryptenState removes all encrypten filter/diff config (both shared
// and per-worktree) so that git-crypt can set its own filter cleanly.
// This is needed before git-crypt unlock in interop tests because encrypten's
// per-WT override and shared filter would otherwise take precedence.
func clearEncryptenState(t *testing.T, repoDir string) {
	t.Helper()
	for _, scope := range []string{"--worktree", ""} {
		for _, section := range []string{"filter.git-crypt", "diff.git-crypt"} {
			args := []string{"config"}
			if scope != "" {
				args = append(args, scope)
			}
			args = append(args, "--remove-section", section)
			cmd := exec.Command("git", args...) //nolint:gosec // test args
			cmd.Dir = repoDir
			_ = cmd.Run() // ignore errors (section may not exist)
		}
	}
}

// verifyEncrypted checks that filename in repoDir starts with the GITCRYPT header.
func verifyEncrypted(t *testing.T, repoDir, filename string) {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(repoDir, filename)) //nolint:gosec // test path
	if err != nil {
		t.Fatalf("reading %s: %v", filename, err)
	}
	if !bytes.HasPrefix(content, []byte("\x00GITCRYPT\x00")) {
		t.Errorf("%s should be encrypted (start with \\x00GITCRYPT\\x00), got prefix %q",
			filename, content[:min(len(content), 20)])
	}
}

func TestGitCryptInitEncryptenUnlock(t *testing.T) {
	gc := buildGitCrypt(t)
	bin := buildBinary(t)
	binDir := filepath.Dir(bin)
	repoDir := initRepo(t)

	// git-crypt init
	cmd := exec.Command(gc, "init") //nolint:gosec // test binary
	cmd.Dir = repoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git-crypt init failed: %v\n%s", err, out)
	}

	// Create .gitattributes and secret file.
	if err := os.WriteFile(filepath.Join(repoDir, ".gitattributes"),
		[]byte("secret.txt filter=git-crypt diff=git-crypt\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "secret.txt"),
		[]byte("hello from git-crypt\n"), 0600); err != nil {
		t.Fatal(err)
	}

	// git add and commit
	addCmd := exec.Command("git", "add", ".") //nolint:gosec // test args
	addCmd.Dir = repoDir
	if out, err := addCmd.CombinedOutput(); err != nil {
		t.Fatalf("git add failed: %v\n%s", err, out)
	}
	commitCmd := exec.Command("git", "commit", "-m", "add secret") //nolint:gosec // test args
	commitCmd.Dir = repoDir
	if out, err := commitCmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit failed: %v\n%s", err, out)
	}

	// Export key
	exportedKey := filepath.Join(t.TempDir(), "exported_key")
	exportCmd := exec.Command(gc, "export-key", exportedKey) //nolint:gosec // test binary
	exportCmd.Dir = repoDir
	if out, err := exportCmd.CombinedOutput(); err != nil {
		t.Fatalf("git-crypt export-key failed: %v\n%s", err, out)
	}

	// git-crypt lock
	lockCmd := exec.Command(gc, "lock") //nolint:gosec // test binary
	lockCmd.Dir = repoDir
	if out, err := lockCmd.CombinedOutput(); err != nil {
		t.Fatalf("git-crypt lock failed: %v\n%s", err, out)
	}

	verifyEncrypted(t, repoDir, "secret.txt")

	// encrypten unlock
	unlockCmd := exec.Command(bin, "unlock", exportedKey) //nolint:gosec // test binary
	unlockCmd.Dir = repoDir
	unlockCmd.Env = envWithBinDir(binDir)
	if out, err := unlockCmd.CombinedOutput(); err != nil {
		t.Fatalf("encrypten unlock failed: %v\n%s", err, out)
	}

	verifyPlaintext(t, repoDir, "secret.txt", "hello from git-crypt\n")
}

func TestEncryptenInitGitCryptUnlock(t *testing.T) {
	gc := buildGitCrypt(t)
	bin := buildBinary(t)
	binDir := filepath.Dir(bin)
	repoDir := initRepo(t)

	// encrypten init
	cmd := exec.Command(bin, "init") //nolint:gosec // test binary
	cmd.Dir = repoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("encrypten init failed: %v\n%s", err, out)
	}

	// Create .gitattributes and secret file.
	if err := os.WriteFile(filepath.Join(repoDir, ".gitattributes"),
		[]byte("secret.txt filter=git-crypt diff=git-crypt\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "secret.txt"),
		[]byte("hello from encrypten\n"), 0600); err != nil {
		t.Fatal(err)
	}

	// git add and commit (encrypten clean filter encrypts)
	addCmd := exec.Command("git", "add", ".") //nolint:gosec // test args
	addCmd.Dir = repoDir
	addCmd.Env = envWithBinDir(binDir)
	if out, err := addCmd.CombinedOutput(); err != nil {
		t.Fatalf("git add failed: %v\n%s", err, out)
	}
	commitCmd := exec.Command("git", "commit", "-m", "add secret") //nolint:gosec // test args
	commitCmd.Dir = repoDir
	commitCmd.Env = envWithBinDir(binDir)
	if out, err := commitCmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit failed: %v\n%s", err, out)
	}

	// Export key
	exportedKey := filepath.Join(t.TempDir(), "exported_key")
	exportCmd := exec.Command(bin, "export-key", exportedKey) //nolint:gosec // test binary
	exportCmd.Dir = repoDir
	if out, err := exportCmd.CombinedOutput(); err != nil {
		t.Fatalf("encrypten export-key failed: %v\n%s", err, out)
	}

	// encrypten lock
	lockCmd := exec.Command(bin, "lock") //nolint:gosec // test binary
	lockCmd.Dir = repoDir
	lockCmd.Env = envWithBinDir(binDir)
	if out, err := lockCmd.CombinedOutput(); err != nil {
		t.Fatalf("encrypten lock failed: %v\n%s", err, out)
	}

	verifyEncrypted(t, repoDir, "secret.txt")

	// Remove per-WT override left by encrypten lock so git-crypt's
	// filter takes effect (git-crypt doesn't know about WT overrides).
	clearEncryptenState(t, repoDir)

	// git-crypt unlock
	unlockCmd := exec.Command(gc, "unlock", exportedKey) //nolint:gosec // test binary
	unlockCmd.Dir = repoDir
	if out, err := unlockCmd.CombinedOutput(); err != nil {
		t.Fatalf("git-crypt unlock failed: %v\n%s", err, out)
	}

	verifyPlaintext(t, repoDir, "secret.txt", "hello from encrypten\n")
}

func TestGitCryptEncryptEncryptenDecrypt(t *testing.T) {
	gc := buildGitCrypt(t)
	bin := buildBinary(t)
	binDir := filepath.Dir(bin)
	repoDir := initRepo(t)

	// git-crypt init
	cmd := exec.Command(gc, "init") //nolint:gosec // test binary
	cmd.Dir = repoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git-crypt init failed: %v\n%s", err, out)
	}

	// Create .gitattributes and secret file.
	if err := os.WriteFile(filepath.Join(repoDir, ".gitattributes"),
		[]byte("secret.txt filter=git-crypt diff=git-crypt\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "secret.txt"),
		[]byte("encrypted by git-crypt\n"), 0600); err != nil {
		t.Fatal(err)
	}

	// git add and commit
	addCmd := exec.Command("git", "add", ".") //nolint:gosec // test args
	addCmd.Dir = repoDir
	if out, err := addCmd.CombinedOutput(); err != nil {
		t.Fatalf("git add failed: %v\n%s", err, out)
	}
	commitCmd := exec.Command("git", "commit", "-m", "add secret") //nolint:gosec // test args
	commitCmd.Dir = repoDir
	if out, err := commitCmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit failed: %v\n%s", err, out)
	}

	// Export key
	exportedKey := filepath.Join(t.TempDir(), "exported_key")
	exportCmd := exec.Command(gc, "export-key", exportedKey) //nolint:gosec // test binary
	exportCmd.Dir = repoDir
	if out, err := exportCmd.CombinedOutput(); err != nil {
		t.Fatalf("git-crypt export-key failed: %v\n%s", err, out)
	}

	// git-crypt lock
	lockCmd := exec.Command(gc, "lock") //nolint:gosec // test binary
	lockCmd.Dir = repoDir
	if out, err := lockCmd.CombinedOutput(); err != nil {
		t.Fatalf("git-crypt lock failed: %v\n%s", err, out)
	}

	verifyEncrypted(t, repoDir, "secret.txt")

	// encrypten unlock
	unlockCmd := exec.Command(bin, "unlock", exportedKey) //nolint:gosec // test binary
	unlockCmd.Dir = repoDir
	unlockCmd.Env = envWithBinDir(binDir)
	if out, err := unlockCmd.CombinedOutput(); err != nil {
		t.Fatalf("encrypten unlock failed: %v\n%s", err, out)
	}

	verifyPlaintext(t, repoDir, "secret.txt", "encrypted by git-crypt\n")
}

func TestEncryptenEncryptGitCryptDecrypt(t *testing.T) {
	gc := buildGitCrypt(t)
	bin := buildBinary(t)
	binDir := filepath.Dir(bin)
	repoDir := initRepo(t)

	// encrypten init
	cmd := exec.Command(bin, "init") //nolint:gosec // test binary
	cmd.Dir = repoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("encrypten init failed: %v\n%s", err, out)
	}

	// Create .gitattributes and secret file.
	if err := os.WriteFile(filepath.Join(repoDir, ".gitattributes"),
		[]byte("secret.txt filter=git-crypt diff=git-crypt\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "secret.txt"),
		[]byte("encrypted by encrypten\n"), 0600); err != nil {
		t.Fatal(err)
	}

	// git add and commit (encrypten clean filter encrypts)
	addCmd := exec.Command("git", "add", ".") //nolint:gosec // test args
	addCmd.Dir = repoDir
	addCmd.Env = envWithBinDir(binDir)
	if out, err := addCmd.CombinedOutput(); err != nil {
		t.Fatalf("git add failed: %v\n%s", err, out)
	}
	commitCmd := exec.Command("git", "commit", "-m", "add secret") //nolint:gosec // test args
	commitCmd.Dir = repoDir
	commitCmd.Env = envWithBinDir(binDir)
	if out, err := commitCmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit failed: %v\n%s", err, out)
	}

	// Export key
	exportedKey := filepath.Join(t.TempDir(), "exported_key")
	exportCmd := exec.Command(bin, "export-key", exportedKey) //nolint:gosec // test binary
	exportCmd.Dir = repoDir
	if out, err := exportCmd.CombinedOutput(); err != nil {
		t.Fatalf("encrypten export-key failed: %v\n%s", err, out)
	}

	// encrypten lock
	lockCmd := exec.Command(bin, "lock") //nolint:gosec // test binary
	lockCmd.Dir = repoDir
	lockCmd.Env = envWithBinDir(binDir)
	if out, err := lockCmd.CombinedOutput(); err != nil {
		t.Fatalf("encrypten lock failed: %v\n%s", err, out)
	}

	verifyEncrypted(t, repoDir, "secret.txt")

	// Remove per-WT override left by encrypten lock.
	clearEncryptenState(t, repoDir)

	// git-crypt unlock
	unlockCmd := exec.Command(gc, "unlock", exportedKey) //nolint:gosec // test binary
	unlockCmd.Dir = repoDir
	if out, err := unlockCmd.CombinedOutput(); err != nil {
		t.Fatalf("git-crypt unlock failed: %v\n%s", err, out)
	}

	verifyPlaintext(t, repoDir, "secret.txt", "encrypted by encrypten\n")
}

func TestMixedUsage(t *testing.T) {
	gc := buildGitCrypt(t)
	bin := buildBinary(t)
	binDir := filepath.Dir(bin)
	repoDir := initRepo(t)

	// Phase 1: git-crypt init + first secret
	initCmd := exec.Command(gc, "init") //nolint:gosec // test binary
	initCmd.Dir = repoDir
	if out, err := initCmd.CombinedOutput(); err != nil {
		t.Fatalf("git-crypt init failed: %v\n%s", err, out)
	}

	if err := os.WriteFile(filepath.Join(repoDir, ".gitattributes"),
		[]byte("secret*.txt filter=git-crypt diff=git-crypt\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "secret1.txt"),
		[]byte("secret one\n"), 0600); err != nil {
		t.Fatal(err)
	}

	addCmd := exec.Command("git", "add", ".") //nolint:gosec // test args
	addCmd.Dir = repoDir
	if out, err := addCmd.CombinedOutput(); err != nil {
		t.Fatalf("git add failed: %v\n%s", err, out)
	}
	commitCmd := exec.Command("git", "commit", "-m", "add secret1") //nolint:gosec // test args
	commitCmd.Dir = repoDir
	if out, err := commitCmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit failed: %v\n%s", err, out)
	}

	// Export key
	exportedKey := filepath.Join(t.TempDir(), "exported_key")
	exportCmd := exec.Command(gc, "export-key", exportedKey) //nolint:gosec // test binary
	exportCmd.Dir = repoDir
	if out, err := exportCmd.CombinedOutput(); err != nil {
		t.Fatalf("git-crypt export-key failed: %v\n%s", err, out)
	}

	// Phase 2: git-crypt lock → encrypten unlock → add secret2
	lockCmd := exec.Command(gc, "lock") //nolint:gosec // test binary
	lockCmd.Dir = repoDir
	if out, err := lockCmd.CombinedOutput(); err != nil {
		t.Fatalf("git-crypt lock failed: %v\n%s", err, out)
	}

	unlockCmd := exec.Command(bin, "unlock", exportedKey) //nolint:gosec // test binary
	unlockCmd.Dir = repoDir
	unlockCmd.Env = envWithBinDir(binDir)
	if out, err := unlockCmd.CombinedOutput(); err != nil {
		t.Fatalf("encrypten unlock failed: %v\n%s", err, out)
	}

	verifyPlaintext(t, repoDir, "secret1.txt", "secret one\n")

	// Add secret2.txt (committed via encrypten clean filter)
	if err := os.WriteFile(filepath.Join(repoDir, "secret2.txt"),
		[]byte("secret two\n"), 0600); err != nil {
		t.Fatal(err)
	}

	addCmd2 := exec.Command("git", "add", "secret2.txt") //nolint:gosec // test args
	addCmd2.Dir = repoDir
	addCmd2.Env = envWithBinDir(binDir)
	if out, err := addCmd2.CombinedOutput(); err != nil {
		t.Fatalf("git add secret2.txt failed: %v\n%s", err, out)
	}
	commitCmd2 := exec.Command("git", "commit", "-m", "add secret2") //nolint:gosec // test args
	commitCmd2.Dir = repoDir
	commitCmd2.Env = envWithBinDir(binDir)
	if out, err := commitCmd2.CombinedOutput(); err != nil {
		t.Fatalf("git commit failed: %v\n%s", err, out)
	}

	// Phase 3: encrypten lock → verify encrypted → git-crypt unlock
	lockCmd2 := exec.Command(bin, "lock") //nolint:gosec // test binary
	lockCmd2.Dir = repoDir
	lockCmd2.Env = envWithBinDir(binDir)
	if out, err := lockCmd2.CombinedOutput(); err != nil {
		t.Fatalf("encrypten lock failed: %v\n%s", err, out)
	}

	verifyEncrypted(t, repoDir, "secret1.txt")
	verifyEncrypted(t, repoDir, "secret2.txt")

	// Remove per-WT override left by encrypten lock.
	clearEncryptenState(t, repoDir)

	unlockCmd2 := exec.Command(gc, "unlock", exportedKey) //nolint:gosec // test binary
	unlockCmd2.Dir = repoDir
	if out, err := unlockCmd2.CombinedOutput(); err != nil {
		t.Fatalf("git-crypt unlock failed: %v\n%s", err, out)
	}

	verifyPlaintext(t, repoDir, "secret1.txt", "secret one\n")
	verifyPlaintext(t, repoDir, "secret2.txt", "secret two\n")
}
