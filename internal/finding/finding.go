package finding

type Finding struct {
	Severity string `json:"severity"` // "critical", "warning", "info", "error"
	Check    string `json:"check"`    // "secrets", "big-blobs", "stale-branches"...
	Message  string `json:"message"`
	Fix      string `json:"fix,omitempty"`
}
