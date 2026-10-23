package painter

// Shine is how shiny the tile's top is; false where it does not shine at all: a kind without
// shine, a flat world, a tile too far off.
func (t *tile) Shine() (float32, bool) {
	shine := t.baseTop().shine * t.Detail()
	return shine, shine > 0 && t.r.heights
}

// Detail is how much of what is fine the tile is drawn with, 0 to 1, by how many pixels it
// spans.
func (t *tile) Detail() float32 {
	return min(max((t.px()-farCell)/(nearCell-farCell), 0), 1)
}

// pixels a cell spans
const (
	farCell  = 6
	nearCell = 12
)
