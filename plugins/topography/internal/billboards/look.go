package billboards

import (
	"math"

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

// Sky is the sky over the entities: the sun that lights them and the weather that leans what
// sways.
type Sky interface {
	Sun() sky.Sun
	Air() air.Weather
}

func inRelief(cam camera.Camera) bool { return cam.Projection().Sorts() }

var (
	_ world.DirectLook = Look{}
	_ world.LyingLook  = Look{}
	_ world.ShadowLook = Look{}
)

// Policy is how an entity casts its shadow as far as distance from the eye, declared to cast it as
// it does: the topography's ShadowPolicy.
type Policy func(declared world.Casting, distance float32) world.Casting

// Look lays the world's entities on the GPU: billboards standing upright in the views in relief,
// the world's own flat sprites from above.
type Look struct {
	flat   world.Look
	sky    Sky
	gpu    *sprites
	policy Policy
}

// New is the look of the entities standing on the relief r under sky, flat the world's own look
// from above, casting their shadows as policy says. The shadows go on ground, or over the frame's
// depth where it is nil (hex prisms).
func New(flat world.Look, sky Sky, r *relief.Relief, ground *terrain.Renderer, policy Policy) Look {
	return Look{flat: flat, sky: sky, gpu: newSprites(sky, r, ground), policy: policy}
}

// Begin readies the billboards, the shadows and the flat sprites for a frame through cam.
func (l Look) Begin(f *render.Frame, cam camera.Camera) {
	l.gpu.begin(f, cam)
	if d, ok := l.flat.(world.DirectLook); ok {
		d.Begin(f, cam)
	}
}

// DrawSprites draws the shadows, then the sprites the frame took.
func (l Look) DrawSprites(t render.Target, cam camera.Camera, u render.Uniforms) {
	l.gpu.draw(t, cam, u)
	if d, ok := l.flat.(world.DirectLook); ok {
		d.DrawSprites(t, cam, u)
	}
}

func (l Look) Sprite(f *render.Frame, cam camera.Camera, box plane.AABB, z world.Z, atlas render.AtlasSource, a render.Appearance, light render.Light) {
	if l.gpu.on { // a billboard ignores the Angle: it stands up, not flat
		l.gpu.add(cam, box, z, atlas, a.SpriteID, light, a.Sway, f.Time())
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
	l.flat.Sprite(f, cam, box, z, atlas, a, lit(l.sky.Sun(), light))
}

// Shadow casts on the ground the shadow of the entity standing in box as z says, as cast and the
// policy say: a patch, the shape of its sprite or none; a Baked shadow is the terrain's.
func (l Look) Shadow(f *render.Frame, cam camera.Camera, box plane.AABB, z world.Z,
	atlas render.AtlasSource, a render.Appearance, cast world.Casting) {
	x0, y0 := float32(box.TopLeft.X), float32(box.TopLeft.Y)
	x1, y1 := float32(box.BottomRight.X), float32(box.BottomRight.Y)
	if !l.gpu.shading || cast == world.Baked || cast == world.NoShadow ||
		ridden(cam, x0, y0, x1, y1) {
		return
	}
	if l.policy != nil {
		cast = l.policy(cast, distance(cam, (x0+x1)/2, (y0+y1)/2, float32(z.Altitude)))
	}
	switch cast {
	case world.Patch:
		l.gpu.cast(box, z, atlas, a.SpriteID, false)
	case world.Silhouette:
		l.gpu.cast(box, z, atlas, a.SpriteID, true)
	}
}

// how far the point (x, y, z) lies from cam's eye; 0 through a camera without one
func distance(cam camera.Camera, x, y, z float32) float32 {
	e, ok := cam.(camera.Eyed)
	if !ok {
		return 0
	}
	ex, ey, ez, ok := e.Eye()
	if !ok {
		return 0
	}
	return float32(math.Sqrt(float64((x-ex)*(x-ex) + (y-ey)*(y-ey) + (z-ez)*(z-ez))))
}

// Lie lays the sprite flat at the entity's Altitude. It neither sways nor casts a shadow.
func (l Look) Lie(f *render.Frame, cam camera.Camera, box plane.AABB, z world.Z,
	atlas render.AtlasSource, a render.Appearance, light render.Light) {
	if l.gpu.on {
		l.gpu.lay(cam, box, z, atlas, a.SpriteID, a.Angle, light)
		return
	}
	a.Sway = 0
	if lies, ok := l.flat.(world.LyingLook); ok {
		lies.Lie(f, cam, box, z, atlas, a, lit(l.sky.Sun(), light))
		return
	}
	l.flat.Sprite(f, cam, box, z, atlas, a, lit(l.sky.Sun(), light))
}

// Lay lays the sprite level at the entity's Altitude over the rectangle lie says, its sides
// down its Height and its edges standing up along its long sides. It does not sway and casts no
// patch: a Baked shadow is the terrain's.
func (l Look) Lay(f *render.Frame, cam camera.Camera, box plane.AABB, dir geom.Vec,
	lie world.Lie, z world.Z, atlas render.AtlasSource, a render.Appearance,
	edge render.SpriteID, light render.Light) {
	if l.gpu.on {
		m := geom.NewVec(box.TopLeft.X+box.Size.X/2, box.TopLeft.Y+box.Size.Y/2)
		c := lie.Corners(m, dir)
		alt, deep := float32(z.Altitude), float32(z.Height)
		l.gpu.level(cam, c, alt, atlas, a.SpriteID, light)
		l.gpu.upright(cam, c[1], c[0], alt-deep, deep, atlas, a.SpriteID, light)
		l.gpu.upright(cam, c[2], c[3], alt-deep, deep, atlas, a.SpriteID, light)
		if lie.Edged {
			high := float32(lie.Edge)
			l.gpu.upright(cam, c[1], c[0], alt, high, atlas, edge, light)
			l.gpu.upright(cam, c[2], c[3], alt, high, atlas, edge, light)
		}
		return
	}
	a.Sway = 0
	if lies, ok := l.flat.(world.LyingLook); ok {
		lies.Lay(f, cam, box, dir, lie, z, atlas, a, edge, lit(l.sky.Sun(), light))
		return
	}
	l.flat.Sprite(f, cam, box, z, atlas, a, lit(l.sky.Sun(), light))
}

// Upright stands the sprite on the four sides of the box from its Altitude up its Height and
// over its top. It casts no patch: a Baked shadow is the terrain's.
func (l Look) Upright(f *render.Frame, cam camera.Camera, box plane.AABB, z world.Z,
	atlas render.AtlasSource, a render.Appearance, light render.Light) {
	x0, y0 := box.TopLeft.X, box.TopLeft.Y
	x1, y1 := box.BottomRight.X, box.BottomRight.Y
	tl, tr := geom.NewVec(x0, y0), geom.NewVec(x1, y0)
	bl, br := geom.NewVec(x0, y1), geom.NewVec(x1, y1)
	alt, high := float32(z.Altitude), tall(z, float32(y0), float32(y1))
	if !l.gpu.on {
		a.Sway = 0
		l.flat.Sprite(f, cam, box, z, atlas, a, lit(l.sky.Sun(), light))
		return
	}
	for _, side := range [4][2]geom.Vec{{tr, tl}, {br, tr}, {bl, br}, {tl, bl}} {
		l.gpu.upright(cam, side[0], side[1], alt, high, atlas, a.SpriteID, light)
	}
	l.gpu.level(cam, [4]geom.Vec{tl, tr, bl, br}, alt+high, atlas, a.SpriteID, light)
}

var _ world.UprightLook = Look{}

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
