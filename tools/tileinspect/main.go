// Command tileinspect renders the tileset zoomed-in with each tile labelled by
// its map ID (tileset index + 1, the value used in level JSON), so the exact
// graphic for each ID can be identified. Run: go run ./tools/tileinspect
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

const (
	src   = "assets/tilesets/platformer.png"
	out   = "tools/tileinspect/tileset_labeled.png"
	scale = 5 // each 16px tile becomes 80px
	cols  = 6
)

func main() {
	f, err := os.Open(src)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	img, err := png.Decode(f)
	f.Close()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	b := img.Bounds()
	tw, th := 16, 16
	gridCols := b.Dx() / tw
	gridRows := b.Dy() / th
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx()*scale, b.Dy()*scale))

	// nearest-neighbour upscale
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			c := img.At(b.Min.X+x, b.Min.Y+y)
			for dy := 0; dy < scale; dy++ {
				for dx := 0; dx < scale; dx++ {
					dst.Set(x*scale+dx, y*scale+dy, c)
				}
			}
		}
	}

	// grid lines
	gridCol := color.RGBA{255, 0, 255, 255}
	for gx := 0; gx <= gridCols; gx++ {
		x := gx * tw * scale
		for y := 0; y < dst.Bounds().Dy(); y++ {
			dst.Set(x, y, gridCol)
			if x > 0 {
				dst.Set(x-1, y, gridCol)
			}
		}
	}
	for gy := 0; gy <= gridRows; gy++ {
		y := gy * th * scale
		for x := 0; x < dst.Bounds().Dx(); x++ {
			dst.Set(x, y, gridCol)
			if y > 0 {
				dst.Set(x, y-1, gridCol)
			}
		}
	}

	// label each tile with its map ID (index+1)
	for gy := 0; gy < gridRows; gy++ {
		for gx := 0; gx < gridCols; gx++ {
			id := gy*cols + gx + 1
			px := gx*tw*scale + 3
			py := gy*th*scale + 14
			label(dst, fmt.Sprintf("%d", id), px, py)
		}
	}

	of, err := os.Create(out)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer of.Close()
	if err := png.Encode(of, dst); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Println("wrote", out)
}

// label draws text with a dark backing box for legibility.
func label(dst *image.RGBA, s string, x, y int) {
	w := len(s)*7 + 2
	for dy := -11; dy <= 3; dy++ {
		for dx := -1; dx < w; dx++ {
			dst.Set(x+dx, y+dy, color.RGBA{0, 0, 0, 220})
		}
	}
	d := &font.Drawer{
		Dst:  dst,
		Src:  image.NewUniform(color.RGBA{255, 255, 0, 255}),
		Face: basicfont.Face7x13,
		Dot:  fixed.P(x, y),
	}
	d.DrawString(s)
}
