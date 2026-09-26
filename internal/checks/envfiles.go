package checks

import (
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/me-intenzo/repoctor/internal/gitutil"
	"github.com/me-intenzo/repoctor/internal/scan"
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

func (EnvFilesCheck) Run(repoPath string, _ *gitutil.Inventory) ([]Finding, error) {
	tracked, err := gitutil.TrackedFiles(repoPath)
	if err != nil {
		return nil, fmt.Errorf("inspect tracked files: %w", err)
	}

	var committed []string
	err = scan.WalkRepo(repoPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !isEnvFileName(d.Name()) {
			return nil
		}
		rel := scan.RelPath(repoPath, path)
		if !tracked[rel] {
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
		severity := "critical"
		message := "committed environment file: " + rel
		if scan.IsTestAssetPath(rel) {
			severity = "info"
			message = "environment file in test/example asset: " + rel
		}
		findings = append(findings, Finding{
			Severity: severity,
			Check:    "env-files",
			Message:  message,
			Fix:      fmt.Sprintf("add to .gitignore and remove from history: git rm --cached %s", gitutil.ShellQuote(rel)),
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
