package game

import (
	"bytes"
	"embed"
	"image"

	"github.com/hajimehoshi/ebiten/v2"
)

// tileClassByID maps Tiled tile IDs to their class/type names.
var tileClassByID = map[int]string{
	11: "standard_barrel",
	84: "pumpkin_barrel",
	90: "trigger_barrel",
}
var pumpkinImage *ebiten.Image

func LoadPumpkinImage(img *ebiten.Image) {
	pumpkinImage = img
}

// InteractionContext contains any dynamic game state needed for interactions.
type InteractionContext struct {
	PumpkinMissed  bool
	SpawnPumpkin   func(x, y float64)
	InitialDropped bool
	PumpkinSpawned bool
}

// Pumpkin is the falling pumpkin entity.
type Pumpkin struct {
	X, Y  float64
	VelY  float64
	Image *ebiten.Image
	Alive bool
}

// NewPumpkin creates a new falling pumpkin.
func NewPumpkin(x, y float64) *Pumpkin {
	return &Pumpkin{
		X:     x,
		Y:     y,
		VelY:  0,
		Alive: true,
		Image: pumpkinImage, // Make sure this variable exists and is initialized
	}
}

// TileInteraction defines a function signature for things the player can interact with.
type TileInteraction func(p *Player, ctx InteractionContext)

var tileInteractions = map[string]TileInteraction{
	"standard_barrel": func(p *Player, ctx InteractionContext) {
		p.onInteract("There’s nothing inside...")
	},

	"pumpkin_barrel": func(p *Player, ctx InteractionContext) {
		p.onInteract("Wow, there’s a pumpkin inside!")

		ts := float64(TileSize)
		ty := int((p.Y + p.Height/2) / ts)
		var tx int
		if p.facingRight {
			tx = int((p.X + p.Width + 1) / ts)
		} else {
			tx = int((p.X - 1) / ts)
		}
		above := ty - 1

		// Only trigger if it's still a pumpkin_barrel
		if ty >= 0 && ty < len(Levels[CurrentLevel].Tiles) &&
			tx >= 0 && tx < len(Levels[CurrentLevel].Tiles[0]) &&
			tileClassByID[Levels[CurrentLevel].Tiles[ty][tx]] == "pumpkin_barrel" {

			// Place the pumpkin above
			if above >= 0 && above < len(Levels[CurrentLevel].Tiles) {
				Levels[CurrentLevel].Tiles[above][tx] = 48 // static pumpkin tile
			}

			// Replace this barrel with a standard one
			Levels[CurrentLevel].Tiles[ty][tx] = 11
		}
	},
	"trigger_barrel": func(p *Player, ctx InteractionContext) {
		// nothing until the initial drop
		if !ctx.InitialDropped {
			p.onInteract("Nothing happens…")
			return
		}
		// only proceed if the previous pumpkin was missed and none is flying
		if !ctx.PumpkinMissed {
			p.onInteract("feeling greedy are we?")
			return
		}
		if ctx.PumpkinSpawned {
			p.onInteract("Catch it, quick!")
			return
		}
		// redrop message
		p.onInteract("…is that something falling from the sky?")
		// spawn exactly one new pumpkin
		ctx.SpawnPumpkin(p.X+p.Width/2, 0)
	},
}

func LoadInteractableAssets(assets embed.FS) {
	data, err := assets.ReadFile("assets/items/pumpkin.png")
	if err != nil {
		panic(err)
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		panic(err)
	}

	pumpkinImage = ebiten.NewImageFromImage(img)

}

// isInteractableObject returns true if the tile is known to have an interaction.
func isInteractableObject(tileID int) bool {
	_, ok := tileClassByID[tileID]
	return ok
}

// Rect returns the bounding box of the pumpkin for collision.
func (p *Pumpkin) Rect() image.Rectangle {
	return image.Rect(
		int(p.X), int(p.Y),
		int(p.X+float64(TileSize)), int(p.Y+float64(TileSize)),
	)
}
