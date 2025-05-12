package game

// TryInteract checks if the player has just pressed 'E' and is beside an interactable object.
// If so, it looks up the tile and triggers its associated interaction logic.
func (p *Player) TryInteract(ctx InteractionContext) {
	if !p.ShouldTriggerInteraction() {
		return
	}

	tx, ty := p.interactionTile()
	tileID := Levels[CurrentLevel].Tiles[ty][tx]

	if !isInteractableObject(tileID) {
		return
	}

	class := tileClassByID[tileID]
	if handler, ok := tileInteractions[class]; ok {
		handler(p, ctx)
	} else {
		p.onInteract("Nothing happens.")
	}
}

// interactionTile returns the tile coordinates beside the player in the direction they’re facing.
func (p *Player) interactionTile() (int, int) {
	ts := float64(TileSize)
	ty := int((p.Y + p.Height/2) / ts)

	var tx int
	if p.facingRight {
		tx = int((p.X + p.Width + 1) / ts)
	} else {
		tx = int((p.X - 1) / ts)
	}

	return tx, ty
}
