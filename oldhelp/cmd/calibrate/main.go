package main

import (
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"

	"github.com/gjrud/warframe-helper/utils/relicdetect"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: calibrate <frame1> [frame2 ...]")
		fmt.Fprintln(os.Stderr, "       calibrate /path/to/frame*.jpg")
		os.Exit(1)
	}

	tmpl, err := relicdetect.LoadTemplate("assets/reward_template.png")
	if err != nil {
		fmt.Fprintln(os.Stderr, "load template:", err)
		os.Exit(1)
	}

	for _, path := range os.Args[1:] {
		f, err := os.Open(path)
		if err != nil {
			fmt.Printf("%s: error: %v\n", path, err)
			continue
		}
		img, _, err := image.Decode(f)
		f.Close()
		if err != nil {
			fmt.Printf("%s: error: %v\n", path, err)
			continue
		}
		fmt.Printf("%s: %d\n", path, relicdetect.SAD(img, tmpl))
	}
}
