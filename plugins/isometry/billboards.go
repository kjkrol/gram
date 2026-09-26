package isometry

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

var _ world.Look = billboards{}

// billboards is how entities stand in the isometric view: a sprite the size of the box upright on
// its centre at its altitude, at the depth of that centre, which ties with the tile it stands on.
type billboards struct{}

func (billboards) Sprite(f *render.Frame, cam camera.Camera, box plane.AABB, alt float32, atlas render.AtlasSource, id render.SpriteID) {
	x0, y0 := float32(box.TopLeft.X), float32(box.TopLeft.Y)
	x1, y1 := float32(box.BottomRight.X), float32(box.BottomRight.Y)
	cx, cy := (x0+x1)/2, (y0+y1)/2
	f.Sprite(render.Objects, cam.Depth(cx, cy, alt), atlas, id, billboard(cam, cx, cy, alt, x1-x0, y1-y0), render.Even(1))
}

func (billboards) Drawn(cam camera.Camera, box geom.AABB, alt float32) render.Corners {
	x0, y0 := float32(box.TopLeft.X), float32(box.TopLeft.Y)
	x1, y1 := float32(box.BottomRight.X), float32(box.BottomRight.Y)
	return billboard(cam, (x0+x1)/2, (y0+y1)/2, alt, x1-x0, y1-y0)
}

// Footprint is the diamond the box covers on the ground at its altitude.
func (billboards) Footprint(cam camera.Camera, box geom.AABB, alt float32, dst []render.Corners) []render.Corners {
	return append(dst, render.ProjectCorners(cam, float32(box.TopLeft.X), float32(box.TopLeft.Y), float32(box.BottomRight.X), float32(box.BottomRight.Y), alt))
}

// billboard is a sprite w x h world units large standing upright at the world point (x, y, z): on
// screen, a rectangle whose bottom edge is centred on the projected point.
func billboard(cam camera.Camera, x, y, z, w, h float32) render.Corners {
	sx, sy := cam.Project(x, y, z)
	zoom := cam.Zoom()
	hw, hh := w*zoom/2, h*zoom
	return render.Corners{{sx - hw, sy - hh}, {sx + hw, sy - hh}, {sx - hw, sy}, {sx + hw, sy}}
}
