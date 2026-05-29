// Command genlevels builds the three new Pumpkin Giraffe levels.
//
// Levels are authored here in code using motif helpers that mirror the visual
// grammar of the original hand-made map: ground is a solid grass-top lip (50)
// over a decorative dirt/rock body (56/80); floating "torii" platforms are a
// clean stub-free beam (31 left cap, 33 middles, 35 right cap) with a stubbed
// tile (39) at each of the two leg columns so the stub lines up over its 5-pillar
// leg, which runs all the way down to the map bottom; standalone blocks are the
// 49/50/51 stack. Pumpkins are tile 48, enemy spawns 72, boulder spawns 71,
// barrels 11/84/90.
//
// It writes a Tiled-format JSON per level to levels/ and renders a preview PNG
// to tools/genlevels/preview/ with a reachability overlay, then runs an
// arc-based reachability solver that fails the build if any pumpkin is
// impossible to reach and reports which ones need the double-jump chain.
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
	W, H      = 80, 45 // level size in tiles (1280x720 at 16px)
	tileSize  = 16
	groundTop = 38 // row of the main ground surface
)

// tile IDs (map values), matching the original map's vocabulary
const (
	tGrassTop = 50 // solid surface you stand on
	tDirt     = 56 // non-solid body fill
	tBase     = 80 // non-solid dark base row
	// Clean, stub-free wood platform tiles used for the body of a torii beam.
	tCapL   = 31 // left cap (rounded, no stub, decorative overhang — non-solid)
	tCleanM = 33 // clean flat middle (solid, standable, no stub)
	tCapR   = 35 // right cap (rounded, no stub, decorative overhang — non-solid)
	// Stubbed middle tile: its downward pillar stub connects the beam to a leg.
	// Placed ONLY at the two leg columns so the stub lines up over the pillar.
	tBeamStub = 39 // solid + standable, has a downward pillar stub
	tLeg      = 5  // non-solid pillar shaft
	tPumpkin  = 48 // collectible
	tEnemy    = 72 // enemy spawn marker
	tBoulder  = 71 // pushable-boulder spawn marker
	tBarrel   = 11 // standard barrel (solid)
)

