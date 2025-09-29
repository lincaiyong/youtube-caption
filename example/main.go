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
	fmt.Println(captions)
}
