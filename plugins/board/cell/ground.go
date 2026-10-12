package cell

// Plot is which cell a cell's entity is.
type Plot struct{ Cell ID }

// Ground is the terrain kind of a cell, on the cell's entity: an effect altering it alters the
// board.
type Ground struct{ Kind Kind }
