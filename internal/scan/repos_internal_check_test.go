package scan

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverReposFindsVariousVCS(t *testing.T) {
	root := t.TempDir()
	mk := func(p string, dir bool) {
		full := filepath.Join(root, p)
		if dir {
			os.MkdirAll(full, 0o755)
		} else {
			os.MkdirAll(filepath.Dir(full), 0o755)
			os.WriteFile(full, []byte("x"), 0o644)
		}
	}
	mk("gitrepo/.git", true)
	mk("worktree/.git", false)
	mk("hgrepo/.hg", true)
	mk("svnrepo/.svn", true)
	mk("bzrrepo/.bzr", true)
	mk("bare.git/HEAD", false)
	mk("bare.git/objects", true)
	mk("random/nothing", false)

	got := DiscoverRepos(root)
	want := map[string]bool{
		filepath.Join(root, "gitrepo"): true,
		filepath.Join(root, "worktree"): true,
		filepath.Join(root, "hgrepo"):   true,
		filepath.Join(root, "svnrepo"):  true,
		filepath.Join(root, "bzrrepo"):  true,
		filepath.Join(root, "bare.git"): true,
	}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for _, g := range got {
		if !want[g] {
			t.Fatalf("unexpected %s in %v", g, got)
		}
	}
}
