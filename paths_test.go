package main

import "testing"

func TestIsTestAssetPath(t *testing.T) {
	tests := map[string]bool{
		"testdata/key.pem":               true,
		"src/__tests__/env/.env":         true,
		"playground/.env":                true,
		"examples/config/.env":           true,
		"config/.env":                    false,
		"deploy/production/private.pem": false,
	}

	for path, want := range tests {
		if got := isTestAssetPath(path); got != want {
			t.Errorf("isTestAssetPath(%q) = %t, want %t", path, got, want)
		}
	}
}