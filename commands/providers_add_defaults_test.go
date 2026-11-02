package commands

import (
	"testing"

	"github.com/spf13/cobra"

	"github.com/AhmedAburady/imagine-cli/providers"
)

// A default the user did not choose must not be pinned into the config.
func TestDropUnchangedDefaults(t *testing.T) {
	fields := []providers.ConfigField{
		{Key: "api_key", Required: true},
		{Key: "vision_model", Default: "model-a"},
		{Key: "location", Default: "global"},
	}
	newCmd := func(args ...string) *cobra.Command {
		cmd := &cobra.Command{Use: "add", Run: func(*cobra.Command, []string) {}}
		cmd.Flags().String("vision-model", "", "")
		cmd.Flags().String("location", "", "")
		cmd.SetArgs(args)
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
		return cmd
	}

	collected := map[string]string{"api_key": "k", "vision_model": "model-a", "location": "global"}
	dropUnchangedDefaults(newCmd(), fields, collected)
	if _, ok := collected["vision_model"]; ok {
		t.Error("unchosen vision_model default was kept")
	}
	if _, ok := collected["location"]; ok {
		t.Error("unchosen location default was kept")
	}
	if collected["api_key"] != "k" {
		t.Error("required field dropped")
	}

	collected = map[string]string{"vision_model": "model-a", "location": "us-central1"}
	dropUnchangedDefaults(newCmd("--vision-model", "model-a"), fields, collected)
	if collected["vision_model"] != "model-a" {
		t.Error("explicit --vision-model equal to the default was dropped")
	}
	if collected["location"] != "us-central1" {
		t.Error("non-default value was dropped")
	}
}
