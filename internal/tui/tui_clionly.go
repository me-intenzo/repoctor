//go:build clionly

package tui

import (
	"errors"
)

// Run reports that this build excludes the TUI.
func Run(_ string) error {
	return errors.New("this build of repoctor is CLI-only; download the full build for --tui")
}
