package relicdetect

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

type monitorSlice struct {
	img    image.Image
	bounds image.Rectangle
}

func (s monitorSlice) ColorModel() color.Model { return s.img.ColorModel() }
func (s monitorSlice) Bounds() image.Rectangle { return image.Rect(0, 0, s.bounds.Dx(), s.bounds.Dy()) }
func (s monitorSlice) At(x, y int) color.Color { return s.img.At(s.bounds.Min.X+x, s.bounds.Min.Y+y) }

const (
	refWidth  = 2560
	refHeight = 1440
	tmplX     = 380
	tmplY     = 62
	tmplW     = 735
	tmplH     = 60

	RewardThreshold uint64 = 4465474
)

type Template struct {
	pix  []byte
	mask []byte // non-nil when template has alpha; indexed by pixel, 0 = skip
}

func LoadTemplate(path string) (Template, error) {
	f, err := os.Open(path)
	if err != nil {
		return Template{}, err
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		return Template{}, err
	}
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	n := w * h
	pix := make([]byte, n*3)
	mask := make([]byte, n)
	hasMask := false
	i := 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, a := img.At(x, y).RGBA()
			pix[i*3] = byte(r >> 8)
			pix[i*3+1] = byte(g >> 8)
			pix[i*3+2] = byte(b >> 8)
			alpha := byte(a >> 8)
			mask[i] = alpha
			if alpha < 255 {
				hasMask = true
			}
			i++
		}
	}
	t := Template{pix: pix}
	if hasMask {
		t.mask = mask
	}
	return t, nil
}

var templateURL = "https://raw.githubusercontent.com/gjrud/warframe-helper/main/assets/reward_template.png"

var templateHTTPClient = &http.Client{Timeout: 30 * time.Second}

func EnsureTemplate(ctx context.Context, path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, templateURL, nil)
	if err != nil {
		return err
	}
	resp, err := templateHTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("fetch template: status %d", resp.StatusCode)
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	return err
}

func scaleAndCropRGB(img image.Image) []byte {
	sb := img.Bounds()
	sw, sh := sb.Dx(), sb.Dy()
	out := make([]byte, tmplW*tmplH*3)
	i := 0
	for dy := 0; dy < tmplH; dy++ {
		sy := sb.Min.Y + (tmplY+dy)*sh/refHeight
		for dx := 0; dx < tmplW; dx++ {
			sx := sb.Min.X + (tmplX+dx)*sw/refWidth
			r, g, b, _ := img.At(sx, sy).RGBA()
			out[i] = byte(r >> 8)
			out[i+1] = byte(g >> 8)
			out[i+2] = byte(b >> 8)
			i += 3
		}
	}
	return out
}

func scaleAndCropImage(img image.Image, r image.Rectangle) *image.NRGBA {
	sb := img.Bounds()
	sw, sh := sb.Dx(), sb.Dy()
	w, h := r.Dx(), r.Dy()
	dst := image.NewNRGBA(image.Rect(0, 0, w, h))
	for dy := 0; dy < h; dy++ {
		sy := sb.Min.Y + (r.Min.Y+dy)*sh/refHeight
		for dx := 0; dx < w; dx++ {
			sx := sb.Min.X + (r.Min.X+dx)*sw/refWidth
			rv, g, b, a := img.At(sx, sy).RGBA()
			dst.SetNRGBA(dx, dy, color.NRGBA{
				R: byte(rv >> 8),
				G: byte(g >> 8),
				B: byte(b >> 8),
				A: byte(a >> 8),
			})
		}
	}
	return dst
}

func SAD(img image.Image, t Template) uint64 {
	region := scaleAndCropRGB(img)
	if t.mask == nil {
		var sum uint64
		for i, v := range region {
			d := int(v) - int(t.pix[i])
			if d < 0 {
				d = -d
			}
			sum += uint64(d)
		}
		return sum
	}
	var sum, n uint64
	pixCount := uint64(tmplW * tmplH)
	for i := uint64(0); i < pixCount; i++ {
		if t.mask[i] == 0 {
			continue
		}
		for c := uint64(0); c < 3; c++ {
			d := int(region[i*3+c]) - int(t.pix[i*3+c])
			if d < 0 {
				d = -d
			}
			sum += uint64(d)
		}
		n++
	}
	if n == 0 {
		return ^uint64(0)
	}
	return sum * pixCount / n
}

// Handles combined multi-monitor images by trying common monitor widths as left-aligned and right-aligned slices.
func FindRewardScreen(img image.Image, t Template, threshold uint64) image.Image {
	if SAD(img, t) < threshold {
		return img
	}
	sb := img.Bounds()
	sw := sb.Dx()
	if sw <= refWidth {
		return nil
	}
	for _, monW := range []int{refWidth, 1920, 3840} {
		if monW >= sw {
			continue
		}
		for _, xOff := range []int{0, sw - monW} {
			if xOff < 0 {
				continue
			}
			s := monitorSlice{
				img:    img,
				bounds: image.Rect(sb.Min.X+xOff, sb.Min.Y, sb.Min.X+xOff+monW, sb.Max.Y),
			}
			if SAD(s, t) < threshold {
				return s
			}
		}
	}
	return nil
}
