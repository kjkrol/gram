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
	heapEdge      = 0.07
)

// The shreds are the same again every cloudTile clouds' widths, the heaps' cores every heapTile
// heaps' cells, four times as far, so a tile of each can be worked out once.
const (
	cloudTile = 30
	heapTile  = 16
)

// CloudTile is how wide a tile of the shreds is, in world units, HeapTile of the heaps' cores: the
// clouds' noise is the same again every HeapTile along x and along y.
const (
	CloudTile = cloudTile * cloudSize
	HeapTile  = heapTile * heapSize
)

// The heaps: how wide a cell of one is, in world units; how far a heap's middle strays from the
// cell's, in cells; the least and the most a heap's radius adds to it, in cells; how far in from
// its edge its top is flat; the most cover a heap waits for before it forms, so a few come under a
// light cover and more under a heavier one; how fast one grows out from its top as the cover
// grows; how much round lobes bulge from their edges; how much the shreds fray them; how much less
// than alone the shreds between them
// show; how much the two go together when mixed, the shreds being in both, twice their
// correlation.
const (
	heapSize  = 3150
	heapStray = 0.8
	heapLeast = 0.25
	heapMore  = 0.2
	heapFlat  = 0.25
	heapReady = 0.6
	heapSlope = 0.6
	heapLobes = 1.2
	heapBumps = 0.3
	heapFloor = 0.2
	heapTorn  = 0.4
)

// Cloud is the clouds' noise over the world point (x, y) with w's wind having carried them as far
// as its Drift says: 0 to 1, the clouds where it is high, torn shreds about a cloud's width
// gathered round big heaps as much as its Billow says — the very noise the shader works out per
// pixel (shaders/cloud_noise.wgsl's cloudField). Shade is how much shadow a value gives under w's
// cover.
func (w Weather) Cloud(x, y float32) float32 {
	b := float64(min(max(w.Billow, 0), 1))
	x, y = x-w.Drift[0], y-w.Drift[1]
	n := shreds(float64(x)/cloudSize, float64(y)/cloudSize)
	if b > 0 {
		n = (1-b)*n + b*heaps(heapCores(float64(x)/heapSize, float64(y)/heapSize), n)
	}
	return float32(0.5 + (n-0.5)/math.Sqrt((1-b)*(1-b)+b*b+heapTorn*b*(1-b))) // the two apart, spread as either alone
}

// shreds is torn clouds at (x, y), in clouds' widths: four octaves of value noise, 0 to 1, the
// same again every cloudTile widths.
func shreds(x, y float64) float64 {
	n := cloudNoise(x, y, cloudTile) + 0.5*cloudNoise(2*x+17.37, 2*y+17.61, 2*cloudTile) +
		0.25*cloudNoise(4*x+31.29, 4*y+31.83, 4*cloudTile) + 0.125*cloudNoise(8*x+53.51, 8*y+53.17, 8*cloudTile)
	return n / 1.875
}

// heapCores is how high the heaps' cores stand at (x, y), in heaps' cells: in each cell one heap
// kept well in from its edges, so the heaps stand far apart, its size and the cover it forms under
// thrown by the cell, its top flat, the highest over the point; the same again every heapTile
// cells. A heap comes whole as the cover reaches its own and grows as the cover grows on, its top
// whole, round lobes bulging from its edge.
func heapCores(x, y float64) float64 {
	cx, cy := math.Floor(x), math.Floor(y)
	top, inside := -10.0, 0.0
	for j := -1.0; j <= 1; j++ {
		for i := -1.0; i <= 1; i++ {
			px, py := cx+i, cy+j
			k := cloudThrow(wrap(px, heapTile), wrap(py, heapTile)) // one throw a cell, turned four ways
			dx := px + 0.5 + heapStray*(k/289-0.5) - x
			dy := py + 0.5 + heapStray*(mod289(37*k)/289-0.5) - y
			r := heapLeast + heapMore*mod289(53*k)/289
			in := min((1-math.Sqrt(dx*dx+dy*dy)/r)/heapFlat, 1)
			if t := 1 - heapReady*mod289(71*k)/289 + heapSlope*(in-1); t > top {
				top, inside = t, in
			}
		}
	}
	lobes := cloudNoise(4*x+0.37, 4*y+0.61, 4*heapTile) + 0.5*cloudNoise(8*x+0.53, 8*y+0.29, 8*heapTile) - 0.75
	return top + heapLobes*lobes*min(max(1-inside, 0), 1)
}

// heaps is billowing clouds of cores core (heapCores) over shreds torn there, 0 to 1: the shreds
// fraying their edges, and under a heavy cover the shreds between them.
func heaps(core, torn float64) float64 {
	return max(0.5+(core+heapBumps*(torn-0.5)-0.5)/cloudContrast, torn-heapFloor)
}

// wrap is v mod period, never negative.
func wrap(v, period float64) float64 { return v - period*math.Floor(v/period) }

// Shade is how thick the clouds are over a point whose noise is n (Cloud) under w's cover: 0 in
// the clear up to 1 under the thickest, heaps' edges sharper than shreds', as the shader shades it.
func (w Weather) Shade(n float32) float32 {
	if w.Clouds <= 0 {
		return 0
	}
	n = min(max((n-0.5)*cloudContrast+0.5, 0), 1)
	b := min(max(w.Billow, 0), 1)
	t := min(max((n-(1-w.Clouds))/(cloudEdge+(heapEdge-cloudEdge)*b), 0), 1)
	return t * t * (3 - 2*t)
}

// cloudNoise is smooth value noise at (x, y): 0 to 1, changing over a unit, the same again every
// period units.
func cloudNoise(x, y, period float64) float64 {
	x0, y0 := math.Floor(x), math.Floor(y)
	u, v := x-x0, y-y0
	u, v = u*u*(3-2*u), v*v*(3-2*v)
	x1, y1 := wrap(x0+1, period), wrap(y0+1, period)
	x0, y0 = wrap(x0, period), wrap(y0, period)
	a, b := cloudHash(x0, y0), cloudHash(x1, y0)
	c, d := cloudHash(x0, y1), cloudHash(x1, y1)
	return (a*(1-u)+b*u)*(1-v) + (c*(1-u)+d*u)*v
}

// cloudHash is a number 0 to 1 fixed for the lattice point (x, y), cloudThrow's over 289.
func cloudHash(x, y float64) float64 { return cloudThrow(x, y) / 289 }

// cloudThrow is a whole number under 289 fixed for the lattice point (x, y): a permutation
// polynomial mod 289 in whole numbers under 2²⁴, which floats hold exactly, so the shader
// (shaders/cloud_noise.wgsl) works out the very same; the lattice is shifted off the origin, where
// the polynomial is small.
func cloudThrow(x, y float64) float64 {
	x, y = mod289(x+17), mod289(y+53)
	p := mod289((34*x + 1) * x)
	return mod289((34*(p+y) + 1) * (p + y))
}

// mod289 is v mod 289, never negative, as the shader's mod has it.
func mod289(v float64) float64 { return v - 289*math.Floor(v/289) }
