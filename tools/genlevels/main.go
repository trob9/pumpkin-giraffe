// Command genlevels builds the three new Pumpkin Giraffe levels.
//
// Levels are authored here in code using motif helpers that mirror the visual
// grammar of the original hand-made map: ground is a solid grass-top lip (50)
// over a decorative dirt/rock body (56/80); floating "torii" platforms are a
// 37/39/38 beam with non-solid 5/6 legs hanging beneath; standalone blocks are
// the 49/50/51 + 55/56/57 + 61/62/63 stack. Pumpkins are tile 48, enemy spawns
// are tile 72, barrels are 11/84/90.
//
// For each level it writes a Tiled-format JSON to levels/ (openable in Tiled)
// and renders a preview PNG to tools/genlevels/preview/ using the real tileset,
// so the layout can be eyeballed without playing through.
//
// Run from the project root:  go run ./tools/genlevels
package main

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
)

const (
	W, H     = 80, 45 // level size in tiles (1280x720 at 16px)
	tileSize = 16
	groundTop = 38 // row of the main ground surface
)

// ---- tile IDs (map values), matching the original map's vocabulary ----
const (
	tGrassTop = 50 // solid surface you stand on
	tDirt     = 56 // non-solid body fill
	tBase     = 80 // non-solid dark base row
	tBeamL    = 37 // platform left cap (solid)
	tBeamM    = 39 // platform middle span (solid)
	tBeamR    = 38 // platform right cap (solid)
	tLeg      = 5  // non-solid pillar shaft
	tFoot     = 6  // non-solid pillar foot
	tPumpkin  = 48 // collectible
	tEnemy    = 72 // enemy spawn marker
	tBarrel   = 11 // standard barrel (solid)
)

// grid is the tile layer being built; [row][col], 0 = empty.
type grid [H][W]int

func (g *grid) set(col, row, id int) {
	if col < 0 || col >= W || row < 0 || row >= H {
		return
	}
	g[row][col] = id
}

// ground lays a solid grass-top surface from col x0..x1 at the given top row,
// with decorative dirt below it down to the dark base row at the map bottom.
func (g *grid) ground(x0, x1, topRow int) {
	for c := x0; c <= x1; c++ {
		g.set(c, topRow, tGrassTop)
		for r := topRow + 1; r < H-1; r++ {
			g.set(c, r, tDirt)
		}
		g.set(c, H-1, tBase)
	}
}

// torii draws a floating platform: a beam of `width` tiles at (x,row) with
// non-solid legs hanging from each end down `legH` tiles, ending in a foot.
func (g *grid) torii(x, row, width, legH int) {
	for i := 0; i < width; i++ {
		id := tBeamM
		if i == 0 {
			id = tBeamL
		} else if i == width-1 {
			id = tBeamR
		}
		g.set(x+i, row, id)
	}
	for _, c := range []int{x, x + width - 1} {
		for r := row + 1; r <= row+legH; r++ {
			g.set(c, r, tLeg)
		}
		g.set(c, row+legH+1, tFoot)
	}
}

// block3x3 places a standalone solid-topped block whose top-left is (x,row).
func (g *grid) block3x3(x, row int) {
	rows := [3][3]int{{49, 50, 51}, {55, 56, 57}, {61, 62, 63}}
	for dr := 0; dr < 3; dr++ {
		for dc := 0; dc < 3; dc++ {
			g.set(x+dc, row+dr, rows[dr][dc])
		}
	}
}

func (g *grid) pumpkin(x, row int) { g.set(x, row, tPumpkin) }
func (g *grid) enemy(x, row int)   { g.set(x, row, tEnemy) }
func (g *grid) barrel(x, row int)  { g.set(x, row, tBarrel) }

// platDef describes a moving platform written into the object layer.
type platDef struct {
	x, y, w, h    int     // pixels; (x,y) top-left
	axis          string  // "x" or "y"
	rng           float64 // travel distance in pixels
	speed         float64 // pixels per frame
}

// level bundles everything one map needs.
type level struct {
	name   string
	g      grid
	plats  []platDef
	spawnX int
	spawnY int
}

// px converts a tile column/row to a top-left pixel coordinate.
func px(tiles int) int { return tiles * tileSize }

