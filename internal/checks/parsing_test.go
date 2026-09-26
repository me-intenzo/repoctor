package checks

import (
	"encoding/json"
	"testing"

	"github.com/me-intenzo/repoctor/internal/finding"
)

func TestParseGoModReplace(t *testing.T) {
	required, replaced := parseGoMod(`module example

require (
    golang.org/x/net v0.16.0
)

replace golang.org/x/net v0.16.0 => golang.org/x/net v0.17.0
`)
	if required["golang.org/x/net"] != "v0.16.0" {
		t.Fatalf("required module not parsed: %#v", required)
	}
	if replaced["golang.org/x/net"].toVersion != "v0.17.0" {
		t.Fatalf("replace directive not parsed: %#v", replaced)
	}
}

func TestGitignoreCoversLastMatch(t *testing.T) {
	if !gitignoreCovers([]string{"*.log"}, "logs/app.log") {
		t.Fatal("expected unanchored log pattern to match")
	}
	if gitignoreCovers([]string{"*.log", "!important.log"}, "important.log") {
		t.Fatal("expected negation to win")
	}
}

func TestFindingJSONKeysAreStable(t *testing.T) {
	data, err := json.Marshal(finding.Finding{Severity: "warning", Check: "deps", Message: "message"})
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `{"severity":"warning","check":"deps","message":"message"}` {
		t.Fatalf("unexpected JSON contract: %s", data)
	}
}
