// Package paths holds filesystem-path helpers used across the CLI.
package paths

import (
	"os/user"
	"path/filepath"
	"strings"
)

// ExpandTilde replaces a leading `~` or `~/` with the current user's home directory.
// Returns the input unchanged if no home can be resolved.
func ExpandTilde(path string) string {
	if path == "~" {
		if usr, err := user.Current(); err == nil {
			return usr.HomeDir
		}
		return path
	}
	if strings.HasPrefix(path, "~/") {
		if usr, err := user.Current(); err == nil {
			return filepath.Join(usr.HomeDir, path[2:])
		}
	}
	return path
}
