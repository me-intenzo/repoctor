package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/me-intenzo/repoctor/internal/checks"
	"github.com/me-intenzo/repoctor/internal/finding"
	"github.com/me-intenzo/repoctor/internal/gitutil"
	"github.com/me-intenzo/repoctor/internal/report"
	"github.com/me-intenzo/repoctor/internal/version"
)

func main() {
	repoPath := flag.String("path", ".", "path to repo")
	jsonOut := flag.Bool("json", false, "output as JSON")
	showVersion := flag.Bool("version", false, "print version")
	flag.Parse()
	if *showVersion {
		fmt.Println(version.Version)
		return
	}

	all := []finding.Finding{}
	checkFailed := false
	inventory, inventoryErr := gitutil.LoadInventory(*repoPath)
	for _, c := range checks.All() {
		if inventoryErr != nil && (c.Name() == "big-blobs" || c.Name() == "secrets") {
			inventory = nil
		}
		findings, err := c.Run(*repoPath, inventory)
		if err != nil {
			fmt.Fprintf(os.Stderr, "check %s failed: %v\n", c.Name(), err)
			checkFailed = true
			all = append(all, finding.Finding{Severity: "error", Check: c.Name(), Message: "check failed: " + err.Error()})
			continue
		}
		all = append(all, findings...)
	}

	if err := report.PrintFindings(all, *jsonOut); err != nil {
		fmt.Fprintf(os.Stderr, "write report: %v\n", err)
		os.Exit(2)
	}

	// exit code 1 if any critical — this makes it CI-friendly
	for _, f := range all {
		if f.Severity == "critical" {
			os.Exit(1)
		}
	}
	if checkFailed {
		os.Exit(2)
	}
}
