package game

// TryInteract is called when the player presses the interact key (E).
// It figures out which tile is immediately next to the player in the direction they face,
// checks if that tile supports any special behavior, and if so, invokes the appropriate handler.
// If the tile isn’t interactive, it does nothing; if it is but has no registered handler,
// it shows a default “Nothing happens.” message.
func (p *Player) TryInteract(ctx InteractionContext) {
	// Determine the tile coordinates beside the player
	tx, ty := p.interactionTile()
	// Look up the numeric ID of that tile in the current level
	tileID := Levels[CurrentLevel].Tiles[ty][tx]

	// If this tile isn’t flagged as interactable, bail out immediately
	if !isInteractableObject(tileID) {
		return
	}

	// Map the numeric tile ID to a logical “class” (e.g., barrel, switch, door)
	class := tileClassByID[tileID]
	// See if we have a function registered to handle that class of tile
	if handler, ok := tileInteractions[class]; ok {
		// Invoke the handler, passing in the player and context (which may drop or spawn pumpkins)
		handler(p, ctx)
	} else {
		// No handler registered: show a fallback message to the player
		p.onInteract("Nothing happens.")
	}
}

// interactionTile calculates which tile the player is “looking at” when they press E.
// It returns the column (tx) and row (ty) indices of that tile in the level grid.
func (p *Player) interactionTile() (int, int) {
	ts := float64(TileSize)
	// Compute the vertical tile row at the player’s mid-height
	ty := int((p.Y + p.Height/2) / ts)

	var tx int
	if p.facingRight {
		// If facing right, pick the tile just to the right of the player’s bounding box
		tx = int((p.X + p.Width + 1) / ts)
	} else {
		// If facing left, pick the tile just to the left of the player’s bounding box
		tx = int((p.X - 1) / ts)
	}

	return tx, ty
}
