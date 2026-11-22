package mosaic

import "github.com/kjkrol/aabbworld/geom"

// Read reads the ground.
type Read struct{ p *Plugin }

// HeightAt is the ground's height at (x, y) as it is drawn there.
func (r *Read) HeightAt(x, y float32) float32 {
	h, _ := r.p.ground.plane.HeightAt(geom.NewVec(x, y))
	return h
}

// Free reports whether (x, y, z) lies over the ground: where a camera may be.
func (r *Read) Free(x, y, z float32) bool { return z > r.HeightAt(x, y) }

// Span is the lowest and the highest the ground's points lie.
func (r *Read) Span() (low, high float32) { return r.p.ground.low, r.p.ground.high }

// Counts is how many cells the ground is drawn in now and how many triangles its sectors hold.
func (r *Read) Counts() (cells, triangles int) {
	return r.p.ground.plane.Drawn(), r.p.ground.triangles
}
