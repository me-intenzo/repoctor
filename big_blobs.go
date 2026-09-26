package main

import (
	"fmt"
	"sort"
)

// BigBlobsCheck flags large files stored in git history. They bloat every clone
// forever, and hosting providers reject the biggest ones outright.
type BigBlobsCheck struct{}

const (
	bigBlobWarn     = 5 << 20  // worth cleaning up
	bigBlobCritical = 50 << 20 // most hosts will refuse the push
)

func (BigBlobsCheck) Name() string { return "big-blobs" }

func (BigBlobsCheck) Run(repoPath string) ([]Finding, error) {
	objects, err := listObjects(repoPath)
	if err != nil {
		return nil, err
	}

	pathByHash := make(map[string]string, len(objects))
	hashes := make([]string, 0, len(objects))
	for _, obj := range objects {
		if obj.Path == "" {
			continue // commits and tags carry no path
		}
		pathByHash[obj.Hash] = obj.Path
		hashes = append(hashes, obj.Hash)
	}

	info, err := batchCheck(repoPath, hashes)
	if err != nil {
		return nil, err
	}

	// One path can appear as several blobs, one per revision. Keep the largest
	// so each file is reported once, at its worst size.
	largest := map[string]int64{}
	for hash, rel := range pathByHash {
		o, ok := info[hash]
		if !ok || o.Type != "blob" || o.Size < bigBlobWarn {
			continue
		}
		if o.Size > largest[rel] {
			largest[rel] = o.Size
		}
	}

	paths := make([]string, 0, len(largest))
	for rel := range largest {
		paths = append(paths, rel)
	}
	sort.Slice(paths, func(i, j int) bool {
		if largest[paths[i]] != largest[paths[j]] {
			return largest[paths[i]] > largest[paths[j]]
		}
		return paths[i] < paths[j]
	})

	findings := make([]Finding, 0, len(paths))
	for _, rel := range paths {
		size := largest[rel]
		severity := "warning"
		if size >= bigBlobCritical {
			severity = "critical"
		}
		findings = append(findings, Finding{
			Severity: severity,
			Check:    "big-blobs",
			Message:  fmt.Sprintf("large file in history: %s (%s)", rel, humanSize(size)),
			// --invert-paths is what makes filter-repo drop the path; without it
			// the command means "keep only this file".
			Fix: fmt.Sprintf("git filter-repo --path %s --invert-paths (needs git-filter-repo; rewrites history)", shellQuote(rel)),
		})
	}
	return findings, nil
}

// humanSize renders a byte count the way a person would read it.
func humanSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit && exp < 3; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGT"[exp])
}
