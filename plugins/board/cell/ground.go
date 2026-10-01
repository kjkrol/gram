package cell

// Plot is a cell's own entity's place: which cell it is. Every cell of a board has one, for good.
type Plot struct{ Cell ID }

// Ground is the terrain kind of a cell, on the cell's entity beside its Plot: an effect altering
// it alters the board. It holds the kind alone, so an effect ending puts back the kind and leaves
// the ground's heights, a topography's, as they are.
type Ground struct{ Kind Kind }
