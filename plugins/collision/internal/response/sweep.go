package response

import (
	"math"

	"github.com/kjkrol/aabbworld/geom"
)

// Sweep tests the path of a box of half sides half, its centre going from from to to, against
// box: where along the path, 0 to 1, the two first touch, the way out of box through the face the
// path enters by — the way the mover came — and whether they touch at all. A path starting
// inside box touches at 0, the way out the shortest; one only grazing an edge does not touch.
func Sweep(from, to, half geom.Vec, box geom.AABB) (along float64, normal geom.Vec, ok bool) {
	lo := geom.NewVec(box.TopLeft.X-half.X, box.TopLeft.Y-half.Y)
	hi := geom.NewVec(box.BottomRight.X+half.X, box.BottomRight.Y+half.Y)
	d := geom.NewVec(to.X-from.X, to.Y-from.Y)

	entryX, exitX, faceX, ok := slab(from.X, d.X, lo.X, hi.X)
	if !ok {
		return 0, geom.Vec{}, false
	}
	entryY, exitY, faceY, ok := slab(from.Y, d.Y, lo.Y, hi.Y)
	if !ok {
		return 0, geom.Vec{}, false
	}
	entry, exit := max(entryX, entryY), min(exitX, exitY)
	if entry > exit || exit <= 0 || entry >= 1 {
		return 0, geom.Vec{}, false
	}
	if entry < 0 { // inside as the step begins: the shortest way out
		return 0, wayOut(from, lo, hi), true
	}
	if entryX > entryY {
		return entry, geom.NewVec(faceX, 0), true
	}
	return entry, geom.NewVec(0, faceY), true
}

// slab is one axis of the test: where along the path it enters and leaves the stretch lo to hi,
// and the way out through the face it enters by; false when a path still along the axis lies
// outside the stretch.
func slab(at, d, lo, hi float64) (entry, exit, face float64, ok bool) {
	if d == 0 {
		if at <= lo || at >= hi {
			return 0, 0, 0, false
		}
		return math.Inf(-1), math.Inf(1), 0, true
	}
	t1, t2 := (lo-at)/d, (hi-at)/d
	if d > 0 {
		return t1, t2, -1, true
	}
	return t2, t1, 1, true
}

// wayOut is the shortest way out of the stretch lo to hi for a point at p inside it.
func wayOut(p, lo, hi geom.Vec) geom.Vec {
	ways := [4]struct {
		depth float64
		dir   geom.Vec
	}{
		{p.X - lo.X, geom.NewVec(-1, 0)},
		{hi.X - p.X, geom.NewVec(1, 0)},
		{p.Y - lo.Y, geom.NewVec(0, -1)},
		{hi.Y - p.Y, geom.NewVec(0, 1)},
	}
	best := 0
	for i, w := range ways {
		if w.depth < ways[best].depth {
			best = i
		}
	}
	return ways[best].dir
}
