//Manages the tilemap: slicing the master tileset PNG into individual tiles, 
//storing them, and drawing the 2D grid (plus optional background) each frame; also holds the global Levels slice.
package game

import (
	"bytes"
	"image"
	_ "image/png" // registers PNG decoder
	"io/fs"

	"github.com/hajimehoshi/ebiten/v2"
)

const (
	TileSize      = 16 // size of each square tile in pixels
	EnemySpawnID  = 72 // special tile ID reserved for enemy spawn points
	BoulderSpawnID = 71 // tile ID that spawns a pushable boulder, then is cleared
)

// Level represents a single game map: a grid of tile IDs, an optional
// background image, and a list of enemies placed via EnemySpawnID tiles.
type Level struct {
	Tiles     [][]int           // 2D array of tile IDs; 0 = empty, >0 = specific tile
	Bg        *ebiten.Image     // optional fullscreen background graphic
	Enemies   []*Enemy          // active enemies for this level
	Platforms []*MovingPlatform // moving platforms read from the map's object layer
	Boulders  []*Boulder        // pushable boulders placed via BoulderSpawnID tiles
	SpawnX    float64           // player start X in world pixels (0 = use default)
	SpawnY    float64           // player start Y in world pixels (0 = use default)
}

var (
	Levels       []Level // all loaded levels
	CurrentLevel = 0     // index of the level currently being played

	tilesetImage *ebiten.Image   // full tileset texture
	tileImages   []*ebiten.Image // individual tile sub-images
)

// LoadTilesetFS reads a single PNG containing all tiles laid out in a grid,
// slices it into TileSize×TileSize sub-images, and stores them in tileImages.
func LoadTilesetFS(fsys fs.FS, path string) error {
	// 1) Read raw image bytes from embedded filesystem
	data, err := fs.ReadFile(fsys, path)
	if err != nil {
		return err
	}

	// 2) Decode into a Go image.Image
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return err
	}

	// 3) Convert to an Ebiten image for efficient GPU drawing
	tilesetImage = ebiten.NewImageFromImage(img)

	// 4) Compute how many tiles fit horizontally and vertically
	cols := tilesetImage.Bounds().Dx() / TileSize
	rows := tilesetImage.Bounds().Dy() / TileSize

	// 5) Extract each tile into its own *ebiten.Image and append to tileImages
	tileImages = make([]*ebiten.Image, 0, cols*rows)
	for ty := 0; ty < rows; ty++ {
		for tx := 0; tx < cols; tx++ {
			// Define the rectangle for this tile in the tileset
			rect := image.Rect(
				tx*TileSize, ty*TileSize,
				(tx+1)*TileSize, (ty+1)*TileSize,
			)
			subImg := tilesetImage.SubImage(rect).(*ebiten.Image)
			tileImages = append(tileImages, subImg)
		}
	}

	return nil
}

// DrawLevel renders the current level to the screen, offset by camX/camY.
// It first draws the background (if provided), then each non-zero tile.
func DrawLevel(screen *ebiten.Image, camX, camY float64) {
	lvl := Levels[CurrentLevel]

	// 1) Background: draw behind everything if it exists
	if lvl.Bg != nil {
		screen.DrawImage(lvl.Bg, nil)
	}

	// 2) Tiles: iterate over each row and column in lvl.Tiles
	for y, row := range lvl.Tiles {
		for x, tileID := range row {
			// Skip empty tiles or invalid IDs
			if tileID <= 0 || tileID > len(tileImages) {
				continue
			}

			// Position the tile image at (x*TileSize, y*TileSize) minus camera offset
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Translate(
				float64(x*TileSize)-camX,
				float64(y*TileSize)-camY,
			)
			// tileID 1 corresponds to tileImages[0], so subtract 1
			screen.DrawImage(tileImages[tileID-1], op)
		}
	}
}

// TileImage returns the Ebiten image for a given tile ID, or nil if out of range.
// This is useful when you need to manually draw a specific tile outside DrawLevel.
func TileImage(id int) *ebiten.Image {
	if id <= 0 || id > len(tileImages) {
		return nil
	}
	return tileImages[id-1]
}
