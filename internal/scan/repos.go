package scan

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

// vcsDirs are directory markers of version-control checkouts beyond git.
var vcsDirs = map[string]string{
	".git":   "git",
	".hg":    "mercurial",
	".svn":   "subversion",
	".bzr":   "bazaar",
	"_darcs": "darcs",
}

// isBareGitRepo reports whether dir looks like a bare git repository:
// it has a HEAD file and an objects directory but no working tree.
func isBareGitRepo(dir string) bool {
	head, err := os.Stat(filepath.Join(dir, "HEAD"))
	if err != nil || head.IsDir() {
		return false
	}
	obj, err := os.Stat(filepath.Join(dir, "objects"))
	return err == nil && obj.IsDir()
}

// DiscoverRepos returns every directory under root that looks like a
// version-controlled repository: git checkouts (including worktrees, where
// .git is a file), mercurial/subversion/bazaar/darcs checkouts, and bare git
// repositories. Results are sorted by path.
func DiscoverRepos(root string) []string {
	seen := map[string]bool{}
	var repos []string
	add := func(path string) {
		if !seen[path] {
			seen[path] = true
			repos = append(repos, path)
		}
	}
	filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d == nil {
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if !d.IsDir() {
			// worktrees and submodules have a .git file
			if d.Name() == ".git" {
				add(filepath.Dir(path))
			}
			return nil
		}
		if _, ok := vcsDirs[d.Name()]; ok {
			add(filepath.Dir(path))
			return fs.SkipDir
		}
		if skipDirs[d.Name()] && path != root {
			return fs.SkipDir
		}
		if isBareGitRepo(path) {
			add(path)
		}
		return nil
	})
	sort.Strings(repos)
	return repos
}

// HomeDir is the default discovery root.
func HomeDir() string {
	h, err := os.UserHomeDir()
	if err != nil || h == "" {
		return "."
	}
	return h
}
