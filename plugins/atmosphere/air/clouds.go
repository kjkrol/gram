package air

import (
	"math"

	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

// Overcast is the colour of a sky of colour sky under clouds covering this much of it, 0 to 1:
// greyer and darker the more it is covered. The composer's shader sees the sky water reflects the
// same way (overcastSky).
func Overcast(sky render.Light, clouds float32) render.Light {
	grey := (0.3*sky[0] + 0.5*sky[1] + 0.2*sky[2]) * overcastGrey
	k := min(max(clouds, 0), 1) * overcastSky
	return render.Light{sky[0] + (grey-sky[0])*k, sky[1] + (grey-sky[1])*k, sky[2] + (grey-sky[2])*k}
}

// How clouds grey the sky: how bright its grey is to the clear sky's brightness, and how far a sky
// all covered goes to that grey.
const (
	overcastGrey = 0.85
	overcastSky  = 0.8
)

// Overhead is the colour of the sky straight up, deeper than at the horizon, where Overcast has it
// pale with the air between: the sky's blue kept, its red and green held back.
func Overhead(sky render.Light, clouds float32) render.Light {
	deep := render.Light{sky[0] * 0.55, sky[1] * 0.7, sky[2] * 0.95}
	return Overcast(deep, clouds)
}

// CloudBase is how high the clouds hang, in metres: over the highest ground of an island and the
// eye flying over it, so they are seen from below.
const CloudBase = 3000.0

// Base is the height of the cloud layer on a world of scale, in world units.
func Base(scale world.Scale) float32 { return float32(scale.Units(CloudBase)) }

// The clouds: how wide one is, in world units; how far their noise is spread to make clouds and
// clear sky between them; how much of a cloud's edge the shadow softens over, as a part of the
// cover. The shader's CloudShadow uses the same.
const (
	cloudSize     = 420
	cloudContrast = 1.8
	cloudEdge     = 0.15
)

// Cloud is the clouds' noise over the world point (x, y) with w's wind having carried them as far
// as its Drift says: 0 to 1, smooth over about a cloud's width, the clouds where it is high. What
// lays their shadows takes it at a piece's corners and the shader shades between them, so no pixel
// works the noise out itself; Shade is how much shadow a value gives under w's cover.
func (w Weather) Cloud(x, y float32) float32 {
	qx, qy := float64(x-w.Drift[0])/cloudSize, float64(y-w.Drift[1])/cloudSize
	n := cloudNoise(qx, qy) + 0.5*cloudNoise(qx*2.03+17, qy*2.03+17) + 0.25*cloudNoise(qx*4.01+31, qy*4.01+31) + 0.125*cloudNoise(qx*8.07+53, qy*8.07+53)
	return float32(n / 1.875)
}

// Shade is how thick the clouds are over a point whose noise is n (Cloud) under w's cover: 0 in
// the clear up to 1 under the thickest, as the shader shades it.
func (w Weather) Shade(n float32) float32 {
	if w.Clouds <= 0 {
		return 0
	}
	n = min(max((n-0.5)*cloudContrast+0.5, 0), 1)
	t := min(max((n-(1-w.Clouds))/cloudEdge, 0), 1)
	return t * t * (3 - 2*t)
}

// Shadowed reports whether clouds of noise cloud at a piece's corners shade any of it under w's
// cover: the shader shades between the corners, so a piece whose corners are all clear is clear.
func (w Weather) Shadowed(cloud [4]float32) bool {
	if w.Clouds <= 0 {
		return false
	}
	for _, n := range cloud {
		if w.Shade(n) > 0 {
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

// Overcast lays over the last sprite added to f, whose corners lie at wo in the world under clouds
// of noise cloud (Cloud at each corner), the shadows of w's clouds — as faint as the sprite and
// fading or blending as it does; nothing under a clear sky, nor over a piece the clouds miss.
func (w Weather) Overcast(f *render.Frame, wo render.World, cloud [4]float32) {
	if !w.Shadowed(cloud) {
		return
	}
	f.Overlay(&render.Overlay{Material: cloudShadow, World: wo, Fraction: cloud, Under: true})
}

// OvercastOn is Overcast over the sprite m drawn earlier, and over all drawn on it since.
func (w Weather) OvercastOn(f *render.Frame, m render.Mark, wo render.World, cloud [4]float32) {
	if !w.Shadowed(cloud) {
		return
	}
	f.OverlayOn(m, &render.Overlay{Material: cloudShadow, World: wo, Fraction: cloud, Under: true})
}

// OvercastQuad lays the clouds' shadows on their own over the screen quad dst, whose corners lie
// at wo in the world under clouds of noise cloud: a flat world's, laid over the screen piece by
// piece rather than tile by tile.
func (w Weather) OvercastQuad(f *render.Frame, tier render.Tier, depth float32, dst render.Corners, wo render.World, cloud [4]float32) {
	if !w.Shadowed(cloud) {
		return
	}
	o := render.Overlay{Material: cloudShadow, World: wo, Red: [4]float32{1, 1, 1, 1}, Fraction: cloud}
	f.Material(tier, depth, dst, &o)
}

// CloudQuad draws the clouds themselves on the screen quad dst, a piece of the sky whose corners
// look at the points wo of the cloud layer under clouds of noise cloud (Cloud at each corner) —
// the same noise that lays their shadows straight under them — hazed as far as haze says each
// corner lies in the air; nothing over a piece the clouds miss.
func (w Weather) CloudQuad(f *render.Frame, tier render.Tier, depth float32, dst render.Corners, wo render.World, cloud, haze [4]float32) {
	if !w.Shadowed(cloud) {
		return
	}
	o := render.Overlay{Material: cloudsOverhead, World: wo, Red: [4]float32{1, 1, 1, 1}, Fraction: cloud}
	for k := range haze {
		o.Custom[k][0] = haze[k]
	}
	f.Material(tier, depth, dst, &o)
}
