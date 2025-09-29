package caption

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/cenkalti/backoff/v4"
)

type Caption struct {
	Events []struct {
		TStartMs int `json:"tStartMs"`
		Segments []struct {
			UTF8      string `json:"utf8"`
			TOffsetMs int    `json:"tOffsetMs"`
			AcAsrConf int    `json:"acAsrConf"`
		} `json:"segs,omitempty"`
	} `json:"events"`
}

const playerURL = "https://www.youtube.com/youtubei/v1/player?prettyPrint=false"

func makeRequestWithRetry(client *http.Client, req *http.Request) (*http.Response, error) {
	var resp *http.Response
	operation := func() error {
		var err error
		resp, err = client.Do(req)
		if err != nil {
			return err
		}
		if resp.StatusCode >= 500 || resp.StatusCode == 429 {
			_ = resp.Body.Close()
			return fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
		}
		return nil
	}
	backoffConfig := backoff.NewExponentialBackOff()
	backoffConfig.MaxElapsedTime = 30 * time.Second
	err := backoff.Retry(operation, backoffConfig)
	return resp, err
}

func makeRequestData(videoId string) []byte {
	var playerReq struct {
		Context struct {
			Client struct {
				ClientName    string `json:"clientName"`
				ClientVersion string `json:"clientVersion"`
			} `json:"client"`
		} `json:"context"`
		VideoID string `json:"videoId"`
	}
	playerReq.VideoID = videoId
	playerReq.Context.Client.ClientName = "WEB"
	playerReq.Context.Client.ClientVersion = "2.20250925.01.00"
	b, _ := json.Marshal(playerReq)
	return b
}

func extractCaptionUrl(resp *http.Response) (string, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}
	var playerResp struct {
		Captions struct {
			PlayerCaptionsTracklistRenderer struct {
				CaptionTracks []struct {
					BaseURL      string `json:"baseUrl"`
					LanguageCode string `json:"languageCode"`
					Kind         string `json:"kind"`
				} `json:"captionTracks"`
			} `json:"playerCaptionsTracklistRenderer"`
		} `json:"captions"`
	}
	if err = json.Unmarshal(body, &playerResp); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %w", err)
	}
	for _, track := range playerResp.Captions.PlayerCaptionsTracklistRenderer.CaptionTracks {
		if track.LanguageCode == "en" && track.Kind == "asr" {
			if track.BaseURL != "" {
				return track.BaseURL + "&fmt=json3", nil
			}
		}
	}
	return "", fmt.Errorf("no en/asr subtitle track found")
}

func requestCaptionUrl(client *http.Client, videoId string) (string, error) {
	data := makeRequestData(videoId)
	req, err := http.NewRequest("POST", playerURL, bytes.NewBuffer(data))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/15.5 Safari/605.1.15,gzip(gfe)")
	resp, err := makeRequestWithRetry(client, req)
	if err != nil {
		return "", fmt.Errorf("failed to get response: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("request returned status %d", resp.StatusCode)
	}

	ret, err := extractCaptionUrl(resp)
	if err != nil {
		return "", fmt.Errorf("failed to extract caption url: %w", err)
	}
	return ret, nil
}

func requestTimedText(client *http.Client, captionUrl string) (*Caption, error) {
	req, err := http.NewRequest("GET", captionUrl, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := makeRequestWithRetry(client, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get response: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request returned status %d", resp.StatusCode)
	}

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

func Download(videoId string) (*Caption, error) {
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
		Timeout: 30 * time.Second,
	}
	captionUrl, err := requestCaptionUrl(client, videoId)
	if err != nil {
		return nil, err
	}
	caption, err := requestTimedText(client, captionUrl)
	if err != nil {
		return nil, err
	}
	return caption, nil
}
