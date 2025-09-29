package main

import (
	"fmt"
	"log"
	"time"

	"github.com/lincaiyong/youtube-caption"
)

func main() {
	testVideoID := "vStJoetOxJg"

	fmt.Println("=== Getting available caption tracks ===")
	tracks, err := caption.GetAvailableTracks(testVideoID)
	if err != nil {
		log.Printf("Error getting tracks: %v\n", err)
		return
	}

	for i, track := range tracks {
		fmt.Printf("%d. %s\n", i+1, track.String())
	}

	fmt.Println("\n=== Downloading default captions ===")
	captions, err := caption.Download(testVideoID)
	if err != nil {
		log.Printf("Error: %v\n", err)
		return
	}

	fmt.Println("\n=== Getting subtitle text ===")
	texts := captions.GetSubtitleText()
	for i, text := range texts[:5] {
		fmt.Printf("%d. [%.1fs-%.1fs] %s\n", i+1, text.StartTime, text.EndTime, text.Text)
	}

	fmt.Println("\n=== Downloading with custom options ===")
	opts := &caption.Options{
		Language:   "en",
		Kind:       "asr",
		Timeout:    15 * time.Second,
		MaxRetries: 2,
		UserAgent:  "Custom User Agent",
	}

	captions2, err := caption.DownloadWithOptions(testVideoID, opts)
	if err != nil {
		log.Printf("Error with custom options: %v\n", err)
		return
	}

	fmt.Printf("\n=== Plain text (first 200 chars) ===\n%s...\n",
		captions2.GetPlainText()[:200])

	fmt.Println("\n=== Saving files ===")
	if err := captions.SaveToFile("captions.json"); err != nil {
		log.Printf("Error saving JSON: %v\n", err)
	} else {
		fmt.Println("Saved captions.json")
	}

	if err := captions.SaveSRT("captions.srt"); err != nil {
		log.Printf("Error saving SRT: %v\n", err)
	} else {
		fmt.Println("Saved captions.srt")
	}

	if err := captions.SaveVTT("captions.vtt"); err != nil {
		log.Printf("Error saving VTT: %v\n", err)
	} else {
		fmt.Println("Saved captions.vtt")
	}

	if err := captions.SavePlainText("captions.txt"); err != nil {
		log.Printf("Error saving plain text: %v\n", err)
	} else {
		fmt.Println("Saved captions.txt")
	}
}
