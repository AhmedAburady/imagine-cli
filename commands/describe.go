package commands

import (
	"github.com/spf13/cobra"

	"github.com/AhmedAburady/imagine-cli/describe"
)

// newDescribeCmd wraps describe.HandleDescribeCommand in a cobra shell so
// `imagine --help` lists it. Flag parsing stays inside the describe package
// (DisableFlagParsing = true forwards raw argv). This is intentional: the
// provider-system refactor defers reworking describe.
func newDescribeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                "describe",
		Short:              "Describe or analyze image style using AI",
		Long:               "Analyze images and produce a style description usable as a generation prompt. Pass --help after `describe` for the describe-specific flags.",
		DisableFlagParsing: true,
		Run: func(cmd *cobra.Command, args []string) {
			describe.HandleDescribeCommand(args)
		},
	}
	return cmd
}
