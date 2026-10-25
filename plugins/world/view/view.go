package view

import (
	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/uid"
)

// View is a rectangle of the world and the entities the Space finds in it, refreshed each tick
// after movement. The zero value sees everything.
type View struct {
	// Bounds is the rectangle seen, as of the last tick.
	Bounds geom.AABB
	// Culled says Bounds is less than the whole world and In is the answer; false means everything.
	Culled bool
	// In is who the Space found in Bounds, when Culled.
	In EntitySet

	bounds func() geom.AABB
	add    func(uid.UID64) // In.Add, bound once

	// a camera seeing a world cut into sectors: each sector's level, how far it sees and from where
	cam     camera.Camera
	sectors Sectors
	levels  []Level
	detail  camera.Detail
	from    geom.Vec
	sighted bool
}

// New is a View over whatever bounds says.
func New(bounds func() geom.AABB) *View {
	v := &View{bounds: bounds}
	v.add = v.In.Add
	return v
}

// Contains reports whether id is in view.
func (v *View) Contains(id uid.UID64) bool { return !v.Culled || v.In.Has(id) }

// CopyInto has dst see what v sees now, keeping dst's memory; no tick refreshes dst.
func (v *View) CopyInto(dst *View) {
	dst.Bounds, dst.Culled = v.Bounds, v.Culled
	dst.In.words = dst.In.words[:0]
	if v.Culled {
		dst.In.words = append(dst.In.words, v.In.words...)
	}
	dst.sectors, dst.detail, dst.from, dst.sighted = v.sectors, v.detail, v.from, v.sighted
	dst.levels = append(dst.levels[:0], v.levels...)
}

func (v *View) refresh(space *aabbworld.Space, worldArea float64) {
	if v.sectors != nil {
		v.see(space)
		return
	}
	b := v.bounds()
	v.Bounds = b
	v.In.Clear()
	if (b.BottomRight.X-b.TopLeft.X)*(b.BottomRight.Y-b.TopLeft.Y) >= worldArea {
		v.Culled = false
		return
	}
	v.Culled = true
	space.Query(b, aabbworld.AnyCapability, v.add)
}
