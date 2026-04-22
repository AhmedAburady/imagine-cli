package commands

import (
	"runtime"

	"github.com/spf13/cobra"
)

func newVersionCmd(version string) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print imagine version information",
		Run: func(cmd *cobra.Command, _ []string) {
			cmd.Printf("imagine %s %s/%s\n", version, runtime.GOOS, runtime.GOARCH)
		},
	}
}
