package checks

import (
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/me-intenzo/repoctor/internal/gitutil"
	"github.com/me-intenzo/repoctor/internal/scan"
)

// DepsCheck matches the exact versions pinned in package-lock.json and go.mod
// against a hand-curated list of well-known vulnerable releases.
//
// The list is deliberately tiny. A scanner that cries wolf over a patched
// dependency gets switched off within a week, so nothing belongs here unless
// the fixed version is documented and the comparison is unambiguous.
type DepsCheck struct{}

func (DepsCheck) Name() string { return "deps" }

// advisory is one known-bad release range: every version below Fixed is
// affected.
type advisory struct {
	Package string // exact name as it appears in the manifest
	Fixed   string // first released version containing the fix
	CVE     string
	Note    string
}

var advisories = []advisory{
	{"lodash", "4.17.21", "CVE-2021-23337", "command injection via template"},
	{"minimist", "1.2.6", "CVE-2021-44906", "prototype pollution via constructor"},
	{"axios", "1.6.0", "CVE-2023-45857", "leaks XSRF token to other origins"},
	{"node-fetch", "2.6.7", "CVE-2022-0235", "sensitive headers leaked on redirect"},
	{"ansi-regex", "5.0.1", "CVE-2021-3807", "ReDoS on crafted input"},
	{"nth-check", "2.0.1", "CVE-2021-3803", "ReDoS on crafted input"},
	{"path-parse", "1.0.7", "CVE-2021-23343", "ReDoS on crafted input"},
	{"glob-parent", "5.1.2", "CVE-2020-28469", "ReDoS on crafted input"},
	{"tar", "6.1.9", "CVE-2021-37713", "arbitrary file overwrite when extracting"},
	{"node-forge", "1.3.0", "CVE-2022-24771", "signature verification bypass"},
	{"json5", "2.2.2", "CVE-2022-46175", "prototype pollution when parsing"},
	{"json-schema", "0.4.0", "CVE-2021-3918", "prototype pollution when validating"},
	{"follow-redirects", "1.14.8", "CVE-2022-0536", "sensitive headers leaked on redirect"},
	{"ejs", "3.1.7", "CVE-2022-29078", "remote code execution via template options"},
	{"async", "3.2.2", "CVE-2021-43138", "prototype pollution"},
	{"shell-quote", "1.7.3", "CVE-2021-42740", "command injection"},
	{"moment", "2.29.4", "CVE-2022-31129", "ReDoS on crafted input"},
	{"express", "4.19.2", "CVE-2024-29041", "open redirect via crafted URL"},

	{"golang.org/x/text", "0.3.8", "CVE-2022-32149", "panic parsing crafted language tags"},
	{"golang.org/x/net", "0.17.0", "CVE-2023-44487", "HTTP/2 rapid reset denial of service"},
	{"golang.org/x/crypto", "0.17.0", "CVE-2023-48795", "Terrapin SSH prefix truncation"},
	{"github.com/gogo/protobuf", "1.3.2", "CVE-2021-3121", "panic on crafted protobuf"},
	{"google.golang.org/protobuf", "1.33.0", "CVE-2024-24786", "infinite loop on crafted input"},
}

// installedPackages maps a package name to every exact version present in the
// repo.
type installedPackages map[string]map[string]bool

// ecosystem decides how a suggested upgrade is phrased.
type ecosystem int

const (
	npmEcosystem ecosystem = iota
	goEcosystem
)

func (e ecosystem) upgrade(pkg, fixed string) string {
	if e == goEcosystem {
		return fmt.Sprintf("go get %s@v%s", pkg, fixed)
	}
	return fmt.Sprintf("npm install %s@%s", pkg, fixed)
}

func (DepsCheck) Run(repoPath string, _ *gitutil.Inventory) ([]Finding, error) {
	var findings []Finding

	var manifests []struct {
		path string
		eco  ecosystem
	}
	err := scan.WalkRepo(repoPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		switch d.Name() {
		case "package-lock.json":
			manifests = append(manifests, struct {
				path string
				eco  ecosystem
			}{path, npmEcosystem})
		case "go.mod":
			manifests = append(manifests, struct {
				path string
				eco  ecosystem
			}{path, goEcosystem})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	for _, manifest := range manifests {
		var installed installedPackages
		var readErr error
		if manifest.eco == npmEcosystem {
			installed, readErr = readPackageLock(manifest.path)
		} else {
			installed, readErr = readGoMod(manifest.path)
		}
		if readErr != nil {
			findings = append(findings, depsParseError(scan.RelPath(repoPath, manifest.path), readErr))
			continue
		}
		findings = append(findings, matchAdvisories(installed, manifest.eco)...)
	}

	sort.Slice(findings, func(i, j int) bool { return findings[i].Message < findings[j].Message })
	return findings, nil
}

// matchAdvisories reports every installed package that sits below an advisory's
// fixed version.
func matchAdvisories(installed installedPackages, eco ecosystem) []Finding {
	var findings []Finding
	for _, adv := range advisories {
		for version := range installed[adv.Package] {
			cmp, ok := compareVersions(version, adv.Fixed)
			if !ok || cmp >= 0 {
				continue // patched, or a version we cannot order confidently
			}
			display := version
			if eco == goEcosystem {
				display = "v" + strings.TrimPrefix(version, "v")
			}
			findings = append(findings, Finding{
				Severity: "warning",
				Check:    "deps",
				Message: fmt.Sprintf("%s@%s is affected by %s (%s); fixed in %s",
					adv.Package, display, adv.CVE, adv.Note, adv.Fixed),
				Fix: eco.upgrade(adv.Package, adv.Fixed),
			})
		}
	}
	return findings
}

// depsParseError reports a manifest we could not read.
func depsParseError(name string, err error) Finding {
	return Finding{
		Severity: "info",
		Check:    "deps",
		Message:  fmt.Sprintf("could not inspect %s: %v", name, err),
	}
}

// addInstalled records a package/version pair, ignoring symlinks and entries
// that pin no version at all.
func addInstalled(installed installedPackages, name string, pkg lockPackage) {
	name = strings.TrimPrefix(strings.TrimPrefix(name, "./"), "/")
	if name == "" || pkg.Version == "" || pkg.Link {
		return
	}
	if installed[name] == nil {
		installed[name] = map[string]bool{}
	}
	installed[name][pkg.Version] = true
}
