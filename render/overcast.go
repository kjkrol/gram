package render

import (
	_ "embed"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
)

//go:embed overcast.kage
var overcastKage []byte

// cloudShadow is the material of the clouds' shadows over the ground (overcast.kage), the first
// registered: the water's materials call its functions.
var cloudShadow = RegisterMaterials(overcastKage, "CloudShadow")[0]

// CloudShadow is the material of the clouds' shadows, for a test telling its overlays apart.
func CloudShadow() MaterialID { return cloudShadow }

// Overcast is the colour of a sky of colour sky under clouds covering this much of it, 0 to 1:
// greyer and darker the more it is covered. The composer's shader sees the sky water reflects the
// same way.
func Overcast(sky Light, clouds float32) Light {
	grey := (0.3*sky[0] + 0.5*sky[1] + 0.2*sky[2]) * overcastGrey
	k := min(max(clouds, 0), 1) * overcastSky
	return Light{sky[0] + (grey-sky[0])*k, sky[1] + (grey-sky[1])*k, sky[2] + (grey-sky[2])*k}
}

// How clouds grey the sky: how bright its grey is to the clear sky's brightness, and how far a sky
// all covered goes to that grey.
const (
	overcastGrey = 0.85
	overcastSky  = 0.8
)

// The clouds: how wide one is, in world units; how far their noise is spread to make clouds and
// clear sky between them; how much of a cloud's edge the shadow softens over, as a part of the
// cover. The shader's CloudShadow uses the same.
const (
	cloudSize     = 420
	cloudContrast = 1.8
	cloudEdge     = 0.15
)

// CloudAt is the clouds' noise over the world point (x, y) with the wind having carried them drift:
// 0 to 1, smooth over about a cloud's width, the clouds where it is high. A Frame's overlays take
// it at their corners (Overcast, OvercastOn, OvercastQuad) and the shader shades between them, so
// no pixel works the noise out itself; CloudCover is how much shadow a value gives under a cover.
func CloudAt(x, y float32, drift [2]float32) float32 {
	qx, qy := float64(x-drift[0])/cloudSize, float64(y-drift[1])/cloudSize
	n := cloudNoise(qx, qy) + 0.5*cloudNoise(qx*2.03+17, qy*2.03+17) + 0.25*cloudNoise(qx*4.01+31, qy*4.01+31) + 0.125*cloudNoise(qx*8.07+53, qy*8.07+53)
	return float32(n / 1.875)
}

// CloudCover is how thick the clouds are over a point whose noise is n (CloudAt) under clouds
// covering cover of the sky: 0 in the clear up to 1 under the thickest, as the shader shades it.
func CloudCover(n, cover float32) float32 {
	if cover <= 0 {
		return 0
	}
	n = min(max((n-0.5)*cloudContrast+0.5, 0), 1)
	t := min(max((n-(1-cover))/cloudEdge, 0), 1)
	return t * t * (3 - 2*t)
}

// Shadowed reports whether clouds of noise cloud at a piece's corners shade any of it under cover:
// the shader shades between the corners, so a piece whose corners are all clear is clear.
func Shadowed(cloud [4]float32, cover float32) bool {
	for _, n := range cloud {
		if CloudCover(n, cover) > 0 {
			return true
		}
	}
	return false
}

// cloudNoise is smooth value noise at (x, y): 0 to 1, changing over a unit.
func cloudNoise(x, y float64) float64 {
	x0, y0 := math.Floor(x), math.Floor(y)
	u, v := x-x0, y-y0
	u, v = u*u*(3-2*u), v*v*(3-2*v)
	ix, iy := int64(x0), int64(y0)
	a, b := cloudHash(ix, iy), cloudHash(ix+1, iy)
	c, d := cloudHash(ix, iy+1), cloudHash(ix+1, iy+1)
	return (a*(1-u)+b*u)*(1-v) + (c*(1-u)+d*u)*v
}

// cloudHash is a number 0 to 1 fixed for the lattice point (x, y).
func cloudHash(x, y int64) float64 {
	h := uint64(x)*0x9E3779B97F4A7C15 ^ uint64(y)*0xC2B2AE3D27D4EB4F ^ 0x2545F4914F6CDD1D
	h ^= h >> 31
	h *= 0xBF58476D1CE4E5B9
	h ^= h >> 29
	return float64(h>>11) / (1 << 53)
}

// Overcast lays over the last sprite added to f, whose corners lie at w in the world under clouds
// of noise cloud (CloudAt at each corner), the shadows of the clouds of the frame's weather — as
// faint as the sprite and fading or blending as it does; nothing under a clear sky, nor over a
// piece the clouds miss.
func (f *Frame) Overcast(w World, cloud [4]float32) {
	if !f.overcast(cloud) {
		return
	}
	f.Overlay(&Overlay{Material: cloudShadow, World: w, Fraction: cloud, Under: true})
}

// OvercastOn is Overcast over the sprite m drawn earlier, and over all drawn on it since.
func (f *Frame) OvercastOn(m Mark, w World, cloud [4]float32) {
	if !f.overcast(cloud) {
		return
	}
	f.OverlayOn(m, &Overlay{Material: cloudShadow, World: w, Fraction: cloud, Under: true})
}

// OvercastQuad lays the clouds' shadows on their own over the screen quad dst, whose corners lie
// at w in the world under clouds of noise cloud: a flat world's, laid over the screen piece by
// piece rather than tile by tile.
func (f *Frame) OvercastQuad(tier Tier, depth float32, dst Corners, w World, cloud [4]float32) {
	if !f.overcast(cloud) {
		return
	}
	o := Overlay{Material: cloudShadow, World: w, Red: [4]float32{1, 1, 1, 1}, Fraction: cloud, Custom: fadeCustoms(dst, Fade{})}
	f.Material(tier, depth, dst, &o)
}

// overcast reports whether a piece under clouds of noise cloud needs the shadows laid this frame.
func (f *Frame) overcast(cloud [4]float32) bool {
	return f.Clouds() > 0 && Shadowed(cloud, f.Clouds())
}

// Material lays o on its own, over nothing: the screen quad dst for o's material to work out,
// with its Red, Fraction and Custom at each corner, on the frame's white texel.
func (f *Frame) Material(tier Tier, depth float32, dst Corners, o *Overlay) {
	w := o.World
	for k, p := range dst {
		c := o.Custom[k]
		f.verts = append(f.verts, ebiten.Vertex{DstX: p[0], DstY: p[1], ColorR: o.Red[k], ColorG: w[k][0], ColorB: w[k][1],
			ColorA: overlayMark + 2*float32(o.Material) + min(max(o.Fraction[k], 0), 1), Custom0: c[0], Custom1: c[1], Custom2: c[2], Custom3: c[3]})
	}
	f.add(tier, depth, nil, quad, 4)
}

// Screen is the Corners of a w by h viewport, and the World under them as cam sees the ground.
func Screen(cam interface {
	Unproject(sx, sy, z float32) (float32, float32)
}, w, h float32) (Corners, World) {
	dst := Corners{{0, 0}, {w, 0}, {0, h}, {w, h}}
	var world World
	for k, p := range dst {
		world[k][0], world[k][1] = cam.Unproject(p[0], p[1], 0)
	}
	return dst, world
}
