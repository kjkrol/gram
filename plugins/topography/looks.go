package topography

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

// inRelief reports whether cam looks at the world in relief — isometrically or in perspective: a
// camera of the topography's not looking from above.
func inRelief(cam camera.Camera) bool {
	c, ok := cam.(*viewCamera)
	return ok && c.relief()
}

var _ world.DirectLook = worldLook{}

// worldLook lays the world's entities as the camera looks, on the GPU: billboards standing upright
// in the views in relief (sprites), hidden by the depth where the ground stands before them, the
// world's own flat sprites from above — lit by the sky's sun on level ground, leaning with its
// wind, each casting its shadow on the relief away from the sun.
type worldLook struct {
	flat world.Look
	d    *dresser
	gpu  *sprites
}

// Begin readies the billboards, the shadows and the flat sprites for a frame through cam.
func (l worldLook) Begin(cam camera.Camera) {
	l.gpu.begin(cam)
	if d, ok := l.flat.(world.DirectLook); ok {
		d.Begin(cam)
	}
}

// DrawSprites draws the shadows, then the billboards or the flat sprites the frame took.
func (l worldLook) DrawSprites(t render.Target, cam camera.Camera, u render.Uniforms) {
	l.gpu.draw(t, cam, u)
	if d, ok := l.flat.(world.DirectLook); ok {
		d.DrawSprites(t, cam, u)
	}
}

func (l worldLook) Sprite(f *render.Frame, cam camera.Camera, box plane.AABB, z world.Z, atlas render.AtlasSource, id render.SpriteID, light render.Light, sway float32) {
	if l.gpu.on {
		l.gpu.add(cam, box, z, atlas, id, light, sway, f.Time())
		return
	}
	if l.gpu.shading {
		l.gpu.shadow(box, z)
	}
	if sway > 0 { // seen from above by its top, as high as it is wide, leaning with the wind
		sizeX, sizeY := float32(box.Size.X), float32(box.Size.Y)
		cx, cy := float32(box.TopLeft.X)+sizeX/2, float32(box.TopLeft.Y)+sizeY/2
		lx, ly := l.d.sky.Air().Sway(f.Time(), cx, cy, sway)
		rise := max(sizeX, sizeY)
		box = plane.NewAABB(geom.NewVec(box.TopLeft.X+float64(lx*rise), box.TopLeft.Y+float64(ly*rise)), box.Size.X, box.Size.Y)
	}
	l.flat.Sprite(f, cam, box, z, atlas, id, lit(l.d.sky.Sun(), light), 0)
}

func (l worldLook) Drawn(cam camera.Camera, box geom.AABB, z world.Z) render.Corners {
	if inRelief(cam) {
		return billboards{d: l.d}.Drawn(cam, box, z)
	}
	return l.flat.Drawn(cam, box, z)
}

func (l worldLook) Footprint(cam camera.Camera, box geom.AABB, alt float32, dst []render.Corners) []render.Corners {
	if inRelief(cam) {
		return billboards{d: l.d}.Footprint(cam, box, alt, dst)
	}
	return l.flat.Footprint(cam, box, alt, dst)
}
