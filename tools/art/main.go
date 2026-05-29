// Command art paints the game's hand-made pixel assets to PNG files under
// assets/. Re-run after editing a shape: go run ./tools/art
//
// Keeping the art as code (small bitmaps + palettes) makes it easy to tweak and
// keeps the repo's binary assets reproducible.
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
)

type pal map[rune]color.RGBA

// paint turns a row-string bitmap + palette into an RGBA image. '.' = transparent.
func paint(rows []string, p pal) *image.RGBA {
	h := len(rows)
	w := 0
	for _, r := range rows {
		if len(r) > w {
			w = len(r)
		}
	}
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y, r := range rows {
		for x, ch := range r {
			if ch == '.' || ch == ' ' {
				continue
			}
			if c, ok := p[ch]; ok {
				img.Set(x, y, c)
			}
		}
	}
	return img
}

func save(img *image.RGBA, path string) {
	os.MkdirAll(filepath.Dir(path), 0o755)
	f, err := os.Create(path)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Println("wrote", path)
}

func main() {
	// ---- Hearts (HUD) : 13x12 ----
	heartRows := []string{
		"...ooo...ooo.",
		"..o###o.o###o",
		".o#####o#####",
		".o###########",
		".o###########",
		"..o#########o",
		"...o#######o.",
		"....o#####o..",
		".....o###o...",
		"......o#o....",
		".......o.....",
	}
	heartFull := pal{
		'o': {90, 20, 30, 255},   // dark outline
		'#': {222, 60, 74, 255},  // red
	}
	heartEmpty := pal{
		'o': {70, 60, 70, 255},   // grey outline
		'#': {30, 26, 36, 200},   // hollow dark
	}
	// soften the empty interior to a faint hollow
	fh := paint(heartRows, heartFull)
	// add a small highlight to the full heart
	fh.Set(3, 2, color.RGBA{255, 150, 160, 255})
	fh.Set(4, 2, color.RGBA{255, 150, 160, 255})
	save(fh, "assets/ui/heart_full.png")
	save(paint(heartRows, heartEmpty), "assets/ui/heart_empty.png")
}
