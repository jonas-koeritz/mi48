package cv

import (
	"image"
	"image/color"

	"gocv.io/x/gocv"
)

func Read(src <-chan *image.Gray16, mat *gocv.Mat) {
	// Get frame from channel
	frame := <-src
	bounds := frame.Bounds()

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			mat.SetShortAt(y, x, int16((frame.At(x, y).(color.Gray16)).Y))
		}
	}
}
