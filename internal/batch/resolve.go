package batch

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/AhmedAburady/imagine-cli/api"
	"github.com/AhmedAburady/imagine-cli/cli"
	"github.com/AhmedAburady/imagine-cli/config"
	"github.com/AhmedAburady/imagine-cli/internal/images"
	"github.com/AhmedAburady/imagine-cli/internal/paths"
	"github.com/AhmedAburady/imagine-cli/providers"
)

// Resolved is one entry with all inputs resolved against CLI defaults
// and ready for the orchestrator.
type Resolved struct {
	Key         string
	Index       int
	DisplayName string // "hero_shot" or "[2]"

	Provider     providers.Provider
	Bundle       providers.Bundle
	Request      providers.Request
	Params       api.Params
	ProviderOpts any
}

// ResolveContext is everything Resolve needs: the parsed spec, the CLI's
// already-bound common options, the cobra command (so the resolver can
// ask which flags the user *explicitly* set), the loaded config, and
// the resolved global default provider.
type ResolveContext struct {
	Spec            *Spec
	CLIOptions      *cli.Options
	Cmd             *cobra.Command
	Config          *config.Config
	DefaultProvider string
}

// Resolve walks every entry, fills in defaults from the CLI invocation,
// validates all of them up front, and returns the resolved jobs. It
// fails fast on parse errors at the entry level but collects the rest
// into a single multi-line error so users see every problem at once
// rather than fixing them one at a time.
func Resolve(rc ResolveContext) ([]Resolved, error) {
	explicit := collectExplicitProviderFlags(rc.Cmd)

	resolved := make([]Resolved, 0, len(rc.Spec.Entries))
	var errs []string

	// Cache Provider instances across entries that share a name. Vertex
	// in particular pays for GCP client setup; a 50-entry batch sharing
	// one provider builds the client once instead of fifty times.
	providerCache := map[string]providers.Provider{}

	for _, entry := range rc.Spec.Entries {
		r, err := resolveOne(entry, rc, explicit, providerCache)
		if err != nil {
			errs = append(errs, fmt.Sprintf("entry %s: %v", displayName(entry), err))
			continue
		}
		resolved = append(resolved, r)
	}

	// Cross-entry: a user-set CLI flag must be claimed by at least one
	// entry's provider. Delegated to the shared gate.
	if unclaimed := crossEntryUnclaimed(explicit, resolved); len(unclaimed) > 0 {
		errs = append(errs, unclaimed...)
	}

	if collisions := detectCollisions(resolved); len(collisions) > 0 {
		errs = append(errs, collisions...)
	}

	if len(errs) > 0 {
		return nil, fmt.Errorf("batch validation:\n  - %s", strings.Join(errs, "\n  - "))
	}
	return resolved, nil
}

// collectExplicitProviderFlags returns provider-private flags the user
// set on the CLI invocation, mapped to typed values. Common flags
// aren't included — the runner uses cli.Options directly for those.
//
// Typing matters: pflag's Value.String() always renders to a string
// (so a bool flag becomes "true"), but flagspec.Parse treats incoming
// values strictly per kind (bool fields reject string "true" to keep
// YAML/JSON content honest). The CLI-side plumbing converts back to
// bool/int here so the strict rule downstream stays meaningful.
func collectExplicitProviderFlags(cmd *cobra.Command) map[string]any {
	out := map[string]any{}
	if cmd == nil {
		return out
	}
	cmd.Flags().Visit(func(fl *pflag.Flag) {
		if cli.IsCommonFlag(fl.Name) {
			return
		}
		raw := fl.Value.String()
		switch fl.Value.Type() {
		case "bool":
			if b, err := strconv.ParseBool(raw); err == nil {
				out[fl.Name] = b
				return
			}
		case "int":
			if n, err := strconv.Atoi(raw); err == nil {
				out[fl.Name] = n
				return
			}
		}
		out[fl.Name] = raw
	})
	return out
}

