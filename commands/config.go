package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/AhmedAburady/imagine-cli/config"
)

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage imagine configuration",
	}

	cmd.AddCommand(
		newConfigSetKeyCmd(),
		newConfigSetProjectCmd(),
		newConfigSetLocationCmd(),
		newConfigShowCmd(),
		newConfigPathCmd(),
	)

	return cmd
}

func newConfigSetKeyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set-key <API_KEY>",
		Short: "Save your Gemini API key",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.SaveAPIKey(args[0]); err != nil {
				return fmt.Errorf("failed to save API key: %w", err)
			}
			cmd.Printf("\033[32m✓\033[0m API key saved\n")
			cmd.Printf("  Location: %s\n", config.DefaultConfigPath())
			return nil
		},
	}
}

func newConfigSetProjectCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set-project <GCP_PROJECT_ID>",
		Short: "Save your GCP project ID (for --vertex)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.SaveGCPProject(args[0]); err != nil {
				return fmt.Errorf("failed to save GCP project: %w", err)
			}
			cmd.Printf("\033[32m✓\033[0m GCP project saved\n")
			cmd.Printf("  Project: %s\n", args[0])
			return nil
		},
	}
}

func newConfigSetLocationCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set-location <GCP_LOCATION>",
		Short: "Save your GCP location (default: global)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.SaveGCPLocation(args[0]); err != nil {
				return fmt.Errorf("failed to save GCP location: %w", err)
			}
			cmd.Printf("\033[32m✓\033[0m GCP location saved\n")
			cmd.Printf("  Location: %s\n", args[0])
			return nil
		},
	}
}

func newConfigShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Show current configuration",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			if cfg.APIKey == "" {
				cmd.Println("API Key: (not set)")
			} else {
				cmd.Printf("API Key: %s\n", maskKey(cfg.APIKey))
			}

			if cfg.GCPProject == "" {
				cmd.Println("GCP Project: (not set)")
			} else {
				cmd.Printf("GCP Project: %s\n", cfg.GCPProject)
			}

			if cfg.GCPLocation == "" {
				cmd.Println("GCP Location: (not set, defaults to 'global')")
			} else {
				cmd.Printf("GCP Location: %s\n", cfg.GCPLocation)
			}
			return nil
		},
	}
}

func newConfigPathCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "path",
		Short: "Show config file location",
		Run: func(cmd *cobra.Command, _ []string) {
			cmd.Println(config.DefaultConfigPath())
		},
	}
}

func maskKey(key string) string {
	if len(key) > 12 {
		return key[:8] + "..." + key[len(key)-4:]
	}
	return key
}
