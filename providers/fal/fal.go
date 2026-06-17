// Package fal implements the Provider interface for fal.ai video models
// (Seedance 2.0). The tier (normal/fast) is modelled as the provider "model";
// the modality (text/image/reference → video) is derived at call time from
// the supplied references. Generate uses fal's queue API (submit → poll →
// fetch) behind the synchronous Provider interface.
package fal

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/AhmedAburady/imagine-cli/internal/images"
	"github.com/AhmedAburady/imagine-cli/internal/transport"
	"github.com/AhmedAburady/imagine-cli/providers"
)

const (
	queueBaseURL = "https://queue.fal.run/"
	pollInterval = 3 * time.Second

	// Reference-to-video caps (§3.7). Sizes use len(ref.Data) bytes.
	maxRefImages  = 9
	maxRefVideos  = 3
	maxRefAudios  = 3
	maxRefFiles   = 12
	maxImageBytes = 30 * 1024 * 1024
	maxAudioBytes = 15 * 1024 * 1024
	maxVideoBytes = 50 * 1024 * 1024
)

// Provider is the fal.ai implementation of providers.Provider. It holds the
// API key used for the Authorization: Key <FAL_KEY> header, plus a singleflight
// cache so identical reference bytes upload to the CDN once across concurrent
// Generate calls (-n>1) and reused instances (batch mode).
type Provider struct {
	apiKey  string
	mu      sync.Mutex
	uploads map[string]*uploadResult
}

// uploadResult memoizes one CDN upload; done is closed once url/err are set.
type uploadResult struct {
	url  string
	err  error
	done chan struct{}
}

// uploadRef uploads data to the CDN at most once per distinct content, keyed by
// sha256. Concurrent callers for the same bytes block on done and share the
// result; failures are evicted so a later call may retry.
func (p *Provider) uploadRef(ctx context.Context, data []byte, mime, name string) (string, error) {
	sum := sha256.Sum256(data)
	key := hex.EncodeToString(sum[:])
	p.mu.Lock()
	if p.uploads == nil {
		p.uploads = make(map[string]*uploadResult)
	}
	if r, ok := p.uploads[key]; ok {
		p.mu.Unlock()
		<-r.done
		return r.url, r.err
	}
	r := &uploadResult{done: make(chan struct{})}
	p.uploads[key] = r
	p.mu.Unlock()

	r.url, r.err = uploadToFalStorage(ctx, p.apiKey, data, mime, name)
	if r.err != nil {
		p.mu.Lock()
		delete(p.uploads, key)
		p.mu.Unlock()
	}
	close(r.done)
	return r.url, r.err
}

// New builds a Provider from the resolved Auth, failing fast on a missing key.
func New(auth providers.Auth) (providers.Provider, error) {
	p := &Provider{apiKey: auth.Get("api_key")}
	if p.apiKey == "" {
		return nil, errors.New("fal requires an API key — run `imagine providers add fal`, or set providers.fal.api_key")
	}
	return p, nil
}

// ConfigSchema is the field set `providers add` collects for fal.
func (p *Provider) ConfigSchema() []providers.ConfigField {
	return []providers.ConfigField{
		{
			Key:         "api_key",
			Title:       "API Key",
			Description: "fal.ai API key (FAL_KEY) from fal.ai/dashboard/keys",
			Secret:      true,
			Required:    true,
		},
	}
}

// Info advertises fal's Seedance 2.0 tiers and video capabilities.
func (p *Provider) Info() providers.Info {
	return providers.Info{
		Name:         "fal",
		DisplayName:  "fal.ai",
		Summary:      "fal.ai video models (Seedance 2.0)",
		DefaultModel: "bytedance/seedance-2.0",
		Models: []providers.ModelInfo{
			{ID: "bytedance/seedance-2.0", Aliases: []string{"seedance", "seedance-2"}, Description: "Seedance 2.0 (normal tier)."},
			{ID: "bytedance/seedance-2.0/fast", Aliases: []string{"seedance-fast", "fast"}, Description: "Seedance 2.0 (fast tier — lower latency/cost)."},
		},
		Capabilities: providers.Capabilities{
			Edit:      true,
			MaxBatchN: 1,
			MediaKind: providers.KindVideo,
			RefKinds:  []providers.MediaKind{providers.KindImage, providers.KindVideo, providers.KindAudio},
			MaxN:      4,
		},
	}
}

// -- Wire types --------------------------------------------------------------

// videoBody is the shared Seedance input schema; modality fields use omitempty
// so one struct serves text/image/reference. GenerateAudio omits omitempty so
// --audio=false sends an explicit false.
type videoBody struct {
	Prompt        string   `json:"prompt"`
	Resolution    string   `json:"resolution,omitempty"`
	Duration      string   `json:"duration,omitempty"`
	AspectRatio   string   `json:"aspect_ratio,omitempty"`
	BitrateMode   string   `json:"bitrate_mode,omitempty"`
	GenerateAudio bool     `json:"generate_audio"`
	Seed          *int     `json:"seed,omitempty"`
	ImageURL      string   `json:"image_url,omitempty"`
	EndImageURL   string   `json:"end_image_url,omitempty"`
	ImageURLs     []string `json:"image_urls,omitempty"`
	VideoURLs     []string `json:"video_urls,omitempty"`
	AudioURLs     []string `json:"audio_urls,omitempty"`
}

