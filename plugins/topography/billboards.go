package topography

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/atmosphere/sky"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

// billboards is where entities stand in relief, for picking and outlines: a sprite as wide as the
// box and as tall as the entity stands (world.Z.Height; as tall as the box is long without one),
// upright on its centre at its altitude — drawn so on the GPU (sprites).
type billboards struct{ d *dresser }

// tall is how tall the billboard of an entity whose box spans x0 to y1 and which stands as z says is.
func tall(z world.Z, y0, y1 float32) float32 {
	if z.Height > 0 {
		return float32(z.Height)
	}
	return y1 - y0
}

func (billboards) Drawn(cam camera.Camera, box geom.AABB, z world.Z) render.Corners {
	x0, y0 := float32(box.TopLeft.X), float32(box.TopLeft.Y)
	x1, y1 := float32(box.BottomRight.X), float32(box.BottomRight.Y)
	return billboard(cam, (x0+x1)/2, (y0+y1)/2, float32(z.Altitude), x1-x0, tall(z, y0, y1))
}

// Footprint is the diamond the box covers on the ground at its altitude.
func (billboards) Footprint(cam camera.Camera, box geom.AABB, alt float32, dst []render.Corners) []render.Corners {
	return append(dst, render.ProjectCorners(cam, float32(box.TopLeft.X), float32(box.TopLeft.Y), float32(box.BottomRight.X), float32(box.BottomRight.Y), alt))
}

// lit is the sun's light on level ground, in the light an entity is asked to be drawn in.
func lit(sun sky.Sun, light render.Light) render.Light {
	l := sun.Light(0, 0, 1)
	return render.Light{l[0] * light[0], l[1] * light[1], l[2] * light[2]}
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
