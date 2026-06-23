// Package modelark implements the Provider interface for BytePlus ModelArk's
// video models (Dreamina Seedance 2.0) via the direct ModelArk REST API. The
// tier (full/fast/mini) is modelled as the provider "model"; the modality
// (text/image/reference → video) is derived at call time from the supplied
// references. Local references are published as public URLs through the shared
// S3 storage brick (the API fetches reference URLs server-side and rejects
// Base64), so the provider declares RequireStorage:true.
package modelark

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/AhmedAburady/imagine-cli/config"
	"github.com/AhmedAburady/imagine-cli/internal/images"
	"github.com/AhmedAburady/imagine-cli/internal/storage"
	"github.com/AhmedAburady/imagine-cli/internal/transport"
	"github.com/AhmedAburady/imagine-cli/providers"
)

const (
	baseURL      = "https://ark.ap-southeast.bytepluses.com/api/v3"
	pollInterval = 5 * time.Second

	// Reference-to-video caps. Sizes use len(ref.Data) bytes.
	maxRefImages  = 9
	maxRefVideos  = 3
	maxRefAudios  = 3
	maxImageBytes = 30 * 1024 * 1024
	maxVideoBytes = 50 * 1024 * 1024
	maxAudioBytes = 15 * 1024 * 1024
)

// arkClient serves create/poll/cancel (Bearer auth). downloadClient fetches the
// finished mp4 with no whole-request deadline — output URLs expire in 24h, so
// we download immediately, and a large 1080p/4k file can outrun a fixed ceiling.
var (
	arkClient      = transport.NewClient(180 * time.Second)
	downloadClient = transport.NewStreamingClient()
)

// Provider is the ModelArk implementation of providers.Provider, holding the
// API key used for the Authorization: Bearer <ARK_API_KEY> header.
type Provider struct {
	apiKey string
}

// New builds a Provider from the resolved Auth, failing fast on a missing key.
func New(auth providers.Auth) (providers.Provider, error) {
	p := &Provider{apiKey: auth.Get("api_key")}
	if p.apiKey == "" {
		return nil, errors.New("modelark requires an API key — run `imagine providers add modelark`, or set providers.modelark.api_key")
	}
	return p, nil
}

// ConfigSchema is the field set `providers add` collects for modelark.
func (p *Provider) ConfigSchema() []providers.ConfigField {
	return []providers.ConfigField{
		{
			Key:         "api_key",
			Title:       "API Key",
			Description: "BytePlus ModelArk API key (ARK_API_KEY) from the ModelArk console",
			Secret:      true,
			Required:    true,
		},
	}
}

