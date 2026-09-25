package board

// Plot is a cell's own entity's place and shape: which cell it is and the heights of its corners.
// Every cell of a board has one, for good; the board and the shaping commands write it.
type Plot struct {
	Cell   CellID
	Relief Relief
}

// Ground is the terrain kind of a cell, on the cell's entity beside its Plot: an effect altering
// it alters the board. It holds the kind alone, so an effect ending puts back the kind and leaves
// the heights shaped meanwhile as they are.
type Ground struct{ Kind CellKind }

// Relief is the ground height at a cell's corners: top-left, top-right, bottom-left, bottom-right.
// A hex cell is level, its four heights one.
type Relief struct{ Corners [4]float32 }

// Level is the mean of the corners.
func (r Relief) Level() float64 {
	return (float64(r.Corners[0]) + float64(r.Corners[1]) + float64(r.Corners[2]) + float64(r.Corners[3])) / 4
}
