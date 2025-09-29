# youtube-caption

A Go SDK for downloading YouTube auto-generated captions.

## Installation

```bash
go get github.com/lincaiyong/youtube-caption
```

## Usage

```go
package main

import (
    "fmt"
    "log"
    "github.com/lincaiyong/youtube-caption"
)

func main() {
    // Download captions
    captions, err := caption.Download("ATqljKtkhm8")
    if err != nil {
        log.Fatal(err)
    }

    // Get text with timestamps
    texts := captions.GetSubtitleText()
    for _, text := range texts[:3] {
        fmt.Printf("%.1fs: %s\n", text.StartTime, text.Text)
    }

    // Save to file
    captions.SaveToFile("captions.json")
}
```

## API

- `caption.Download(videoID)` - Download English captions
- `caption.DownloadWithOptions(videoID, language, kind)` - Download with options
- `captions.GetSubtitleText()` - Extract text with timestamps
- `captions.SaveToFile(filename)` - Save to JSON file

## Example

```bash
go run example/main.go
```

## License

MIT
