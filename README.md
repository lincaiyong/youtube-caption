# youtube-caption

A simple Go library for downloading YouTube auto-generated captions.

## Installation

```bash
go get github.com/lincaiyong/youtube-caption
```

## Quick Start

```go
package main

import (
    "fmt"
    "log"
    "github.com/lincaiyong/youtube-caption"
)

func main() {
    captions, err := caption.Download("vStJoetOxJg")
    if err != nil {
        log.Fatal(err)
    }

    // Get text with timestamps
    texts := captions.GetSubtitleText()
    for _, text := range texts[:3] {
        fmt.Printf("%.1fs: %s\n", text.StartTime, text.Text)
    }

    // Export to different formats
    captions.SaveSRT("captions.srt")
    captions.SaveVTT("captions.vtt")
    captions.SavePlainText("captions.txt")
}
```

## Features

- Download YouTube auto-generated captions
- Export to SRT, VTT, plain text, or JSON
- Custom language and timeout options
- Get available caption tracks

## API

```go
// Basic usage
caption.Download(videoID)
caption.GetAvailableTracks(videoID)

// With options
opts := &caption.Options{
    Language: "en",
    Timeout:  15 * time.Second,
}
caption.DownloadWithOptions(videoID, opts)

// Export methods
captions.GetSubtitleText()  // []SubtitleText
captions.GetPlainText()     // string
captions.GetSRT()           // string
captions.GetVTT()           // string
```

## License

MIT