// ---------------------------------------------------------------------------
// Level 1 — "Rolling Start": gentle intro to moving platforms.
// Mostly solid ground with two small gaps bridged by slow horizontal platforms.
// ---------------------------------------------------------------------------
func buildLevel1() level {
	var g grid
	// continuous ground in three segments, two gaps between them
	g.ground(0, 24, groundTop)
	g.ground(34, 54, groundTop)
	g.ground(64, 79, groundTop)

	// a couple of low torii to hop on, plus a 3x3 block landmark
	g.torii(14, 32, 4, 4)
	g.block3x3(46, 35)

	// pumpkins: a friendly trail leading rightward, all reachable with single
	// jumps (the torii stays decorative so nothing needs the secret double jump)
	g.pumpkin(6, groundTop-1)
	g.pumpkin(30, 34) // floats above moving platform 1 — grab it while riding
	g.pumpkin(40, groundTop-1)
	g.pumpkin(47, 34) // on the 3x3 block (single jump up)
	g.pumpkin(72, groundTop-1)

	// one slow enemy on the middle segment
	g.enemy(50, groundTop-1)

	// a barrel for flavour near the start
	g.barrel(10, groundTop-1)

	// two horizontal moving platforms, one per gap, slow and forgiving
	plats := []platDef{
		{x: px(26), y: px(groundTop - 2), w: 48, h: 16, axis: "x", rng: 80, speed: 0.6},
		{x: px(56), y: px(groundTop - 2), w: 48, h: 16, axis: "x", rng: 96, speed: 0.6},
	}
	return level{name: "level1", g: g, plats: plats, spawnX: px(2), spawnY: px(groundTop - 4)}
}

// ---------------------------------------------------------------------------
// Level 2 — "Gap Gauntlet": medium. More gaps, a vertical lift, static torii
// to chain between, a couple of enemies, pumpkins that need a platform ride.
// ---------------------------------------------------------------------------
func buildLevel2() level {
	var g grid
	g.ground(0, 14, groundTop)
	g.ground(24, 32, groundTop)
	g.ground(44, 52, groundTop)
	g.ground(70, 79, groundTop)

	// static torii staircase rising toward the right
	g.torii(18, 33, 3, 5)
	g.torii(36, 29, 3, 6)
	g.torii(58, 31, 4, 6)
	g.block3x3(48, 35)

	// pumpkins: some on ground, some up on torii reached via platforms
	g.pumpkin(5, groundTop-1)
	g.pumpkin(19, 32)  // on a torii
	g.pumpkin(37, 28)  // higher torii
	g.pumpkin(49, 34)  // on the block
	g.pumpkin(74, groundTop-1)

	// enemies guarding two segments
	g.enemy(26, groundTop-1)
	g.enemy(46, groundTop-1)

	g.barrel(12, groundTop-1)

	plats := []platDef{
		// horizontal bridge across the first big gap
		{x: px(16), y: px(groundTop - 2), w: 48, h: 16, axis: "x", rng: 112, speed: 0.7},
		// vertical lift up to the high torii
		{x: px(34), y: px(groundTop - 2), w: 32, h: 16, axis: "y", rng: 144, speed: 0.8},
		// horizontal carry over the final gap
		{x: px(54), y: px(groundTop - 2), w: 48, h: 16, axis: "x", rng: 200, speed: 0.8},
	}
	return level{name: "level2", g: g, plats: plats, spawnX: px(2), spawnY: px(groundTop - 4)}
}

