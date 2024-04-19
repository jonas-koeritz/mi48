package main

import (
	"image/png"
	"log"
	"os"

	"github.com/jonas-koeritz/mi48"
)

func main() {
	c, err := mi48.Open()

	if err != nil {
		log.Printf("ERROR: %s\n", err)
		return
	}

	log.Printf("Opened MI48 device: %+v\n", c)

	_, err = c.SetFramerate(25.0)
	if err != nil {
		log.Printf("ERROR: %s\n", err)
	}

	cancel, stream, err := c.StartStream()
	if err != nil {
		log.Printf("ERROR: %s\n", err)
		return
	}
	defer cancel()

	frame := <-stream

	frameFile, err := os.Create("frame.png")
	if err != nil {
		log.Printf("ERROR: %s\n", err)
		return
	}
	defer frameFile.Close()

	err = png.Encode(frameFile, frame)
	if err != nil {
		log.Printf("ERROR: %s\n", err)
		return
	}
}
