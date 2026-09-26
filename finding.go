package main

type Finding struct {
	Severity string // "critical", "warning", "info"
	Check    string // "secrets", "big-blobs", "stale-branches"...
	Message  string
	Fix      string
}