// ---------------------------------------------------------------------------
// Level 3 — "Skyline Sprint": hard. Sparse ground, lots of moving platforms
// (faster), pumpkins out over the void and up high, several enemies.
// ---------------------------------------------------------------------------
func buildLevel3() level {
	var g grid
	g.ground(0, 8, groundTop)
	g.ground(20, 26, groundTop)
	g.ground(40, 45, groundTop)
	g.ground(74, 79, groundTop)

	// high torii skyline
	g.torii(12, 28, 3, 8)
	g.torii(30, 24, 3, 9)
	g.torii(52, 27, 4, 8)
	g.torii(64, 22, 3, 10)

	// pumpkins demanding platform rides and good jumps
	g.pumpkin(4, groundTop-1)
	g.pumpkin(31, 23)  // top of a high torii
	g.pumpkin(42, groundTop-1)
	g.pumpkin(53, 26)  // on a torii over the void
	g.pumpkin(65, 21)  // highest torii

	// several enemies
	g.enemy(22, groundTop-1)
	g.enemy(41, groundTop-1)
	g.enemy(75, groundTop-1)

	plats := []platDef{
		{x: px(10), y: px(groundTop - 4), w: 32, h: 16, axis: "x", rng: 140, speed: 0.95},
		{x: px(28), y: px(groundTop - 2), w: 32, h: 16, axis: "y", rng: 200, speed: 1.0},
		{x: px(46), y: px(groundTop - 6), w: 32, h: 16, axis: "x", rng: 180, speed: 0.95},
		{x: px(60), y: px(groundTop - 2), w: 32, h: 16, axis: "y", rng: 240, speed: 1.0},
	}
	return level{name: "level3", g: g, plats: plats, spawnX: px(2), spawnY: px(groundTop - 4)}
}

// ---- Tiled JSON output structs ----

type tmap struct {
	Height      int        `json:"height"`
	Width       int        `json:"width"`
	TileWidth   int        `json:"tilewidth"`
	TileHeight  int        `json:"tileheight"`
	Infinite    bool       `json:"infinite"`
	Orientation string     `json:"orientation"`
	RenderOrder string     `json:"renderorder"`
	Type        string     `json:"type"`
	Version     string     `json:"version"`
	TiledVer    string     `json:"tiledversion"`
	NextLayerID int        `json:"nextlayerid"`
	NextObjID   int        `json:"nextobjectid"`
	Tilesets    []tset     `json:"tilesets"`
	Layers      []tlayer   `json:"layers"`
}

type tset struct {
	FirstGID int    `json:"firstgid"`
	Source   string `json:"source"`
}

type tlayer struct {
	ID          int      `json:"id"`
	Type        string   `json:"type"`
	Name        string   `json:"name"`
	Opacity     float64  `json:"opacity"`
	Visible     bool     `json:"visible"`
	X           int      `json:"x"`
	Y           int      `json:"y"`
	Width       int      `json:"width,omitempty"`
	Height      int      `json:"height,omitempty"`
	Data        []int    `json:"data,omitempty"`
	Image       string   `json:"image,omitempty"`
	ImageWidth  int      `json:"imagewidth,omitempty"`
	ImageHeight int      `json:"imageheight,omitempty"`
	Objects     []tobj   `json:"objects,omitempty"`
}

type tobj struct {
	ID         int     `json:"id"`
	Name       string  `json:"name"`
	X          int     `json:"x"`
	Y          int     `json:"y"`
	Width      int     `json:"width"`
	Height     int     `json:"height"`
	Point      bool    `json:"point,omitempty"`
	Properties []tprop `json:"properties,omitempty"`
}

type tprop struct {
	Name  string      `json:"name"`
	Type  string      `json:"type"`
	Value interface{} `json:"value"`
}

func (l level) toTiledJSON() tmap {
	// flatten grid to row-major data
	data := make([]int, 0, W*H)
	for r := 0; r < H; r++ {
		for c := 0; c < W; c++ {
			data = append(data, l.g[r][c])
		}
	}

	// object layer: spawn point + moving platforms
	objs := []tobj{{ID: 1, Name: "spawn", X: l.spawnX, Y: l.spawnY, Point: true}}
	for i, p := range l.plats {
		objs = append(objs, tobj{
			ID: i + 2, Name: "plat", X: p.x, Y: p.y, Width: p.w, Height: p.h,
			Properties: []tprop{
				{Name: "axis", Type: "string", Value: p.axis},
				{Name: "range", Type: "float", Value: p.rng},
				{Name: "speed", Type: "float", Value: p.speed},
			},
		})
	}

	return tmap{
		Height: H, Width: W, TileWidth: tileSize, TileHeight: tileSize,
		Infinite: false, Orientation: "orthogonal", RenderOrder: "right-down",
		Type: "map", Version: "1.10", TiledVer: "1.11.2",
		NextLayerID: 4, NextObjID: len(objs) + 1,
		Tilesets: []tset{{FirstGID: 1, Source: "../tilesets/platformer.tsx"}},
		Layers: []tlayer{
			{ID: 2, Type: "imagelayer", Name: "Image Layer 1", Opacity: 1, Visible: true,
				Image: "night_background.png", ImageWidth: 1280, ImageHeight: 720},
			{ID: 1, Type: "tilelayer", Name: "Tile Layer 1", Opacity: 1, Visible: true,
				Width: W, Height: H, Data: data},
			{ID: 3, Type: "objectgroup", Name: "objects", Opacity: 1, Visible: true, Objects: objs},
		},
	}
}