func resolveOne(entry Entry, rc ResolveContext, explicit map[string]any, providerCache map[string]providers.Provider) (Resolved, error) {
	common, providerKeys, err := splitEntryRaw(entry.Raw)
	if err != nil {
		return Resolved{}, err
	}

	if common.prompt == "" {
		return Resolved{}, fmt.Errorf("prompt is required")
	}

	// Effective provider: entry override → CLI --provider/config default.
	provName := common.provider
	if provName == "" {
		provName = rc.DefaultProvider
	}
	if provName == "" {
		return Resolved{}, fmt.Errorf("no provider resolved (set provider: in entry or --provider on CLI)")
	}
	bundle, ok := providers.Get(provName)
	if !ok {
		return Resolved{}, fmt.Errorf("unknown provider %q (available: %v)", provName, providers.List())
	}
	if bundle.ParseOptions == nil {
		return Resolved{}, fmt.Errorf("provider %q does not support batch invocation", provName)
	}

	// Common-flag values: CLI defaults overlaid with entry overrides.
	effOutput := rc.CLIOptions.Output
	if common.output != nil {
		effOutput = *common.output
	}
	effOutput = paths.ExpandTilde(effOutput)

	effFilename := rc.CLIOptions.OutputFilename
	if common.filename != nil {
		effFilename = *common.filename
	}
	if effFilename == "" && entry.Key != "" {
		effFilename = sanitizeStem(entry.Key)
	}

	effCount := rc.CLIOptions.NumImages
	if common.count != nil {
		effCount = *common.count
	}
	if effCount < 1 || effCount > 20 {
		return Resolved{}, fmt.Errorf("count must be between 1 and 20 (got %d)", effCount)
	}

	var effInputs []string
	if common.input != nil {
		effInputs = common.input
	} else {
		effInputs = append([]string(nil), rc.CLIOptions.RefInputs...)
	}
	for i, ref := range effInputs {
		effInputs[i] = paths.ExpandTilde(ref)
	}
	for _, ref := range effInputs {
		info, err := os.Stat(ref)
		if os.IsNotExist(err) {
			return Resolved{}, fmt.Errorf("reference path does not exist: %s", ref)
		}
		if err != nil {
			return Resolved{}, fmt.Errorf("cannot access reference %s: %v", ref, err)
		}
		if info.IsDir() {
			count, _ := images.CountInDir(ref)
			if count == 0 {
				return Resolved{}, fmt.Errorf("no images found in reference directory: %s", ref)
			}
		} else if !images.IsSupported(ref) {
			return Resolved{}, fmt.Errorf("unsupported image format: %s", ref)
		}
	}

	effReplace := false
	if common.replace != nil {
		effReplace = *common.replace
	}
	if common.filename != nil && *common.filename != "" && effReplace {
		return Resolved{}, fmt.Errorf("filename and replace are mutually exclusive")
	}
	if effReplace {
		if len(effInputs) == 0 {
			return Resolved{}, fmt.Errorf("replace: true requires input")
		}
		if len(effInputs) > 1 {
			return Resolved{}, fmt.Errorf("replace: true only works with a single input file")
		}
		info, _ := os.Stat(effInputs[0])
		if info != nil && info.IsDir() {
			return Resolved{}, fmt.Errorf("replace: true only works with a single file, not a folder")
		}
	}

	// Provider-private values: explicit CLI flags filtered by this
	// provider's supported set (so --thinking high silently skips
	// openai entries), layered with entry overrides.
	supportedSet := make(map[string]bool, len(bundle.SupportedFlags))
	for _, f := range bundle.SupportedFlags {
		supportedSet[f] = true
	}
	pvtMap := map[string]any{}
	for name, val := range explicit {
		if supportedSet[name] {
			pvtMap[name] = val
		}
	}
	for k, v := range providerKeys {
		pvtMap[k] = v
	}

	providerOpts, err := bundle.ParseOptions(pvtMap, providers.Common{Filename: effFilename})
	if err != nil {
		return Resolved{}, err
	}

	// Model-level gate via the shared providers.CheckModel helper. The
	// "set" names are pvtMap's keys — explicit CLI + entry overrides —
	// which is the per-entry equivalent of cmd.Flags().Visit.
	if errs := providers.CheckModel(mapKeys(pvtMap), bundle, providerOpts); len(errs) > 0 {
		return Resolved{}, errs[0]
	}

	// Auth + Provider construction. Cached by provider name — entries
	// sharing a provider reuse one instance. Missing credentials still
	// surface here on the first miss.
	providerInst, ok := providerCache[provName]
	if !ok {
		var auth providers.Auth
		if rc.Config != nil {
			auth = providers.Auth(rc.Config.Providers[provName])
		}
		providerInst, err = bundle.Factory(auth)
		if err != nil {
			return Resolved{}, err
		}
		providerCache[provName] = providerInst
	}

	refs, err := loadReferences(effInputs)
	if err != nil {
		return Resolved{}, err
	}

	req := providers.Request{
		Prompt:     common.prompt,
		References: refs,
		Options:    providerOpts,
	}
	refInputPath := ""
	if len(effInputs) == 1 {
		refInputPath = effInputs[0]
	}
	params := api.Params{
		OutputFolder:     effOutput,
		OutputFilename:   effFilename,
		NumImages:        effCount,
		PreserveFilename: effReplace,
		RefInputPath:     refInputPath,
	}

	return Resolved{
		Key:          entry.Key,
		Index:        entry.Index,
		DisplayName:  displayName(entry),
		Provider:     providerInst,
		Bundle:       bundle,
		Request:      req,
		Params:       params,
		ProviderOpts: providerOpts,
	}, nil
}

