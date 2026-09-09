package relicdetect

import (
	"bufio"
	"encoding/csv"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

const (
	panelW      = 311
	panelG      = 12
	nameY       = 520
	nameH       = 140
	nameXOffset = 5
	nameW       = 301
)

// panelNameRegion returns the bounding rectangle for the item name text
// in panel i (0-indexed) of a reward screen showing count panels total,
// at reference resolution (2560×1440).
func panelNameRegion(i, count int) image.Rectangle {
	totalW := count*panelW + (count-1)*panelG
	leftmost := refWidth/2 - totalW/2
	x := leftmost + i*(panelW+panelG) + nameXOffset
	return image.Rect(x, nameY, x+nameW, nameY+nameH)
}

func loadWordSet(path string) map[string]struct{} {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	words := make(map[string]struct{})
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		word := strings.ToLower(strings.TrimSpace(scanner.Text()))
		if word != "" {
			words[word] = struct{}{}
		}
	}
	return words
}

// ExtractItemNames OCRs up to 4 item name regions from img (any resolution).
// It tries squad sizes 4 down to 1, picks the count with the most total OCR
// text, and stops early once average chars per panel reaches 15.
// wordListPath is an optional path to a Tesseract user-words file; pass "" to skip.
func ExtractItemNames(img image.Image, wordListPath string) ([]string, error) {
	var knownWords map[string]struct{}
	if wordListPath != "" {
		knownWords = loadWordSet(wordListPath)
	}
	var bestNames []string
	bestTotal := 0
	for count := 4; count >= 1; count-- {
		names := make([]string, 0, count)
		total := 0
		allFilled := true
		for i := 0; i < count; i++ {
			region := panelNameRegion(i, count)
			cropped := scaleAndCropImage(img, region)
			text, err := ocrRegion(cropped, wordListPath, knownWords)
			if err != nil {
				return nil, err
			}
			if len(text) < 4 {
				allFilled = false
				break
			}
			total += len(text)
			names = append(names, text)
		}
		if !allFilled {
			continue
		}
		if total > bestTotal {
			bestTotal = total
			bestNames = names
		}
		if total >= 15*count {
			break
		}
	}
	return bestNames, nil
}

func toGray(img image.Image) *image.Gray {
	bounds := img.Bounds()
	srcW, srcH := bounds.Dx(), bounds.Dy()
	g := image.NewGray(image.Rect(0, 0, srcW, srcH))
	for sy := range srcH {
		for sx := range srcW {
			r, gv, b, _ := img.At(bounds.Min.X+sx, bounds.Min.Y+sy).RGBA()
			// BT.601 luminance
			lum := uint8((19595*r + 38470*gv + 7471*b + 1<<15) >> 24)
			g.SetGray(sx, sy, color.Gray{Y: lum})
		}
	}
	return g
}

func upscaleGray(img image.Image, scale int) *image.Gray {
	src := toGray(img)
	srcW, srcH := src.Bounds().Dx(), src.Bounds().Dy()
	dstW, dstH := srcW*scale, srcH*scale
	out := image.NewGray(image.Rect(0, 0, dstW, dstH))
	sf := float64(scale)
	for dy := range dstH {
		fy := (float64(dy)+0.5)/sf - 0.5
		y0, y1 := int(fy), int(fy)+1
		wy := fy - float64(y0)
		if y0 < 0 {
			y0 = 0
		}
		if y1 >= srcH {
			y1 = srcH - 1
		}
		row0 := y0 * src.Stride
		row1 := y1 * src.Stride
		for dx := range dstW {
			fx := (float64(dx)+0.5)/sf - 0.5
			x0, x1 := int(fx), int(fx)+1
			wx := fx - float64(x0)
			if x0 < 0 {
				x0 = 0
			}
			if x1 >= srcW {
				x1 = srcW - 1
			}
			v := float64(src.Pix[row0+x0])*(1-wx)*(1-wy) +
				float64(src.Pix[row0+x1])*wx*(1-wy) +
				float64(src.Pix[row1+x0])*(1-wx)*wy +
				float64(src.Pix[row1+x1])*wx*wy
			out.SetGray(dx, dy, color.Gray{Y: uint8(v + 0.5)})
		}
	}
	return out
}

func binarize(img *image.Gray) *image.Gray {
	bounds := img.Bounds()
	out := image.NewGray(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if img.GrayAt(x, y).Y > 128 {
				out.SetGray(x, y, color.Gray{Y: 0})
			} else {
				out.SetGray(x, y, color.Gray{Y: 255})
			}
		}
	}
	return out
}

func ocrRegion(img image.Image, wordListPath string, knownWords map[string]struct{}) (string, error) {
	processed := binarize(upscaleGray(img, 3))

	f, err := os.CreateTemp("", "wf-ocr-*.png")
	if err != nil {
		return "", err
	}
	defer os.Remove(f.Name())
	if err := png.Encode(f, processed); err != nil {
		f.Close()
		return "", err
	}
	f.Close()

	args := []string{
		f.Name(), "stdout",
		"--psm", "6", "--oem", "3", "-l", "eng",
		"--dpi", "300",
		"-c", "tessedit_char_whitelist=ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789 '",
	}
	if wordListPath != "" {
		args = append(args, "-c", "user_words_file="+wordListPath)
	}
	args = append(args, "tsv")

	out, err := exec.Command("tesseract", args...).Output()
	if err != nil {
		return "", err
	}

	r := csv.NewReader(strings.NewReader(string(out)))
	r.Comma = '\t'
	r.LazyQuotes = true
	records, err := r.ReadAll()
	if err != nil {
		return "", err
	}

	var words []string
	for _, rec := range records[1:] { // skip header
		if len(rec) < 12 {
			continue
		}
		word := strings.TrimSpace(rec[11])
		if len(word) < 3 {
			continue
		}
		conf, err := strconv.ParseFloat(rec[10], 64)
		if err != nil {
			continue
		}
		_, inWordList := knownWords[strings.ToLower(word)]
		if conf < 60 && !inWordList {
			continue
		}
		words = append(words, word)
	}
	return strings.Join(words, " "), nil
}
