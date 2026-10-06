package billboards

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/atmosphere/air"
	"github.com/kjkrol/gram/plugins/atmosphere/sky"
	"github.com/kjkrol/gram/plugins/topography/internal/relief"
	"github.com/kjkrol/gram/plugins/topography/internal/terrain"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

// Sky is the sky over the entities: the sun that lights them and casts their shadows, the weather
// that leans what sways and hazes the far off.
type Sky interface {
	Sun() sky.Sun
	Air() air.Weather
}

// inRelief reports whether cam looks at the world in relief — isometrically or in perspective, its
// projection sorting what it draws by depth — not from above.
func inRelief(cam camera.Camera) bool { return cam.Projection().Sorts() }

var _ world.DirectLook = Look{}

// Look lays the world's entities as the camera looks, on the GPU: billboards standing upright
// in the views in relief (sprites), hidden by the depth where the ground stands before them, the
// world's own flat sprites from above — lit by the sky's sun on level ground, leaning with its
// wind, each casting its shadow on the relief away from the sun.
type Look struct {
	flat world.Look
	sky  Sky
	gpu  *sprites
}

// New is the look of the entities standing on the relief r under sky, flat is the world's own look
// from above; the shadows go on ground, over the frame's depth where it is nil (hex prisms).
func New(flat world.Look, sky Sky, r *relief.Relief, ground *terrain.Renderer) Look {
	return Look{flat: flat, sky: sky, gpu: newSprites(sky, r, ground)}
}

// Begin readies the billboards, the shadows and the flat sprites for a frame through cam.
func (l Look) Begin(cam camera.Camera) {
	l.gpu.begin(cam)
	if d, ok := l.flat.(world.DirectLook); ok {
		d.Begin(cam)
	}
}

// DrawSprites draws the shadows, then the billboards or the flat sprites the frame took.
func (l Look) DrawSprites(t render.Target, cam camera.Camera, u render.Uniforms) {
	l.gpu.draw(t, cam, u)
	if d, ok := l.flat.(world.DirectLook); ok {
		d.DrawSprites(t, cam, u)
	}
}

func (l Look) Sprite(f *render.Frame, cam camera.Camera, box plane.AABB, z world.Z, atlas render.AtlasSource, a render.Appearance, light render.Light) {
	if l.gpu.on { // a billboard in relief ignores the Angle: it stands up, not flat
		l.gpu.add(cam, box, z, atlas, a.SpriteID, light, a.Sway, f.Time())
		return
	}
	if l.gpu.shading {
		l.gpu.shadow(box, z)
	}
	if a.Sway > 0 { // seen from above by its top, as high as it is wide, leaning with the wind
		sizeX, sizeY := float32(box.Size.X), float32(box.Size.Y)
		cx, cy := float32(box.TopLeft.X)+sizeX/2, float32(box.TopLeft.Y)+sizeY/2
		lx, ly := l.sky.Air().Sway(f.Time(), cx, cy, a.Sway)
		rise := max(sizeX, sizeY)
		box = plane.NewAABB(geom.NewVec(box.TopLeft.X+float64(lx*rise), box.TopLeft.Y+float64(ly*rise)), box.Size.X, box.Size.Y)
	}
	a.Sway = 0 // leant here: the flat look takes the sprite as it stands
	l.flat.Sprite(f, cam, box, z, atlas, a, lit(l.sky.Sun(), light))
}

func (l Look) Drawn(cam camera.Camera, box geom.AABB, z world.Z) render.Corners {
	if inRelief(cam) {
		return drawn(cam, box, z)
	}
	return l.flat.Drawn(cam, box, z)
}

func (l Look) Footprint(cam camera.Camera, box geom.AABB, alt float32, dst []render.Corners) []render.Corners {
	if inRelief(cam) {
		return footprint(cam, box, alt, dst)
	}
	return l.flat.Footprint(cam, box, alt, dst)
}
