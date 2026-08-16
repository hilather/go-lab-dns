package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// emptyTreeSHA is git's well-known empty tree (no files).
const emptyTreeSHA = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"

func gitOutput(root string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), msg)
	}
	return strings.TrimSpace(stdout.String()), nil
}

func gitBytes(root string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("git %s: %s", strings.Join(args, " "), msg)
	}
	return stdout.Bytes(), nil
}

func rejectDirty(root string) error {
	out, err := gitOutput(root, "status", "--porcelain")
	if err != nil {
		return err
	}
	if out != "" {
		return fmt.Errorf("dirty worktree; commit or stash before release-diff:\n%s", out)
	}
	return nil
}

func verifyRef(root, ref string) error {
	if ref == emptyTreeSHA {
		return nil
	}
	_, err := gitOutput(root, "rev-parse", "--verify", ref+"^{object}")
	if err != nil {
		return fmt.Errorf("unknown git ref %q: %w", ref, err)
	}
	return nil
}

func fileAt(root, ref, rel string) (data []byte, exists bool, err error) {
	if ref == emptyTreeSHA {
		return nil, false, nil
	}
	spec := ref + ":" + filepath.ToSlash(rel)
	data, err = gitBytes(root, "show", spec)
	if err != nil {
		if isMissingPath(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return data, true, nil
}

func isMissingPath(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "does not exist") ||
		strings.Contains(msg, "exists on disk, but not in")
}

func previousTagName(root string) (string, error) {
	// Prefer the nearest reachable tag that is not HEAD itself.
	out, err := gitOutput(root, "describe", "--tags", "--abbrev=0", "HEAD^")
	if err == nil && out != "" {
		return out, nil
	}
	// No previous tag: empty tree so every surface is "added".
	return emptyTreeSHA, nil
}

func repoRootFrom(wd string) (string, error) {
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found from %s", wd)
		}
		dir = parent
	}
}
