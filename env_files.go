package main

import (
	"fmt"
	"io/fs"
	"sort"
	"strings"
)

// EnvFilesCheck flags environment files that were actually committed. A
// gitignored .env sitting in the working tree is normal and must not be
// reported, so the filesystem walk is intersected with git's index.
type EnvFilesCheck struct{}

// envPlaceholders are template names that are meant to be committed.
var envPlaceholders = map[string]bool{
	"example": true,
	"sample":  true,
}

func (EnvFilesCheck) Name() string { return "env-files" }

func (EnvFilesCheck) Run(repoPath string) ([]Finding, error) {
	tracked, isRepo := trackedFiles(repoPath)

	var committed []string
	err := walkRepo(repoPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !isEnvFileName(d.Name()) {
			return nil
		}
		rel := relPath(repoPath, path)
		if isRepo && !tracked[rel] {
			return nil // on disk but ignored by git — exactly what we want
		}
		committed = append(committed, rel)
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Strings(committed)
	findings := make([]Finding, 0, len(committed))
	for _, rel := range committed {
		findings = append(findings, Finding{
			Severity: "critical",
			Check:    "env-files",
			Message:  "committed environment file: " + rel,
			Fix:      fmt.Sprintf("add to .gitignore and remove from history: git rm --cached %s", shellQuote(rel)),
		})
	}
	return findings, nil
}

// isEnvFileName reports whether name is a real environment file, as opposed to
// a committed .env.example-style template.
func isEnvFileName(name string) bool {
	if name == ".env" {
		return true
	}
	suffix, ok := strings.CutPrefix(name, ".env.")
	if !ok {
		return false
	}
	return !envPlaceholders[strings.ToLower(suffix)]
}
