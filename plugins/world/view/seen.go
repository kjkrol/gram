package view

import (
	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/lod"
)

// Sectors is a world's grid of sectors as a View sees it: how many and how many in a row (the
// sector i lies at column i % Cols, row i / Cols), each one's rectangle, the one a point lies in,
// the level a camera sees one at (the level ticked for a camera that is no observer) and the
// world's ladder. An observer sees a sector at the level it is ticked at or coarser, never finer.
type Sectors interface {
	Len() int
	Cols() int
	Box(i int) geom.AABB
	Of(x, y float32) int
	Level(cam camera.Camera, i int) lod.Level
	Ladder() lod.Ladder
}

// NewSeen is a View of what cam shows of a world cut into sectors: the sectors its picture
// covers within its sight, each at its level, and the entities in them.
func NewSeen(cam camera.Camera, sectors Sectors) *View {
	v := &View{cam: cam, sectors: sectors, levels: make([]lod.Level, sectors.Len())}
	v.add = v.In.Add
	return v
}

// Level is how fully what stands at the world point (x, y) is drawn: the level the camera sees
// its sector at, shifted coarser by its LevelShift (camera.Shifted) to lod.L3 at most but never
// from lod.L0, lod.L4 off the picture or past the sight; lod.L0 in a View of no sectors.
func (v *View) Level(x, y float32) lod.Level {
	if v.sectors == nil {
		return lod.L0
	}
	return v.levels[v.sectors.Of(x, y)]
}

// Sectors calls each with every sector seen, its index in the Grid, its rectangle and its level,
// under lod.L4; a View of no sectors has none.
func (v *View) Sectors(each func(i int, box geom.AABB, level lod.Level)) {
	for i, l := range v.levels {
		if l != lod.L4 {
			each(i, v.sectors.Box(i), l)
		}
	}
}

// Grid is the world's sectors the View sees, nil in a world of none.
func (v *View) Grid() Sectors { return v.sectors }

// Fog is how far into the fog before the camera's sight the world point (x, y) lies, 0 short of
// it to 1 at the sight's end.
func (v *View) Fog(x, y float32) float32 {
	if !v.sighted || v.fog <= 0 {
		return 0
	}
	from := v.sight - v.fog
	t := (f32.Hypot(x-v.from.X, y-v.from.Y) - from) / v.fog
	t = min(max(t, 0), 1)
	return t * t * (3 - 2*t)
}

// Fogging is the fog before the camera's sight ends as a shader takes it: from where (x, y), where
// it begins and where it ends; all zero without a sight.
func (v *View) Fogging() [4]float32 {
	if !v.sighted {
		return [4]float32{}
	}
	return [4]float32{v.from.X, v.from.Y, v.sight - v.fog, v.sight}
}

// the sectors the camera's picture covers within its sight, at their levels, and who is in them
// and in the picture's bounds
func (v *View) see(space *aabbworld.Space) {
	ladder := v.sectors.Ladder()
	v.sight, v.fog = ladder.Sight*camera.SightOf(v.cam), ladder.Fog
	x, y, ok := camera.Viewpoint(v.cam)
	v.from, v.sighted = geom.NewVec(x, y), ok && v.sight > 0
	v.Bounds, v.Culled = v.cam.Bounds(), true
	v.In.Clear()
	// on a torus the bounds may run past the world's edge: their copies a world away meet the
	// sectors across the seam
	cfg := space.Config
	wx, wy := float32(cfg.Width), float32(cfg.Height)
	// the frame budget's shift is the picture's alone, the tick's marks unshifted; it hides
	// nothing, hiding being the ladder's, and leaves what is seen at L0 full
	shift := camera.LevelShiftOf(v.cam)
	for i := range v.levels {
		v.levels[i] = lod.L4
		level := v.sectors.Level(v.cam, i)
		if level > lod.L0 && level < lod.L4 {
			level += min(shift, lod.L3-level)
		}
		box := v.sectors.Box(i)
		if level == lod.L4 || !v.cam.Visible(box) {
			continue
		}
		v.levels[i] = level
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
