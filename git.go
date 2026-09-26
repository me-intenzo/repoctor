package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
)

// shellQuote wraps a path in quotes only when it contains whitespace, so the
// suggested commands we print stay copy-pasteable.
func shellQuote(s string) string {
	if strings.ContainsAny(s, " \t") {
		return `"` + s + `"`
	}
	return s
}

// runGit executes git inside repoPath and returns its stdout. stderr is folded
// into the error so callers get git's own explanation of the failure.
func runGit(repoPath string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = repoPath

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return nil, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, msg)
		}
		return nil, fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return stdout.Bytes(), nil
}

// gitLines runs git and returns its stdout split into non-empty lines.
func gitLines(repoPath string, args ...string) ([]string, error) {
	out, err := runGit(repoPath, args...)
	if err != nil {
		return nil, err
	}
	var lines []string
	for _, line := range strings.Split(string(out), "\n") {
		if line = strings.TrimRight(line, "\r"); line != "" {
			lines = append(lines, line)
		}
	}
	return lines, nil
}

// trackedFiles returns the set of files git knows about, as slash-separated
// paths relative to the repo root. The second result is false when repoPath is
// not a git repo, which lets callers fall back to filesystem-only scanning.
func trackedFiles(repoPath string) (map[string]bool, bool) {
	out, err := runGit(repoPath, "ls-files", "-z")
	if err != nil {
		return nil, false
	}
	files := make(map[string]bool)
	for _, name := range strings.Split(string(out), "\x00") {
		if name != "" {
			files[name] = true
		}
	}
	return files, true
}

// gitObject is one object reachable from the repo's refs. Commits and tags
// carry an empty Path.
type gitObject struct {
	Hash string
	Path string
}

// listObjects walks all reachable objects, newest refs first. Each object is
// listed once, with the path it was first encountered at.
func listObjects(repoPath string) ([]gitObject, error) {
	lines, err := gitLines(repoPath, "rev-list", "--objects", "--all")
	if err != nil {
		return nil, err
	}

	objects := make([]gitObject, 0, len(lines))
	for _, line := range lines {
		hash, path, _ := strings.Cut(line, " ")
		objects = append(objects, gitObject{Hash: hash, Path: path})
	}
	return objects, nil
}

// objectInfo is what `git cat-file --batch-check` reports for one object.
type objectInfo struct {
	Type string
	Size int64
}

// batchCheck looks up the type and size of every hash in one git process.
// Hashes git cannot resolve are simply absent from the result.
func batchCheck(repoPath string, hashes []string) (map[string]objectInfo, error) {
	if len(hashes) == 0 {
		return map[string]objectInfo{}, nil
	}

	cmd := exec.Command("git", "cat-file", "--batch-check", "--buffer")
	cmd.Dir = repoPath
	cmd.Stdin = strings.NewReader(strings.Join(hashes, "\n") + "\n")

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git cat-file --batch-check: %w", err)
	}

	info := make(map[string]objectInfo, len(hashes))
	for _, line := range strings.Split(string(out), "\n") {
		// "<sha> <type> <size>", or "<sha> missing" for a bad hash.
		fields := strings.Fields(line)
		if len(fields) != 3 {
			continue
		}
		size, err := strconv.ParseInt(fields[2], 10, 64)
		if err != nil {
			continue
		}
		info[fields[0]] = objectInfo{Type: fields[1], Size: size}
	}
	return info, nil
}

// streamBlobs feeds every hash to a single `git cat-file --batch` and calls fn
// with each blob's contents. Blobs git cannot resolve are skipped.
func streamBlobs(repoPath string, hashes []string, fn func(hash string, content []byte)) error {
	if len(hashes) == 0 {
		return nil
	}

	cmd := exec.Command("git", "cat-file", "--batch")
	cmd.Dir = repoPath

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("git cat-file --batch: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("git cat-file --batch: %w", err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("git cat-file --batch: %w", err)
	}

	// Write on a goroutine: git will not drain stdin until we drain its stdout.
	go func() {
		w := bufio.NewWriter(stdin)
		for _, hash := range hashes {
			fmt.Fprintln(w, hash)
		}
		w.Flush()
		stdin.Close()
	}()

	r := bufio.NewReader(stdout)
	// git answers one header per hash, in the order we sent them.
	for range hashes {
		header, err := r.ReadString('\n')
		if err != nil {
			break
		}
		// "<sha> <type> <size>", or "<sha> missing" with no payload following.
		fields := strings.Fields(strings.TrimSpace(header))
		if len(fields) != 3 {
			continue
		}
		size, err := strconv.ParseInt(fields[2], 10, 64)
		if err != nil || size < 0 {
			break
		}

		content := make([]byte, size)
		if _, err := io.ReadFull(r, content); err != nil {
			break
		}
		if _, err := r.Discard(1); err != nil { // trailing newline
			break
		}
		fn(fields[0], content)
	}

	if err := cmd.Wait(); err != nil && stderr.Len() > 0 {
		return fmt.Errorf("git cat-file --batch: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}
