package main

import (
	"fmt"
	"log"

	"github.com/lincaiyong/youtube-caption"
)

func main() {
	testVideoId := "vStJoetOxJg"
	captions, err := caption.Download(testVideoId)
	if err != nil {
		log.Printf("Error: %v\n", err)
		return
	}

	texts := captions.GetSubtitleText()
	fmt.Printf("Downloaded %d caption segments\n", len(texts))

	if len(texts) > 0 {
		fmt.Printf("First caption: %.1fs - %s\n", texts[0].StartTime, texts[0].Text)
	}
}
