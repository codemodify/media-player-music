// Package appicon prepares the app's logo for window and system-tray icons.
package appicon

import (
	"bytes"
	"fmt"
	"image"
	"image/png"

	"goframework.io/graphics-paint2d"
	"golang.org/x/image/draw"
)

// Images decodes the embedded logo and scales it for title bars, taskbars,
// application switchers and the system tray, preserving its transparency.
func Images(data []byte) ([]*paint2d.Image, error) {
	source, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode player logo: %w", err)
	}
	sizes := []int{16, 22, 24, 32, 48, 64, 96, 128}
	images := make([]*paint2d.Image, 0, len(sizes))
	for _, size := range sizes {
		resized := image.NewRGBA(image.Rect(0, 0, size, size))
		draw.CatmullRom.Scale(resized, resized.Bounds(), source, source.Bounds(), draw.Src, nil)
		images = append(images, paint2d.NewImageFromNRGBA(resized))
	}
	return images, nil
}
