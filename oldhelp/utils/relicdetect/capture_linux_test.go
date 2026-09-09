//go:build linux

package relicdetect

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"strings"
	"testing"
)

func TestCaptureToFileReadsWrittenPNG(t *testing.T) {
	img, err := captureToFile("stub", func(path string) error {
		f, err := os.Create(path)
		if err != nil {
			return err
		}
		defer f.Close()
		src := image.NewNRGBA(image.Rect(0, 0, 3, 2))
		src.SetNRGBA(2, 1, color.NRGBA{R: 9, G: 8, B: 7, A: 255})
		return png.Encode(f, src)
	})
	if err != nil {
		t.Fatalf("captureToFile() error = %v", err)
	}
	if got := img.Bounds(); got != image.Rect(0, 0, 3, 2) {
		t.Fatalf("captureToFile() bounds = %v, want %v", got, image.Rect(0, 0, 3, 2))
	}
	r, g, b, _ := img.At(2, 1).RGBA()
	if uint8(r>>8) != 9 || uint8(g>>8) != 8 || uint8(b>>8) != 7 {
		t.Fatalf("captureToFile() pixel = (%d,%d,%d), want (%d,%d,%d)", r>>8, g>>8, b>>8, 9, 8, 7)
	}
}

func TestCaptureToFileWrapsRunError(t *testing.T) {
	_, err := captureToFile("grim", func(string) error {
		return os.ErrPermission
	})
	if err == nil {
		t.Fatal("captureToFile() error = nil, want wrapped error")
	}
	if !strings.Contains(err.Error(), "grim: ") {
		t.Fatalf("captureToFile() error = %q, want prefixed tool name", err)
	}
}

func TestCaptureToFileInvalidPNG(t *testing.T) {
	_, err := captureToFile("stub", func(path string) error {
		return os.WriteFile(path, []byte("not-a-png"), 0644)
	})
	if err == nil {
		t.Fatal("captureToFile() error = nil, want decode error")
	}
}
