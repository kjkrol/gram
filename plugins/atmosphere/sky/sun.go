package sky

import (
	"embed"
	"image/color"
	"math"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

//go:embed shaders/*.wgsl
var shaders embed.FS

// The sun's uniforms in the composer's shader (shaders/sun.wgsl), registered as the package is set
// up: a material may read them, and Sun.Frame sets them.
var _ = render.RegisterMaterials(render.Files(shaders, "shaders/sun.wgsl"), []render.Uniform{
	{Name: "Sun", Size: 3}, {Name: "SunStrength", Size: 1}, {Name: "SunColor", Size: 3}, {Name: "SkyColor", Size: 3}, {Name: "Ambience", Size: 3},
})

// Sun is the light over the world: Dir points from the ground towards the sun (x and y along the
// world, z up), Strength is how much it lights a surface facing it square on, Ambient how much of
// the sky's light every surface gets anyway. Color is the colour of the sun's light and Sky the
// colour of the sky — the light it gives, what water reflects, what shows beyond the world; zero
// is white.
type Sun struct {
	Dir      [3]float32
	Strength float32
	Ambient  float32
	Color    render.Light
	Sky      render.Light
}

// DefaultSun stands high over the world's south-east, towards the viewer of the isometric view,
// so the faces it shows are lit and the slopes turned away from it darken; its light and sky are
// white. It is the sun of a relief under no sky of its own.
var DefaultSun = Sun{Dir: [3]float32{0.522, 0.282, 0.805}, Strength: 0.708, Ambient: 0.35}

// Light is the light the sun casts on a surface whose normal is (nx, ny, nz): Ambient of the sky's,
// and Strength of its own by how square on the surface faces it; none of it from behind.
func (s Sun) Light(nx, ny, nz float32) render.Light { return s.Shaded(nx, ny, nz, 1) }

// Shaded is Light where only lit of the sun, 0 to 1, reaches the surface: the rest is in shadow
// and gets the sky's light alone.
func (s Sun) Shaded(nx, ny, nz, lit float32) render.Light { return s.Lamp().Shaded(nx, ny, nz, lit) }

// Lamp is the sun made ready to light many surfaces: its way as a unit vector, its light and the
// sky's already scaled.
func (s Sun) Lamp() Lamp {
	var l Lamp
	if d := float32(math.Sqrt(float64(s.Dir[0]*s.Dir[0] + s.Dir[1]*s.Dir[1] + s.Dir[2]*s.Dir[2]))); d > 0 {
		l.dir = [3]float32{s.Dir[0] / d, s.Dir[1] / d, s.Dir[2] / d}
	}
	sun, sky := white(s.Color), white(s.Sky)
	for c := range l.sun {
		l.sun[c], l.sky[c] = s.Strength*sun[c], s.Ambient*sky[c]
	}
	return l
}

// Lamp is a Sun ready to light surfaces: Shaded as the Sun's, without working the sun out again.
type Lamp struct {
	dir      [3]float32
	sun, sky render.Light
}

// Shaded is the light on a surface whose normal is (nx, ny, nz), lit of the sun reaching it.
func (l Lamp) Shaded(nx, ny, nz, lit float32) render.Light {
	direct := float32(0)
	if n := float32(math.Sqrt(float64(nx*nx + ny*ny + nz*nz))); n > 0 {
		direct = max((nx*l.dir[0]+ny*l.dir[1]+nz*l.dir[2])/n, 0) * lit
	}
	return render.Light{l.sky[0] + direct*l.sun[0], l.sky[1] + direct*l.sun[1], l.sky[2] + direct*l.sun[2]}
}

// SkyLight is the colour of the sky, white for the zero one.
func (s Sun) SkyLight() render.Light { return white(s.Sky) }

// Frame hands f the sun as its shader reads it (sun.kage): the way towards it, its strength, the
// colours of its light and of the sky, and the light every surface gets from the sky.
func (s Sun) Frame(f *render.Frame) {
	sky := white(s.Sky)
	sun := white(s.Color)
	f.Uniform("Sun", s.Dir[0], s.Dir[1], s.Dir[2])
	f.Uniform("SunStrength", s.Strength)
	f.Uniform("SunColor", sun[0], sun[1], sun[2])
	f.Uniform("SkyColor", sky[0], sky[1], sky[2])
	f.Uniform("Ambience", s.Ambient*sky[0], s.Ambient*sky[1], s.Ambient*sky[2])
}

// white is l, or white for the zero light.
func white(l render.Light) render.Light {
	if l == (render.Light{}) {
		return render.Light{1, 1, 1}
	}
	return l
}

// ShadowTier puts the shadows of what stands over the ground and its grid and under what stands.
const ShadowTier = render.Ground + 20

// maxShadowReach caps how far a unit of height casts its shadow: a sun on the horizon would cast it
// for ever.
const maxShadowReach = 6

// shadowVeil is how dark a shadow lays on the ground at its middle under a sun of shadowFull: a
// weaker light — the moon, the sun low at dawn — casts it paler.
const (
	shadowVeil = 110
	shadowFull = 0.6
)

// Patch is where a shadow lies on the ground, world units: its middle (X, Y), the way away from
// the sun (UX, UY), how far it reaches from the middle along that way (Along) and across it
// (Wide), how far in from each side it fades out (Fade) and how dark it is inside (Veil, 0 to 1).
type Patch struct{ X, Y, UX, UY, Along, Wide, Fade, Veil float32 }

// ShadowOf is the shadow of an entity standing in box as z says, on the ground groundAt gives
// (nil: level at 0), away from the sun: a soft patch as wide as the box, stretched by its height
// and pushed off by how far above the ground it stands, solid in its middle and soft for the
// outer half of each side; false with the sun down or too faint.
func (s Sun) ShadowOf(box geom.AABB, z world.Z, groundAt func(x, y float32) float32) (Patch, bool) {
	sx, sy, sz := s.Dir[0], s.Dir[1], s.Dir[2]
	veil := float32(uint8(shadowVeil*min(s.Strength/shadowFull, 1))) / 255
	if sz <= 0 || s.Strength <= 0 || veil <= 0 {
		return Patch{}, false
	}
	ground := float32(0)
	cx, cy := float32(box.TopLeft.X+box.BottomRight.X)/2, float32(box.TopLeft.Y+box.BottomRight.Y)/2
	if groundAt != nil {
		ground = groundAt(cx, cy)
	}
	half := float32(max(box.BottomRight.X-box.TopLeft.X, box.BottomRight.Y-box.TopLeft.Y)) / 2
	// away from the sun, and how far a unit of height casts its shadow that way
	ux, uy := float32(1), float32(0)
	across := float32(math.Hypot(float64(sx), float64(sy)))
	if across > 0 {
		ux, uy = -sx/across, -sy/across
	}
	reach := min(across/sz, maxShadowReach)
	above := max(float32(z.Altitude)-ground, 0)
	start, length := above*reach, float32(z.Height)*reach
	mid := start + length/2
	return Patch{X: cx + ux*mid, Y: cy + uy*mid, UX: ux, UY: uy, Along: length/2 + half, Wide: half, Fade: half / 2, Veil: veil}, true
}

// Shadow lays on f the shadow of an entity standing in box as z says (ShadowOf), its corners on
// the ground groundAt gives, at the depth of its nearest corner.
func (s Sun) Shadow(f *render.Frame, cam camera.Camera, box geom.AABB, z world.Z, groundAt func(x, y float32) float32) {
	p, ok := s.ShadowOf(box, z, groundAt)
	if !ok {
		return
	}
	ground := func(x, y float32) float32 {
		if groundAt == nil {
			return 0
		}
		return groundAt(x, y)
	}
	var dst render.Corners
	depth := float32(math.Inf(-1))
	for k, c := range [4][2]float32{{-p.Along, -p.Wide}, {p.Along, -p.Wide}, {-p.Along, p.Wide}, {p.Along, p.Wide}} {
		x, y := p.X+p.UX*c[0]-p.UY*c[1], p.Y+p.UY*c[0]+p.UX*c[1]
		g := ground(x, y)
		dst[k][0], dst[k][1] = cam.Project(x, y, g)
		depth = max(depth, cam.Depth(x, y, g))
	}
	cx, cy := float32(box.TopLeft.X+box.BottomRight.X)/2, float32(box.TopLeft.Y+box.BottomRight.Y)/2
	scale := camera.ScaleAt(cam, cx, cy, ground(cx, cy))
	if scale == 0 {
		return // not in front of the eye
	}
	fade := p.Fade * scale
	veil := color.RGBA{A: uint8(p.Veil*255 + 0.5)}
	f.Soft(ShadowTier, depth, dst, veil, render.Fade{Left: fade, Right: fade, Top: fade, Bottom: fade})
}
