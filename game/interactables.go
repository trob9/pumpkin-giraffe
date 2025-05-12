package game

import (
	"bytes"
	"embed"
	"image"

	_ "image/png" // register PNG decoder so we can decode .png files

	"github.com/hajimehoshi/ebiten/v2"
)

// tileClassByID maps raw numeric tile IDs from our Tiled JSON maps
// to logical class names. This abstraction lets us assign behavior
// based on a human-readable string instead of hard-coding numbers everywhere.
var tileClassByID = map[int]string{
	11: "standard_barrel", // generic barrel that does nothing special
	84: "pumpkin_barrel",  // barrel containing a pumpkin to pop out
	90: "trigger_barrel",  // barrel that re-drops pumpkins based on game state
}

// pumpkinImage holds the shared image resource for all Pumpkin instances.
// It’s loaded once from disk and reused for performance.
var pumpkinImage *ebiten.Image

// LoadPumpkinImage allows an external initializer (e.g., main.go) to
// supply the loaded Ebiten image for pumpkins. This must be called
// before creating any Pumpkins via NewPumpkin.
func LoadPumpkinImage(img *ebiten.Image) {
	pumpkinImage = img
}

// InteractionContext bundles dynamic state needed during an interact call.
// - PumpkinMissed: true if the last pumpkin fell off-screen uncollected.
// - SpawnPumpkin: callback to create a new falling pumpkin at given coords.
// - InitialDropped: true once the first pumpkin has dropped after all enemies died.
// - PumpkinSpawned: true if there is currently a pumpkin falling in the world.
type InteractionContext struct {
	PumpkinMissed  bool
	SpawnPumpkin   func(x, y float64)
	InitialDropped bool
	PumpkinSpawned bool
}

// Pumpkin represents a single falling pumpkin entity.
// - X, Y: world coordinates (top-left corner).
// - VelY: current vertical velocity for gravity.
// - Image: reference to a shared *ebiten.Image for rendering.
// - Alive: true while the pumpkin is active (not yet caught or missed).
type Pumpkin struct {
	X, Y  float64
	VelY  float64
	Image *ebiten.Image
	Alive bool
}

// NewPumpkin constructs a new Pumpkin at the specified (x, y) position.
// It sets initial velocity to zero and marks it alive. Make sure pumpkinImage
// has been initialized first via LoadPumpkinImage or similar.
func NewPumpkin(x, y float64) *Pumpkin {
	return &Pumpkin{
		X:     x,
		Y:     y,
		VelY:  0,            // starts with no vertical motion
		Alive: true,         // active until caught or missed
		Image: pumpkinImage, // shared texture
	}
}

// TileInteraction is the signature for all tile-specific handlers.
// It receives the Player and the dynamic InteractionContext.
type TileInteraction func(p *Player, ctx InteractionContext)

// tileInteractions maps each logical tile class to its handling function.
// When the player interacts with a tile, we look up its class and invoke
// the corresponding function to perform the behavior.
var tileInteractions = map[string]TileInteraction{
	// Standard barrel: just displays a “there’s nothing here” message.
	"standard_barrel": func(p *Player, ctx InteractionContext) {
		p.onInteract("There’s nothing inside...")
	},

	// Pumpkin barrel: pops out a static pumpkin tile above and
	// converts itself into a standard barrel.
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

		// Verify this tile is still a pumpkin_barrel before modifying it
		if ty >= 0 && ty < len(Levels[CurrentLevel].Tiles) &&
			tx >= 0 && tx < len(Levels[CurrentLevel].Tiles[0]) &&
			tileClassByID[Levels[CurrentLevel].Tiles[ty][tx]] == "pumpkin_barrel" {

			// Place a static pumpkin tile directly above the barrel
			if above >= 0 && above < len(Levels[CurrentLevel].Tiles) {
				Levels[CurrentLevel].Tiles[above][tx] = 48 // ID 48 = pumpkin pickup
			}
			// Change the barrel into a standard barrel so it won’t pop again
			Levels[CurrentLevel].Tiles[ty][tx] = 11
		}
	},

	// Trigger barrel: spawns a new falling pumpkin if conditions met:
	// - initial pumpkin has already dropped,
	// - last pumpkin was missed,
	// - currently there is no pumpkin in flight.
	"trigger_barrel": func(p *Player, ctx InteractionContext) {
		if !ctx.InitialDropped {
			p.onInteract("Nothing happens…")
			return
		}
		if !ctx.PumpkinMissed {
			p.onInteract("Feeling greedy, are we?")
			return
		}
		if ctx.PumpkinSpawned {
			p.onInteract("Catch it, quick!")
			return
		}
		p.onInteract("…is that something falling from the sky?")
		// Use the provided callback to spawn exactly one pumpkin from above
		ctx.SpawnPumpkin(p.X+p.Width/2, 0)
	},
}

// LoadInteractableAssets reads and decodes the pumpkin sprite image
// from the embedded filesystem. It sets pumpkinImage so NewPumpkin
// will use this texture. Panics if the file is missing or decode fails.
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

// isInteractableObject returns true if the given tile ID corresponds to
// any tile class we know how to interact with (i.e., has an entry in tileClassByID).
func isInteractableObject(tileID int) bool {
	_, ok := tileClassByID[tileID]
	return ok
}

// Rect computes the axis-aligned bounding box for this pumpkin,
// in pixel coordinates. Used for collision detection with the player.
func (p *Pumpkin) Rect() image.Rectangle {
	return image.Rect(
		int(p.X),                   // left
		int(p.Y),                   // top
		int(p.X+float64(TileSize)), // right
		int(p.Y+float64(TileSize)), // bottom
	)
}
