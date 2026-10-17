package billboards

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/atmosphere/sky"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

// as tall as the box is long without a Height
func tall(z world.Z, y0, y1 float32) float32 {
	if z.Height > 0 {
		return float32(z.Height)
	}
	return y1 - y0
}

func drawn(cam camera.Camera, box geom.AABB, z world.Z) render.Corners {
	x0, y0 := float32(box.TopLeft.X), float32(box.TopLeft.Y)
	x1, y1 := float32(box.BottomRight.X), float32(box.BottomRight.Y)
	return billboard(cam, (x0+x1)/2, (y0+y1)/2, float32(z.Altitude), x1-x0, tall(z, y0, y1))
}

func footprint(cam camera.Camera, box geom.AABB, alt float32, dst []render.Corners) []render.Corners {
	return append(dst, render.ProjectCorners(cam, float32(box.TopLeft.X), float32(box.TopLeft.Y), float32(box.BottomRight.X), float32(box.BottomRight.Y), alt))
}

func lit(sun sky.Sun, light render.Light) render.Light {
	l := sun.Light(0, 0, 1)
	return render.Light{l[0] * light[0], l[1] * light[1], l[2] * light[2]}
}

// sunShare is how much of the light on a surface whose normal is (nx, ny, nz) is the sun's, 0 to
// 1: what a shadow over it takes away
func sunShare(sun sky.Sun, nx, ny, nz float32) float32 {
	all, sky := sun.Shaded(nx, ny, nz, 1), sun.Shaded(nx, ny, nz, 0)
	total := all[0] + all[1] + all[2]
	if total <= 0 {
		return 0
	}
	return 1 - (sky[0]+sky[1]+sky[2])/total
}

func ridden(cam camera.Camera, x0, y0, x1, y1 float32) bool {
	if camera.HowOf(cam) != camera.Inside {
		return false
	}
	e, ok := cam.(camera.Eyed)
	if !ok {
		return false
	}
	x, y, _, ok := e.Eye()
	return ok && x >= x0 && x <= x1 && y >= y0 && y <= y1
}

// the bottom edge centred on the projected point, as wide as the camera draws w across there and
// as tall as it draws h standing upright
func billboard(cam camera.Camera, x, y, z, w, h float32) render.Corners {
	sx, sy := cam.Project(x, y, z)
	_, top := cam.Project(x, y, z+h)
	hw, hh := w*camera.ScaleAt(cam, x, y, z)/2, max(sy-top, 0)
	return render.Corners{{sx - hw, sy - hh}, {sx + hw, sy - hh}, {sx - hw, sy}, {sx + hw, sy}}
}
