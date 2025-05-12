package loader

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"path"

	"PumpkinGiraffe/game"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

type tiledMap struct {
	Width  int          `json:"width"`
	Height int          `json:"height"`
	Layers []tiledLayer `json:"layers"`
}

type tiledLayer struct {
	Type    string  `json:"type"`            // "tilelayer" or "imagelayer"
	Data    []int   `json:"data,omitempty"`  // only for tilelayers
	Image   string  `json:"image,omitempty"` // only for imagelayers
	OffsetX float64 `json:"offsetx,omitempty"`
	OffsetY float64 `json:"offsety,omitempty"`
}

// LoadLevelFromFS reads the first valid tilelayer from the embedded FS.
func LoadLevelFromFS(fsys fs.FS, path string) ([][]int, error) {
	raw, err := fs.ReadFile(fsys, path)
	if err != nil {
		return nil, err
	}
	var tm tiledMap
	if err := json.Unmarshal(raw, &tm); err != nil {
		return nil, err
	}

	// find the first tilelayer whose Data length matches width*height
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

	// build 2D grid
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

// LoadLevelFS loads a full Level (tiles, background, enemies) from the given fs.FS.
func LoadLevelFS(fsys fs.FS, path string) (game.Level, error) {
	// 1) tiles
	tiles, err := LoadLevelFromFS(fsys, path)
	if err != nil {
		return game.Level{}, err
	}
	// 2) background
	bgPath, offX, offY, err := LoadBackgroundFromFS(fsys, path)
	if err != nil {
		return game.Level{}, err
	}
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
		opts.GeoM.Translate(offX, offY)
		bgImg.DrawImage(img, opts)
	}
	// 3) spawn enemies
	var enemies []*game.Enemy
	for y := range tiles {
		for x, id := range tiles[y] {
			if id == game.EnemySpawnID {
				tiles[y][x] = 0
				sx := float64(x * game.TileSize)
				sy := float64((y + 1) * game.TileSize)
				enemies = append(enemies, game.NewEnemyFS(fsys, sx, sy))
			}
		}
	}
	return game.Level{Tiles: tiles, Bg: bgImg, Enemies: enemies}, nil
}

// LoadBackgroundFromFS reads the first imagelayer from the embedded FS.
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
	// find first imagelayer:
	for _, lyr := range tm.Layers {
		if lyr.Type == "imagelayer" && lyr.Image != "" {
			// dirname("levels/test_level.json") == "levels"
			full := path.Join(dir, lyr.Image) // yields "levels/night_background.png"
			// join into "levels/night_background.png"
			return full, lyr.OffsetX, lyr.OffsetY, nil
		}
	}
	return "", 0, 0, nil
}
