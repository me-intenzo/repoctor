package main

import (
	"path"
	"strings"
)

func isTestAssetPath(name string) bool {
	for _, part := range strings.Split(path.Clean(name), "/") {
		switch strings.ToLower(part) {
		case "test", "tests", "testdata", "fixtures", "__tests__", "playground", "example", "examples":
			return true
		}
	}
	return false
}