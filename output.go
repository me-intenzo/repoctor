package main

import (
	"encoding/json"
	"fmt"
	"os"
)

func printFindings(findings []Finding, asJSON bool) {
	if asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(findings)
		return
	}
	for _, f := range findings {
		icon := "ℹ️ "
		switch f.Severity {
		case "critical":
			icon = "🔴"
		case "warning":
			icon = "🟡"
		}
		fmt.Printf("%s [%s] %s\n", icon, f.Check, f.Message)
		if f.Fix != "" {
			fmt.Printf("   fix: %s\n", f.Fix)
		}
	}
	fmt.Printf("\n%d findings\n", len(findings))
}
