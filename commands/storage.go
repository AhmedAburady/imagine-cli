package commands

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/AhmedAburady/imagine-cli/config"
	"github.com/AhmedAburady/imagine-cli/internal/storage"
	"github.com/AhmedAburady/imagine-cli/providers"
)

// storageFields drives both the `storage set` flag set and its interactive
// wizard. Modelled as []providers.ConfigField so it reuses the exact
// onboarding engine that powers `providers add` — flags, TTY wizard, non-TTY
// error, and secret masking all come for free.
var storageFields = []providers.ConfigField{
	{Key: "endpoint", Title: "Endpoint", Description: "S3-compatible endpoint URL (e.g. https://tos-ap-southeast-1.bytepluses.com)", Required: true},
	{Key: "region", Title: "Region", Description: "Bucket region (e.g. ap-southeast-1)"},
	{Key: "bucket", Title: "Bucket", Description: "Bucket name", Required: true},
	{Key: "access_key", Title: "Access Key", Description: "S3 access key ID", Required: true},
	{Key: "secret_key", Title: "Secret Key", Description: "S3 secret access key (supports ${ENV} / op://)", Secret: true, Required: true},
	{Key: "path_prefix", Title: "Path Prefix", Description: "Key prefix for uploaded objects", Default: "imagine-refs/"},
	{Key: "public_url_base", Title: "Public URL Base", Description: "CDN/custom-domain base for public URLs (optional)"},
}

// newStorageCmd builds the `imagine storage` command tree: show/set/test/clear.
func newStorageCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "storage",
		Short: "Configure S3-compatible storage for uploading references",
		Long: "Configure the S3-compatible storage imagine uploads references to.\n\n" +
			"Use a dedicated, public-read bucket on any S3-compatible backend (BytePlus\n" +
			"TOS, MinIO, Cloudflare R2, …). References are uploaded there and fetched\n" +
			"server-side by the provider, so the bucket must allow anonymous reads.\n" +
			"Run `imagine storage test` to verify.",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE:          runStorageShow,
	}
	cmd.AddCommand(
		newStorageShowCmd(),
		newStorageSetCmd(),
		newStorageTestCmd(),
		newStorageClearCmd(),
	)
	return cmd
}

func newStorageShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:           "show",
		Short:         "Show the current storage configuration (secrets masked)",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE:          runStorageShow,
	}
}

func newStorageSetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "set",
		Short:         "Set storage configuration (interactive or via flags)",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE:          runStorageSet,
	}
	registerFieldFlags(cmd, storageFields)
	return cmd
}

func newStorageTestCmd() *cobra.Command {
	return &cobra.Command{
		Use:           "test",
		Short:         "Round-trip a marker object to verify writes and public reads",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE:          runStorageTest,
	}
}

func newStorageClearCmd() *cobra.Command {
	return &cobra.Command{
		Use:           "clear",
		Short:         "Remove the storage configuration",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE:          runStorageClear,
	}
}

func runStorageShow(cmd *cobra.Command, _ []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	out := cmd.OutOrStdout()
	fmt.Fprintln(out)
	fmt.Fprintln(out, titleStyle.Render("STORAGE"))
	fmt.Fprintln(out)

	if cfg.Storage == nil {
		fmt.Fprintf(out, "  %s\n\n", dimStyle.Render("Not configured. Run `imagine storage set` to add it."))
		return nil
	}

	// Display the raw (unresolved) values so a ${ENV}/op:// reference shows as
	// written, masking only Secret fields. Rows derive from the single
	// storageFields schema so adding a field can't desync the display.
	values := storageConfigToMap(cfg.Storage)
	for _, f := range storageFields {
		val := values[f.Key]
		if val == "" {
			continue
		}
		if f.Secret {
			val = maskSecret(val)
		}
		fmt.Fprintf(out, "  %s  %s\n", boldStyle.Width(16).Render(f.Key), val)
	}
	fmt.Fprintln(out)
	fmt.Fprintf(out, "  %s\n\n", dimStyle.Render(config.DefaultConfigPath()))
	return nil
}

func runStorageSet(cmd *cobra.Command, _ []string) error {
	// Load existing storage so `set` is a merge, not a destructive overwrite:
	// fields the user doesn't supply keep their stored value (and pre-fill the
	// wizard) instead of reverting to the schema default or being dropped.
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	existing := storageConfigToMap(cfg.Storage)

	fields := make([]providers.ConfigField, len(storageFields))
	copy(fields, storageFields)
	for i := range fields {
		if v := existing[fields[i].Key]; v != "" {
			fields[i].Default = v
		}
	}

	collected, err := collectFields(cmd, fields)
	if err != nil {
		return err
	}
	if collected == nil {
		return nil // user cancelled the wizard
	}
	sc := &config.StorageConfig{
		Endpoint:      collected["endpoint"],
		Region:        collected["region"],
		Bucket:        collected["bucket"],
		AccessKey:     collected["access_key"],
		SecretKey:     collected["secret_key"],
		PathPrefix:    collected["path_prefix"],
		PublicURLBase: collected["public_url_base"],
	}
	if err := config.SaveStorage(sc); err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "\n  %s  storage written to %s\n",
		successStyle.Render("✓"), dimStyle.Render(config.DefaultConfigPath()))
	fmt.Fprintf(out, "  %s  %s\n\n",
		bulletDim, dimStyle.Render("this must be a dedicated public-read bucket; run `imagine storage test` to verify"))
	return nil
}

func runStorageTest(cmd *cobra.Command, _ []string) error {
	sc, err := storage.Get(cmd.Context())
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "  %s  %s\n", bulletDim, dimStyle.Render("verifying the bucket is public-read (signed write → anonymous read)"))
	fmt.Fprintf(out, "  %s  testing %s/%s ...\n", bulletDim, strings.TrimRight(sc.Endpoint, "/"), sc.Bucket)
	if err := storage.Test(cmd.Context(), sc); err != nil {
		return err
	}
	fmt.Fprintf(out, "  %s  storage is writable and publicly readable\n", successStyle.Render("✓"))
	return nil
}

func runStorageClear(cmd *cobra.Command, _ []string) error {
	if err := config.ClearStorage(); err != nil {
		return fmt.Errorf("clear storage: %w", err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "  %s  storage configuration removed\n", successStyle.Render("✓"))
	return nil
}

// storageConfigToMap flattens a StorageConfig into a key→value map keyed by
// the same field keys as storageFields. Single source for both `storage show`
// and the `storage set` merge. nil config yields an empty map.
func storageConfigToMap(sc *config.StorageConfig) map[string]string {
	if sc == nil {
		return map[string]string{}
	}
	return map[string]string{
		"endpoint":        sc.Endpoint,
		"region":          sc.Region,
		"bucket":          sc.Bucket,
		"access_key":      sc.AccessKey,
		"secret_key":      sc.SecretKey,
		"path_prefix":     sc.PathPrefix,
		"public_url_base": sc.PublicURLBase,
	}
}

// maskSecret shows the first 4 characters of a non-reference secret, masking
// the rest. ${ENV} / op:// references are shown verbatim — they aren't secrets
// themselves, and seeing the reference is the point.
func maskSecret(v string) string {
	if strings.HasPrefix(v, "${") || strings.HasPrefix(v, "op://") {
		return v
	}
	if len(v) <= 4 {
		return strings.Repeat("•", len(v))
	}
	return v[:4] + strings.Repeat("•", len(v)-4)
}
