package caption

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/cenkalti/backoff/v4"
)

// PlayerRequest represents the YouTube player API request
type PlayerRequest struct {
	Context   Context `json:"context"`
	VideoID   string  `json:"videoId"`
	ContentOK bool    `json:"contentCheckOk"`
	RacyOK    bool    `json:"racyCheckOk"`
}

// Context represents the client context in the request
type Context struct {
	Client Client `json:"client"`
}

// Client represents the client information
type Client struct {
	ClientName       string `json:"clientName"`
	ClientVersion    string `json:"clientVersion"`
	UserAgent        string `json:"userAgent"`
	HL               string `json:"hl"`
	TimeZone         string `json:"timeZone"`
	UTCOffsetMinutes int    `json:"utcOffsetMinutes"`
}

// PlayerResponse represents the YouTube player API response
type PlayerResponse struct {
	Captions Captions `json:"captions"`
}

// Captions contains caption track information
type Captions struct {
	PlayerCaptionsTracklistRenderer PlayerCaptionsTracklistRenderer `json:"playerCaptionsTracklistRenderer"`
}

// PlayerCaptionsTracklistRenderer contains the caption tracks
type PlayerCaptionsTracklistRenderer struct {
	CaptionTracks []CaptionTrack `json:"captionTracks"`
}

// CaptionTrack represents a single caption track
type CaptionTrack struct {
	BaseURL      string `json:"baseUrl"`
	LanguageCode string `json:"languageCode"`
	Kind         string `json:"kind"`
}

// SubtitleResponse represents the subtitle content response
type SubtitleResponse struct {
	Events []SubtitleEvent `json:"events"`
}

// SubtitleEvent represents a single subtitle event
type SubtitleEvent struct {
	TStartMs int               `json:"tStartMs"`
	Segs     []SubtitleSegment `json:"segs,omitempty"`
}

// SubtitleSegment represents a segment of subtitle text
type SubtitleSegment struct {
	UTF8 string `json:"utf8"`
}

// createHTTPClient creates an HTTP client with retry logic and SSL skip
func createHTTPClient() *http.Client {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}

	return client
}

// makeRequestWithRetry makes HTTP request with exponential backoff retry
func makeRequestWithRetry(client *http.Client, req *http.Request) (*http.Response, error) {
	var resp *http.Response
	var err error

	operation := func() error {
		resp, err = client.Do(req)
		if err != nil {
			return err
		}

		// Retry on 5xx errors and 429 (Too Many Requests)
		if resp.StatusCode >= 500 || resp.StatusCode == 429 {
			resp.Body.Close()
			return fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
		}

		return nil
	}

	backoffConfig := backoff.NewExponentialBackOff()
	backoffConfig.MaxElapsedTime = 30 * time.Second

	err = backoff.Retry(operation, backoffConfig)
	return resp, err
}

// Download downloads YouTube subtitles for the given video ID
func Download(videoID string) (*SubtitleResponse, error) {
	return DownloadWithOptions(videoID, "en", "asr")
}

// DownloadWithOptions downloads YouTube subtitles with specified language and kind
func DownloadWithOptions(videoID, language, kind string) (*SubtitleResponse, error) {
	client := createHTTPClient()

	// Step 1: Get player information
	playerURL := "https://www.youtube.com/youtubei/v1/player?prettyPrint=false"

	playerReq := PlayerRequest{
		Context: Context{
			Client: Client{
				ClientName:       "WEB",
				ClientVersion:    "2.20250925.01.00",
				UserAgent:        "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/15.5 Safari/605.1.15,gzip(gfe)",
				HL:               "en",
				TimeZone:         "UTC",
				UTCOffsetMinutes: 0,
			},
		},
		VideoID:   videoID,
		ContentOK: true,
		RacyOK:    true,
	}

	playerData, err := json.Marshal(playerReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal player request: %w", err)
	}

	req, err := http.NewRequest("POST", playerURL, bytes.NewBuffer(playerData))
	if err != nil {
		return nil, fmt.Errorf("failed to create player request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/15.5 Safari/605.1.15,gzip(gfe)")

	resp, err := makeRequestWithRetry(client, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get player response: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("player API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read player response: %w", err)
	}

	var playerResp PlayerResponse
	if err := json.Unmarshal(body, &playerResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal player response: %w", err)
	}

	// Step 2: Find the subtitle URL
	var subtitleURL string
	for _, track := range playerResp.Captions.PlayerCaptionsTracklistRenderer.CaptionTracks {
		if track.LanguageCode == language && track.Kind == kind {
			if track.BaseURL != "" {
				subtitleURL = track.BaseURL + "&fmt=json3"
				break
			}
		}
	}

	if subtitleURL == "" {
		return nil, fmt.Errorf("no subtitle track found for language=%s, kind=%s", language, kind)
	}

	// Step 3: Download subtitle content
	req, err = http.NewRequest("GET", subtitleURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create subtitle request: %w", err)
	}

	resp, err = makeRequestWithRetry(client, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get subtitle response: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("subtitle API returned status %d", resp.StatusCode)
	}

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read subtitle response: %w", err)
	}

	var subtitleResp SubtitleResponse
	if err := json.Unmarshal(body, &subtitleResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal subtitle response: %w", err)
	}

	return &subtitleResp, nil
}

// GetSubtitleText extracts plain text from subtitle events
func (sr *SubtitleResponse) GetSubtitleText() []SubtitleText {
	var texts []SubtitleText

	for _, event := range sr.Events {
		if len(event.Segs) > 0 {
			var text string
			for _, seg := range event.Segs {
				text += seg.UTF8
			}
			if text != "" {
				texts = append(texts, SubtitleText{
					StartTime: float64(event.TStartMs) / 1000.0,
					Text:      text,
				})
			}
		}
	}

	return texts
}

// SubtitleText represents a subtitle text with timestamp
type SubtitleText struct {
	StartTime float64 `json:"startTime"`
	Text      string  `json:"text"`
}

// SaveToFile saves subtitle response to a JSON file
func (sr *SubtitleResponse) SaveToFile(filename string) error {
	data, err := json.MarshalIndent(sr, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal subtitle data: %w", err)
	}

	return writeFile(filename, data)
}

// writeFile writes data to a file
func writeFile(filename string, data []byte) error {
	return os.WriteFile(filename, data, 0644)
}