// submitResp is the queue submit acknowledgement carrying the poll/fetch URLs.
type submitResp struct {
	RequestID   string `json:"request_id"`
	StatusURL   string `json:"status_url"`
	ResponseURL string `json:"response_url"`
	CancelURL   string `json:"cancel_url"`
}

// statusResp is one poll of the queue status endpoint.
type statusResp struct {
	Status    string `json:"status"`
	Error     string `json:"error"`
	ErrorType string `json:"error_type"`
}

// resultResp is the completed job's output: a public CDN video URL + the seed used.
type resultResp struct {
	Video struct {
		URL string `json:"url"`
	} `json:"video"`
	Seed int `json:"seed"`
}

// Generate runs one Seedance job over fal's queue API: classify references into
// a modality, upload local refs to the CDN, submit, poll until COMPLETED, then
// download the resulting mp4 bytes.
func (p *Provider) Generate(ctx context.Context, req providers.Request) (*providers.Response, error) {
	opts, ok := req.Options.(*Options)
	if !ok {
		return nil, fmt.Errorf("fal: internal error: unexpected options type %T", req.Options)
	}

	var imageRefs, videoRefs, audioRefs []images.Reference
	for _, ref := range req.References {
		switch images.KindOf(ref.MimeType) {
		case "image":
			imageRefs = append(imageRefs, ref)
		case "video":
			videoRefs = append(videoRefs, ref)
		case "audio":
			audioRefs = append(audioRefs, ref)
		default:
			return nil, fmt.Errorf("fal: unsupported reference type %q", ref.MimeType)
		}
	}

	body := videoBody{
		Prompt:        req.Prompt,
		Resolution:    opts.Resolution,
		Duration:      opts.Duration,
		AspectRatio:   opts.AspectRatio,
		BitrateMode:   opts.Bitrate,
		GenerateAudio: opts.Audio,
	}
	if opts.Seed != -1 {
		body.Seed = &opts.Seed
	}

	var modality string
	switch {
	case len(imageRefs) == 0 && len(videoRefs) == 0 && len(audioRefs) == 0:
		modality = "text"
		if opts.EndImage != "" {
			return nil, errors.New("fal: --end-image requires an input image (-i)")
		}

	case len(imageRefs) == 1 && len(videoRefs) == 0 && len(audioRefs) == 0:
		modality = "image"
		if len(imageRefs[0].Data) > maxImageBytes {
			return nil, errors.New("fal: input image exceeds 30 MB")
		}
		imageURL, err := p.uploadRef(ctx, imageRefs[0].Data, imageRefs[0].MimeType,
			fmt.Sprintf("image_0%s", images.ExtForMime(imageRefs[0].MimeType)))
		if err != nil {
			return nil, err
		}
		body.ImageURL = imageURL
		if opts.EndImage != "" {
			endRefs, err := images.Load(opts.EndImage)
			if err != nil {
				return nil, fmt.Errorf("fal: load --end-image: %w", err)
			}
			if len(endRefs) == 0 {
				return nil, fmt.Errorf("fal: --end-image %q contains no image", opts.EndImage)
			}
			end := endRefs[0]
			if len(end.Data) > maxImageBytes {
				return nil, errors.New("fal: --end-image exceeds 30 MB")
			}
			endURL, err := p.uploadRef(ctx, end.Data, end.MimeType,
				fmt.Sprintf("end_image_0%s", images.ExtForMime(end.MimeType)))
			if err != nil {
				return nil, err
			}
			body.EndImageURL = endURL
		}

	default:
		modality = "reference"
		if opts.EndImage != "" {
			return nil, errors.New("fal: --end-image is only valid with a single -i input image")
		}
		if err := validateReferenceConstraints(imageRefs, videoRefs, audioRefs); err != nil {
			return nil, err
		}
		urls, err := p.uploadBucket(ctx, "image", imageRefs)
		if err != nil {
			return nil, err
		}
		body.ImageURLs = urls
		if urls, err = p.uploadBucket(ctx, "video", videoRefs); err != nil {
			return nil, err
		}
		body.VideoURLs = urls
		if urls, err = p.uploadBucket(ctx, "audio", audioRefs); err != nil {
			return nil, err
		}
		body.AudioURLs = urls
	}

	endpoint := queueBaseURL + opts.Model + "/" + modality + "-to-video"

	submit, err := transport.PostJSON[submitResp](ctx, falClient, endpoint, transport.Key(p.apiKey), body)
	if err != nil {
		return nil, fmt.Errorf("fal: submit job: %w", err)
	}

	if err := p.poll(ctx, submit); err != nil {
		return nil, err
	}

	result, err := transport.GetJSON[resultResp](ctx, falClient, submit.ResponseURL, transport.Key(p.apiKey))
	if err != nil {
		// A 422 at the result stage is fal's post-generation content moderation
		// rejecting the produced video, not a transport fault — say so plainly.
		var apiErr *transport.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusUnprocessableEntity {
			return nil, fmt.Errorf("fal: the generated video was rejected by content moderation: %w", err)
		}
		return nil, fmt.Errorf("fal: fetch result: %w", err)
	}

	data, err := transport.GetBytes(ctx, downloadClient, result.Video.URL, transport.NoAuth())
	if err != nil {
		return nil, fmt.Errorf("fal: download video: %w", err)
	}
	return &providers.Response{Assets: []providers.GeneratedAsset{{Data: data, MimeType: "video/mp4"}}}, nil
}