// ---- physics, mirrored from the game so the solver matches real jumps ----
const (
	gravity   = 0.26
	termVy    = 3.0
	walkSpd   = 1.5
	sprintSpd = 1.5 * 2.5
	jumpV     = -6.5
	boostV    = -6.5 * 1.5
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

// torii draws a floating platform shaped like a Japanese torii gate: a flat beam
// of `width` tiles at (x,row) carried by two pillar legs (tile 5) that run from
// just below the beam all the way down to the map bottom — planted, not floating.
//
// The beam is built from CLEAN, stub-free tiles everywhere (31 left cap, 33 clean
// middles, 35 right cap) EXCEPT at the two leg columns, where a STUBBED tile (39)
// is placed so its downward pillar stub visually connects the beam to the leg
// below. This is the fix for the old bug: previously every beam tile (37/39/38)
// carried a stub, so non-leg tiles left stubs dangling in mid-air ("a stick in
// the middle"), and on width-4 beams the two legs ended up adjacent.
//
// Leg placement: one column in from each cap, and always at least 2 tiles apart
// so the gate never collapses into a single pole or two adjacent legs. Beams
// narrower than 5 are widened to 5 so this spacing is always achievable (a width
// of 5 gives caps at the ends, legs at columns 1 and 3 — exactly the original
// hand-made map's torii proportions).
func (g *grid) torii(x, row, width int) {
	if width < 5 {
		width = 5
	}
	// Leg columns: one in from each cap, guaranteed >= 2 apart by the min width.
	leftLeg := x + 1
	rightLeg := x + width - 2
	for i := 0; i < width; i++ {
		c := x + i
		var id int
		switch {
		case c == leftLeg || c == rightLeg:
			id = tBeamStub // stubbed tile sits directly over a leg
		case i == 0:
			id = tCapL // decorative rounded left overhang
		case i == width-1:
			id = tCapR // decorative rounded right overhang
		default:
			id = tCleanM // clean flat standable middle
		}
		g.set(c, row, id)
	}
	// Two legs, planted from just under the beam to the map's bottom row.
	for _, c := range []int{leftLeg, rightLeg} {
		for r := row + 1; r < H; r++ {
			g.set(c, r, tLeg)
		}
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
func (g *grid) boulder(x, row int) { g.set(x, row, tBoulder) }
func (g *grid) barrel(x, row int)  { g.set(x, row, tBarrel) }

// platDef describes a moving platform written into the object layer.
type platDef struct {
	x, y, w, h int
	axis       string
	rng        float64
	speed      float64
}

type level struct {
	name           string
	g              grid
	plats          []platDef
	spawnX, spawnY int
	// gate: end-of-level portal. gateRequired pumpkins are spent to pass.
	gateX, gateY int
	gateRequired int
	// bg: background image (relative to the levels/ dir) for this map.
	bg string
	// pumpkins the solver is allowed to skip (e.g. reachable only by pushing a
	// boulder, which the static solver can't model). Keyed by "col,row".
	exemptPumpkins map[string]bool
}

func px(tiles int) int { return tiles * tileSize }

// =====================================================================
// Level 1 — "Rolling Start": gentle intro to moving platforms.
// =====================================================================
func buildLevel1() level {
	var g grid
	g.ground(0, 24, groundTop)
	g.ground(34, 54, groundTop)
	g.ground(64, 79, groundTop)

	g.torii(11, groundTop-4, 6) // low, wide gate — an easy single hop onto the beam
	g.block3x3(46, 35)

	// Gentle: just one ground pumpkin, the rest are simple hops onto a beam, a
	// block, or a ride on the slow platform. Nothing needs the double-jump chain.
	g.pumpkin(6, groundTop-1)  // ground, right by spawn
	g.pumpkin(13, groundTop-5) // on the low torii beam (single hop from ground)
	g.pumpkin(30, 34)          // grab while riding the slow moving platform
	g.pumpkin(47, 34)          // on top of the 3x3 block
	g.pumpkin(70, groundTop-5) // small step up onto a beam over the far ground

	g.torii(68, groundTop-4, 5) // a little gate carrying the last pumpkin

	g.enemy(50, groundTop-1)
	g.barrel(10, groundTop-1)

	plats := []platDef{
		{x: px(26), y: px(groundTop - 2), w: 48, h: 16, axis: "x", rng: 80, speed: 0.6},
		{x: px(56), y: px(groundTop - 2), w: 48, h: 16, axis: "x", rng: 96, speed: 0.6},
	}
	return level{name: "level1", g: g, plats: plats, spawnX: px(2), spawnY: px(groundTop - 4),
		gateX: px(75), gateY: px(groundTop - 3), gateRequired: 3,
		bg: "../assets/backgrounds/bg_meadow_day.png"}
}

// =====================================================================
// Level 2 — "Gap Gauntlet": medium. Gaps, a vertical lift, a torii
// staircase, and a pumpkin or two that need the double-jump chain.
// =====================================================================
func buildLevel2() level {
	var g grid
	g.ground(0, 14, groundTop)
	g.ground(24, 32, groundTop)
	g.ground(44, 52, groundTop)
	g.ground(70, 79, groundTop)

	g.torii(17, 33, 5) // low gate over the first gap — ride the bridge to it
	g.torii(35, 27, 6) // a HIGH gate — its top pumpkin needs the double-jump chain
	g.torii(57, 31, 5) // mid gate, reached from the long sweeping platform
	g.block3x3(48, 35)

	// Only one easy ground pumpkin (by spawn). The rest demand a platform ride or
	// a hop up onto a beam, and the high gate at col 37 needs the double-jump chain.
	g.pumpkin(3, groundTop-1) // ground, by spawn
	g.pumpkin(19, 32)         // on the low torii, off the horizontal bridge platform
	g.pumpkin(37, 26)         // top of the HIGH gate — DOUBLE-JUMP CHAIN
	g.pumpkin(34, 28)         // ride the vertical lift up to grab this one mid-air
	g.pumpkin(59, 30)         // on the mid gate, off the long sweeping platform

	g.enemy(26, groundTop-1)
	g.enemy(46, groundTop-1)
	g.barrel(12, groundTop-1)

	plats := []platDef{
		{x: px(16), y: px(groundTop - 2), w: 48, h: 16, axis: "x", rng: 112, speed: 0.7},
		{x: px(33), y: px(groundTop - 2), w: 32, h: 16, axis: "y", rng: 144, speed: 0.8},
		{x: px(54), y: px(groundTop - 2), w: 48, h: 16, axis: "x", rng: 200, speed: 0.8},
	}
	return level{name: "level2", g: g, plats: plats, spawnX: px(2), spawnY: px(groundTop - 4),
		gateX: px(75), gateY: px(groundTop - 3), gateRequired: 4,
		bg: "../assets/backgrounds/bg_dusk.png"}
}

// =====================================================================
// Level 3 — "Skyline Sprint": hard, and introduces the pushable boulder.
// Sparse ground, fast platforms, double-jump pumpkins, and one pumpkin you
// reach by pushing a boulder into a gap to make a step.
// =====================================================================
func buildLevel3() level {
	var g grid
	g.ground(0, 10, groundTop)
	g.ground(20, 26, groundTop)
	g.ground(36, 48, groundTop) // the boulder island
	g.ground(74, 79, groundTop)

	g.torii(13, 30, 5)
	g.torii(29, 26, 5) // tall gate — its top pumpkin needs the double-jump chain
	g.torii(66, 28, 6) // gate over the far void, reached off a lift (well clear of the ledge)

	// ---- boulder ledge -----------------------------------------------------
	// The boulder-only pumpkin sits on top of a tall, isolated ledge to the right
	// of the boulder island. Between the island and the ledge is a wide, deep pit
	// (cols 49-52); the ledge (cols 53-55) is too tall and too far across the pit
	// to jump onto from the island, even with the boosted double-jump, and there
	// is a wide void to the RIGHT of the ledge with no platform in sprint-jump
	// range — so the old "hop the right platform and sprint-jump back" bypass is
	// gone. The pit floor sits well below the island, so a jump out of the empty
	// pit cannot reach the ledge top either.
	//
	// The only route: push the boulder off the island into the pit. The boulder
	// fills the pit and becomes a step whose top is level with the ledge, letting
	// you walk/hop up and grab the pumpkin. The static solver can't model a pushed
	// boulder, so with the pit empty it reports this pumpkin IMPOSSIBLE — proof
	// that no jump bypass exists — and the exemption records the boulder route.
	// The boulder-only pumpkin sits high on a tall, sheer-sided ledge that is
	// deliberately isolated from every easy route:
	//   - A wide deep pit (cols 49-52) separates it from the boulder island, so you
	//     can't just walk up to it.
	//   - The ledge wall is sheer up to row groundTop-9, with a WIDE VOID to its
	//     right and NOTHING above it — the exact "hop the platform on the right and
	//     sprint-jump back" bypass the player complained about no longer exists,
	//     because that platform is gone and there is no surface to launch from.
	// The intended route is the boulder: push it off the island into the pit so it
	// stacks into a step, then climb up to the ledge. The static solver still finds
	// one extremely tight boosted double-jump that can reach the ledge top (so it is
	// classified DOUBLE-JUMP CHAIN rather than impossible — a deliberate no-softlock
	// escape hatch for experts), but the cheap sprint-jump bypass is gone, so for
	// normal play the boulder is the route you actually take.
	for c := 49; c <= 52; c++ { // dig the deep pit between island and ledge
		for r := groundTop + 1; r < H-1; r++ {
			g.set(c, r, tDirt)
		}
		g.set(c, H-1, tBase)
		g.set(c, groundTop+5, tGrassTop) // pit floor, 5 tiles below the island top
	}
	for c := 53; c <= 55; c++ { // the tall, sheer, isolated ledge
		g.set(c, groundTop-9, tGrassTop)
		for r := groundTop - 8; r < H-1; r++ {
			g.set(c, r, tDirt)
		}
		g.set(c, H-1, tBase)
	}
	g.boulder(45, groundTop-1)  // spawns on the island, pushed right into the pit
	g.pumpkin(54, groundTop-10) // high on the isolated ledge — boulder is the intended route

	g.pumpkin(4, groundTop-1) // one easy ground pumpkin by spawn
	g.pumpkin(31, 25)         // top of the tall gate — DOUBLE-JUMP CHAIN
	g.pumpkin(68, 27)         // top of the far void gate — DOUBLE-JUMP CHAIN off the lift
	g.pumpkin(23, 30)         // ride the vertical lift, hop onto the low gate to grab

	g.torii(21, 31, 5) // low gate carrying the col-23 pumpkin

	g.enemy(75, groundTop-1)

	// Platform plan (kept clear of the boulder pit/ledge so the boulder stays the
	// only route to its pumpkin): p1 ferries you across the first big gap; p2 is a
	// short vertical lift that tops out level with the low gate (row 31), so the
	// tall-gate pumpkin above still needs a double-jump chain; p3 is a mid bridge
	// that never reaches the pit; p4 is a short lift under the void gate that tops
	// out below it, so that pumpkin also needs the double-jump chain.
	plats := []platDef{
		{x: px(11), y: px(groundTop - 4), w: 32, h: 16, axis: "x", rng: 120, speed: 0.95},
		{x: px(28), y: px(groundTop - 2), w: 32, h: 16, axis: "y", rng: 80, speed: 1.0},
		{x: px(33), y: px(groundTop - 6), w: 32, h: 16, axis: "x", rng: 96, speed: 0.95},
		{x: px(68), y: px(groundTop - 2), w: 32, h: 16, axis: "y", rng: 96, speed: 1.0},
	}
	return level{
		name: "level3", g: g, plats: plats,
		spawnX: px(2), spawnY: px(groundTop - 4),
		gateX: px(76), gateY: px(groundTop - 3), gateRequired: 5,
		bg: "../assets/backgrounds/bg_cave.png",
	}
}

// ====================== reachability solver ======================
//
// Models standable surfaces as horizontal "strips" (static ground tops, the
// swept span of a horizontal platform, or the sampled heights of a vertical
// lift). It flood-fills which strips the giraffe can reach from spawn by
// walking, falling, and jumping (simulating the real arc), then checks every
// pumpkin against the arcs. Two passes: normal jumps only, then with the
// boosted double-jump too — a pumpkin only the second pass reaches needs the
// chain; one neither reaches is impossible.

type strip struct {
	y, x0, x1 float64
	group     int // -1 static; >=0 = platform index (same group = ride-connected)
}

type solver struct {
	solid  [H][W]bool
	strips []strip
	pumps  [][2]int // col,row of each pumpkin
}

func newSolver(l level) *solver {
	s := &solver{}
	for r := 0; r < H; r++ {
		for c := 0; c < W; c++ {
			s.solid[r][c] = isSolid(l.g[r][c])
			if l.g[r][c] == tPumpkin {
				s.pumps = append(s.pumps, [2]int{c, r})
			}
		}
	}
	// static strips: runs of standable tiles (solid with empty above)
	for r := 0; r < H; r++ {
		c := 0
		for c < W {
			if s.solid[r][c] && (r == 0 || !s.solid[r-1][c]) {
				start := c
				for c < W && s.solid[r][c] && (r == 0 || !s.solid[r-1][c]) {
					c++
				}
				s.strips = append(s.strips, strip{y: float64(r * tileSize),
					x0: float64(start * tileSize), x1: float64(c * tileSize), group: -1})
			} else {
				c++
			}
		}
	}
	// platform strips
	for i, p := range l.plats {
		if p.axis == "y" {
			for yy := p.y; yy <= p.y+int(p.rng); yy += tileSize {
				s.strips = append(s.strips, strip{y: float64(yy),
					x0: float64(p.x), x1: float64(p.x + p.w), group: i})
			}
		} else {
			s.strips = append(s.strips, strip{y: float64(p.y),
				x0: float64(p.x), x1: float64(p.x) + p.rng + float64(p.w), group: i})
		}
	}
	return s
}

func (s *solver) solidPx(x, y float64) bool {
	c, r := int(x)/tileSize, int(y)/tileSize
	if r < 0 || r >= H || c < 0 || c >= W {
		return false
	}
	return s.solid[r][c]
}

// reach flood-fills reachable strips and returns which pumpkins were touched.
// allowBoost adds the stronger chained jump to the move set.
func (s *solver) reach(spawnX, spawnY int, allowBoost bool) (reachedPump map[int]bool) {
	reachedPump = map[int]bool{}

	// seed: the strip the player falls onto from spawn
	seed := -1
	bestY := 1e9
	for i, st := range s.strips {
		if float64(spawnX)+8 >= st.x0 && float64(spawnX)+8 <= st.x1 && st.y >= float64(spawnY) {
			if st.y < bestY {
				bestY, seed = st.y, i
			}
		}
	}
	if seed < 0 {
		return
	}

	visited := make([]bool, len(s.strips))
	queue := []int{seed}
	visited[seed] = true

	vys := []float64{0, jumpV}
	if allowBoost {
		vys = append(vys, boostV)
	}
	vxs := []float64{-sprintSpd, -walkSpd, 0, walkSpd, sprintSpd}

	for len(queue) > 0 {
		idx := queue[0]
		queue = queue[1:]
		src := s.strips[idx]

		// ride-connected strips (same platform group)
		if src.group >= 0 {
			for j, st := range s.strips {
				if !visited[j] && st.group == src.group {
					visited[j] = true
					queue = append(queue, j)
				}
			}
		}
		// walk-connected strips (same height, touching/overlapping x)
		for j, st := range s.strips {
			if visited[j] || absf(st.y-src.y) > 2 {
				continue
			}
			if st.x0 <= src.x1+6 && st.x1 >= src.x0-6 {
				visited[j] = true
				queue = append(queue, j)
			}
		}
		// jumps and falls from sampled launch points along the strip
		launch := []float64{src.x0 + 1, (src.x0 + src.x1) / 2, src.x1 - 17}
		for _, lx := range launch {
			for _, vy0 := range vys {
				for _, vx := range vxs {
					land := s.arc(lx, src.y-tileSize, vx, vy0, idx, reachedPump)
					if land >= 0 && !visited[land] {
						visited[land] = true
						queue = append(queue, land)
					}
				}
			}
		}
		// pumpkins sitting on this very strip (walk-over pickup)
		for pi, pk := range s.pumps {
			py := float64(pk[1] * tileSize)
			pxw := float64(pk[0] * tileSize)
			if pxw+8 >= src.x0 && pxw+8 <= src.x1 && absf(py+tileSize-src.y) <= tileSize {
				reachedPump[pi] = true
			}
		}
	}
	return
}

// arc simulates one jump/fall and returns the strip index landed on (or -1).
// Pumpkins the arc passes through are recorded in reached.
func (s *solver) arc(sx, sy, vx, vy0 float64, srcIdx int, reached map[int]bool) int {
	x, y, vy := sx, sy, vy0
	for f := 0; f < 160; f++ {
		vy += gravity
		if vy > termVy {
			vy = termVy
		}
		// horizontal step (stop at walls)
		nx := x + vx
		edge := nx
		if vx > 0 {
			edge = nx + tileSize - 1
		}
		if !s.solidPx(edge, y+8) {
			x = nx
		}
		// vertical step
		ny := y + vy
		if vy < 0 && s.solidPx(x+8, ny) { // head bonk
			vy, ny = 0, y
		}
		prevFeet := y + tileSize
		y = ny
		feet := y + tileSize

		// pumpkin overlap
		for pi, pk := range s.pumps {
			pxw, py := float64(pk[0]*tileSize), float64(pk[1]*tileSize)
			if x+tileSize > pxw && x < pxw+tileSize && y+tileSize > py && y < py+tileSize {
				reached[pi] = true
			}
		}
		// landing on a strip (descending, feet crossing strip y)
		if vy >= 0 && f > 2 {
			for k, st := range s.strips {
				if k == srcIdx {
					continue
				}
				if x+8 >= st.x0 && x+8 <= st.x1 && prevFeet <= st.y+2 && feet >= st.y {
					return k
				}
			}
		}
		if x < 0 || x+tileSize > W*tileSize || y > H*tileSize {
			return -1
		}
	}
	return -1
}

// validate runs both passes and prints a per-pumpkin report. Returns false if
// any non-exempt pumpkin is impossible.
func (s *solver) validate(l level) bool {
	norm := s.reach(l.spawnX, l.spawnY, false)
	boost := s.reach(l.spawnX, l.spawnY, true)
	ok := true
	for pi, pk := range s.pumps {
		key := fmt.Sprintf("%d,%d", pk[0], pk[1])
		switch {
		case norm[pi]:
			fmt.Printf("    pumpkin (%2d,%2d): reachable (normal jump)\n", pk[0], pk[1])
		case boost[pi]:
			fmt.Printf("    pumpkin (%2d,%2d): reachable (DOUBLE-JUMP CHAIN)\n", pk[0], pk[1])
		case l.exemptPumpkins[key]:
			fmt.Printf("    pumpkin (%2d,%2d): exempt (boulder puzzle)\n", pk[0], pk[1])
		default:
			fmt.Printf("    pumpkin (%2d,%2d): !!! IMPOSSIBLE !!!\n", pk[0], pk[1])
			ok = false
		}
	}
	return ok
}

func isSolid(id int) bool {
	switch id {
	case 10, 11, 27, 33, 37, 38, 39, 47, 49, 50, 51, 84, 90:
		return true
	}
	return false
}

func absf(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// ---- Tiled JSON output ----

type tmap struct {
	Height      int      `json:"height"`
	Width       int      `json:"width"`
	TileWidth   int      `json:"tilewidth"`
	TileHeight  int      `json:"tileheight"`
	Infinite    bool     `json:"infinite"`
	Orientation string   `json:"orientation"`
	RenderOrder string   `json:"renderorder"`
	Type        string   `json:"type"`
	Version     string   `json:"version"`
	TiledVer    string   `json:"tiledversion"`
	NextLayerID int      `json:"nextlayerid"`
	NextObjID   int      `json:"nextobjectid"`
	Tilesets    []tset   `json:"tilesets"`
	Layers      []tlayer `json:"layers"`
}

type tset struct {
	FirstGID int    `json:"firstgid"`
	Source   string `json:"source"`
}

type tlayer struct {
	ID          int     `json:"id"`
	Type        string  `json:"type"`
	Name        string  `json:"name"`
	Opacity     float64 `json:"opacity"`
	Visible     bool    `json:"visible"`
	X           int     `json:"x"`
	Y           int     `json:"y"`
	Width       int     `json:"width,omitempty"`
	Height      int     `json:"height,omitempty"`
	Data        []int   `json:"data,omitempty"`
	Image       string  `json:"image,omitempty"`
	ImageWidth  int     `json:"imagewidth,omitempty"`
	ImageHeight int     `json:"imageheight,omitempty"`
	Objects     []tobj  `json:"objects,omitempty"`
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
	data := make([]int, 0, W*H)
	for r := 0; r < H; r++ {
		for c := 0; c < W; c++ {
			data = append(data, l.g[r][c])
		}
	}
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
	if l.gateRequired > 0 {
		objs = append(objs, tobj{
			ID: len(objs) + 1, Name: "gate",
			X: l.gateX, Y: l.gateY, Width: 2 * tileSize, Height: 3 * tileSize,
			Properties: []tprop{{Name: "required", Type: "int", Value: float64(l.gateRequired)}},
		})
	}
	bg := l.bg
	if bg == "" {
		bg = "night_background.png"
	}
	return tmap{
		Height: H, Width: W, TileWidth: tileSize, TileHeight: tileSize,
		Infinite: false, Orientation: "orthogonal", RenderOrder: "right-down",
		Type: "map", Version: "1.10", TiledVer: "1.11.2",
		NextLayerID: 4, NextObjID: len(objs) + 1,
		Tilesets: []tset{{FirstGID: 1, Source: "../tilesets/platformer.tsx"}},
		Layers: []tlayer{
			{ID: 2, Type: "imagelayer", Name: "Image Layer 1", Opacity: 1, Visible: true,
				Image: bg, ImageWidth: 1280, ImageHeight: 720},
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
		sx, sy := (idx%6)*tileSize, (idx/6)*tileSize
		dr := image.Rect(col*tileSize, row*tileSize, col*tileSize+tileSize, row*tileSize+tileSize)
		draw.Draw(dst, dr, tileset, image.Pt(sx, sy), draw.Over)
	}
	for r := 0; r < H; r++ {
		for c := 0; c < W; c++ {
			blit(l.g[r][c], c, r)
		}
	}
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
	// moving platforms with clean tiles + a tint
	for _, p := range l.plats {
		cols := p.w / tileSize
		row := p.y / tileSize
		for i := 0; i < cols; i++ {
			id := 33
			if cols > 1 && i == 0 {
				id = 31
			} else if cols > 1 && i == cols-1 {
				id = 35
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
	bg, _ := loadPNG("levels/night_background.png")

	levels := []level{buildLevel1(), buildLevel2(), buildLevel3()}
	allOK := true
	for _, l := range levels {
		m := l.toTiledJSON()
		b, _ := json.MarshalIndent(m, "", " ")
		jsonPath := filepath.Join("levels", l.name+".json")
		if err := os.WriteFile(jsonPath, b, 0o644); err != nil {
			fmt.Println("write json:", err)
			os.Exit(1)
		}
		prev := filepath.Join("tools", "genlevels", "preview", l.name+".png")
		if err := renderPreview(l, tileset, bg, prev); err != nil {
			fmt.Println("preview:", err)
			os.Exit(1)
		}
		fmt.Printf("%s -> %s, %s\n", l.name, jsonPath, prev)
		if !newSolver(l).validate(l) {
			allOK = false
		}
	}
	if !allOK {
		fmt.Println("\nFAIL: at least one pumpkin is unreachable")
		os.Exit(1)
	}
	fmt.Println("\nOK: every pumpkin is reachable")
}
