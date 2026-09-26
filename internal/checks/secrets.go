package checks

import (
	"errors"
	"fmt"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/me-intenzo/repoctor/internal/gitutil"
	"github.com/me-intenzo/repoctor/internal/scan"
)

// SecretsCheck scans the contents of every blob in history for credential
// patterns. Findings name the kind, file and commit but never echo the match:
// the report is meant to be shareable, and the credential is already exposed.
type SecretsCheck struct{}

// maxSecretScanSize caps how much blob content we read. Secrets live in source
// and config files; a multi-megabyte blob is data, not a private key.
const maxSecretScanSize = 2 << 20

func (SecretsCheck) Name() string { return "secrets" }

// secretPattern pairs a credential type with its detector.
var secretPatterns = []struct {
	Type string
	Re   *regexp.Regexp
}{
	{"AWS access key ID", regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`)},
	{"GitHub personal access token", regexp.MustCompile(`\bghp_[A-Za-z0-9]{36}\b`)},
	{"private key", regexp.MustCompile(`-----BEGIN (RSA |EC |OPENSSH |DSA |PGP )?PRIVATE KEY-----`)},
}

// textExtensions are the file types worth regex-scanning. Binary formats are
// skipped because a run of random bytes — base64 payloads especially — can look
// like a key prefix and would be reported as a false positive.
var textExtensions = map[string]bool{
	"bash": true, "bat": true, "c": true, "cc": true, "cfg": true, "cjs": true,
	"cmd": true, "conf": true, "cpp": true, "crt": true, "cs": true, "css": true,
	"csv": true, "env": true, "go": true, "gradle": true, "h": true, "hcl": true,
	"hpp": true, "htm": true, "html": true, "ini": true, "java": true, "js": true,
	"json": true, "json5": true, "jsonc": true, "jsx": true, "key": true, "kt": true,
	"kts": true, "less": true, "lock": true, "m": true, "markdown": true, "md": true,
	"mjs": true, "mm": true, "mod": true, "npmrc": true, "pem": true, "php": true,
	"plist": true, "properties": true, "ps1": true, "psm1": true, "pub": true,
	"py": true, "pyi": true, "rb": true, "rs": true, "rst": true, "sass": true,
	"scala": true, "scss": true, "sh": true, "sql": true, "sum": true, "swift": true,
	"tf": true, "tfvars": true, "tfstate": true, "toml": true, "ts": true, "tsx": true,
	"txt": true, "xml": true, "yaml": true, "yml": true, "zsh": true,
}

// secretHit is one credential type found in one file, with the blob it lives in.
type secretHit struct {
	Path string
	Type string
	Hash string
}

func (SecretsCheck) Run(repoPath string, inventory *gitutil.Inventory) ([]Finding, error) {
	if inventory == nil {
		return nil, errors.New("git object inventory unavailable")
	}

	pathByHash := make(map[string]string, len(inventory.Objects))
	hashes := make([]string, 0, len(inventory.Objects))
	for _, obj := range inventory.Objects {
		if obj.Path == "" || !isTextPath(obj.Path) {
			continue
		}
		pathByHash[obj.Hash] = obj.Path
		hashes = append(hashes, obj.Hash)
	}

	scannable := make([]string, 0, len(hashes))
	for _, hash := range hashes {
		if o, ok := inventory.Info[hash]; ok && o.Type == "blob" && o.Size <= maxSecretScanSize {
			scannable = append(scannable, hash)
		}
	}

	// A file keeps its blob across most revisions, so the same credential would
	// otherwise be reported once per version of the file.
	seen := map[string]bool{}
	var hits []secretHit
	var err error
	err = gitutil.StreamBlobs(repoPath, scannable, func(hash string, content []byte) {
		rel := pathByHash[hash]
		for _, p := range secretPatterns {
			if !p.Re.Match(content) {
				continue
			}
			key := rel + "\x00" + p.Type
			if seen[key] {
				continue
			}
			seen[key] = true
			hits = append(hits, secretHit{Path: rel, Type: p.Type, Hash: hash})
		}
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(hits, func(i, j int) bool {
		if hits[i].Path != hits[j].Path {
			return hits[i].Path < hits[j].Path
		}
		return hits[i].Type < hits[j].Type
	})

	introductions, _ := gitutil.BlobIntroductions(repoPath)
	findings := make([]Finding, 0, len(hits))
	for _, h := range hits {
		severity := "critical"
		message := secretMessage(h, introductions[h.Hash])
		if scan.IsTestAssetPath(h.Path) {
			severity = "info"
			message = "secret-like test/example asset: " + message
		}
		findings = append(findings, Finding{
			Severity: severity,
			Check:    "secrets",
			Message:  message,
			Fix: fmt.Sprintf("rotate this credential, then purge it from history: git filter-repo --path %s --invert-paths",
				gitutil.ShellQuote(h.Path)),
		})
	}
	return findings, nil
}

// secretMessage describes a hit without reproducing the credential itself.
func secretMessage(h secretHit, introduction gitutil.Introduction) string {
	msg := fmt.Sprintf("%s in %s", h.Type, h.Path)
	if introduction.Commit == "" {
		return msg
	}
	if introduction.Date != "" {
		return fmt.Sprintf("%s (commit %s, %s)", msg, introduction.Commit, introduction.Date)
	}
	return fmt.Sprintf("%s (commit %s)", msg, introduction.Commit)
}

// isTextPath reports whether a path is worth scanning for secrets.
func isTextPath(p string) bool {
	base := strings.ToLower(path.Base(p))
	if strings.HasPrefix(base, ".env.") {
		return true
	}
	// Dotfiles such as .npmrc and .netrc hold tokens and have no extension as
	// far as path.Ext is concerned.
	if strings.HasPrefix(base, ".") && !strings.Contains(base[1:], ".") {
		return true
	}
	ext := path.Ext(base)
	if ext == "" {
		// Extensionless names are usually scripts, keys or config.
		return true
	}
	return textExtensions[ext[1:]]
}
