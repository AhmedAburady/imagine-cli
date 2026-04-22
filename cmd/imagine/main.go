package main

import (
	"os"

	"github.com/AhmedAburady/imagine-cli/cli"
	"github.com/AhmedAburady/imagine-cli/config"
	"github.com/AhmedAburady/imagine-cli/describe"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "config":
			if cli.HandleConfigCommand(os.Args[1:]) {
				return
			}
		case "describe":
			describe.HandleDescribeCommand(os.Args[2:])
			return
		}
	}

	opts, cliMode := cli.ParseFlags()

	if opts.Version {
		cli.PrintVersion()
		return
	}
	if opts.Help {
		cli.PrintHelp()
		return
	}

	if !cliMode {
		cli.PrintHelp()
		return
	}

	apiKey := config.GetAPIKey()
	if apiKey == "" {
		apiKey = cli.PromptForAPIKey()
	}
	cli.Run(opts, apiKey)
}
