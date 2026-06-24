package fal

// Options is fal's private parameter struct. flagspec reflects the tags at
// registration to bind Cobra flags, and again at PreRunE (or per-batch-entry
// parse) to populate this struct from input. Generate type-asserts
// Request.Options to *Options.
//
// Shared flag names (model, size, aspect-ratio) register idempotently
// alongside other providers' Options; the active provider registers first, so
// its desc text wins in `--help`.
type Options struct {
	Model       string `flag:"model,m"        desc:"Tier: seedance (default) or seedance-fast" enum:"@models"`
	Resolution  string `flag:"size,s"         desc:"Resolution: 480p, 720p, 1080p (default: 720p)" enum:"480p,720p,1080p" default:"720p"`
	AspectRatio string `flag:"aspect-ratio,a" desc:"Aspect ratio: auto,21:9,16:9,4:3,1:1,3:4,9:16" enum:"auto,21:9,16:9,4:3,1:1,3:4,9:16" default:"auto"`
	Duration    string `flag:"duration"       desc:"Seconds: auto or 4-15 (default: auto)" default:"auto"`
	Audio       bool   `flag:"audio"          desc:"Generate synchronized audio" default:"true"`
	Bitrate     string `flag:"bitrate"        desc:"Bitrate mode: standard, high" enum:"standard,high" default:"standard"`
	Seed        int    `flag:"seed"           desc:"Random seed (omit for random)" default:"-1"`
	EndImage    string `flag:"end-image"      desc:"i2v only: end-frame image path"`
}

// RequestLabel implements providers.RequestLabeler for status output.
func (o *Options) RequestLabel() string {
	return o.Model
}

// ResolvedModel implements providers.ResolvedModeler so the model-level
// flag-support gate can look up per-model SupportedFlags.
func (o *Options) ResolvedModel() string {
	return o.Model
}
