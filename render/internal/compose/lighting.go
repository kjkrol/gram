package compose

import (
	"math"
	"slices"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/render/internal/light"
	"github.com/kjkrol/gram/render/internal/wgsl"
)

// how far over the ground the live shadows lie among what lies on it: under the board's grid
const castLift = 0.05

// lighting is the composer's light over a frame through a camera: the scene's lights in the
// frame's uniforms, the lamps nearest the camera among them, the light on what the sources draw
// (Frame.Lit), the shadows of what they say casts (Frame.Cast) laid once they have all composed,
// and the ground's shade baked before the ground is drawn. It draws as a Direct of the composer.
type lighting struct {
	lights func() *light.Lights
	index  int32 // the composer's Direct it draws as
	now    *light.Lights
	beam   light.Beam
	near   []light.Lamp
	nearer func(a, b light.Lamp) int // byDistance, bound once
	at     [2]float32                // the camera's viewpoint
	eye    [3]float32
	eyed   bool
	right  [2]float32 // the camera's way across, whose side a silhouette is seen from

	casters []caster
	casts   []float32
	atlas   AtlasSource
	baker   Baker
	caster  Caster
}

// a thing casting a live shadow, as a source said (Frame.Cast)
type caster struct {
	box         geom.AABB
	alt, height float32
	cast        light.Casting
	atlas       AtlasSource
	sprite      SpriteID
}

// begin readies the light for the frame f through cam, its ground's shade planned unless p is the
// part over the ground
func (l *lighting) begin(f *Frame, cam camera.Camera, p part) {
	l.casters, l.casts, l.now = l.casters[:0], l.casts[:0], nil
	if l.lights != nil {
		l.now = l.lights()
	}
	if l.now == nil {
		return
	}
	f.lighting = l
	l.now.Sun.Frame(f)
	l.beam = l.now.Sun.Beam()
	l.look(cam)
	f.Uniform("LampCount", float32(len(l.near)))
	for k, lamp := range l.near {
		f.Uniform(wgsl.LampNames[k][0], lamp.X, lamp.Y, lamp.Z, lamp.Reach)
		f.Uniform(wgsl.LampNames[k][1], lamp.Color[0], lamp.Color[1], lamp.Color[2])
	}
	if p != overPart {
		l.plan(f)
	}
}

// look takes the camera's eye, its way across and the lamps nearest it, wgsl.Lamps at most
func (l *lighting) look(cam camera.Camera) {
	l.eyed, l.right = false, [2]float32{1, 0}
	if e, ok := cam.(camera.Eyed); ok {
		var x, y, z float32
		if x, y, z, l.eyed = e.Eye(); l.eyed {
			l.eye = [3]float32{x, y, z}
		}
	}
	if r, ok := cam.(camera.Rays); ok {
		if field, ok := r.Rays(); ok {
			right := field.DX
			if field.DDX != ([3]float32{}) || field.DDY != ([3]float32{}) {
				right = field.DDX
			}
			l.right = [2]float32{right[0], right[1]}
		}
	}
	x, y, ok := camera.Viewpoint(cam)
	if !ok {
		b := cam.Bounds()
		x, y = (b.TopLeft.X+b.BottomRight.X)/2, (b.TopLeft.Y+b.BottomRight.Y)/2
	}
	l.at = [2]float32{x, y}
	if l.nearer == nil {
		l.nearer = l.byDistance
	}
	l.near = append(l.near[:0], l.now.Lamps...)
	slices.SortFunc(l.near, l.nearer)
	if len(l.near) > wgsl.Lamps {
		l.near = l.near[:wgsl.Lamps]
	}
}

func (l *lighting) byDistance(a, b light.Lamp) int {
	x, y := l.at[0], l.at[1]
	da := (a.X-x)*(a.X-x) + (a.Y-y)*(a.Y-y)
	db := (b.X-x)*(b.X-x) + (b.Y-y)*(b.Y-y)
	switch {
	case da < db:
		return -1
	case da > db:
		return 1
	}
	return 0
}

// plan has the ground's shade baked as the lights say, drawn first of the opaque pass
func (l *lighting) plan(f *Frame) {
	l.baker.Coarse(l.now.Coarse)
	if lat := l.now.Ground; lat.Cols <= 0 || l.now.Sun.Strength <= 0 {
		l.baker.Drop()
		return
	}
	if shade, shown := l.baker.Plan(l.now.Ground, l.now.Sun.Dir, l.now.Blocks); shown {
		f.GroundShade(shade)
	}
	if l.baker.Pending() {
		f.source = l.index
		f.DrawOpaque()
		f.source = 0
	}
}

