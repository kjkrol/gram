package air

import (
	"math"

	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

// Overcast is the colour of a sky of colour sky under clouds covering this much of it, 0 to 1.
func Overcast(sky render.Light, clouds float32) render.Light {
	grey := (0.3*sky[0] + 0.5*sky[1] + 0.2*sky[2]) * overcastGrey
	k := min(max(clouds, 0), 1) * overcastSky
	return render.Light{sky[0] + (grey-sky[0])*k, sky[1] + (grey-sky[1])*k, sky[2] + (grey-sky[2])*k}
}

const (
	overcastGrey = 0.85
	overcastSky  = 0.8
)

// Overhead is the colour of the sky straight up, deeper than Overcast has it at the horizon.
func Overhead(sky render.Light, clouds float32) render.Light {
	deep := render.Light{sky[0] * 0.55, sky[1] * 0.7, sky[2] * 0.95}
	return Overcast(deep, clouds)
}

// CloudBase is how high the clouds hang, in metres.
const CloudBase = 6000.0

// Base is the height of the cloud layer on a world of scale, in world units.
func Base(scale world.Scale) float32 { return float32(scale.Units(CloudBase)) }

// cloudSize in world units; the shaders have the same numbers
const (
	cloudSize     = 420
	cloudContrast = 1.8
	cloudEdge     = 0.15
	heapEdge      = 0.07
)

// the noise repeats, so a tile of it can be baked once
const (
	cloudTile = 30
	heapTile  = 16
)

// CloudTile is how wide a tile of the shreds is, in world units, HeapTile of the heaps' cores;
// the clouds' noise is the same again every HeapTile along x and along y.
const (
	CloudTile = cloudTile * cloudSize
	HeapTile  = heapTile * heapSize
)

// heapSize in world units, the rest in heaps' cells or parts of the cover
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

// Cloud is the clouds' noise over the world point (x, y), heaped as its mass is, 0 to 1, the
// clouds where it is high: the very noise the shaders work out per pixel (cloudField).
func (w Weather) Cloud(x, y float32) float32 { return w.cloud(x, y, w.At(x, y).Billow) }

func (w Weather) cloud(x, y, billow float32) float32 {
	b := float64(min(max(billow, 0), 1))
	x, y = x-w.Drift[0], y-w.Drift[1]
	n := shreds(float64(x)/cloudSize, float64(y)/cloudSize)
	if b > 0 {
		n = (1-b)*n + b*heaps(heapCores(float64(x)/heapSize, float64(y)/heapSize), n)
	}
	return float32(0.5 + (n-0.5)/math.Sqrt((1-b)*(1-b)+b*b+heapTorn*b*(1-b)))
}

func shreds(x, y float64) float64 {
	n := cloudNoise(x, y, cloudTile) + 0.5*cloudNoise(2*x+17.37, 2*y+17.61, 2*cloudTile) +
		0.25*cloudNoise(4*x+31.29, 4*y+31.83, 4*cloudTile) + 0.125*cloudNoise(8*x+53.51, 8*y+53.17, 8*cloudTile)
	return n / 1.875
}

// one heap a cell, kept well in from its edges so the heaps stand apart
func heapCores(x, y float64) float64 {
	cx, cy := math.Floor(x), math.Floor(y)
	top, inside := -10.0, 0.0
	for j := -1.0; j <= 1; j++ {
		for i := -1.0; i <= 1; i++ {
			px, py := cx+i, cy+j
			k := cloudThrow(wrap(px, heapTile), wrap(py, heapTile))
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

func heaps(core, torn float64) float64 {
	return max(0.5+(core+heapBumps*(torn-0.5)-0.5)/cloudContrast, torn-heapFloor)
}

func wrap(v, period float64) float64 { return v - period*math.Floor(v/period) }

// Shade is how thick the clouds are over the world point (x, y), their noise (Cloud) cut at its
// mass's cover, 0 in the clear up to 1 under the thickest.
func (w Weather) Shade(x, y float32) float32 {
	m := w.At(x, y)
	return shade(w.cloud(x, y, m.Billow), m)
}

// FallsAt is the rain and the snow coming down at the world point (x, y), 0 to 1: its mass's,
// only as thick as the clouds over it let fall, none from their gaps.
func (w Weather) FallsAt(x, y float32) (rain, snow float32) {
	m := w.At(x, y)
	rain, snow = w.Falls(m)
	if rain+snow <= 0 {
		return 0, 0
	}
	t := min(shade(w.cloud(x, y, m.Billow), m)/fallUnder, 1)
	under := t * t * (3 - 2*t)
	return rain * under, snow * under
}

// how thick a cloud must be for all its rain or snow to fall: under a thinner one, less
const fallUnder = 0.35

// how thick the clouds of noise n are under the mass m, as cloudCover has it
func shade(n float32, m Mass) float32 {
	if m.Cover <= 0 {
		return 0
	}
	n = min(max((n-0.5)*cloudContrast+0.5, 0), 1)
	b := min(max(m.Billow, 0), 1)
	t := min(max((n-(1-m.Cover))/(cloudEdge+(heapEdge-cloudEdge)*b), 0), 1)
	return t * t * (3 - 2*t)
}

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

func cloudHash(x, y float64) float64 { return cloudThrow(x, y) / 289 }

// a permutation polynomial mod 289 in whole numbers under 2²⁴, which floats hold exactly, so the
// shader works out the very same; the lattice is shifted off the origin, where it is small
func cloudThrow(x, y float64) float64 {
	x, y = mod289(x+17), mod289(y+53)
	p := mod289((34*x + 1) * x)
	return mod289((34*(p+y) + 1) * (p + y))
}

func mod289(v float64) float64 { return v - 289*math.Floor(v/289) }
