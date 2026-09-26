package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"regexp"
	"sort"
	"strconv"
	"strings"
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

func (DepsCheck) Run(repoPath string) ([]Finding, error) {
	var findings []Finding

	npm, err := readPackageLock(filepath.Join(repoPath, "package-lock.json"))
	switch {
	case err == nil:
		findings = append(findings, matchAdvisories(npm, npmEcosystem)...)
	case !os.IsNotExist(err):
		// Unreadable rather than absent: say so, because silently reporting
		// nothing would look like a clean bill of health.
		findings = append(findings, depsParseError("package-lock.json", err))
	}

	mods, err := readGoMod(filepath.Join(repoPath, "go.mod"))
	switch {
	case err == nil:
		findings = append(findings, matchAdvisories(mods, goEcosystem)...)
	case !os.IsNotExist(err):
		findings = append(findings, depsParseError("go.mod", err))
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

// goReplace is one replace directive: a module may be repointed at another path
// and version, or at a local directory.
type goReplace struct {
	fromVersion string // empty means the directive applies to every version
	toPath      string
	toVersion   string // empty means a filesystem replacement
}

// readGoMod collects the module versions the main module's go.mod selects,
// after applying replace directives — the replacement is what actually builds.
func readGoMod(filename string) (installedPackages, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	required, replaced := parseGoMod(string(data))

	installed := installedPackages{}
	for module, version := range required {
		if rule, ok := replaced[module]; ok {
			switch {
			case rule.fromVersion != "" && rule.fromVersion != version:
				// The directive does not apply to the version we selected.
			case rule.toVersion == "":
				continue // replaced by a local directory; nothing to look up
			default:
				module, version = rule.toPath, rule.toVersion
			}
		}
		if installed[module] == nil {
			installed[module] = map[string]bool{}
		}
		installed[module][version] = true
	}
	return installed, nil
}

// parseGoMod extracts the selected versions and the replace directives from a
// go.mod. It reads the two directives that affect which code gets built and
// ignores the rest.
func parseGoMod(src string) (required map[string]string, replaced map[string]goReplace) {
	required = map[string]string{}
	replaced = map[string]goReplace{}

	block := ""
	for _, raw := range strings.Split(src, "\n") {
		line := stripGoComment(raw)
		if line == "" {
			continue
		}
		if block != "" {
			if line == ")" {
				block = ""
				continue
			}
			applyGoDirective(block, line, required, replaced)
			continue
		}

		keyword, rest, matched := strings.Cut(line, " ")
		if !matched {
			continue
		}
		switch keyword {
		case "require", "replace", "exclude":
			if strings.TrimSpace(rest) == "(" {
				block = keyword
				continue
			}
			applyGoDirective(keyword, rest, required, replaced)
		}
	}
	return required, replaced
}

// applyGoDirective records one require or replace line, from a block or from a
// single-line directive.
func applyGoDirective(kind, line string, required map[string]string, replaced map[string]goReplace) {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return
	}

	switch kind {
	case "require":
		if len(fields) >= 2 {
			required[fields[0]] = fields[1]
		}
	case "replace":
		// "old [version] => new [version]"
		arrow := slices.Index(fields, "=>")
		if arrow < 1 || arrow == len(fields)-1 {
			return
		}
		left, right := fields[:arrow], fields[arrow+1:]
		rule := goReplace{toPath: right[0]}
		if len(right) > 1 {
			rule.toVersion = right[1]
		}
		if len(left) > 1 {
			rule.fromVersion = left[1]
		}
		replaced[left[0]] = rule
	}
}

// stripGoComment removes a trailing // comment, which is how go.mod marks
// indirect dependencies. Module paths never contain "//", so cutting at the
// first one is safe.
func stripGoComment(line string) string {
	line, _, _ = strings.Cut(line, "//")
	return strings.TrimSpace(line)
}

// goPseudoVersion matches the fake tags Go generates for untagged revisions,
// e.g. v0.0.0-20240101120000-abcdef123456. Their numeric core describes the
// last real release, not the code they contain, so they cannot be ordered
// against an advisory.
var goPseudoVersion = regexp.MustCompile(`^\d+\.\d+\.\d+-(0\.)?\d{14}-[0-9a-f]{12}$`)

// compareVersions orders two release versions. ok is false when either side is
// not a plain release — a git hash, a range, a Go pseudo-version — because
// guessing at those is exactly how false positives happen.
func compareVersions(a, b string) (cmp int, ok bool) {
	aCore, aPre, aOK := parseVersion(a)
	bCore, bPre, bOK := parseVersion(b)
	if !aOK || !bOK {
		return 0, false
	}

	for i := 0; i < len(aCore) || i < len(bCore); i++ {
		var x, y int
		if i < len(aCore) {
			x = aCore[i]
		}
		if i < len(bCore) {
			y = bCore[i]
		}
		if x != y {
			if x < y {
				return -1, true
			}
			return 1, true
		}
	}

	// A pre-release sorts below the release it precedes: 1.2.3-rc1 < 1.2.3.
	switch {
	case aPre == bPre:
		return 0, true
	case aPre == "":
		return 1, true
	case bPre == "":
		return -1, true
	case aPre < bPre:
		return -1, true
	default:
		return 1, true
	}
}

// parseVersion splits a version into its numeric core and pre-release tag,
// dropping a leading "v" (go.mod) and build metadata (Go's "+incompatible").
func parseVersion(s string) (core []int, pre string, ok bool) {
	s = strings.TrimSpace(s)
	if base, _, found := strings.Cut(s, "+"); found {
		s = base
	}
	s = strings.TrimPrefix(strings.TrimPrefix(s, "v"), "V")
	if goPseudoVersion.MatchString(s) {
		return nil, "", false
	}

	coreStr, pre, _ := strings.Cut(s, "-")
	parts := strings.Split(coreStr, ".")
	if len(parts) > 3 {
		return nil, "", false
	}
	for _, part := range parts {
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 {
			return nil, "", false
		}
		core = append(core, n)
	}
	return core, pre, true
}
