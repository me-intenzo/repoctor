package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/me-intenzo/repoctor/internal/report"
	"github.com/me-intenzo/repoctor/internal/run"
	"github.com/me-intenzo/repoctor/internal/tui"
	"github.com/me-intenzo/repoctor/internal/version"
)

func main() {
	repoPath := flag.String("path", ".", "path to repo")
	jsonOut := flag.Bool("json", false, "output as JSON")
	tuiOut := flag.Bool("tui", false, "interactive terminal UI")
	showVersion := flag.Bool("version", false, "print version")
	flag.Parse()
	if *showVersion {
		fmt.Println(version.Version)
		return
	}

	if *tuiOut {
		if err := tui.Run(*repoPath); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		return
	}

	all, checkFailed := run.All(*repoPath)

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

