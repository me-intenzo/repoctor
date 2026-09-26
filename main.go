package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
    repoPath := flag.String("path", ".", "path to repo")
    jsonOut := flag.Bool("json", false, "output as JSON")
    flag.Parse()

    checks := []Check{
        EnvFilesCheck{},
        GitignoreCheck{},
        BigBlobsCheck{},
        SecretsCheck{},
        StaleBranchesCheck{},
        DepsCheck{},
    }

    var all []Finding
    for _, c := range checks {
        findings, err := c.Run(*repoPath)
        if err != nil {
            fmt.Fprintf(os.Stderr, "check %s failed: %v\n", c.Name(), err)
            continue
        }
        all = append(all, findings...)
    }

    printFindings(all, *jsonOut)

    // exit code 1 if any critical — this makes it CI-friendly
    for _, f := range all {
        if f.Severity == "critical" {
            os.Exit(1)
        }
    }
}