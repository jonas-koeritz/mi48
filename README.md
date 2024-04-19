# Go-MI48

This is a Go implementation of the USB protocol of the MI48XX Thermal Image Processor by Meridian Innovation

## Usage with GoCV

Use a `Read()` function like this to fill a gocv.Mat with thermal image data

```Go
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
```
