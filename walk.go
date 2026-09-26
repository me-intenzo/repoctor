package main

import (
	"io/fs"
	"path/filepath"
)

// skipDirs are never worth descending into: git's own storage, or dependency
// trees where a committed .env says nothing about the repo's hygiene.
var skipDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
	"vendor":       true,
	"__pycache__":  true,
	".venv":        true,
	"venv":         true,
}

// walkRepo visits every entry under root except the trees in skipDirs. The
// callback may return fs.SkipDir to prune a directory itself.
func walkRepo(root string, fn fs.WalkDirFunc) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// A single unreadable entry is not worth failing the whole check.
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if callErr := fn(path, d, nil); callErr != nil {
			return callErr
		}
		if path != root && d.IsDir() && skipDirs[d.Name()] {
			return fs.SkipDir
		}
		return nil
	})
}

// relPath renders a walk path as a slash-separated path relative to root, which
// is how git reports the same file.
func relPath(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(rel)
}