// ---- preview rendering ----

func loadPNG(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return png.Decode(f)
}

func fillRect(dst *image.RGBA, x, y, w, h int, c image.Image) {
	draw.Draw(dst, image.Rect(x, y, x+w, y+h), c, image.Point{}, draw.Over)
}

// renderPreview composites the level onto a 1280x720 image using the real
// tileset, then overlays moving platforms (yellow tint), enemy spawns (red),
// and the player spawn (cyan).
func renderPreview(l level, tileset, bg image.Image, outPath string) error {
	dst := image.NewRGBA(image.Rect(0, 0, W*tileSize, H*tileSize))
	if bg != nil {
		draw.Draw(dst, dst.Bounds(), bg, bg.Bounds().Min, draw.Src)
	}

	blit := func(id, col, row int) {
		if id <= 0 {
			return
		}
		idx := id - 1
		sx := (idx % 6) * tileSize
		sy := (idx / 6) * tileSize
		dr := image.Rect(col*tileSize, row*tileSize, col*tileSize+tileSize, row*tileSize+tileSize)
		draw.Draw(dst, dr, tileset, image.Pt(sx, sy), draw.Over)
	}

	for r := 0; r < H; r++ {
		for c := 0; c < W; c++ {
			blit(l.g[r][c], c, r)
		}
	}

	// markers
	red := image.NewUniform(color.RGBA{220, 40, 40, 200})
	cyan := image.NewUniform(color.RGBA{40, 220, 220, 220})
	yellow := image.NewUniform(color.RGBA{240, 220, 60, 120})
	for r := 0; r < H; r++ {
		for c := 0; c < W; c++ {
			if l.g[r][c] == tEnemy {
				fillRect(dst, c*tileSize, r*tileSize, tileSize, tileSize, red)
			}
		}
	}
	for _, p := range l.plats {
		// draw the platform tiles at its start position, then a translucent tint
		cols := p.w / tileSize
		row := p.y / tileSize
		for i := 0; i < cols; i++ {
			id := tBeamM
			if cols > 1 && i == 0 {
				id = tBeamL
			} else if cols > 1 && i == cols-1 {
				id = tBeamR
			}
			blit(id, p.x/tileSize+i, row)
		}
		fillRect(dst, p.x, p.y, p.w, p.h, yellow)
	}
	fillRect(dst, l.spawnX, l.spawnY, tileSize, tileSize, cyan)

	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return err
	}
	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, dst)
}

func main() {
	tileset, err := loadPNG("assets/tilesets/platformer.png")
	if err != nil {
		fmt.Println("tileset:", err)
		os.Exit(1)
	}
	bg, err := loadPNG("levels/night_background.png")
	if err != nil {
		fmt.Println("bg (continuing without):", err)
		bg = nil
	}

	levels := []level{buildLevel1(), buildLevel2(), buildLevel3()}
	for _, l := range levels {
		// write JSON
		m := l.toTiledJSON()
		b, err := json.MarshalIndent(m, "", " ")
		if err != nil {
			fmt.Println("marshal:", err)
			os.Exit(1)
		}
		jsonPath := filepath.Join("levels", l.name+".json")
		if err := os.WriteFile(jsonPath, b, 0o644); err != nil {
			fmt.Println("write json:", err)
			os.Exit(1)
		}
		fmt.Println("wrote", jsonPath)

		// render preview
		prev := filepath.Join("tools", "genlevels", "preview", l.name+".png")
		if err := renderPreview(l, tileset, bg, prev); err != nil {
			fmt.Println("preview:", err)
			os.Exit(1)
		}
		fmt.Println("wrote", prev)
	}
}
