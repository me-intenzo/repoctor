package report

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/me-intenzo/repoctor/internal/finding"
)

func PrintFindings(findings []finding.Finding, asJSON bool) error {
	if findings == nil {
		findings = []finding.Finding{}
	}
	if asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(findings)
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
	return nil
}
