// Loads level JSON (and any related resources) from the
// embedded levels directory and returns fully populated Level structs.
package loader

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"path"

	"github.com/TRob9/pumpkin-giraffe/game"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

// tiledMap mirrors the structure of a Tiled JSON export at the top level.
// - Width/Height: dimensions in tiles.
// - Layers: each layer can be a tile grid or an image layer.
type tiledMap struct {
	Width  int          `json:"width"`
	Height int          `json:"height"`
	Layers []tiledLayer `json:"layers"`
}

// tiledLayer represents one layer inside a Tiled map.
// - Type: "tilelayer" (grid data), "imagelayer" (background), or "objectgroup" (objects).
// - Data: flat array of tile IDs, only for tile layers.
// - Image: filename for image layers.
// - OffsetX/Y: position to draw image layers.
// - Objects: spawn points and moving platforms, only for object layers.
type tiledLayer struct {
	Type    string        `json:"type"`
	Data    []int         `json:"data,omitempty"`
	Image   string        `json:"image,omitempty"`
	OffsetX float64       `json:"offsetx,omitempty"`
	OffsetY float64       `json:"offsety,omitempty"`
	Objects []tiledObject `json:"objects,omitempty"`
}

// tiledObject is one entry in an object layer. A "spawn"-named object marks the
// player's start; any other rectangle object is treated as a moving platform,
// configured by its custom properties (axis, range, speed).
type tiledObject struct {
	Name       string          `json:"name"`
	X          float64         `json:"x"`
	Y          float64         `json:"y"`
	Width      float64         `json:"width"`
	Height     float64         `json:"height"`
	Properties []tiledProperty `json:"properties"`
}

// tiledProperty is a Tiled custom property. Value is decoded as interface{},
// so JSON strings arrive as string and JSON numbers as float64.
type tiledProperty struct {
	Name  string      `json:"name"`
	Value interface{} `json:"value"`
}

// LoadLevelFromFS reads a Tiled JSON file from fsys and returns a 2D tile ID grid.
// It picks the first tilelayer whose Data length matches width*height, then reshapes
// the flat slice into a matrix [row][col].
func LoadLevelFromFS(fsys fs.FS, path string) ([][]int, error) {
	// 1) Read raw JSON bytes
	raw, err := fs.ReadFile(fsys, path)
	if err != nil {
		return nil, err
	}

	// 2) Parse into our tiledMap struct
	var tm tiledMap
	if err := json.Unmarshal(raw, &tm); err != nil {
		return nil, err
	}

	// 3) Find valid tilelayer data
	var tileData []int
	for _, lyr := range tm.Layers {
		if lyr.Type != "tilelayer" {
			continue
		}
		if len(lyr.Data) == tm.Width*tm.Height {
			tileData = lyr.Data
			break
		}
	}
	if tileData == nil {
		return nil, fmt.Errorf("no valid tilelayer in %s", path)
	}

	// 4) Build 2D grid from flat data
	grid := make([][]int, tm.Height)
	for y := 0; y < tm.Height; y++ {
		row := make([]int, tm.Width)
		for x := 0; x < tm.Width; x++ {
			row[x] = tileData[y*tm.Width+x]
		}
		grid[y] = row
	}
	return grid, nil
}

