package relicdetect

import "image"

type Capturer interface {
	CaptureAll() ([]image.Image, error)
}
