package checks

import (
	"github.com/me-intenzo/repoctor/internal/finding"
	"github.com/me-intenzo/repoctor/internal/gitutil"
)

// Finding is the report entry every check produces. It is an alias, so checks
// can name it unqualified while the report package keeps the real definition.
type Finding = finding.Finding

// Check is one repository-hygiene inspection: it inspects repoPath and reports
// what it found. A non-nil error means the inspection could not be performed,
// which is not the same as finding nothing.
type Check interface {
	Name() string
	Run(repoPath string, inventory *gitutil.Inventory) ([]Finding, error)
}

// All returns every check repoctor runs, in the order their findings appear in
// the report.
func All() []Check {
	return []Check{
		EnvFilesCheck{},
		GitignoreCheck{},
		BigBlobsCheck{},
		SecretsCheck{},
		StaleBranchesCheck{},
		DepsCheck{},
	}
}

var (
	_ Check = EnvFilesCheck{}
	_ Check = GitignoreCheck{}
	_ Check = BigBlobsCheck{}
	_ Check = SecretsCheck{}
	_ Check = StaleBranchesCheck{}
	_ Check = DepsCheck{}
)
