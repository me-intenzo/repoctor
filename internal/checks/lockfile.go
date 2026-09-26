package checks

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// packageLock covers both lockfile shapes: v2 and v3 keep a flat "packages"
// map, v1 nests "dependencies" recursively.
type packageLock struct {
	Packages     map[string]lockPackage `json:"packages"`
	Dependencies map[string]lockPackage `json:"dependencies"`
}

type lockPackage struct {
	Version      string                 `json:"version"`
	Link         bool                   `json:"link"`
	Dependencies map[string]lockPackage `json:"dependencies"`
}

// readPackageLock collects every exact version pinned in an npm lockfile.
func readPackageLock(filename string) (installedPackages, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	var lock packageLock
	if err := json.Unmarshal(data, &lock); err != nil {
		return nil, fmt.Errorf("not valid JSON: %w", err)
	}

	installed := installedPackages{}
	for key, pkg := range lock.Packages {
		// Keys look like "node_modules/lodash" or "node_modules/@scope/name";
		// the root and workspace entries carry no node_modules prefix and are
		// not dependencies of this project.
		if idx := strings.LastIndex(key, "node_modules/"); idx >= 0 {
			addInstalled(installed, key[idx+len("node_modules/"):], pkg)
		}
	}
	for name, pkg := range lock.Dependencies {
		addLockDependency(installed, name, pkg)
	}
	return installed, nil
}

// addLockDependency walks the nested tree used by lockfile v1.
func addLockDependency(installed installedPackages, name string, pkg lockPackage) {
	addInstalled(installed, name, pkg)
	for nested, dep := range pkg.Dependencies {
		addLockDependency(installed, nested, dep)
	}
}
