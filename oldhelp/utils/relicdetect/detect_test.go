package relicdetect

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func fillNRGBA(img *image.NRGBA, r image.Rectangle, c color.NRGBA) {
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			img.SetNRGBA(x, y, c)
		}
	}
}

func newRewardScreen(templateColor, bg color.NRGBA) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, refWidth, refHeight))
	fillNRGBA(img, img.Bounds(), bg)
	fillNRGBA(img, image.Rect(tmplX, tmplY, tmplX+tmplW, tmplY+tmplH), templateColor)
	return img
}

func solidTemplate(c color.NRGBA) Template {
	pix := make([]byte, tmplW*tmplH*3)
	for i := 0; i < tmplW*tmplH; i++ {
		pix[i*3] = c.R
		pix[i*3+1] = c.G
		pix[i*3+2] = c.B
	}
	return Template{pix: pix}
}

func encodePNG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("png.Encode() error = %v", err)
	}
	return buf.Bytes()
}

func TestLoadTemplate(t *testing.T) {
	tests := []struct {
		name     string
		build    func() image.Image
		wantPix  []byte
		wantMask []byte
	}{
		{
			name: "opaque png",
			build: func() image.Image {
				img := image.NewNRGBA(image.Rect(0, 0, 2, 1))
				img.SetNRGBA(0, 0, color.NRGBA{R: 1, G: 2, B: 3, A: 255})
				img.SetNRGBA(1, 0, color.NRGBA{R: 4, G: 5, B: 6, A: 255})
				return img
			},
			wantPix: []byte{1, 2, 3, 4, 5, 6},
		},
		{
			name: "alpha png builds mask",
			build: func() image.Image {
				img := image.NewNRGBA(image.Rect(0, 0, 2, 1))
				img.SetNRGBA(0, 0, color.NRGBA{R: 7, G: 8, B: 9, A: 255})
				img.SetNRGBA(1, 0, color.NRGBA{R: 10, G: 11, B: 12, A: 128})
				return img
			},
			// png.Decode returns colors through RGBA(), so translucent pixels are premultiplied.
			wantPix:  []byte{7, 8, 9, 5, 5, 6},
			wantMask: []byte{255, 128},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "template.png")
			if err := os.WriteFile(path, encodePNG(t, tt.build()), 0644); err != nil {
				t.Fatalf("os.WriteFile() error = %v", err)
			}

			got, err := LoadTemplate(path)
			if err != nil {
				t.Fatalf("LoadTemplate() error = %v", err)
			}
			if !bytes.Equal(got.pix, tt.wantPix) {
				t.Fatalf("LoadTemplate() pix = %v, want %v", got.pix, tt.wantPix)
			}
			if !bytes.Equal(got.mask, tt.wantMask) {
				t.Fatalf("LoadTemplate() mask = %v, want %v", got.mask, tt.wantMask)
			}
		})
	}
}

func TestEnsureTemplate(t *testing.T) {
	t.Run("skips existing file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "reward_template.png")
		if err := os.WriteFile(path, []byte("keep"), 0644); err != nil {
			t.Fatalf("os.WriteFile() error = %v", err)
		}

		oldURL := templateURL
		templateURL = "http://127.0.0.1:1/should-not-run"
		defer func() { templateURL = oldURL }()

		if err := EnsureTemplate(context.Background(), path); err != nil {
			t.Fatalf("EnsureTemplate() error = %v", err)
		}
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("os.ReadFile() error = %v", err)
		}
		if string(got) != "keep" {
			t.Fatalf("EnsureTemplate() rewrote existing file to %q", string(got))
		}
	})

	t.Run("downloads missing file", func(t *testing.T) {
		want := []byte("png-bytes")
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Fatalf("request method = %q, want %q", r.Method, http.MethodGet)
			}
			_, _ = w.Write(want)
		}))
		defer server.Close()

		oldURL := templateURL
		oldClient := templateHTTPClient
		templateURL = server.URL
		templateHTTPClient = server.Client()
		defer func() {
			templateURL = oldURL
			templateHTTPClient = oldClient
		}()

		path := filepath.Join(t.TempDir(), "nested", "reward_template.png")
		if err := EnsureTemplate(context.Background(), path); err != nil {
			t.Fatalf("EnsureTemplate() error = %v", err)
		}
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("os.ReadFile() error = %v", err)
		}
		if !bytes.Equal(got, want) {
			 t.Fatalf("EnsureTemplate() wrote %q, want %q", got, want)
		}
	})
}

func TestSAD(t *testing.T) {
	t.Run("exact match", func(t *testing.T) {
		tmplColor := color.NRGBA{R: 12, G: 34, B: 56, A: 255}
		img := newRewardScreen(tmplColor, color.NRGBA{R: 99, G: 88, B: 77, A: 255})

		if got := SAD(img, solidTemplate(tmplColor)); got != 0 {
			t.Fatalf("SAD() = %d, want 0", got)
		}
	})

	t.Run("all masked pixels skipped", func(t *testing.T) {
		img := newRewardScreen(color.NRGBA{R: 1, G: 2, B: 3, A: 255}, color.NRGBA{A: 255})
		tmpl := solidTemplate(color.NRGBA{R: 4, G: 5, B: 6, A: 255})
		tmpl.mask = make([]byte, tmplW*tmplH)

		if got := SAD(img, tmpl); got != ^uint64(0) {
			t.Fatalf("SAD() = %d, want %d", got, ^uint64(0))
		}
	})
}

func TestFindRewardScreenFindsRightMonitorSlice(t *testing.T) {
	tmplColor := color.NRGBA{R: 40, G: 90, B: 140, A: 255}
	tmpl := solidTemplate(tmplColor)
	left := newRewardScreen(color.NRGBA{R: 0, G: 0, B: 0, A: 255}, color.NRGBA{R: 20, G: 20, B: 20, A: 255})
	right := newRewardScreen(tmplColor, color.NRGBA{R: 10, G: 10, B: 10, A: 255})
	combined := image.NewNRGBA(image.Rect(0, 0, refWidth*2, refHeight))
	fillNRGBA(combined, combined.Bounds(), color.NRGBA{A: 255})
	for y := 0; y < refHeight; y++ {
		for x := 0; x < refWidth; x++ {
			combined.Set(x, y, left.At(x, y))
			combined.Set(x+refWidth, y, right.At(x, y))
		}
	}

	got := FindRewardScreen(combined, tmpl, 1)
	if got == nil {
		t.Fatal("FindRewardScreen() = nil, want matching monitor slice")
	}
	if got.Bounds().Dx() != refWidth || got.Bounds().Dy() != refHeight {
		t.Fatalf("FindRewardScreen() bounds = %v, want %dx%d", got.Bounds(), refWidth, refHeight)
	}
	if sad := SAD(got, tmpl); sad != 0 {
		t.Fatalf("FindRewardScreen() returned slice with SAD %d, want 0", sad)
	}
}
