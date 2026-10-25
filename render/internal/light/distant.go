package light

import (
	"math"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/internal/f32"
)

// Distant is a light from far off, the same everywhere: Dir points from the ground towards it (z
// up), Strength is how much it lights a surface facing it square on, Ambient how much light every
// surface gets anyway, in AmbientColor; a zero Color or AmbientColor is white.
type Distant struct {
	Dir          [3]float32
	Strength     float32
	Ambient      float32
	Color        Light
	AmbientColor Light
}

// Light is the light cast on a surface whose normal is (nx, ny, nz).
func (s Distant) Light(nx, ny, nz float32) Light { return s.Shaded(nx, ny, nz, 1) }

// Shaded is Light where only lit of the direct light, 0 to 1, reaches the surface.
func (s Distant) Shaded(nx, ny, nz, lit float32) Light {
	return s.Beam().Shaded(nx, ny, nz, lit)
}

// Beam is the light made ready to light many surfaces.
func (s Distant) Beam() Beam {
	var l Beam
	d := float32(math.Sqrt(float64(s.Dir[0]*s.Dir[0] + s.Dir[1]*s.Dir[1] + s.Dir[2]*s.Dir[2])))
	if d > 0 {
		l.dir = [3]float32{s.Dir[0] / d, s.Dir[1] / d, s.Dir[2] / d}
	}
	sun, sky := white(s.Color), white(s.AmbientColor)
	for c := range l.sun {
		l.sun[c], l.sky[c] = s.Strength*sun[c], s.Ambient*sky[c]
	}
	return l
}

// Beam is a Distant ready to light many surfaces.
type Beam struct {
	dir      [3]float32
	sun, sky Light
}

// Shaded is the light on a surface whose normal is (nx, ny, nz), lit of the light reaching it.
func (l Beam) Shaded(nx, ny, nz, lit float32) Light {
	direct := float32(0)
	if n := float32(math.Sqrt(float64(nx*nx + ny*ny + nz*nz))); n > 0 {
		direct = max((nx*l.dir[0]+ny*l.dir[1]+nz*l.dir[2])/n, 0) * lit
	}
	return Light{
		l.sky[0] + direct*l.sun[0],
		l.sky[1] + direct*l.sun[1],
		l.sky[2] + direct*l.sun[2],
	}
}

// AmbientLight is the colour of the ambient light, white for the zero one.
func (s Distant) AmbientLight() Light { return white(s.AmbientColor) }

// Frame hands f the light's uniforms (the library's light.wgsl).
func (s Distant) Frame(f Uniformer) {
	sky := white(s.AmbientColor)
	sun := white(s.Color)
	f.Uniform("LightWay", s.Dir[0], s.Dir[1], s.Dir[2])
	f.Uniform("LightStrength", s.Strength)
	f.Uniform("LightColor", sun[0], sun[1], sun[2])
	f.Uniform("AmbientColor", sky[0], sky[1], sky[2])
	f.Uniform("Ambience", s.Ambient*sky[0], s.Ambient*sky[1], s.Ambient*sky[2])
}

// a light on the horizon would cast a shadow for ever
const maxShadowReach = 6

const (
	shadowVeil = 110
	shadowFull = 0.6
)

// Umbra is where a shadow lies on the ground, in world units: its middle (X, Y), the way away
// from the light (UX, UY), its reach along that way (Along) and across it (Wide), how far in from
// each side it fades (Fade) and how dark it is (Veil, 0 to 1).
type Umbra struct{ X, Y, UX, UY, Along, Wide, Fade, Veil float32 }

// ShadowOf is the shadow of what stands in box from alt up height, on the ground groundAt gives
// (nil: level at 0). It is false with the light below the horizon or too faint.
func (s Distant) ShadowOf(box geom.AABB, alt, height float32,
	groundAt func(x, y float32) float32) (Umbra, bool) {
	sx, sy, sz := s.Dir[0], s.Dir[1], s.Dir[2]
	veil := float32(uint8(shadowVeil*min(s.Strength/shadowFull, 1))) / 255
	if sz <= 0 || s.Strength <= 0 || veil <= 0 {
		return Umbra{}, false
	}
	ground := float32(0)
	cx, cy := (box.TopLeft.X+box.BottomRight.X)/2, (box.TopLeft.Y+box.BottomRight.Y)/2
	if groundAt != nil {
		ground = groundAt(cx, cy)
	}
	half := max(box.BottomRight.X-box.TopLeft.X, box.BottomRight.Y-box.TopLeft.Y) / 2
	ux, uy := float32(1), float32(0)
	across := f32.Hypot(sx, sy)
	if across > 0 {
		ux, uy = -sx/across, -sy/across
	}
	reach := min(across/sz, maxShadowReach)
	above := max(alt-ground, 0)
	start, length := above*reach, height*reach
	mid := start + length/2
	return Umbra{
		X:     cx + ux*mid,
		Y:     cy + uy*mid,
		UX:    ux,
		UY:    uy,
		Along: length/2 + half,
		Wide:  half,
		Fade:  half / 2,
		Veil:  veil,
	}, true
}

// SilhouetteOf is where the drawn shape of what stands in box from alt up height lies on the
// ground groundAt gives (nil: level at 0): from its feet away from the light, as long as it
// stands tall there, as wide as its box across, unfaded. It is false with the light down or too
// faint.
func (s Distant) SilhouetteOf(box geom.AABB, alt, height float32,
	groundAt func(x, y float32) float32) (Umbra, bool) {
	p, ok := s.ShadowOf(box, alt, height, groundAt)
	if !ok {
		return Umbra{}, false
	}
	sx, sy, sz := s.Dir[0], s.Dir[1], s.Dir[2]
	reach := min(f32.Hypot(sx, sy)/sz, maxShadowReach)
	cx, cy := (box.TopLeft.X+box.BottomRight.X)/2, (box.TopLeft.Y+box.BottomRight.Y)/2
	ground := float32(0)
	if groundAt != nil {
		ground = groundAt(cx, cy)
	}
	start := max(alt-ground, 0) * reach
	length := max(height*reach, 1e-3)
	mid := start + length/2
	p.X, p.Y = cx+p.UX*mid, cy+p.UY*mid
	p.Along, p.Wide, p.Fade = length/2, (box.BottomRight.X-box.TopLeft.X)/2, 0
	return p, true
}
