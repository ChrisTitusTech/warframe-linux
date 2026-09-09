//go:build manual && linux

package relicdetect

import "testing"

func TestManualProbeCapture(t *testing.T) {
	c, err := ProbeCapture()
	if err != nil {
		t.Fatalf("capture environment unavailable: %v", err)
	}
	imgs, err := c.CaptureAll()
	if err != nil {
		t.Fatalf("capture execution unavailable: %v", err)
	}
	if len(imgs) == 0 {
		t.Fatal("CaptureAll() returned no images")
	}
	for i, img := range imgs {
		if img == nil {
			t.Fatalf("CaptureAll() image %d = nil", i)
		}
		if img.Bounds().Dx() == 0 || img.Bounds().Dy() == 0 {
			t.Fatalf("CaptureAll() image %d bounds = %v", i, img.Bounds())
		}
	}
}