// commonOverrides holds entry overrides for common flags. Pointers
// distinguish "key absent" from "key present, value zero".
type commonOverrides struct {
	prompt   string
	provider string
	output   *string
	filename *string
	count    *int
	input    []string
	replace  *bool
}

// splitEntryRaw walks raw and partitions common-flag keys into c.
// Remaining keys are returned for the provider's ParseOptions.
func splitEntryRaw(raw map[string]any) (c commonOverrides, providerKeys map[string]any, err error) {
	providerKeys = map[string]any{}
	for k, v := range raw {
		switch k {
		case "prompt":
			s, e := asString(v)
			if e != nil {
				return c, nil, fmt.Errorf("prompt: %v", e)
			}
			c.prompt = s
		case "provider":
			s, e := asString(v)
			if e != nil {
				return c, nil, fmt.Errorf("provider: %v", e)
			}
			c.provider = s
		case "output":
			s, e := asString(v)
			if e != nil {
				return c, nil, fmt.Errorf("output: %v", e)
			}
			c.output = &s
		case "filename":
			s, e := asString(v)
			if e != nil {
				return c, nil, fmt.Errorf("filename: %v", e)
			}
			c.filename = &s
		case "count":
			n, e := asInt(v)
			if e != nil {
				return c, nil, fmt.Errorf("count: %v", e)
			}
			c.count = &n
		case "input":
			inputs, e := asStringSlice(v)
			if e != nil {
				return c, nil, fmt.Errorf("input: %v", e)
			}
			c.input = inputs
		case "replace":
			b, e := asBool(v)
			if e != nil {
				return c, nil, fmt.Errorf("replace: %v", e)
			}
			c.replace = &b
		default:
			providerKeys[k] = v
		}
	}
	return c, providerKeys, nil
}

