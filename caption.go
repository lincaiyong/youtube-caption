package caption

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"time"

	"github.com/cenkalti/backoff/v4"
)

var (
	ErrInvalidVideoID  = errors.New("invalid video ID")
	ErrNoCaptionsFound = errors.New("no captions found for this video")
	ErrRateLimited     = errors.New("rate limited by YouTube")
)

type CaptionTrack struct {
	BaseURL      string `json:"baseUrl"`
	LanguageCode string `json:"languageCode"`
	Name         struct {
		SimpleText string `json:"simpleText"`
	} `json:"name"`
	Kind string `json:"kind"`
}

type CaptionEvent struct {
	TStartMs int              `json:"tStartMs"`
	Segments []CaptionSegment `json:"segs,omitempty"`
}

type CaptionSegment struct {
	UTF8      string `json:"utf8"`
	TOffsetMs int    `json:"tOffsetMs"`
	AcAsrConf int    `json:"acAsrConf"`
}

type Caption struct {
	Events []CaptionEvent `json:"events"`
}

type SubtitleText struct {
	StartTime float64
	EndTime   float64
	Text      string
}

type Options struct {
	Language   string
	Kind       string
	Timeout    time.Duration
	MaxRetries int
	UserAgent  string
}

const (
	playerURL         = "https://www.youtube.com/youtubei/v1/player?prettyPrint=false"
	defaultUA         = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/15.5 Safari/605.1.15"
	defaultTimeout    = 30 * time.Second
	defaultMaxRetries = 3
)

var videoIDRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]{11}$`)

func validateVideoID(videoID string) error {
	if videoID == "" {
		return ErrInvalidVideoID
	}
	if !videoIDRegex.MatchString(videoID) {
		return ErrInvalidVideoID
	}
	return nil
}

func makeRequestWithRetry(ctx context.Context, client *http.Client, req *http.Request, maxRetries int) (*http.Response, error) {
	var resp *http.Response
	operation := func() error {
		reqWithCtx := req.WithContext(ctx)
		var err error
		resp, err = client.Do(reqWithCtx)
		if err != nil {
			return err
		}
		switch resp.StatusCode {
		case http.StatusOK:
			return nil
		case http.StatusTooManyRequests:
			_ = resp.Body.Close()
			return ErrRateLimited
		case http.StatusNotFound:
			_ = resp.Body.Close()
			return ErrNoCaptionsFound
		default:
			if resp.StatusCode >= 500 {
				_ = resp.Body.Close()
				return fmt.Errorf("server error: HTTP %d", resp.StatusCode)
			}
			return fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
		}
	}

	backoffConfig := backoff.NewExponentialBackOff()
	backoffConfig.MaxElapsedTime = time.Duration(maxRetries) * 10 * time.Second
	err := backoff.Retry(operation, backoffConfig)
	return resp, err
}

func makeRequestData(videoID string) ([]byte, error) {
	var playerReq struct {
		Context struct {
			Client struct {
				ClientName    string `json:"clientName"`
				ClientVersion string `json:"clientVersion"`
			} `json:"client"`
		} `json:"context"`
		VideoID string `json:"videoId"`
	}
	playerReq.VideoID = videoID
	playerReq.Context.Client.ClientName = "WEB"
	playerReq.Context.Client.ClientVersion = "2.20250925.01.00"
	return json.Marshal(playerReq)
}

func extractCaptionTracks(resp *http.Response) ([]CaptionTrack, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	var playerResp struct {
		Captions struct {
			PlayerCaptionsTracklistRenderer struct {
				CaptionTracks []CaptionTrack `json:"captionTracks"`
			} `json:"playerCaptionsTracklistRenderer"`
		} `json:"captions"`
	}
	if err = json.Unmarshal(body, &playerResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	tracks := playerResp.Captions.PlayerCaptionsTracklistRenderer.CaptionTracks
	if len(tracks) == 0 {
		return nil, ErrNoCaptionsFound
	}
	return tracks, nil
}

func findCaptionTrack(tracks []CaptionTrack, opts *Options) (*CaptionTrack, error) {
	for _, track := range tracks {
		if track.LanguageCode == opts.Language && track.Kind == opts.Kind {
			if track.BaseURL != "" {
				return &track, nil
			}
		}
	}

	for _, track := range tracks {
		if track.LanguageCode == opts.Language {
			if track.BaseURL != "" {
				return &track, nil
			}
		}
	}

	if len(tracks) > 0 && tracks[0].BaseURL != "" {
		return &tracks[0], nil
	}

	return nil, ErrNoCaptionsFound
}

func requestCaptionTrack(ctx context.Context, client *http.Client, videoID string, opts *Options) (*CaptionTrack, error) {
	data, err := makeRequestData(videoID)
	if err != nil {
		return nil, fmt.Errorf("failed to create request data: %w", err)
	}

	req, err := http.NewRequest("POST", playerURL, bytes.NewBuffer(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", opts.UserAgent)

	resp, err := makeRequestWithRetry(ctx, client, req, opts.MaxRetries)
	if err != nil {
		return nil, fmt.Errorf("failed to get response: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	tracks, err := extractCaptionTracks(resp)
	if err != nil {
		return nil, fmt.Errorf("failed to extract caption tracks: %w", err)
	}

	track, err := findCaptionTrack(tracks, opts)
	if err != nil {
		return nil, err
	}

	return track, nil
}

func requestTimedText(ctx context.Context, client *http.Client, track *CaptionTrack, opts *Options) (*Caption, error) {
	captionURL := track.BaseURL + "&fmt=json3"
	req, err := http.NewRequest("GET", captionURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", opts.UserAgent)

	resp, err := makeRequestWithRetry(ctx, client, req, opts.MaxRetries)
	if err != nil {
		return nil, fmt.Errorf("failed to get response: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read subtitle response: %w", err)
	}

	var caption Caption
	if err = json.Unmarshal(body, &caption); err != nil {
		return nil, fmt.Errorf("failed to unmarshal subtitle response: %w", err)
	}
	return &caption, nil
}

func newHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			MaxIdleConns:       10,
			IdleConnTimeout:    30 * time.Second,
			DisableCompression: false,
		},
	}
}

func DefaultOptions() *Options {
	return &Options{
		Language:   "en",
		Kind:       "asr",
		Timeout:    defaultTimeout,
		MaxRetries: defaultMaxRetries,
		UserAgent:  defaultUA,
	}
}

func Download(videoID string) (*Caption, error) {
	return DownloadWithOptions(videoID, DefaultOptions())
}

func DownloadWithOptions(videoID string, opts *Options) (*Caption, error) {
	ctx, cancel := context.WithTimeout(context.Background(), opts.Timeout)
	defer cancel()
	return DownloadWithContext(ctx, videoID, opts)
}

func DownloadWithContext(ctx context.Context, videoID string, opts *Options) (*Caption, error) {
	if err := validateVideoID(videoID); err != nil {
		return nil, err
	}

	if opts == nil {
		opts = DefaultOptions()
	}

	client := newHTTPClient(opts.Timeout)

	track, err := requestCaptionTrack(ctx, client, videoID, opts)
	if err != nil {
		return nil, err
	}

	caption, err := requestTimedText(ctx, client, track, opts)
	if err != nil {
		return nil, err
	}

	return caption, nil
}

func GetAvailableTracks(videoID string) ([]CaptionTrack, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()
	return GetAvailableTracksWithContext(ctx, videoID)
}

func GetAvailableTracksWithContext(ctx context.Context, videoID string) ([]CaptionTrack, error) {
	if err := validateVideoID(videoID); err != nil {
		return nil, err
	}

	opts := DefaultOptions()
	client := newHTTPClient(opts.Timeout)

	data, err := makeRequestData(videoID)
	if err != nil {
		return nil, fmt.Errorf("failed to create request data: %w", err)
	}

	req, err := http.NewRequest("POST", playerURL, bytes.NewBuffer(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", opts.UserAgent)

	resp, err := makeRequestWithRetry(ctx, client, req, opts.MaxRetries)
	if err != nil {
		return nil, fmt.Errorf("failed to get response: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	return extractCaptionTracks(resp)
}
