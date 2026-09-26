package checks

import (
	"regexp"
	"strconv"
	"strings"
)

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
	default:
		return comparePrerelease(aPre, bPre), true
	}
}

func comparePrerelease(a, b string) int {
	aParts, bParts := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(aParts) && i < len(bParts); i++ {
		x, y := aParts[i], bParts[i]
		if cmp, ok := compareMixedIdentifier(x, y); ok {
			if cmp != 0 {
				return cmp
			}
			continue
		}
		xNum, xErr := strconv.Atoi(x)
		yNum, yErr := strconv.Atoi(y)
		switch {
		case xErr == nil && yErr == nil:
			if xNum < yNum {
				return -1
			}
			if xNum > yNum {
				return 1
			}
		case xErr == nil:
			return -1
		case yErr == nil:
			return 1
		case x < y:
			return -1
		case x > y:
			return 1
		}
	}
	if len(aParts) < len(bParts) {
		return -1
	}
	if len(aParts) > len(bParts) {
		return 1
	}
	return 0
}

func compareMixedIdentifier(a, b string) (int, bool) {
	aPrefix, aNumber, aOK := mixedIdentifier(a)
	bPrefix, bNumber, bOK := mixedIdentifier(b)
	if !aOK || !bOK || aPrefix != bPrefix {
		return 0, false
	}
	if aNumber < bNumber {
		return -1, true
	}
	if aNumber > bNumber {
		return 1, true
	}
	return 0, true
}

func mixedIdentifier(s string) (prefix string, number int, ok bool) {
	cut := len(s)
	for cut > 0 && s[cut-1] >= '0' && s[cut-1] <= '9' {
		cut--
	}
	if cut == len(s) || cut == 0 {
		return "", 0, false
	}
	n, err := strconv.Atoi(s[cut:])
	if err != nil {
		return "", 0, false
	}
	return s[:cut], n, true
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