// crossEntryUnclaimed adapts providers.CheckClaimedSomewhere to the
// batch's per-entry-resolved bundle list. Returns string-formatted
// errors so the caller can append them to its multi-line aggregate.
func crossEntryUnclaimed(explicit map[string]any, resolved []Resolved) []string {
	if len(resolved) == 0 {
		return nil
	}
	bundles := make([]providers.Bundle, 0, len(resolved))
	for _, r := range resolved {
		bundles = append(bundles, r.Bundle)
	}
	errs := providers.CheckClaimedSomewhere(mapKeys(explicit), bundles)
	out := make([]string, 0, len(errs))
	for _, e := range errs {
		out = append(out, e.Error())
	}
	return out
}

// mapKeys returns the keys of a map[string]any in arbitrary order.
// Used as input to providers.Check* helpers, which sort their output
// for stable error messages — so the input order doesn't matter.
func mapKeys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// detectCollisions simulates ResolveFilename for every (entry, image)
// pair and reports duplicate full output paths. Auto-named entries
// (timestamp-based) collide unpredictably, so they're excluded — only
// stems coming from explicit filename: or entry-key fallback are
// checked. Mirrors the rules in internal/images/naming.go.
func detectCollisions(resolved []Resolved) []string {
	seen := map[string]string{}
	var collisions []string
	for _, r := range resolved {
		if r.Params.OutputFilename == "" && !r.Params.PreserveFilename {
			continue // timestamped — every image has a unique name
		}
		for i := 0; i < r.Params.NumImages; i++ {
			name := images.ResolveFilename(images.FilenameParams{
				Custom:       r.Params.OutputFilename,
				Preserve:     r.Params.PreserveFilename,
				RefInputPath: r.Params.RefInputPath,
				Index:        i,
				Total:        r.Params.NumImages,
			})
			full := filepath.Join(r.Params.OutputFolder, name)
			if prev, ok := seen[full]; ok {
				collisions = append(collisions, fmt.Sprintf("filename collision: entry %s and entry %s both produce %s", prev, r.DisplayName, full))
				continue
			}
			seen[full] = r.DisplayName
		}
	}
	sort.Strings(collisions)
	return collisions
}

// displayName is the entry's identifier in error messages and the
// summary table. Map-form entries use their key; list-form entries have
// no name, so we render the 1-based index. Error messages prepend "entry"
// already, so this returns just the index ("entry 1: ..." not "entry [0]: ...").
func displayName(e Entry) string {
	if e.Key != "" {
		return e.Key
	}
	return fmt.Sprintf("%d", e.Index+1)
}

func loadReferences(inputs []string) ([]images.Reference, error) {
	var refs []images.Reference
	for _, in := range inputs {
		loaded, err := images.Load(in)
		if err != nil {
			return nil, fmt.Errorf("failed to load references: %w", err)
		}
		refs = append(refs, loaded...)
	}
	return refs, nil
}

// --- coercion helpers (entry-level common-flag values) ----------------------

func asString(v any) (string, error) {
	switch x := v.(type) {
	case nil:
		return "", nil
	case string:
		return x, nil
	default:
		return "", fmt.Errorf("must be a string (got %T)", v)
	}
}

func asInt(v any) (int, error) {
	switch x := v.(type) {
	case int:
		return x, nil
	case int64:
		return int(x), nil
	case float64:
		if x != float64(int64(x)) {
			return 0, fmt.Errorf("must be an integer (got %v)", x)
		}
		return int(x), nil
	default:
		return 0, fmt.Errorf("must be an integer (got %T)", v)
	}
}

func asBool(v any) (bool, error) {
	if b, ok := v.(bool); ok {
		return b, nil
	}
	return false, fmt.Errorf("must be true or false (got %T)", v)
}

func asStringSlice(v any) ([]string, error) {
	switch x := v.(type) {
	case nil:
		return nil, nil
	case string:
		return []string{x}, nil
	case []any:
		out := make([]string, len(x))
		for i, item := range x {
			s, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("must be a list of strings, item %d is %T", i, item)
			}
			out[i] = s
		}
		return out, nil
	default:
		return nil, fmt.Errorf("must be a list of strings (got %T)", v)
	}
}
