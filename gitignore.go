package main

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// GitignoreCheck warns when a repo holds generated trees or log files that
// .gitignore does not cover — the usual reason junk ends up committed.
type GitignoreCheck struct{}

// gitignoreTarget is something that is almost never worth committing. Match is
// the name or glob looked for on disk; Pattern is what we suggest ignoring.
type gitignoreTarget struct {
	Pattern string
	Match   string
	IsDir   bool
}

var gitignoreTargets = []gitignoreTarget{
	{Pattern: "node_modules/", Match: "node_modules", IsDir: true},
	{Pattern: "__pycache__/", Match: "__pycache__", IsDir: true},
	{Pattern: "*.log", Match: "*.log"},
	{Pattern: "dist/", Match: "dist", IsDir: true},
}

func (GitignoreCheck) Name() string { return "gitignore" }

func (GitignoreCheck) Run(repoPath string) ([]Finding, error) {
	patterns, hasGitignore, err := readGitignore(repoPath)
	if err != nil {
		return nil, err
	}

	// Record one example per target, so coverage is judged against a real path
	// rather than the glob we happen to print.
	found := map[string]string{}
	err = walkRepo(repoPath, func(p string, d fs.DirEntry, err error) error {
		if err != nil || p == repoPath {
			return nil
		}
		for _, t := range gitignoreTargets {
			if t.IsDir != d.IsDir() {
				continue
			}
			if t.IsDir {
				if d.Name() != t.Match {
					continue
				}
			} else if ok, _ := path.Match(t.Match, d.Name()); !ok {
				continue
			}
			if _, seen := found[t.Pattern]; !seen {
				found[t.Pattern] = relPath(repoPath, p)
			}
			if t.IsDir {
				return fs.SkipDir // never walk a whole dependency tree
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	var findings []Finding
	for _, t := range gitignoreTargets {
		foundPath, present := found[t.Pattern]
		if !present || gitignoreCovers(patterns, foundPath) {
			continue
		}

		message := fmt.Sprintf("%s exists in the repo but .gitignore does not ignore it", t.Pattern)
		if !hasGitignore {
			message = fmt.Sprintf("%s exists in the repo but there is no .gitignore", t.Pattern)
		}
		findings = append(findings, Finding{
			Severity: "warning",
			Check:    "gitignore",
			Message:  message,
			Fix:      fmt.Sprintf("add %q to .gitignore", t.Pattern),
		})
	}
	return findings, nil
}

// readGitignore returns the patterns in the repo's root .gitignore and whether
// the file exists at all.
func readGitignore(repoPath string) ([]string, bool, error) {
	data, err := os.ReadFile(filepath.Join(repoPath, ".gitignore"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}

	var patterns []string
	for _, line := range strings.Split(string(data), "\n") {
		if line = strings.TrimSpace(line); line != "" && !strings.HasPrefix(line, "#") {
			patterns = append(patterns, line)
		}
	}
	return patterns, true, nil
}

// gitignoreCovers reports whether patterns ignore the given repo-relative path,
// applying git's last-match-wins rule so that "dist/" followed by "!dist/" is
// not mistaken for coverage.
func gitignoreCovers(patterns []string, target string) bool {
	ignored := false
	for _, pattern := range patterns {
		negated := strings.HasPrefix(pattern, "!")
		if matchIgnorePattern(strings.TrimPrefix(pattern, "!"), target) {
			ignored = !negated
		}
	}
	return ignored
}

// matchIgnorePattern compares one .gitignore pattern against a repo-relative
// path. As in git, a pattern with no slash matches that name at any depth while
// one with a slash is anchored to the repo root. It is deliberately narrow: it
// judges the four targets above, not gitignore in general.
func matchIgnorePattern(pattern, target string) bool {
	pattern = strings.TrimSuffix(strings.TrimPrefix(pattern, "/"), "/")
	pattern = strings.TrimPrefix(pattern, "**/")
	if pattern == "" {
		return false
	}
	if !strings.Contains(pattern, "/") {
		target = path.Base(target)
	}
	if ok, _ := path.Match(pattern, target); ok {
		return true
	}
	// "node_modules/**" ignores the contents, which covers the tree.
	if base, ok := strings.CutSuffix(pattern, "/**"); ok {
		return base == target
	}
	return false
}
