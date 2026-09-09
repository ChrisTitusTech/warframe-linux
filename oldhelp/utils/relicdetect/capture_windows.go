//go:build windows

package relicdetect

import (
	"fmt"
	"image"

	"github.com/kbinani/screenshot"
)

type winCapturer struct{}

func ProbeCapture() (Capturer, error) {
	if screenshot.NumActiveDisplays() == 0 {
		return nil, fmt.Errorf("no active displays found")
	}
	return winCapturer{}, nil
}

func (w winCapturer) CaptureAll() ([]image.Image, error) {
	n := screenshot.NumActiveDisplays()
	imgs := make([]image.Image, 0, n)
	for i := 0; i < n; i++ {
		img, err := screenshot.CaptureDisplay(i)
		if err != nil {
			return nil, fmt.Errorf("display %d: %w", i, err)
		}
		imgs = append(imgs, img)
	}
	return imgs, nil
}