// poll loops on the queue status URL until COMPLETED, threading ctx so Ctrl-C
// best-effort cancels the in-flight job before returning ctx.Err().
func (p *Provider) poll(ctx context.Context, submit *submitResp) error {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		status, err := transport.GetJSON[statusResp](ctx, falClient, submit.StatusURL, transport.Key(p.apiKey))
		if err != nil {
			return fmt.Errorf("fal: poll status: %w", err)
		}
		// Surface a failure whenever the queue reports one, regardless of the
		// accompanying status string. Documented failures arrive as COMPLETED +
		// error, but the status enum is happy-path-only (design doc §9), so we
		// don't assume COMPLETED is the only terminal state that carries an error.
		if status.Error != "" {
			return fmt.Errorf("fal: job failed (%s): %s", status.ErrorType, status.Error)
		}
		if status.Status == "COMPLETED" {
			return nil
		}
		// Defend against an undocumented terminal status with no error field:
		// without this, FAILED/CANCELLED/ERROR would poll forever (the enum is
		// happy-path-only per design doc §9).
		switch status.Status {
		case "FAILED", "CANCELLED", "ERROR":
			return fmt.Errorf("fal: job ended with status %q", status.Status)
		}

		select {
		case <-ctx.Done():
			p.cancel(submit.CancelURL)
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

// cancel issues a best-effort PUT to the queue cancel URL; result is ignored.
// It runs on a fresh short-lived context: the caller's ctx is already cancelled
// (that's why we're here), so reusing it would abort the cancel before it sends.
func (p *Provider) cancel(cancelURL string) {
	if cancelURL == "" {
		return
	}
	ctx, done := context.WithTimeout(context.Background(), 5*time.Second)
	defer done()
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, cancelURL, nil)
	if err != nil {
		return
	}
	if err := transport.Key(p.apiKey).Apply(req); err != nil {
		return
	}
	if resp, err := falClient.Do(req); err == nil {
		_ = resp.Body.Close()
	}
}

// validateReferenceConstraints enforces the §3.7 reference-to-video caps before
// any upload, so an over-limit request fails cheaply on local bytes.
func validateReferenceConstraints(imageRefs, videoRefs, audioRefs []images.Reference) error {
	if len(imageRefs) > maxRefImages {
		return fmt.Errorf("fal: too many image references (%d, max %d)", len(imageRefs), maxRefImages)
	}
	if len(videoRefs) > maxRefVideos {
		return fmt.Errorf("fal: too many video references (%d, max %d)", len(videoRefs), maxRefVideos)
	}
	if len(audioRefs) > maxRefAudios {
		return fmt.Errorf("fal: too many audio references (%d, max %d)", len(audioRefs), maxRefAudios)
	}
	if total := len(imageRefs) + len(videoRefs) + len(audioRefs); total > maxRefFiles {
		return fmt.Errorf("fal: too many references (%d, max %d total)", total, maxRefFiles)
	}
	if len(audioRefs) > 0 && len(imageRefs)+len(videoRefs) == 0 {
		return errors.New("fal: an audio reference requires at least one image or video reference")
	}
	for i, ref := range imageRefs {
		if len(ref.Data) > maxImageBytes {
			return fmt.Errorf("fal: image reference %d exceeds 30 MB", i+1)
		}
	}
	for i, ref := range audioRefs {
		if len(ref.Data) > maxAudioBytes {
			return fmt.Errorf("fal: audio reference %d exceeds 15 MB", i+1)
		}
	}
	var videoTotal int
	for _, ref := range videoRefs {
		videoTotal += len(ref.Data)
	}
	if videoTotal > maxVideoBytes {
		return errors.New("fal: combined video references exceed 50 MB")
	}
	return nil
}

// uploadBucket uploads each reference in a modality bucket to the CDN, returning
// the resulting public URLs in order (nil for an empty bucket).
func (p *Provider) uploadBucket(ctx context.Context, kind string, refs []images.Reference) ([]string, error) {
	if len(refs) == 0 {
		return nil, nil
	}
	urls := make([]string, 0, len(refs))
	for i, ref := range refs {
		name := fmt.Sprintf("%s_%d%s", kind, i, images.ExtForMime(ref.MimeType))
		url, err := p.uploadRef(ctx, ref.Data, ref.MimeType, name)
		if err != nil {
			return nil, err
		}
		urls = append(urls, url)
	}
	return urls, nil
}
