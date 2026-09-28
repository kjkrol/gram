package topography

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

func (billboards) Sprite(f *render.Frame, cam camera.Camera, box plane.AABB, alt float32, atlas render.AtlasSource, id render.SpriteID, light render.Light, sway float32) {
	x0, y0 := float32(box.TopLeft.X), float32(box.TopLeft.Y)
	x1, y1 := float32(box.BottomRight.X), float32(box.BottomRight.Y)
	if ridden(cam, x0, y0, x1, y1) {
		return // an eye does not see what it rides in
	}
	cx, cy := (x0+x1)/2, (y0+y1)/2
	if camera.ScaleAt(cam, cx, cy, alt) == 0 {
		return // not in front of the eye
	}
	corners := billboard(cam, cx, cy, alt, x1-x0, y1-y0)
	if sway > 0 { // its top leans with the wind, as far as it stands high
		lx, ly := render.Sway(f.Time(), f.Wind(), cx, cy, sway)
		h := y1 - y0
		fx, fy := cam.Project(cx, cy, alt)
		tx, ty := cam.Project(cx+lx*h, cy+ly*h, alt)
		for k := range 2 {
			corners[k][0] += tx - fx
			corners[k][1] += ty - fy
		}
	}
	f.Sprite(render.Objects, cam.Depth(cx, cy, alt), atlas, id, corners, render.Lit(light))
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

// ridden reports whether the eye of cam rides in the box x0, y0 to x1, y1: a camera in first
// person, its eye over the box.
func ridden(cam camera.Camera, x0, y0, x1, y1 float32) bool {
	c, ok := cam.(*viewCamera)
	if !ok || !c.insideUnit() {
		return false
	}
	e := c.persp.eye()
	return e[0] >= x0 && e[0] <= x1 && e[1] >= y0 && e[1] <= y1
}

// billboard is a sprite w x h world units large standing upright at the world point (x, y, z): on
// screen, a rectangle whose bottom edge is centred on the projected point, as large as the camera
// draws a world unit there.
func billboard(cam camera.Camera, x, y, z, w, h float32) render.Corners {
	sx, sy := cam.Project(x, y, z)
	zoom := camera.ScaleAt(cam, x, y, z)
	hw, hh := w*zoom/2, h*zoom
	return render.Corners{{sx - hw, sy - hh}, {sx + hw, sy - hh}, {sx - hw, sy}, {sx + hw, sy}}
}
