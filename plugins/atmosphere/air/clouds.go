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

// CloudBase is how high the clouds hang, in metres: well over the highest ground of an island and
// the eye flying over it, so they are seen from below and look far.
const CloudBase = 6000.0

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
// as its Drift says: 0 to 1, smooth over about a cloud's width, the clouds where it is high — the
// very noise the shader works out per pixel (shaders/cloud_noise.wgsl's cloudField), so what is read here of
// a piece's corners, to tell a clear piece from a clouded one, is what the pixels get. Shade is
// how much shadow a value gives under w's cover.
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

// cloudNoise is smooth value noise at (x, y): 0 to 1, changing over a unit.
func cloudNoise(x, y float64) float64 {
	x0, y0 := math.Floor(x), math.Floor(y)
	u, v := x-x0, y-y0
	u, v = u*u*(3-2*u), v*v*(3-2*v)
	a, b := cloudHash(x0, y0), cloudHash(x0+1, y0)
	c, d := cloudHash(x0, y0+1), cloudHash(x0+1, y0+1)
	return (a*(1-u)+b*u)*(1-v) + (c*(1-u)+d*u)*v
}

// cloudHash is a number 0 to 1 fixed for the lattice point (x, y): a permutation polynomial mod
// 289 in whole numbers under 2²⁴, which floats hold exactly, so the shader (shaders/cloud_noise.wgsl) works
// out the very same; the lattice is shifted off the origin, where the polynomial is small.
func cloudHash(x, y float64) float64 {
	x, y = mod289(x+17), mod289(y+53)
	p := mod289((34*x + 1) * x)
	return mod289((34*(p+y)+1)*(p+y)) / 289
}

// mod289 is v mod 289, never negative, as the shader's mod has it.
func mod289(v float64) float64 { return v - 289*math.Floor(v/289) }
