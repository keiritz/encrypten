package e2e_test

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// setupBenchRepo creates a repo with n encrypted files committed.
// Returns (repoDir, exportedKeyPath).
func setupBenchRepo(b *testing.B, bin, binDir string, n int) (string, string) {
	b.Helper()
	repoDir := filepath.Join(b.TempDir(), "repo")
	for _, args := range [][]string{
		{"init", repoDir},
		{"-C", repoDir, "config", "user.email", "bench@test.com"},
		{"-C", repoDir, "config", "user.name", "Bench"},
		{"-C", repoDir, "commit", "--allow-empty", "-m", "init"},
	} {
		cmd := exec.Command("git", args...) //nolint:gosec // bench args
		if out, err := cmd.CombinedOutput(); err != nil {
			b.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}

	// encrypten init
	initCmd := exec.Command(bin, "init") //nolint:gosec // bench binary
	initCmd.Dir = repoDir
	if out, err := initCmd.CombinedOutput(); err != nil {
		b.Fatalf("init failed: %v\n%s", err, out)
	}

	// Create .gitattributes
	attr := ""
	for i := range n {
		attr += fmt.Sprintf("secret_%04d.txt filter=git-crypt diff=git-crypt\n", i)
	}
	if err := os.WriteFile(filepath.Join(repoDir, ".gitattributes"), []byte(attr), 0600); err != nil {
		b.Fatal(err)
	}

	// Create n secret files (1 KB each)
	content := make([]byte, 1024)
	for i := range content {
		content[i] = byte('A' + (i % 26))
	}
	for i := range n {
		name := fmt.Sprintf("secret_%04d.txt", i)
		if err := os.WriteFile(filepath.Join(repoDir, name), content, 0600); err != nil {
			b.Fatal(err)
		}
	}

	// git add + commit
	addCmd := exec.Command("git", "add", ".") //nolint:gosec // bench args
	addCmd.Dir = repoDir
	addCmd.Env = envWithBinDir(binDir)
	if out, err := addCmd.CombinedOutput(); err != nil {
		b.Fatalf("git add failed: %v\n%s", err, out)
	}
	commitCmd := exec.Command("git", "commit", "-m", "add secrets") //nolint:gosec // bench args
	commitCmd.Dir = repoDir
	commitCmd.Env = envWithBinDir(binDir)
	if out, err := commitCmd.CombinedOutput(); err != nil {
		b.Fatalf("git commit failed: %v\n%s", err, out)
	}

	// Export key
	exportedKey := filepath.Join(b.TempDir(), "exported_key")
	exportCmd := exec.Command(bin, "export-key", exportedKey) //nolint:gosec // bench binary
	exportCmd.Dir = repoDir
	if out, err := exportCmd.CombinedOutput(); err != nil {
		b.Fatalf("export-key failed: %v\n%s", err, out)
	}

	return repoDir, exportedKey
}

// cloneRepo creates a full copy of a repo for each benchmark iteration.
func cloneRepo(b *testing.B, src string) string {
	b.Helper()
	dst := filepath.Join(b.TempDir(), fmt.Sprintf("iter-%d", b.N))
	cmd := exec.Command("git", "clone", "--local", src, dst) //nolint:gosec // bench args
	if out, err := cmd.CombinedOutput(); err != nil {
		b.Fatalf("git clone failed: %v\n%s", err, out)
	}
	// Copy user config for git operations
	for _, args := range [][]string{
		{"-C", dst, "config", "user.email", "bench@test.com"},
		{"-C", dst, "config", "user.name", "Bench"},
	} {
		c := exec.Command("git", args...) //nolint:gosec // bench args
		_ = c.Run()
	}
	return dst
}

func BenchmarkEncryptenLockUnlock(b *testing.B) {
	bin := buildBinaryForBench(b)
	binDir := filepath.Dir(bin)

	for _, n := range []int{1, 10, 50, 100} {
		b.Run(fmt.Sprintf("files=%d", n), func(b *testing.B) {
			// Setup: create a source repo once.
			srcRepo, exportedKey := setupBenchRepo(b, bin, binDir, n)

			b.Run("lock", func(b *testing.B) {
				for range b.N {
					repo := cloneRepo(b, srcRepo)

					// Setup: unlock the clone
					unlockCmd := exec.Command(bin, "unlock", exportedKey) //nolint:gosec // bench binary
					unlockCmd.Dir = repo
					unlockCmd.Env = envWithBinDir(binDir)
					if out, err := unlockCmd.CombinedOutput(); err != nil {
						b.Fatalf("unlock (setup) failed: %v\n%s", err, out)
					}

					b.ResetTimer()

					lockCmd := exec.Command(bin, "lock") //nolint:gosec // bench binary
					lockCmd.Dir = repo
					lockCmd.Env = envWithBinDir(binDir)
					if out, err := lockCmd.CombinedOutput(); err != nil {
						b.Fatalf("lock failed: %v\n%s", err, out)
					}
				}
			})

			b.Run("unlock", func(b *testing.B) {
				for range b.N {
					repo := cloneRepo(b, srcRepo)

					// Setup: unlock then lock to get a locked clone
					unlockCmd := exec.Command(bin, "unlock", exportedKey) //nolint:gosec // bench binary
					unlockCmd.Dir = repo
					unlockCmd.Env = envWithBinDir(binDir)
					if out, err := unlockCmd.CombinedOutput(); err != nil {
						b.Fatalf("unlock (setup) failed: %v\n%s", err, out)
					}
					lockCmd := exec.Command(bin, "lock") //nolint:gosec // bench binary
					lockCmd.Dir = repo
					lockCmd.Env = envWithBinDir(binDir)
					if out, err := lockCmd.CombinedOutput(); err != nil {
						b.Fatalf("lock (setup) failed: %v\n%s", err, out)
					}

					b.ResetTimer()

					unlockCmd2 := exec.Command(bin, "unlock", exportedKey) //nolint:gosec // bench binary
					unlockCmd2.Dir = repo
					unlockCmd2.Env = envWithBinDir(binDir)
					if out, err := unlockCmd2.CombinedOutput(); err != nil {
						b.Fatalf("unlock failed: %v\n%s", err, out)
					}
				}
			})
		})
	}
}

func BenchmarkGitCryptLockUnlock(b *testing.B) {
	gc := buildGitCryptForBench(b)
	bin := buildBinaryForBench(b) // need encrypten for setupBenchRepo
	binDir := filepath.Dir(bin)

	for _, n := range []int{1, 10, 50, 100} {
		b.Run(fmt.Sprintf("files=%d", n), func(b *testing.B) {
			srcRepo, exportedKey := setupBenchRepo(b, bin, binDir, n)

			b.Run("lock", func(b *testing.B) {
				for range b.N {
					repo := cloneRepo(b, srcRepo)

					// Setup: unlock with git-crypt
					unlockCmd := exec.Command(gc, "unlock", exportedKey) //nolint:gosec // bench binary
					unlockCmd.Dir = repo
					if out, err := unlockCmd.CombinedOutput(); err != nil {
						b.Fatalf("git-crypt unlock (setup) failed: %v\n%s", err, out)
					}

					b.ResetTimer()

					lockCmd := exec.Command(gc, "lock") //nolint:gosec // bench binary
					lockCmd.Dir = repo
					if out, err := lockCmd.CombinedOutput(); err != nil {
						b.Fatalf("git-crypt lock failed: %v\n%s", err, out)
					}
				}
			})

			b.Run("unlock", func(b *testing.B) {
				for range b.N {
					repo := cloneRepo(b, srcRepo)

					// Setup: unlock then lock with git-crypt
					unlockCmd := exec.Command(gc, "unlock", exportedKey) //nolint:gosec // bench binary
					unlockCmd.Dir = repo
					if out, err := unlockCmd.CombinedOutput(); err != nil {
						b.Fatalf("git-crypt unlock (setup) failed: %v\n%s", err, out)
					}
					lockCmd := exec.Command(gc, "lock") //nolint:gosec // bench binary
					lockCmd.Dir = repo
					if out, err := lockCmd.CombinedOutput(); err != nil {
						b.Fatalf("git-crypt lock (setup) failed: %v\n%s", err, out)
					}

					b.ResetTimer()

					unlockCmd2 := exec.Command(gc, "unlock", exportedKey) //nolint:gosec // bench binary
					unlockCmd2.Dir = repo
					if out, err := unlockCmd2.CombinedOutput(); err != nil {
						b.Fatalf("git-crypt unlock failed: %v\n%s", err, out)
					}
				}
			})
		})
	}
}

func buildBinaryForBench(b *testing.B) string {
	b.Helper()
	dir := b.TempDir()
	bin := filepath.Join(dir, "encrypten")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/encrypten/") //nolint:gosec // build args
	cmd.Dir = filepath.Join("..")
	out, err := cmd.CombinedOutput()
	if err != nil {
		b.Fatalf("build failed: %v\n%s", err, out)
	}
	return bin
}

func buildGitCryptForBench(b *testing.B) string {
	b.Helper()
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
		makeCmd.Env = append(os.Environ(), "CXXFLAGS="+cxxflags, "LDFLAGS="+ldflags)
		if out, err := makeCmd.CombinedOutput(); err != nil {
			gitCryptErr = fmt.Errorf("make failed: %w\n%s", err, out)
			return
		}

		gitCryptBin = filepath.Join(dir, "git-crypt")
	})
	if gitCryptErr != nil {
		b.Skipf("git-crypt build failed: %v", gitCryptErr)
	}
	return gitCryptBin
}