// Info advertises ModelArk's Seedance 2.0 tiers and video capabilities.
func (p *Provider) Info() providers.Info {
	return providers.Info{
		Name:         "modelark",
		DisplayName:  "BytePlus ModelArk",
		Summary:      "BytePlus ModelArk video models (Dreamina Seedance 2.0 direct API)",
		DefaultModel: "dreamina-seedance-2-0-260128",
		Models: []providers.ModelInfo{
			{ID: "dreamina-seedance-2-0-260128", Aliases: []string{"seedance", "seedance-2"}, Description: "Seedance 2.0 (highest quality; up to 1080p/4k)."},
			{ID: "dreamina-seedance-2-0-fast-260128", Aliases: []string{"seedance-fast", "fast"}, Description: "Seedance 2.0 Fast (≤720p)."},
			{ID: "dreamina-seedance-2-0-mini-260615", Aliases: []string{"seedance-mini", "mini"}, Description: "Seedance 2.0 Mini (≤720p; no first+last frame)."},
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

type urlObject struct {
	URL string `json:"url"`
}

// contentItem is one element of the create request's content[] array. Type is
// the discriminator; exactly one of Text/ImageURL/VideoURL/AudioURL is set;
// Role disambiguates a media item's purpose.
type contentItem struct {
	Type     string     `json:"type"` // text | image_url | video_url | audio_url
	Text     string     `json:"text,omitempty"`
	ImageURL *urlObject `json:"image_url,omitempty"`
	VideoURL *urlObject `json:"video_url,omitempty"`
	AudioURL *urlObject `json:"audio_url,omitempty"`
	Role     string     `json:"role,omitempty"` // first_frame|last_frame|reference_image|reference_video|reference_audio
}

type createReq struct {
	Model         string        `json:"model"`
	Content       []contentItem `json:"content"`
	GenerateAudio bool          `json:"generate_audio"`       // explicit (API default is true)
	Resolution    string        `json:"resolution,omitempty"` //
	Ratio         string        `json:"ratio,omitempty"`      //
	Duration      int           `json:"duration"`             // -1 = auto; NOT omitempty (omitempty drops 0, never -1)
	Watermark     bool          `json:"watermark"`            // always false
}

type createResp struct {
	ID string `json:"id"` // "cgt-..."
}

type taskResp struct {
	Status  string       `json:"status"` // queued|running|succeeded|failed|cancelled|expired
	Content *taskContent `json:"content,omitempty"`
	Error   *taskError   `json:"error,omitempty"`
}

type taskContent struct {
	VideoURL string `json:"video_url"`
}

type taskError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Generate runs one Seedance job: classify references into a modality, upload
// local refs through the storage brick, create the task, poll until terminal,
// then download the resulting mp4 bytes.
func (p *Provider) Generate(ctx context.Context, req providers.Request) (*providers.Response, error) {
	opts, ok := req.Options.(*Options)
	if !ok {
		return nil, fmt.Errorf("modelark: internal error: unexpected options type %T", req.Options)
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
			return nil, fmt.Errorf("modelark: unsupported reference type %q", ref.MimeType)
		}
	}

	// Resolve storage credentials once per generation (a single ${ENV}/op://
	// round-trip) and reuse them across every reference upload — only when
	// there's something to upload. Text-to-video touches no storage.
	var sc *config.StorageConfig
	if len(req.References) > 0 {
		resolved, err := storage.Get(ctx)
		if err != nil {
			return nil, err
		}
		sc = resolved
	}

	content, err := p.buildContent(ctx, sc, req.Prompt, opts, imageRefs, videoRefs, audioRefs)
	if err != nil {
		return nil, err
	}

	body := createReq{
		Model:         opts.Model,
		Content:       content,
		GenerateAudio: opts.Audio,
		Resolution:    opts.Resolution,
		Ratio:         opts.AspectRatio,
		Duration:      opts.Duration,
		Watermark:     false,
	}

	created, err := transport.PostJSON[createResp](ctx, arkClient, baseURL+"/contents/generations/tasks", transport.Bearer(p.apiKey), body)
	if err != nil {
		return nil, fmt.Errorf("modelark: create task: %w", err)
	}

	videoURL, err := p.poll(ctx, created.ID)
	if err != nil {
		return nil, err
	}

	data, err := transport.GetBytes(ctx, downloadClient, videoURL, transport.NoAuth())
	if err != nil {
		return nil, fmt.Errorf("modelark: download video: %w", err)
	}
	return &providers.Response{Assets: []providers.GeneratedAsset{{Data: data, MimeType: "video/mp4"}}}, nil
}

// roledRef pairs a reference with the content role it plays.
type roledRef struct {
	ref  images.Reference
	role string
}

// buildContent classifies references into the (mutually exclusive, per API)
// modality, uploads every local reference through the storage brick using the
// pre-resolved config sc, and assembles the content[] array with the prompt
// prepended as a text item. sc is nil only when there are no references.
func (p *Provider) buildContent(ctx context.Context, sc *config.StorageConfig, prompt string, opts *Options, imageRefs, videoRefs, audioRefs []images.Reference) ([]contentItem, error) {
	content := []contentItem{{Type: "text", Text: prompt}}

	var items []roledRef
	switch {
	case len(imageRefs) == 0 && len(videoRefs) == 0 && len(audioRefs) == 0:
		// text → video
		if opts.EndImage != "" {
			return nil, errors.New("modelark: --end-image requires an input image (-i)")
		}

	case len(imageRefs) == 1 && len(videoRefs) == 0 && len(audioRefs) == 0:
		// image → video (first frame, optionally first+last frame)
		if len(imageRefs[0].Data) > maxImageBytes {
			return nil, errors.New("modelark: input image exceeds 30 MB")
		}
		items = append(items, roledRef{imageRefs[0], "first_frame"})

		if opts.EndImage != "" {
			endRefs, err := images.Load(opts.EndImage)
			if err != nil {
				return nil, fmt.Errorf("modelark: load --end-image: %w", err)
			}
			if len(endRefs) == 0 {
				return nil, fmt.Errorf("modelark: --end-image %q contains no image", opts.EndImage)
			}
			if len(endRefs[0].Data) > maxImageBytes {
				return nil, errors.New("modelark: --end-image exceeds 30 MB")
			}
			items = append(items, roledRef{endRefs[0], "last_frame"})
		}

	default:
		// multimodal reference → video
		if opts.EndImage != "" {
			return nil, errors.New("modelark: --end-image is only valid with a single -i input image")
		}
		if err := validateReferenceConstraints(imageRefs, videoRefs, audioRefs); err != nil {
			return nil, err
		}
		for _, ref := range imageRefs {
			items = append(items, roledRef{ref, "reference_image"})
		}
		for _, ref := range videoRefs {
			items = append(items, roledRef{ref, "reference_video"})
		}
		for _, ref := range audioRefs {
			items = append(items, roledRef{ref, "reference_audio"})
		}
	}

	for _, it := range items {
		item, err := p.uploadItem(ctx, sc, it.ref, it.role)
		if err != nil {
			return nil, err
		}
		content = append(content, item)
	}
	return content, nil
}

// uploadItem publishes one reference through the storage brick (reusing the
// pre-resolved config) and wraps the public URL in a content item with the
// matching type + role.
func (p *Provider) uploadItem(ctx context.Context, sc *config.StorageConfig, ref images.Reference, role string) (contentItem, error) {
	url, err := storage.UploadWith(ctx, sc, ref.Data, ref.MimeType)
	if err != nil {
		return contentItem{}, fmt.Errorf("modelark: upload reference: %w", err)
	}
	switch images.KindOf(ref.MimeType) {
	case "image":
		return contentItem{Type: "image_url", ImageURL: &urlObject{URL: url}, Role: role}, nil
	case "video":
		return contentItem{Type: "video_url", VideoURL: &urlObject{URL: url}, Role: role}, nil
	case "audio":
		return contentItem{Type: "audio_url", AudioURL: &urlObject{URL: url}, Role: role}, nil
	default:
		return contentItem{}, fmt.Errorf("modelark: unsupported reference type %q", ref.MimeType)
	}
}

// poll loops on the task retrieve endpoint until a terminal status, returning
// the output video URL on success. On ctx cancellation it best-effort cancels
// the task (only queued tasks are cancellable server-side) before returning.
func (p *Provider) poll(ctx context.Context, id string) (string, error) {
	url := baseURL + "/contents/generations/tasks/" + id
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		task, err := transport.GetJSON[taskResp](ctx, arkClient, url, transport.Bearer(p.apiKey))
		if err != nil {
			return "", fmt.Errorf("modelark: poll task: %w", err)
		}
		switch task.Status {
		case "succeeded":
			if task.Content == nil || task.Content.VideoURL == "" {
				return "", errors.New("modelark: task succeeded but no video URL was returned")
			}
			return task.Content.VideoURL, nil
		case "failed", "expired", "cancelled":
			return "", fmt.Errorf("modelark: task %s: %s", task.Status, taskErrorMessage(task))
		case "queued", "running":
			// Non-terminal — keep polling below.
		default:
			// Any other (or empty) status is outside the documented enum.
			// Treat it as terminal so a contract change can't make the loop
			// poll forever (there is no overall poll deadline; the per-request
			// timeout doesn't bound the loop).
			return "", fmt.Errorf("modelark: unexpected task status %q: %s", task.Status, taskErrorMessage(task))
		}

		select {
		case <-ctx.Done():
			p.cancel(id)
			return "", ctx.Err()
		case <-ticker.C:
		}
	}
}

// taskErrorMessage renders the task's error code/message, or a placeholder.
func taskErrorMessage(task *taskResp) string {
	if task.Error == nil {
		return "no error detail"
	}
	if task.Error.Code != "" {
		return task.Error.Code + ": " + task.Error.Message
	}
	return task.Error.Message
}

// cancel issues a best-effort DELETE on a fresh short-lived context (the
// caller's ctx is already cancelled — that's why we're here). Only queued
// tasks are cancellable server-side; a running task's DELETE is harmless.
func (p *Provider) cancel(id string) {
	ctx, done := context.WithTimeout(context.Background(), 5*time.Second)
	defer done()
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, baseURL+"/contents/generations/tasks/"+id, nil)
	if err != nil {
		return
	}
	if err := transport.Bearer(p.apiKey).Apply(req); err != nil {
		return
	}
	if resp, err := arkClient.Do(req); err == nil {
		_ = resp.Body.Close()
	}
}

