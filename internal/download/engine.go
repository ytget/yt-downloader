package download

import (
	"context"

	"github.com/ytget/ytdlp/v2"
	"github.com/ytget/ytdlp/v2/types"
)

// ProgressFunc reports media download progress.
type ProgressFunc func(p ytdlp.Progress)

// MediaRequest configures a single media download/resolve operation.
type MediaRequest struct {
	URL           string
	Quality       string
	Ext           string
	OutputPath    string
	AudioLanguage string // multi-language adaptive audio preference
	OnProgress    ProgressFunc
}

// CaptionsEngine fetches YouTube caption tracks.
type CaptionsEngine interface {
	GetVideoInfo(ctx context.Context, videoURL string) (*ytdlp.VideoInfo, error)
	DownloadCaptions(ctx context.Context, videoURL, preferredLang string) (*types.Captions, types.CaptionTrack, error)
}

// MediaEngine resolves and downloads progressive media.
type MediaEngine interface {
	Resolve(ctx context.Context, req MediaRequest) (*ytdlp.VideoInfo, error)
	Download(ctx context.Context, req MediaRequest) (*ytdlp.VideoInfo, error)
}

// YTDLPEngine is the production MediaEngine + CaptionsEngine backed by ytdlp/v2.
type YTDLPEngine struct{}

// NewYTDLPEngine returns the default production engine.
func NewYTDLPEngine() *YTDLPEngine {
	return &YTDLPEngine{}
}

func (e *YTDLPEngine) base(req MediaRequest) *ytdlp.Downloader {
	d := ytdlp.New().WithFormat(req.Quality, req.Ext)
	return d
}

// Resolve implements MediaEngine.
func (e *YTDLPEngine) Resolve(ctx context.Context, req MediaRequest) (*ytdlp.VideoInfo, error) {
	d := e.base(req)
	_, info, err := d.ResolveURL(ctx, req.URL)
	return info, err
}

// Download implements MediaEngine.
func (e *YTDLPEngine) Download(ctx context.Context, req MediaRequest) (*ytdlp.VideoInfo, error) {
	d := e.base(req)
	if req.OutputPath != "" {
		d = d.WithOutputPath(req.OutputPath)
	}
	if req.OnProgress != nil {
		d = d.WithProgress(func(p ytdlp.Progress) {
			req.OnProgress(p)
		})
	}
	return d.Download(ctx, req.URL)
}

// GetVideoInfo implements CaptionsEngine.
func (e *YTDLPEngine) GetVideoInfo(ctx context.Context, videoURL string) (*ytdlp.VideoInfo, error) {
	return ytdlp.New().GetVideoInfo(ctx, videoURL)
}

// DownloadCaptions implements CaptionsEngine.
func (e *YTDLPEngine) DownloadCaptions(ctx context.Context, videoURL, preferredLang string) (*types.Captions, types.CaptionTrack, error) {
	return ytdlp.New().DownloadCaptions(ctx, videoURL, preferredLang)
}

// Ensure YTDLPEngine satisfies both interfaces.
var (
	_ MediaEngine    = (*YTDLPEngine)(nil)
	_ CaptionsEngine = (*YTDLPEngine)(nil)
)
