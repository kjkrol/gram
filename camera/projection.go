package camera

// Projection is how a world point at a height lands on the screen at zoom 1, with the world's
// origin at the screen's: the mapping a Camera draws through, pure arithmetic without a window.
type Projection interface {
	// Project maps the world point (x, y) at height z to screen units.
	Project(x, y, z float32) (sx, sy float32)
	// Unproject inverts Project at height z: the world point drawn at (sx, sy).
	Unproject(sx, sy, z float32) (x, y float32)
	// Depth orders drawing: what is further back is smaller and drawn first.
	Depth(x, y, z float32) float32
	// Wraps reports whether a world wrapping at its edges can be drawn through this projection.
	Wraps() bool
	// Sorts reports whether what is drawn through it must go back to front by Depth: whether
	// something nearer can hide something further back.
	Sorts() bool
	// Toward is the way from the world towards whoever looks through it, a unit vector (x and y
	// along the world, z up): what a surface must face to throw the sun back at the eye.
	Toward() [3]float32
}

// TopDown is the plain map view: screen x and y are world x and y, height is not drawn.
type TopDown struct{}

func (TopDown) Project(x, y, _ float32) (float32, float32)     { return x, y }
func (TopDown) Unproject(sx, sy, _ float32) (float32, float32) { return sx, sy }
func (TopDown) Depth(_, y, _ float32) float32                  { return y }
func (TopDown) Wraps() bool                                    { return true }
func (TopDown) Sorts() bool                                    { return false }
func (TopDown) Toward() [3]float32                             { return [3]float32{0, 0, 1} }