// LoadLevelFS loads a complete game.Level, including:
// 1) Tiles from the JSON.
// 2) Optional background image from the first imagelayer.
// 3) Enemy spawns wherever the tile ID matches EnemySpawnID.
func LoadLevelFS(fsys fs.FS, path string) (game.Level, error) {
	// 1) Load tile grid
	tiles, err := LoadLevelFromFS(fsys, path)
	if err != nil {
		return game.Level{}, err
	}

	// 2) Load background image and its offsets
	bgPath, offX, offY, err := LoadBackgroundFromFS(fsys, path)
	if err != nil {
		return game.Level{}, err
	}

	// 2a) If an image layer was found, decode and draw it into a new Ebiten image
	var bgImg *ebiten.Image
	if bgPath != "" {
		f, err := fsys.Open(bgPath)
		if err != nil {
			return game.Level{}, err
		}
		img, _, err := ebitenutil.NewImageFromReader(f)
		f.Close()
		if err != nil {
			return game.Level{}, err
		}
		w, h := img.Size()
		bgImg = ebiten.NewImage(w, h)
		opts := &ebiten.DrawImageOptions{}
		opts.GeoM.Translate(offX, offY) // position image with offsets
		bgImg.DrawImage(img, opts)
	}

	// 3) Scan the tile grid for enemy and boulder spawn points
	var enemies []*game.Enemy
	var boulders []*game.Boulder
	for y := range tiles {
		for x, id := range tiles[y] {
			switch id {
			case game.EnemySpawnID:
				// Clear the spawn tile so it isn’t drawn as floor
				tiles[y][x] = 0
				// Convert tile coords to world pixels: align bottom of sprite
				sx := float64(x * game.TileSize)
				sy := float64((y + 1) * game.TileSize)
				enemies = append(enemies, game.NewEnemyFS(fsys, sx, sy))
			case game.BoulderSpawnID:
				// Clear the marker; the boulder will fall to the ground below
				tiles[y][x] = 0
				boulders = append(boulders,
					game.NewBoulder(float64(x*game.TileSize), float64(y*game.TileSize)))
			}
		}
	}

	// 4) Scan object layers for the spawn point and any moving platforms.
	platforms, spawnX, spawnY, err := loadObjects(fsys, path)
	if err != nil {
		return game.Level{}, err
	}

	// Return the assembled Level struct
	return game.Level{
		Tiles:     tiles,
		Bg:        bgImg,
		Enemies:   enemies,
		Platforms: platforms,
		Boulders:  boulders,
		SpawnX:    spawnX,
		SpawnY:    spawnY,
	}, nil
}

// loadObjects re-reads the map and walks any object layers. An object named
// "spawn" sets the player start; every other object becomes a moving platform
// built from its axis/range/speed properties (with sensible defaults).
func loadObjects(fsys fs.FS, pathToJSON string) (platforms []*game.MovingPlatform, spawnX, spawnY float64, err error) {
	data, err := fs.ReadFile(fsys, pathToJSON)
	if err != nil {
		return nil, 0, 0, err
	}
	var tm struct{ Layers []tiledLayer }
	if err := json.Unmarshal(data, &tm); err != nil {
		return nil, 0, 0, err
	}

	for _, lyr := range tm.Layers {
		if lyr.Type != "objectgroup" {
			continue
		}
		for _, obj := range lyr.Objects {
			if obj.Name == "spawn" {
				spawnX, spawnY = obj.X, obj.Y
				continue
			}

			// Defaults are overridden by any matching custom property.
			axis := "x"
			rng := 64.0
			speed := 0.6
			for _, p := range obj.Properties {
				switch p.Name {
				case "axis":
					if s, ok := p.Value.(string); ok {
						axis = s
					}
				case "range":
					if f, ok := p.Value.(float64); ok {
						rng = f
					}
				case "speed":
					if f, ok := p.Value.(float64); ok {
						speed = f
					}
				}
			}

			w := obj.Width
			if w <= 0 {
				w = float64(game.TileSize)
			}
			h := obj.Height
			if h <= 0 {
				h = float64(game.TileSize)
			}
			platforms = append(platforms,
				game.NewMovingPlatform(obj.X, obj.Y, w, h, rng, speed, axis))
		}
	}
	return platforms, spawnX, spawnY, nil
}

// LoadBackgroundFromFS reads the same JSON, but looks for the first imagelayer.
// Returns the image file path, its X/Y offsets, or empty values if none found.
func LoadBackgroundFromFS(fsys fs.FS, pathToJSON string) (string, float64, float64, error) {
	data, err := fs.ReadFile(fsys, pathToJSON)
	if err != nil {
		return "", 0, 0, err
	}
	var tm struct{ Layers []tiledLayer }
	if err := json.Unmarshal(data, &tm); err != nil {
		return "", 0, 0, err
	}

	dir := path.Dir(pathToJSON)
	// Find the first image layer in the JSON
	for _, lyr := range tm.Layers {
		if lyr.Type == "imagelayer" && lyr.Image != "" {
			// Build a relative path: e.g. "levels/night_background.png"
			full := path.Join(dir, lyr.Image)
			return full, lyr.OffsetX, lyr.OffsetY, nil
		}
	}
	// No image layer found
	return "", 0, 0, nil
}
