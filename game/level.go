package game

import (
	"bytes"
	"image"
	_ "image/png"
	"io/fs"

	"github.com/hajimehoshi/ebiten/v2"
)

const TileSize = 16
const EnemySpawnID = 72

// Level holds the tile grid, an optional background image, and any enemies.
type Level struct {
	Tiles   [][]int
	Bg      *ebiten.Image
	Enemies []*Enemy
}

var (
	Levels       []Level
	CurrentLevel = 0

	tilesetImage *ebiten.Image
	tileImages   []*ebiten.Image
)

// LoadTileset slices your tileset PNG into tileImages.
func LoadTilesetFS(fsys fs.FS, path string) error {
	// 1) read the entire file into memory
	data, err := fs.ReadFile(fsys, path)
	if err != nil {
		return err
	}
	// 2) decode it
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return err
	}
	// 3) convert to Ebiten image
	tilesetImage = ebiten.NewImageFromImage(img)

	// 4) slice into individual tileImages
	tw := tilesetImage.Bounds().Dx() / TileSize
	th := tilesetImage.Bounds().Dy() / TileSize
	tileImages = make([]*ebiten.Image, 0, tw*th)
	for yy := 0; yy < th; yy++ {
		for xx := 0; xx < tw; xx++ {
			sub := tilesetImage.SubImage(image.Rect(
				xx*TileSize, yy*TileSize,
				(xx+1)*TileSize, (yy+1)*TileSize,
			)).(*ebiten.Image)
			tileImages = append(tileImages, sub)
		}
	}
	return nil
}

// DrawLevel draws the background (if any) then the tiles.
func DrawLevel(screen *ebiten.Image, camX, camY float64) {
	lvl := Levels[CurrentLevel]

	// 1) draw background
	if lvl.Bg != nil {
		screen.DrawImage(lvl.Bg, nil)
	}
	// 2) draw tiles
	for y, row := range lvl.Tiles {
		for x, tile := range row {
			if tile > 0 && tile <= len(tileImages) {
				op := &ebiten.DrawImageOptions{}
				op.GeoM.Translate(
					float64(x*TileSize)-camX,
					float64(y*TileSize)-camY,
				)
				screen.DrawImage(tileImages[tile-1], op)
			}
		}
	}
}
func TileImage(id int) *ebiten.Image {
	if id <= 0 || id > len(tileImages) {
		return nil
	}
	return tileImages[id-1]
}
