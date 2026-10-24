package billboards

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/atmosphere/air"
	"github.com/kjkrol/gram/plugins/light"
	"github.com/kjkrol/gram/plugins/topography/internal/relief"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

// Sky is the sky over the entities: the sun that lights them and the weather that leans what
// sways.
type Sky interface {
	Distant() light.Distant
	Air() air.Weather
}

func inRelief(cam camera.Camera) bool { return cam.Projection().Sorts() }

var (
	_ world.DirectLook = Look{}
	_ world.LyingLook  = Look{}
)

// Look lays the world's entities on the GPU: billboards standing upright in the views in relief,
// the world's own flat sprites from above.
type Look struct {
	flat world.Look
	sky  Sky
	gpu  *sprites
}

// New is the look of the entities standing on the relief under sky as the frame drawn holds it,
// hazed as it says (drawn), flat the world's own look from above.
func New(flat world.Look, sky Sky, drawn func() (ground *relief.Relief, hazed bool)) Look {
	return Look{flat: flat, sky: sky, gpu: newSprites(sky, drawn)}
}

// Fog has the billboards and the flat sprites laid next turn into the fog by amount.
func (l Look) Fog(amount float32) {
	l.gpu.fog = amount
	if f, ok := l.flat.(world.FoggyLook); ok {
		f.Fog(amount)
	}
}

// Begin readies the billboards and the flat sprites for a frame through cam.
func (l Look) Begin(f *render.Frame, cam camera.Camera) {
	l.gpu.begin(cam)
	if d, ok := l.flat.(world.DirectLook); ok {
		d.Begin(f, cam)
	}
}

// DrawSprites draws the sprites the frame took.
func (l Look) DrawSprites(t render.Target, cam camera.Camera, u render.Uniforms) {
	l.gpu.draw(t, cam, u)
	if d, ok := l.flat.(world.DirectLook); ok {
		d.DrawSprites(t, cam, u)
	}
}

func (l Look) Sprite(f *render.Frame, cam camera.Camera, box plane.AABB, z world.Z,
	atlas render.AtlasSource, a render.Appearance, lit render.Illumination) {
	if l.gpu.on { // a billboard ignores the Angle: it stands up, not flat
		l.gpu.add(cam, box, z, atlas, a.SpriteID, lit, a.Sway, f.Time())
		return
	}
	if a.Sway > 0 { // from above the top leans as far as the entity is wide
		sizeX, sizeY := float32(box.Size.X), float32(box.Size.Y)
		cx, cy := float32(box.TopLeft.X)+sizeX/2, float32(box.TopLeft.Y)+sizeY/2
		lx, ly := l.sky.Air().Sway(f.Time(), cx, cy, a.Sway)
		rise := max(sizeX, sizeY)
		box = plane.NewAABB(geom.NewVec(box.TopLeft.X+float64(lx*rise), box.TopLeft.Y+float64(ly*rise)), box.Size.X, box.Size.Y)
	}
	a.Sway = 0 // leant here already
	l.flat.Sprite(f, cam, box, z, atlas, a, lit)
}

// Lie lays the sprite flat at the entity's Altitude. It does not sway.
func (l Look) Lie(f *render.Frame, cam camera.Camera, box plane.AABB, z world.Z,
	atlas render.AtlasSource, a render.Appearance, lit render.Illumination) {
	if l.gpu.on {
		l.gpu.lay(cam, box, z, atlas, a.SpriteID, a.Angle, lit)
		return
	}
	a.Sway = 0
	if lies, ok := l.flat.(world.LyingLook); ok {
		lies.Lie(f, cam, box, z, atlas, a, lit)
		return
	}
	l.flat.Sprite(f, cam, box, z, atlas, a, lit)
}

// Lay lays the sprite level at the entity's Altitude over the rectangle lie says, its sides
// down its Height and its edges standing up along its long sides. It does not sway.
func (l Look) Lay(f *render.Frame, cam camera.Camera, box plane.AABB, dir geom.Vec,
	lie world.Lie, z world.Z, atlas render.AtlasSource, a render.Appearance,
	edge render.SpriteID, lit render.Illumination) {
	if l.gpu.on {
		m := geom.NewVec(box.TopLeft.X+box.Size.X/2, box.TopLeft.Y+box.Size.Y/2)
		c := lie.Corners(m, dir)
		alt, deep := float32(z.Altitude), float32(z.Height)
		l.gpu.level(cam, c, alt, atlas, a.SpriteID, lit)
		l.gpu.upright(cam, c[1], c[0], alt-deep, deep, atlas, a.SpriteID, lit)
		l.gpu.upright(cam, c[2], c[3], alt-deep, deep, atlas, a.SpriteID, lit)
		if lie.Edged {
			high := float32(lie.Edge)
			l.gpu.upright(cam, c[1], c[0], alt, high, atlas, edge, lit)
			l.gpu.upright(cam, c[2], c[3], alt, high, atlas, edge, lit)
		}
		return
	}
	a.Sway = 0
	if lies, ok := l.flat.(world.LyingLook); ok {
		lies.Lay(f, cam, box, dir, lie, z, atlas, a, edge, lit)
		return
	}
	l.flat.Sprite(f, cam, box, z, atlas, a, lit)
}

// Upright stands the sprite on the four sides of the box from its Altitude up its Height and
// over its top.
func (l Look) Upright(f *render.Frame, cam camera.Camera, box plane.AABB, z world.Z,
	atlas render.AtlasSource, a render.Appearance, lit render.Illumination) {
	x0, y0 := box.TopLeft.X, box.TopLeft.Y
	x1, y1 := box.BottomRight.X, box.BottomRight.Y
	tl, tr := geom.NewVec(x0, y0), geom.NewVec(x1, y0)
	bl, br := geom.NewVec(x0, y1), geom.NewVec(x1, y1)
	alt, high := float32(z.Altitude), tall(z, float32(y0), float32(y1))
	if !l.gpu.on {
		a.Sway = 0
		l.flat.Sprite(f, cam, box, z, atlas, a, lit)
		return
	}
	for _, side := range [4][2]geom.Vec{{tr, tl}, {br, tr}, {bl, br}, {tl, bl}} {
		l.gpu.upright(cam, side[0], side[1], alt, high, atlas, a.SpriteID, lit)
	}
	l.gpu.level(cam, [4]geom.Vec{tl, tr, bl, br}, alt+high, atlas, a.SpriteID, lit)
}

var _ world.UprightLook = Look{}
var _ world.FoggyLook = Look{}

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
