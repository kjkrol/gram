package view

import (
	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/internal/f32"
)

// Level is how finely a camera draws what lies in a sector: Exact, Coarse, Fogged (coarsely and
// fading into the fog); Hidden, not at all.
type Level uint8

const (
	Hidden Level = iota
	Exact
	Coarse
	Fogged
)

// Sectors is a world's grid of sectors as a View sees it: how many and how many in a row (the
// sector i lies at column i % Cols, row i / Cols), each one's rectangle, the one a point lies
// in, and whether one lies beyond every band.
type Sectors interface {
	Len() int
	Cols() int
	Box(i int) geom.AABB
	Of(x, y float32) int
	Off(i int) bool
}

// NewSeen is a View of what cam shows of a world cut into sectors: the sectors its picture
// covers within its sight, each at its Level, and the entities in them.
func NewSeen(cam camera.Camera, sectors Sectors) *View {
	v := &View{cam: cam, sectors: sectors, levels: make([]Level, sectors.Len())}
	v.add = v.In.Add
	return v
}

// Level is how finely what stands at the world point (x, y) is drawn; Exact in a View of no
// sectors.
func (v *View) Level(x, y float32) Level {
	if v.sectors == nil {
		return Exact
	}
	return v.levels[v.sectors.Of(x, y)]
}

// Sectors calls each with every sector seen, its index in the Grid, its rectangle and its Level;
// a View of no sectors has none.
func (v *View) Sectors(each func(i int, box geom.AABB, level Level)) {
	for i, l := range v.levels {
		if l != Hidden {
			each(i, v.sectors.Box(i), l)
		}
	}
}

// Grid is the world's sectors the View sees, nil in a world of none.
func (v *View) Grid() Sectors { return v.sectors }

// Fog is how far into the fog before the camera's sight the world point (x, y) lies, 0 short of
// it to 1 at the sight's end.
func (v *View) Fog(x, y float32) float32 {
	if !v.sighted || v.detail.Fog <= 0 {
		return 0
	}
	from := v.detail.Sight - v.detail.Fog
	t := (f32.Hypot(x-v.from.X, y-v.from.Y) - from) / v.detail.Fog
	t = min(max(t, 0), 1)
	return t * t * (3 - 2*t)
}

// Fogging is the fog before the camera's sight ends as a shader takes it: from where (x, y), where
// it begins and where it ends; all zero without a sight.
func (v *View) Fogging() [4]float32 {
	if !v.sighted {
		return [4]float32{}
	}
	return [4]float32{v.from.X, v.from.Y,
		v.detail.Sight - v.detail.Fog, v.detail.Sight}
}

// the sectors the camera's picture covers within its sight, at their levels, and who is in them
// and in the picture's bounds
func (v *View) see(space *aabbworld.Space) {
	v.detail = camera.DetailOf(v.cam)
	x, y, ok := camera.Viewpoint(v.cam)
	v.from, v.sighted = geom.NewVec(x, y), ok && v.detail.Sight > 0
	v.Bounds, v.Culled = v.cam.Bounds(), true
	v.In.Clear()
	// on a torus the bounds may run past the world's edge: their copies a world away meet the
	// sectors across the seam
	cfg := space.Config
	wx, wy := float32(cfg.Width), float32(cfg.Height)
	for i := range v.levels {
		v.levels[i] = v.level(i)
		if v.levels[i] == Hidden {
			continue
		}
		box := v.sectors.Box(i)
		for kx := -1; kx <= 1; kx++ {
			for ky := -1; ky <= 1; ky++ {
				if kx != 0 && !cfg.Edges.WrapsX() || ky != 0 && !cfg.Edges.WrapsY() {
					continue
				}
				by := geom.NewVec(float32(kx)*wx, float32(ky)*wy)
				seen := geom.NewAABB(v.Bounds.TopLeft.Add(by), v.Bounds.BottomRight.Add(by))
				if in, ok := overlap(box, seen); ok {
					space.Query(in, aabbworld.AnyCapability, v.add)
				}
			}
		}
	}
}

// the part of a within b, false where they do not meet
func overlap(a, b geom.AABB) (geom.AABB, bool) {
	tl := geom.NewVec(max(a.TopLeft.X, b.TopLeft.X), max(a.TopLeft.Y, b.TopLeft.Y))
	br := geom.NewVec(min(a.BottomRight.X, b.BottomRight.X), min(a.BottomRight.Y, b.BottomRight.Y))
	return geom.NewAABB(tl, br), tl.X < br.X && tl.Y < br.Y
}

func (v *View) level(i int) Level {
	box := v.sectors.Box(i)
	if v.sectors.Off(i) {
		return Hidden
	}
	near := nearest(box, v.from)
	if v.sighted {
		d := f32.Hypot(near.X-v.from.X, near.Y-v.from.Y)
		switch {
		case d > v.detail.Sight:
			return Hidden
		case d > v.detail.Sight-v.detail.Fog:
			if v.cam.Visible(box) {
				return Fogged
			}
			return Hidden
		}
	}
	if !v.cam.Visible(box) {
		return Hidden
	}
	// behind the eye the scale is 0: a sector reaching under it is seen near
	if s := camera.ScaleAt(v.cam, near.X, near.Y, 0); v.detail.Coarse > 0 &&
		s > 0 && s < v.detail.Coarse {
		return Coarse
	}
	return Exact
}

// the point of box nearest p
func nearest(box geom.AABB, p geom.Vec) geom.Vec {
	return geom.NewVec(min(max(p.X, box.TopLeft.X), box.BottomRight.X),
		min(max(p.Y, box.TopLeft.Y), box.BottomRight.Y))
}
