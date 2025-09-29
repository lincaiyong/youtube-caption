package caption

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

func (c *Caption) GetSubtitleText() []SubtitleText {
	var result []SubtitleText
	for _, event := range c.Events {
		if len(event.Segments) == 0 {
			continue
		}

		var text strings.Builder
		startTime := float64(event.TStartMs) / 1000.0
		endTime := startTime

		for _, seg := range event.Segments {
			if seg.UTF8 != "\n" {
				text.WriteString(seg.UTF8)
				segEndTime := float64(event.TStartMs+seg.TOffsetMs) / 1000.0
				if segEndTime > endTime {
					endTime = segEndTime
				}
			}
		}

		textStr := strings.TrimSpace(text.String())
		if textStr != "" {
			result = append(result, SubtitleText{
				StartTime: startTime,
				EndTime:   endTime,
				Text:      textStr,
			})
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].StartTime < result[j].StartTime
	})

	return result
}

func (c *Caption) GetPlainText() string {
	subtitles := c.GetSubtitleText()
	var result strings.Builder

	for _, sub := range subtitles {
		result.WriteString(sub.Text)
		result.WriteString(" ")
	}

	return strings.TrimSpace(result.String())
}

func (c *Caption) GetSRT() string {
	subtitles := c.GetSubtitleText()
	var result strings.Builder

	for i, sub := range subtitles {
		result.WriteString(fmt.Sprintf("%d\n", i+1))
		result.WriteString(fmt.Sprintf("%s --> %s\n",
			formatSRTTime(sub.StartTime),
			formatSRTTime(sub.EndTime)))
		result.WriteString(sub.Text)
		result.WriteString("\n\n")
	}

	return result.String()
}

func (c *Caption) GetVTT() string {
	subtitles := c.GetSubtitleText()
	var result strings.Builder

	result.WriteString("WEBVTT\n\n")

	for _, sub := range subtitles {
		result.WriteString(fmt.Sprintf("%s --> %s\n",
			formatVTTTime(sub.StartTime),
			formatVTTTime(sub.EndTime)))
		result.WriteString(sub.Text)
		result.WriteString("\n\n")
	}

	return result.String()
}

func (c *Caption) SaveToFile(filename string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal caption: %w", err)
	}
	return os.WriteFile(filename, data, 0644)
}

func (c *Caption) SaveSRT(filename string) error {
	return os.WriteFile(filename, []byte(c.GetSRT()), 0644)
}

func (c *Caption) SaveVTT(filename string) error {
	return os.WriteFile(filename, []byte(c.GetVTT()), 0644)
}

func (c *Caption) SavePlainText(filename string) error {
	return os.WriteFile(filename, []byte(c.GetPlainText()), 0644)
}

func formatSRTTime(seconds float64) string {
	t := time.Duration(seconds * float64(time.Second))
	hours := int(t.Hours())
	minutes := int(t.Minutes()) % 60
	secs := int(t.Seconds()) % 60
	millis := int(t.Nanoseconds()/1000000) % 1000
	return fmt.Sprintf("%02d:%02d:%02d,%03d", hours, minutes, secs, millis)
}

func formatVTTTime(seconds float64) string {
	t := time.Duration(seconds * float64(time.Second))
	hours := int(t.Hours())
	minutes := int(t.Minutes()) % 60
	secs := int(t.Seconds()) % 60
	millis := int(t.Nanoseconds()/1000000) % 1000
	return fmt.Sprintf("%02d:%02d:%02d.%03d", hours, minutes, secs, millis)
}

func (ct *CaptionTrack) String() string {
	return fmt.Sprintf("%s (%s) - %s", ct.Name.SimpleText, ct.LanguageCode, ct.Kind)
}