// validateReferenceConstraints enforces the reference-to-video caps before any
// upload, so an over-limit request fails cheaply on local bytes. Count/size
// caps only; the API additionally enforces total-duration and pixel bounds
// server-side, which we intentionally don't replicate here.
func validateReferenceConstraints(imageRefs, videoRefs, audioRefs []images.Reference) error {
	if len(imageRefs) > maxRefImages {
		return fmt.Errorf("modelark: too many image references (%d, max %d)", len(imageRefs), maxRefImages)
	}
	if len(videoRefs) > maxRefVideos {
		return fmt.Errorf("modelark: too many video references (%d, max %d)", len(videoRefs), maxRefVideos)
	}
	if len(audioRefs) > maxRefAudios {
		return fmt.Errorf("modelark: too many audio references (%d, max %d)", len(audioRefs), maxRefAudios)
	}
	if len(audioRefs) > 0 && len(imageRefs)+len(videoRefs) == 0 {
		return errors.New("modelark: an audio reference requires at least one image or video reference")
	}
	for i, ref := range imageRefs {
		if len(ref.Data) > maxImageBytes {
			return fmt.Errorf("modelark: image reference %d exceeds 30 MB", i+1)
		}
	}
	for i, ref := range videoRefs {
		if len(ref.Data) > maxVideoBytes {
			return fmt.Errorf("modelark: video reference %d exceeds 50 MB", i+1)
		}
	}
	for i, ref := range audioRefs {
		if len(ref.Data) > maxAudioBytes {
			return fmt.Errorf("modelark: audio reference %d exceeds 15 MB", i+1)
		}
	}
	return nil
}
