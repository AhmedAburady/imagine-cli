package modelark

import (
	"fmt"

	"github.com/AhmedAburady/imagine-cli/providers"
)

// Options is modelark's private parameter struct. flagspec reflects the tags
// to bind Cobra flags and to parse batch-file entries; Generate type-asserts
// Request.Options to *Options.
//
// Shared flag names (model, size, aspect-ratio) register idempotently
// alongside other providers (notably fal). Whichever provider is active
// registers first, so its desc text wins in `--help`.
type Options struct {
	Model       string `flag:"model,m"        desc:"Tier: seedance (default), fast, or mini" enum:"@models"`
	Resolution  string `flag:"size,s"         desc:"Resolution: 480p/720p (all); 1080p/4k (full only)" enum:"480p,720p,1080p,4k" default:"720p"`
	AspectRatio string `flag:"aspect-ratio,a" desc:"Aspect ratio: adaptive,21:9,16:9,4:3,1:1,3:4,9:16" enum:"adaptive,21:9,16:9,4:3,1:1,3:4,9:16" default:"adaptive"`
	Duration    int    `flag:"duration"       desc:"Seconds: 4-15, or -1 for auto" default:"-1" range:"-1:15"`
	Audio       bool   `flag:"audio"          desc:"Generate synchronized audio" default:"true"`
	EndImage    string `flag:"end-image"      desc:"i2v only: end-frame image path (first+last frame; not on mini)"`
}

// resolutionRank orders the resolution enum so a per-model maximum can be
// expressed as a single comparison rather than membership tests.
var resolutionRank = map[string]int{"480p": 1, "720p": 2, "1080p": 3, "4k": 4}

// modelTier captures the per-model limits the flat enum/range tags can't
// express. Keyed by canonical model ID — not inferred from substrings of the
// ID, so a future rename can't silently misclassify a tier.
type modelTier struct {
	maxResolution  string // highest resolution this model accepts
	firstLastFrame bool   // supports --end-image (first+last frame)
}

var modelTiers = map[string]modelTier{
	"dreamina-seedance-2-0-260128":      {maxResolution: "4k", firstLastFrame: true},
	"dreamina-seedance-2-0-fast-260128": {maxResolution: "720p", firstLastFrame: true},
	"dreamina-seedance-2-0-mini-260615": {maxResolution: "720p", firstLastFrame: false},
}

// Validate runs after flagspec population (Model is already the canonical ID).
// It enforces the rules the flat enum/range tags can't express:
//   - resolution must not exceed the model's maximum (fast/mini cap at 720p)
//   - duration must be -1 (auto) or 4..15 (the range tag also admits 0..3)
//   - --end-image (first+last frame) is unsupported on the mini tier
//
// For an unrecognised model ID (e.g. a newly shipped tier not yet in the
// table) the per-model rules are skipped and the API validates server-side —
// preferable to guessing and rejecting a valid request.
func (o *Options) Validate(_ providers.Info) error {
	if o.Duration != -1 && (o.Duration < 4 || o.Duration > 15) {
		return fmt.Errorf("--duration must be -1 (auto) or between 4 and 15 (got %d)", o.Duration)
	}
	tier, known := modelTiers[o.Model]
	if !known {
		return nil
	}
	if resolutionRank[o.Resolution] > resolutionRank[tier.maxResolution] {
		return fmt.Errorf("resolution %s exceeds the maximum %s for model %s", o.Resolution, tier.maxResolution, o.Model)
	}
	if o.EndImage != "" && !tier.firstLastFrame {
		return fmt.Errorf("--end-image (first+last frame) is not supported on model %s", o.Model)
	}
	return nil
}

// RequestLabel implements providers.RequestLabeler for status output.
func (o *Options) RequestLabel() string { return o.Model }

// ResolvedModel implements providers.ResolvedModeler for the model-level
// flag-support gate.
func (o *Options) ResolvedModel() string { return o.Model }
