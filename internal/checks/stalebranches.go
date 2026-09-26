package checks

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/me-intenzo/repoctor/internal/gitutil"
)

// StaleBranchesCheck lists branches whose last commit is older than half a
// year. Severity is info: an old branch is often deliberate, and deleting the
// wrong one annoys a colleague, so this is a nudge rather than an alarm.
type StaleBranchesCheck struct{}

const staleBranchDays = 180

func (StaleBranchesCheck) Name() string { return "stale-branches" }

func (StaleBranchesCheck) Run(repoPath string, _ *gitutil.Inventory) ([]Finding, error) {
	// branch --format uses %09 as a hex-escaped tab; git log's format language
	// uses %x09 for the same byte, so do not copy this format string between them.
	lines, err := gitutil.GitLines(repoPath, "branch", "-a",
		"--format=%(HEAD)%09%(refname)%09%(committerdate:unix)")
	if err != nil {
		return nil, err
	}

	cutoff := time.Now().AddDate(0, 0, -staleBranchDays)
	type staleBranch struct {
		name string
		fix  string
		last time.Time
	}
	var stale []staleBranch

	for _, line := range lines {
		fields := strings.Split(line, "\t")
		if len(fields) != 3 {
			continue
		}
		head, refname, rawDate := fields[0], fields[1], fields[2]
		if head == "*" {
			continue // the checked-out branch, i.e. current work
		}
		if strings.HasSuffix(refname, "/HEAD") {
			continue // refs/remotes/origin/HEAD is a symref, not a branch
		}

		name, fix, ok := branchTarget(refname)
		if !ok {
			continue
		}
		secs, err := strconv.ParseInt(rawDate, 10, 64)
		if err != nil || secs <= 0 {
			continue // no commit date to judge
		}

		last := time.Unix(secs, 0)
		if !last.Before(cutoff) {
			continue
		}
		stale = append(stale, staleBranch{name: name, fix: fix, last: last})
	}

	sort.Slice(stale, func(i, j int) bool {
		if !stale[i].last.Equal(stale[j].last) {
			return stale[i].last.Before(stale[j].last)
		}
		return stale[i].name < stale[j].name
	})

	now := time.Now()
	findings := make([]Finding, 0, len(stale))
	for _, b := range stale {
		days := int(now.Sub(b.last).Hours() / 24)
		findings = append(findings, Finding{
			Severity: "info",
			Check:    "stale-branches",
			Message: fmt.Sprintf("branch %q has no commits in %d days (last: %s)",
				b.name, days, b.last.UTC().Format("2006-01-02")),
			Fix: b.fix,
		})
	}
	return findings, nil
}

// branchTarget turns a full refname into a display name and the command that
// deletes it. A remote branch needs a push, not a local delete.
func branchTarget(refname string) (name, fix string, ok bool) {
	if short, isLocal := strings.CutPrefix(refname, "refs/heads/"); isLocal {
		return short, "git branch -d " + gitutil.ShellQuote(short), true
	}

	rest, isRemote := strings.CutPrefix(refname, "refs/remotes/")
	if !isRemote {
		return "", "", false
	}
	remote, branch, found := strings.Cut(rest, "/")
	if !found || branch == "" {
		return "", "", false
	}
	return rest, fmt.Sprintf("git push %s --delete %s", gitutil.ShellQuote(remote), gitutil.ShellQuote(branch)), true
}
