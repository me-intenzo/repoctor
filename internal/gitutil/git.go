package gitutil

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
)

// ShellQuote returns a single-quoted POSIX shell word. Control characters are
// rejected because they make copy-pasted commands ambiguous even when quoted.
func ShellQuote(s string) string {
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			return "'<invalid path>'"
		}
	}
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
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

// GitLines runs git and returns its stdout split into non-empty lines.
func GitLines(repoPath string, args ...string) ([]string, error) {
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

// TrackedFiles returns the set of files git knows about, as slash-separated
// paths relative to the repo root. Git errors are returned so callers never
// mistake an unreadable index for a clean repository.
func TrackedFiles(repoPath string) (map[string]bool, error) {
	out, err := runGit(repoPath, "ls-files", "-z")
	if err != nil {
		return nil, err
	}
	files := make(map[string]bool)
	for _, name := range strings.Split(string(out), "\x00") {
		if name != "" {
			files[name] = true
		}
	}
	return files, nil
}

// Object is one object reachable from the repo's refs. Commits and tags
// carry an empty Path.
type Object struct {
	Hash string
	Path string
}

type Introduction struct {
	Commit string
	Date   string
}

// BlobIntroductions maps reachable blob hashes to the first commit that
// referenced them. One raw history walk replaces one git log process per hit.
func BlobIntroductions(repoPath string) (map[string]Introduction, error) {
	lines, err := GitLines(repoPath, "log", "--all", "--reverse", "--format=commit %h %ad", "--date=short", "--raw", "--no-renames", "--full-index", "--abbrev=40")
	if err != nil {
		return nil, err
	}
	introductions := map[string]Introduction{}
	var current Introduction
	for _, line := range lines {
		if strings.HasPrefix(line, "commit ") {
			fields := strings.Fields(line)
			if len(fields) == 3 {
				current = Introduction{Commit: fields[1], Date: fields[2]}
			}
			continue
		}
		if !strings.HasPrefix(line, ":") || current.Commit == "" {
			continue
		}
		metadata, _, ok := strings.Cut(line, "\t")
		if !ok {
			continue
		}
		fields := strings.Fields(metadata)
		if len(fields) < 5 {
			continue
		}
		for _, hash := range fields[2:4] {
			if hash != strings.Repeat("0", len(hash)) {
				if _, seen := introductions[hash]; !seen {
					introductions[hash] = current
				}
			}
		}
	}
	return introductions, nil
}

// ListObjects walks all reachable objects, newest refs first. Each object is
// listed once, with the path it was first encountered at.
func ListObjects(repoPath string) ([]Object, error) {
	lines, err := GitLines(repoPath, "rev-list", "--objects", "--all")
	if err != nil {
		return nil, err
	}

	objects := make([]Object, 0, len(lines))
	for _, line := range lines {
		hash, path, _ := strings.Cut(line, " ")
		objects = append(objects, Object{Hash: hash, Path: path})
	}
	return objects, nil
}

// ObjectInfo is what `git cat-file --batch-check` reports for one object.
type ObjectInfo struct {
	Type string
	Size int64
}

// Inventory contains the reachable objects and their metadata. Blob-heavy
// checks share one inventory so a scan does not repeat the same Git walks.
type Inventory struct {
	Objects []Object
	Info    map[string]ObjectInfo
}

func LoadInventory(repoPath string) (*Inventory, error) {
	objects, err := ListObjects(repoPath)
	if err != nil {
		return nil, err
	}
	hashes := make([]string, 0, len(objects))
	for _, object := range objects {
		hashes = append(hashes, object.Hash)
	}
	info, err := BatchCheck(repoPath, hashes)
	if err != nil {
		return nil, err
	}
	return &Inventory{Objects: objects, Info: info}, nil
}

// BatchCheck looks up the type and size of every hash in one git process.
// Hashes git cannot resolve are simply absent from the result.
func BatchCheck(repoPath string, hashes []string) (map[string]ObjectInfo, error) {
	if len(hashes) == 0 {
		return map[string]ObjectInfo{}, nil
	}

	cmd := exec.Command("git", "cat-file", "--batch-check", "--buffer")
	cmd.Dir = repoPath
	cmd.Stdin = strings.NewReader(strings.Join(hashes, "\n") + "\n")

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git cat-file --batch-check: %w", err)
	}

	info := make(map[string]ObjectInfo, len(hashes))
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
		info[fields[0]] = ObjectInfo{Type: fields[1], Size: size}
	}
	return info, nil
}

// StreamBlobs feeds every hash to a single `git cat-file --batch` and calls fn
// with each blob's contents. Blobs git cannot resolve are skipped.
func StreamBlobs(repoPath string, hashes []string, fn func(hash string, content []byte)) error {
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

	writeErr := make(chan error, 1)
	// Write on a goroutine: git will not drain stdin until we drain its stdout.
	go func() {
		w := bufio.NewWriter(stdin)
		for _, hash := range hashes {
			if _, err := fmt.Fprintln(w, hash); err != nil {
				writeErr <- err
				return
			}
		}
		writeErr <- w.Flush()
		stdin.Close()
	}()

	r := bufio.NewReader(stdout)
	var scanErr error
	// git answers one header per hash, in the order we sent them.
	for range hashes {
		header, err := r.ReadString('\n')
		if err != nil {
			scanErr = fmt.Errorf("read blob header: %w", err)
			break
		}
		// "<sha> <type> <size>", or "<sha> missing" with no payload following.
		fields := strings.Fields(strings.TrimSpace(header))
		if len(fields) != 3 {
			scanErr = fmt.Errorf("malformed blob header %q", strings.TrimSpace(header))
			break
		}
		size, err := strconv.ParseInt(fields[2], 10, 64)
		if err != nil || size < 0 {
			scanErr = fmt.Errorf("invalid blob size in header %q", strings.TrimSpace(header))
			break
		}

		content := make([]byte, size)
		if _, err := io.ReadFull(r, content); err != nil {
			scanErr = fmt.Errorf("read blob %s: %w", fields[0], err)
			break
		}
		if newline, err := r.ReadByte(); err != nil || newline != '\n' { // trailing newline
			if err != nil {
				scanErr = fmt.Errorf("read blob %s delimiter: %w", fields[0], err)
			} else {
				scanErr = fmt.Errorf("read blob %s delimiter: got %q", fields[0], newline)
			}
			break
		}
		fn(fields[0], content)
	}

	if scanErr != nil {
		stdin.Close()
		stdout.Close()
		_ = cmd.Process.Kill()
	}
	writeFailure := <-writeErr
	if err := cmd.Wait(); err != nil {
		if stderr.Len() > 0 {
			return fmt.Errorf("git cat-file --batch: %w: %s", err, strings.TrimSpace(stderr.String()))
		}
		if scanErr != nil {
			return scanErr
		}
		return fmt.Errorf("git cat-file --batch: %w", err)
	}
	if writeFailure != nil {
		return fmt.Errorf("write blob requests: %w", writeFailure)
	}
	if scanErr != nil {
		return scanErr
	}
	return nil
}
