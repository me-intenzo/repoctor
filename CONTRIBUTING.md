# Contributing

Issues and pull requests are welcome. If a check fires where it should not,
open an issue with the `repoctor` output and the repository layout that
triggered it. False-positive reports are how the check rules get better.

## Development setup

```bash
git clone https://github.com/me-intenzo/repoctor
cd repoctor
go test ./...
go vet ./...
go build -o repoctor .
```

## Adding a check

A new check is a type in [internal/checks/](internal/checks/) implementing the
`Check` interface, added to `All()` in
[internal/checks/checks.go](internal/checks/checks.go):

```go
type Check interface {
	Name() string
	Run(repoPath string, inventory *gitutil.Inventory) ([]Finding, error)
}
```

A check returns an error only when it could not *run*, which is not the same as
finding nothing. Every finding it emits must carry a `Fix` the user can paste
into a shell.
