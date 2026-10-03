package run

import (
	"fmt"
	"os"

	"github.com/me-intenzo/repoctor/internal/checks"
	"github.com/me-intenzo/repoctor/internal/finding"
	"github.com/me-intenzo/repoctor/internal/gitutil"
)

// All runs every check against repoPath and returns the findings plus whether
// any check failed to run.
func All(repoPath string) ([]finding.Finding, bool) {
	all := []finding.Finding{}
	checkFailed := false
	inventory, inventoryErr := gitutil.LoadInventory(repoPath)
	for _, c := range checks.All() {
		if inventoryErr != nil && (c.Name() == "big-blobs" || c.Name() == "secrets") {
			inventory = nil
		}
		findings, err := c.Run(repoPath, inventory)
		if err != nil {
			fmt.Fprintf(os.Stderr, "check %s failed: %v\n", c.Name(), err)
			checkFailed = true
			all = append(all, finding.Finding{Severity: "error", Check: c.Name(), Message: "check failed: " + err.Error()})
			continue
		}
		all = append(all, findings...)
	}
	return all, checkFailed
}
