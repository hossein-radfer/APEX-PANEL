package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// repoURL is this project's own GitHub repository -- hardcoded rather than
// read from any config, since `go install`-style self-update only makes
// sense against the one canonical upstream this binary itself was built
// from (mirrors serviceName's own "exactly one supported deployment shape"
// reasoning).
const repoURL = "https://github.com/hossein-radfer/APEX-PANEL.git"

// runCmd runs a command with its stdout/stderr streamed live to this
// terminal -- every step below (git clone, npm install, go build) can take
// anywhere from seconds to minutes, and an operator watching an SSH
// session needs to see it's making progress, not stare at a silent
// terminal wondering if the menu has hung.
func runCmd(dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// builtSource is the result of a successful buildFromSource call: the
// built binary, the cloned repo it came from (so a caller like
// actionReinstall can reach into deploy/ for other files without cloning
// a second time), and a cleanup func that removes the whole temp
// directory tree once the caller is done with both.
type builtSource struct {
	BinaryPath string
	RepoDir    string
	Cleanup    func()
}

// buildFromSource clones repoURL into a fresh temp directory and builds
// the frontend and the Go binary exactly as deploy/README.md documents
// doing by hand. The caller (actionUpdate/actionReinstall) decides what
// to do with the result -- this function never touches /opt/mwp or the
// systemd unit itself, so a build failure here never leaves a partially-
// replaced install.
func buildFromSource() (*builtSource, error) {
	tmpDir, err := os.MkdirTemp("", "mwp-build-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp build directory: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(tmpDir) }

	repoDir := filepath.Join(tmpDir, "repo")

	fmt.Println("==> Cloning", repoURL)
	if err := runCmd(tmpDir, "git", "clone", "--depth", "1", repoURL, repoDir); err != nil {
		cleanup()
		return nil, fmt.Errorf("git clone failed: %w", err)
	}

	uiDir := filepath.Join(repoDir, "ui")
	fmt.Println("==> Installing frontend dependencies (npm install)")
	if err := runCmd(uiDir, "npm", "install"); err != nil {
		cleanup()
		return nil, fmt.Errorf("npm install failed: %w", err)
	}

	fmt.Println("==> Building frontend (npm run build)")
	if err := runCmd(uiDir, "npm", "run", "build"); err != nil {
		cleanup()
		return nil, fmt.Errorf("frontend build failed: %w", err)
	}

	fmt.Println("==> Building mwp binary (go build)")
	outputPath := filepath.Join(tmpDir, "mwp")
	buildCmd := exec.Command("go", "build", "-o", outputPath, "-ldflags=-s -w", "./api/cmd")
	buildCmd.Dir = repoDir
	buildCmd.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS=linux", "GOARCH=amd64")
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	if err := buildCmd.Run(); err != nil {
		cleanup()
		return nil, fmt.Errorf("go build failed: %w", err)
	}

	return &builtSource{BinaryPath: outputPath, RepoDir: repoDir, Cleanup: cleanup}, nil
}

// installBinary atomically replaces /usr/local/bin/mwp -- a rename within
// the same filesystem (both paths under tmpDir's parent /tmp and the
// final destination are expected to be on the same root filesystem on a
// typical single-disk VPS) so there is never a moment where the path
// exists but is empty/truncated, unlike a copy that could be interrupted
// mid-write. Falls back to copy+remove if the rename can't cross
// filesystems (e.g. /tmp is a separate tmpfs mount).
func installBinary(builtPath string) error {
	const dest = "/usr/local/bin/mwp"

	if err := os.Chmod(builtPath, 0o755); err != nil {
		return fmt.Errorf("failed to make new binary executable: %w", err)
	}

	if err := os.Rename(builtPath, dest); err == nil {
		return nil
	}

	// Cross-device rename failed -- copy the bytes across manually.
	data, err := os.ReadFile(builtPath)
	if err != nil {
		return fmt.Errorf("failed to read built binary: %w", err)
	}
	if err := os.WriteFile(dest, data, 0o755); err != nil {
		return fmt.Errorf("failed to write %s: %w", dest, err)
	}
	return nil
}
