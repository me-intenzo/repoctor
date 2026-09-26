package checks

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsEnvFileName(t *testing.T) {
	for name, want := range map[string]bool{
		".env": true, ".env.production": true, ".env.local": true,
		".env.example": false, ".env.sample": false, "config.env": false,
	} {
		if got := isEnvFileName(name); got != want {
			t.Errorf("isEnvFileName(%q) = %t, want %t", name, got, want)
		}
	}
}

func TestCompareVersionsPrerelease(t *testing.T) {
	for _, test := range []struct {
		a, b string
		want int
	}{
		{"1.0.0-rc2", "1.0.0-rc10", -1},
		{"1.0.0-rc10", "1.0.0-rc2", 1},
		{"1.0.0-rc1", "1.0.0", -1},
	} {
		got, ok := compareVersions(test.a, test.b)
		if !ok || got != test.want {
			t.Errorf("compareVersions(%q, %q) = %d, %t; want %d, true", test.a, test.b, got, ok, test.want)
		}
	}
}

func TestReadPackageLockNestedWorkspace(t *testing.T) {
	dir := t.TempDir()
	lock := `{"packages":{"packages/app/node_modules/lodash":{"version":"4.17.20"}}}`
	filename := filepath.Join(dir, "package-lock.json")
	if err := os.WriteFile(filename, []byte(lock), 0600); err != nil {
		t.Fatal(err)
	}
	installed, err := readPackageLock(filename)
	if err != nil {
		t.Fatal(err)
	}
	if !installed["lodash"]["4.17.20"] {
		t.Fatalf("nested workspace dependency was not collected: %#v", installed)
	}
}

func TestHumanSize(t *testing.T) {
	for bytes, want := range map[int64]string{0: "0 B", 1024: "1.0 KB", 5 << 20: "5.0 MB"} {
		if got := humanSize(bytes); got != want {
			t.Errorf("humanSize(%d) = %q, want %q", bytes, got, want)
		}
	}
}
