package scan

import (
	"path"
	"strings"
)

// IsTestAssetPath reports whether a path is commonly used for fixtures or
// examples. Callers should downgrade such findings, not silently discard them.
func IsTestAssetPath(name string) bool {
	for _, part := range strings.Split(path.Clean(name), "/") {
		switch strings.ToLower(part) {
		case "test", "tests", "testdata", "fixtures", "__tests__", "playground", "example", "examples":
			return true
		}
	}
	return false
}
