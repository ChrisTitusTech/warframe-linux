//go:build linux

package relicdetect

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"os/exec"
	"strings"
)

type grimCapturer struct {
	outputs []string
}

type spectacleCapturer struct{}

type scrotCapturer struct{}

func ProbeCapture() (Capturer, error) {
	wlroots := os.Getenv("SWAYSOCK") != "" || os.Getenv("HYPRLAND_INSTANCE_SIGNATURE") != ""
	if wlroots {
		if _, err := exec.LookPath("grim"); err == nil {
			return grimCapturer{outputs: grimListOutputs()}, nil
		}
	}
	if _, err := exec.LookPath("spectacle"); err == nil {
		return spectacleCapturer{}, nil
	}
	if _, err := exec.LookPath("scrot"); err == nil {
		return scrotCapturer{}, nil
	}
	if !wlroots {
		if _, err := exec.LookPath("grim"); err == nil {
			return grimCapturer{outputs: grimListOutputs()}, nil
		}
	}
	return nil, fmt.Errorf("no screenshot tool found: install grim, spectacle, or scrot")
}

func grimListOutputs() []string {
	out, err := exec.Command("wlr-randr").Output()
	if err != nil {
		return nil
	}
	var names []string
	for _, line := range strings.Split(string(out), "\n") {
		if line == "" || line[0] == ' ' || line[0] == '\t' {
			continue
		}
		if fields := strings.Fields(line); len(fields) > 0 {
			names = append(names, fields[0])
		}
	}
	return names
}

func (g grimCapturer) CaptureAll() ([]image.Image, error) {
	if len(g.outputs) == 0 {
		img, err := grimCapture("")
		if err != nil {
			return nil, err
		}
		return []image.Image{img}, nil
	}
	imgs := make([]image.Image, 0, len(g.outputs))
	for _, output := range g.outputs {
		img, err := grimCapture(output)
		if err != nil {
			return nil, err
		}
		imgs = append(imgs, img)
	}
	return imgs, nil
}

func grimCapture(output string) (image.Image, error) {
	return captureToFile("grim", func(path string) error {
		args := []string{"-t", "png"}
		if output != "" {
			args = append(args, "-o", output)
		}
		args = append(args, path)
		return exec.Command("grim", args...).Run()
	})
}

func captureToFile(name string, run func(path string) error) (image.Image, error) {
	f, err := os.CreateTemp("", "wf-cap-*.png")
	if err != nil {
		return nil, err
	}
	f.Close()
	defer os.Remove(f.Name())

	if err := run(f.Name()); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	return readPNG(f.Name())
}

func (s spectacleCapturer) CaptureAll() ([]image.Image, error) {
	img, err := captureToFile("spectacle", func(path string) error {
		return exec.Command("spectacle", "-b", "-n", "-f", "-a", "-o", path).Run()
	})
	if err != nil {
		return nil, err
	}
	return []image.Image{img}, nil
}

func (s scrotCapturer) CaptureAll() ([]image.Image, error) {
	img, err := captureToFile("scrot", func(path string) error {
		return exec.Command("scrot", path).Run()
	})
	if err != nil {
		return nil, err
	}
	return []image.Image{img}, nil
}

func readPNG(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return png.Decode(f)
}
