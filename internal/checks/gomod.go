package checks

import (
	"os"
	"slices"
	"strings"
)

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