// end lays the shadows of what the sources said casts, once they have all composed
func (l *lighting) end(f *Frame) {
	if l.now == nil {
		return
	}
	for i := range l.casters {
		l.cast(&l.casters[i])
	}
	if len(l.casts) > 0 {
		f.ReadGround()
		f.source = l.index
		f.DrawAt(f.OnGround(castLift))
		f.source = 0
	}
}

// Compose is nothing: the composer readies the light itself (begin, end).
func (*lighting) Compose(*Frame, camera.Camera) {}

// Draw bakes the shade planned in the opaque pass, and lays the casts after the ground is drawn.
func (l *lighting) Draw(t Target, cam camera.Camera, u Uniforms) {
	switch t.Pass {
	case Opaque:
		l.baker.Bake(u)
	case SeeThrough:
		if l.now != nil {
			lat := l.now.Ground
			l.caster.Draw(t, cam, l.atlas, l.casts, lat.Low, lat.Low+lat.Span)
		}
	}
}

// on is the light on what stands at (x, y, z) taking takes: the distant light's on a surface
// facing up and the lamps nearest the camera's, the share of it the distant light's; none taken,
// drawn as it is
func (l *lighting) on(x, y, z float32, takes light.Taking) light.Illumination {
	if takes == 0 {
		return light.Illumination{Light: light.Light{1, 1, 1}}
	}
	lit := light.Illumination{Light: l.beam.Shaded(0, 0, 1, 1), Takes: takes}
	if sum := lit.Light[0] + lit.Light[1] + lit.Light[2]; sum > 0 {
		sky := l.beam.Shaded(0, 0, 1, 0)
		lit.Direct = 1 - (sky[0]+sky[1]+sky[2])/sum
	}
	if takes&light.TakesLamps == 0 {
		return lit
	}
	for _, lamp := range l.near {
		if c, ok := lamp.Light(x, y, z); ok {
			for k := range lit.Lamps {
				lit.Lamps[k] += c[k]
			}
		}
	}
	if sum := lit.Light[0] + lit.Light[1] + lit.Light[2]; sum > 0 {
		lit.Direct *= sum / (sum + lit.Lamps[0] + lit.Lamps[1] + lit.Lamps[2])
	}
	for k := range lit.Light {
		lit.Light[k] += lit.Lamps[k]
	}
	return lit
}

// cast gathers the live shadow of c through the policy: the distant light's, and on level ground
// the casting lamps' too
func (l *lighting) cast(c *caster) {
	casting := c.cast
	if l.now.Policy != nil {
		casting = l.now.Policy(c.cast, l.distance(c))
	}
	if casting == light.Baked || casting == light.NoShadow {
		return
	}
	shape := casting == light.Silhouette
	sun, groundAt := l.now.Sun, l.now.GroundAt
	if u, ok := sun.ShadowOf(c.box, c.alt, c.height, groundAt); ok {
		if shape {
			u, _ = sun.SilhouetteOf(c.box, c.alt, c.height, groundAt)
		}
		l.gather(u, c, shape)
	}
	if groundAt != nil {
		return
	}
	for _, lamp := range l.near[:min(len(l.near), l.now.LampShadows)] {
		if u, ok := lamp.ShadowOf(c.box, c.alt, c.height, shape); ok {
			l.gather(u, c, shape)
		}
	}
}

// gather adds the cast u, in the shape of c's sprite where shape says, seen from the side the
// sprite faces: its left on the left of the eye's way across
func (l *lighting) gather(u light.Umbra, c *caster, shape bool) {
	var u0, v0, u1, v1 float32
	if shape && c.atlas != nil {
		u0, v0, u1, v1 = c.atlas.UV(c.sprite)
		if -u.UY*l.right[0]+u.UX*l.right[1] < 0 {
			u0, u1 = u1, u0
		}
	}
	if c.atlas != nil {
		l.atlas = c.atlas
	}
	l.casts = append(l.casts, u.X, u.Y, u.UX, u.UY, u.Along, u.Wide, u.Fade, u.Veil,
		u0, v0, u1, v1)
}

// how far from the eye c's box's middle stands, 0 through a camera without one
func (l *lighting) distance(c *caster) float32 {
	if !l.eyed {
		return 0
	}
	dx := (c.box.TopLeft.X+c.box.BottomRight.X)/2 - l.eye[0]
	dy := (c.box.TopLeft.Y+c.box.BottomRight.Y)/2 - l.eye[1]
	dz := c.alt - l.eye[2]
	return float32(math.Sqrt(float64(dx*dx + dy*dy + dz*dz)))
}

// taken is the shade t takes: all for a zero one, none for Shines
func taken(t light.Taking) light.Taking {
	switch {
	case t == 0:
		return light.TakesAll
	case t&light.Shines != 0:
		return 0
	}
	return t
}
