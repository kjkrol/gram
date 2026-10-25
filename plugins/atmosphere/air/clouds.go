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

// Layer is a layer of clouds: Low the heaps, High the torn shreds over them.
type Layer uint8

const (
	Low Layer = iota
	High
)

// CloudLow is how high the low clouds hang, in metres, CloudBase the high ones.
const (
	CloudLow  = 1500.0
	CloudBase = 6000.0
)

// Bases is how high each Layer of clouds hangs on a world of scale, in world units, over no
// ground in particular.
func Bases(scale world.Scale) [2]float32 {
	return [2]float32{scale.Units(CloudLow), scale.Units(CloudBase)}
}

// LeastRelief is the least relief the low clouds hang over, in metres: lower hills are taken as
// this high, so they do not stand up into the clouds.
const LeastRelief = 1000.0

// ReliefOf is the ground from low up to its peaks, metres over the world's zero in world units
// on a world of scale, as Weather.Relief has it: where it begins and how high it rises.
func ReliefOf(scale world.Scale, low, peaks float32) [2]float32 {
	return [2]float32{low, max(peaks-low, scale.Units(LeastRelief))}
}

// BasesOver is how high each Layer of clouds hangs over the relief (ReliefOf) on a world of
// scale, as Weather.Bases has it: the low ones' lowest base, the high ones over their tops.
func BasesOver(scale world.Scale, relief [2]float32) [2]float32 {
	floor := relief[0] + relief[1]*bandOver[1]
	high := max(scale.Units(CloudBase), floor+scale.Units(highOverLow))
	return [2]float32{relief[0] + relief[1]*bandUnder[0], high}
}

// metres the high clouds' base stands at least over the low ones' decks
const highOverLow = 2500.0

// Base is how high the layer l of clouds hangs, world units: the low ones' lowest base.
func (w Weather) Base(l Layer) float32 {
	if w.OneLayer {
		return w.Bases[High]
	}
	return w.Bases[l]
}

// Top is how high the layer l of clouds reaches, world units: the heaps' lumps, the shreds' band.
func (w Weather) Top(l Layer) float32 {
	if l == High && !w.OneLayer {
		return w.Bases[High]
	}
	if w.OneLayer || w.Relief[1] <= 0 {
		return w.Base(l) + lowRise*lowDeep*cloudSize
	}
	return w.Relief[0] + w.Relief[1]*(bandOver[0]+lowRise*(bandOver[1]-bandOver[0]))
}

// BaseSpan is how low and how high the bases of the layer l of clouds hang, world units.
func (w Weather) BaseSpan(l Layer) (lowest, highest float32) {
	if l == High || w.OneLayer || w.Relief[1] <= 0 {
		return w.Base(l), w.Base(l)
	}
	return w.Base(Low), w.Relief[0] + w.Relief[1]*bandOver[0]
}

// LowBand is how high the low clouds hang over the world point (x, y), world units: their base and
// the top of their deck, the heaps' lumps rising past it. Each of the three bands of Ceilings
// holds places of its own over the Relief, drifting slower than the clouds, so a cloud flying on
// rises or sinks slowly.
func (w Weather) LowBand(x, y float32) (base, top float32) {
	if w.OneLayer || w.Relief[1] <= 0 {
		base = w.Base(Low)
		return base, base + lowDeep*cloudSize
	}
	q := float64(x)/ceilingWide - float64(ceilingLag*w.Drift[0])/ceilingWide + ceilingOff
	r := float64(y)/ceilingWide - float64(ceilingLag*w.Drift[1])/ceilingWide + ceilingOff
	n := float32(cloudNoise(q, r, CloudTile/ceilingWide))
	u := clamp01((n-0.5)*ceilingSpread + 0.5)
	c0, c1 := w.cuts()
	a, b := ceilingStep(c0, u), ceilingStep(c1, u)
	under, cover, over := 1-a, max(a-b, 0), b
	lo := under*bandUnder[0] + cover*bandCover[0] + over*bandOver[0]
	hi := under*bandUnder[1] + cover*bandCover[1] + over*bandOver[1]
	return w.Relief[0] + w.Relief[1]*lo, w.Relief[0] + w.Relief[1]*hi
}

// where along the ceilings' noise the clouds under the peaks give way to those covering them,
// and those to the ones over them, as the shaders' CloudCeilings
func (w Weather) cuts() (c0, c1 float32) {
	s := w.Ceilings
	sum := s[0] + s[1] + s[2]
	if sum <= 0 {
		return 1.0 / 3, 2.0 / 3
	}
	return s[0] / sum, (s[0] + s[1]) / sum
}

// how far past the cut c the ceilings' noise u stands, 0 to 1, a cut at either end hard
func ceilingStep(c, u float32) float32 {
	switch {
	case c <= 0:
		return 1
	case c >= 1:
		return 0
	}
	soft := min(ceilingSoft, c, 1-c)
	t := clamp01((u - c + soft) / (2 * soft))
	return t * t * (3 - 2*t)
}

// the three bands the low clouds hang in, their base and their deck's top as shares of the
// relief: under the peaks, covering them, over them; how wide the places of one band are, world
// units, a tile holding a whole number of them; how fast they drift as against the clouds; how
// far their noise is spread and how softly one band gives way to the next; the shaders have the
// same
var (
	bandUnder = [2]float32{0.68, 0.79}
	bandCover = [2]float32{0.74, 1.05}
	bandOver  = [2]float32{1.05, 1.25}
)

const (
	ceilingWide   = 2520
	ceilingLag    = 0.6
	ceilingOff    = 29
	ceilingSpread = 1.8
	ceilingSoft   = 0.08
)

// Share is the share of m the layer l of clouds holds (Mass.Layer); in one layer, all of it Low's.
func (w Weather) Share(m Mass, l Layer) Mass {
	switch {
	case !w.OneLayer:
		return m.Layer(l)
	case l == Low:
		return m
	}
	return Mass{}
}

// Layer is the share of m the layer l of clouds holds: the heaps the more the sky is covered and
// the more heaped, the shreds there with a few clouds already and the fewer the more heaped, in
// patches (Weather.HighAt), the two together or apart.
func (m Mass) Layer(l Layer) Mass {
	cover, billow := min(max(m.Cover, 0), 1), min(max(m.Billow, 0), 1)
	if l == Low {
		m.Cover, m.Billow = cover*(lowLeast+(1-lowLeast)*billow), 1
		return m
	}
	m.Cover, m.Billow = min(highEarly*cover, 1)*(1-highHeaped*billow), 0
	return m
}

// WithLow is m with low the cover of its low layer (Layer), its high layer kept as far as the two
// can stand together: a heap holds some shreds above it, and heaps grow heaped.
func (m Mass) WithLow(low float32) Mass {
	low = clamp01(low)
	if low <= 0 {
		m.Cover = 0
		return m
	}
	const a, e, h = lowLeast, highEarly, highHeaped
	high := m.Layer(High).Cover
	cover := func(billow float32) float32 { return low / (a + (1-a)*billow) }
	// under 1/e of cover the shreds grow with it: both shares solved together
	b := clamp01((e*low - high*a) / max(high*(1-a)+e*low*h, 1e-6))
	if c := cover(b); c < 1.0/e {
		m.Cover, m.Billow = c, b
		return m
	}
	b = clamp01((1 - high) / h)
	c := cover(b)
	if c > 1 {
		c, b = 1, clamp01((low-a)/(1-a))
	}
	m.Cover, m.Billow = c, b
	return m
}

// how much of the cover the low clouds hold, unheaped, all of it heaped; how soon the high ones
// come, a few clouds already holding all of them, and how many fewer the more heaped; the shaders
// have the same
const (
	lowLeast   = 0.35
	highEarly  = 2
	highHeaped = 0.5
)

// HighAt is the high clouds' mass m (Mass.Layer) at the world point (x, y): there in patches,
// none between.
func (w Weather) HighAt(m Mass, x, y float32) Mass {
	q := float64(x-w.Drift[0])/patchWide + 17
	r := float64(y-w.Drift[1])/patchWide + 17
	t := min(max((cloudNoise(q, r, CloudTile/patchWide)-0.3)/0.4, 0), 1)
	m.Cover *= float32(t * t * (3 - 2*t))
	return m
}

// HighHeight is how high the high clouds hang over the world point (x, y), world units: each
// sheet of shreds at its own height through their band, from into the low clouds' bodies up to
// their Base.
func (w Weather) HighHeight(x, y float32) float32 {
	n := float32(cloudNoise(float64(x-w.Drift[0])/bandWide, float64(y-w.Drift[1])/bandWide,
		CloudTile/bandWide))
	floor := w.highFloor()
	return floor + (w.Bases[High]-floor)*n
}

// where the high clouds' band begins: into the low clouds' bodies, or over their highest deck
func (w Weather) highFloor() float32 {
	if w.Relief[1] <= 0 {
		return w.Bases[Low] + overlap*lowDeep*cloudSize
	}
	return w.Relief[0] + w.Relief[1]*bandOver[1]
}

// how wide the high clouds' patches and the sheets of one height are, world units, a tile holding
// a whole number of each; how deep the heaps' bodies stand, in clouds' widths, and how far into
// them the high clouds' band begins; the shaders have the same
const (
	patchWide = 2520
	bandWide  = 1260
	lowDeep   = 0.45
	lowRise   = 1.6
	overlap   = 0.6
)

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
	heapSize  = 787.5
	heapStray = 0.8
	// the ellipse up to so much longer than wide, the puffs on its rim, how big of its width and
	// how far out
	heapStretch   = 1.2
	heapPuffs     = 5
	heapPuffLeast = 0.35
	heapPuffMore  = 0.5
	heapPuffOut   = 0.8
	heapLeast     = 0.25
	heapMore      = 0.2
	heapFlat      = 0.25
	heapReady     = 0.6
	heapSlope     = 0.6
	heapLobes     = 1.2
	heapBumps     = 0.3
	heapFloor     = 0.2
	heapTorn      = 0.4
)

// Cloud is the noise of the layer l of clouds over the world point (x, y), 0 to 1, the clouds
// where it is high: the heaps' or the shreds', which the shaders look up in their Tile; in one
// layer, both mixed as heaped as the mass there is.
func (w Weather) Cloud(l Layer, x, y float32) float32 {
	return w.cloud(l, x, y, w.Share(w.At(x, y), l).Billow)
}

func (w Weather) cloud(l Layer, x, y, billow float32) float32 {
	torn, heap := w.noise(x, y, l == Low && billow > 0)
	if l == High {
		return torn
	}
	return mix(torn, heap, billow)
}

// the shreds' noise and the heaps' mixed as billow says, spread apart as either alone, as
// cloudMix has it
func mix(torn, heap, billow float32) float32 {
	b := float64(clamp01(billow))
	if b <= 0 {
		return torn
	}
	n := (1-b)*float64(torn) + b*float64(heap)
	return clamp01(float32(0.5 + (n-0.5)/math.Sqrt((1-b)*(1-b)+b*b+heapTorn*b*(1-b))))
}

// the shreds' noise over (x, y) and, if heaped, the heaps' grown out of them
func (w Weather) noise(x, y float32, heaped bool) (torn, heap float32) {
	x, y = x-w.Drift[0], y-w.Drift[1]
	n := shreds(float64(x)/cloudSize, float64(y)/cloudSize)
	if heaped {
		heap = clamp01(float32(heaps(heapCores(float64(x)/heapSize, float64(y)/heapSize), n)))
	}
	return clamp01(float32(n)), heap
}

func shreds(x, y float64) float64 {
	n := cloudNoise(x, y, cloudTile) + 0.5*cloudNoise(2*x+17.37, 2*y+17.61, 2*cloudTile) +
		0.25*cloudNoise(4*x+31.29, 4*y+31.83, 4*cloudTile) + 0.125*cloudNoise(8*x+53.51, 8*y+53.17, 8*cloudTile)
	return n / 1.875
}

// one heap a cell, kept well in from its edges so the heaps stand apart: an ellipse with puffs on
// its rim, as the shaders' heapCores
func heapCores(x, y float64) float64 {
	cx, cy := math.Floor(x), math.Floor(y)
	top, inside := -10.0, 0.0
	for j := -1.0; j <= 1; j++ {
		for i := -1.0; i <= 1; i++ {
			px, py := cx+i, cy+j
			k := cloudThrow(wrap(px, heapTile), wrap(py, heapTile))
			ox := x - (px + 0.5 + heapStray*(heapRoll(k, 1)-0.5))
			oy := y - (py + 0.5 + heapStray*(heapRoll(k, 37)-0.5))
			wide := heapLeast + heapMore*heapRoll(k, 53)
			narrow := wide / (1 + heapStretch*heapRoll(k, 59))
			wy, wx := math.Sincos(math.Pi * heapRoll(k, 61))
			along, across := (ox*wx+oy*wy)/wide, (oy*wx-ox*wy)/narrow
			in := 1 - math.Sqrt(along*along+across*across)
			for l := range heapPuffs {
				sr, cr := math.Sincos(2 * math.Pi * heapRoll(k, 67+7*float64(l)))
				puff := narrow * (heapPuffLeast + heapPuffMore*heapRoll(k, 73+7*float64(l)))
				rx := heapPuffOut * (cr*wide*wx - sr*narrow*wy)
				ry := heapPuffOut * (cr*wide*wy + sr*narrow*wx)
				in = max(in, 1-math.Hypot(ox-rx, oy-ry)/puff)
			}
			in = min(in/heapFlat, 1)
			top = max(top, 1-heapReady*heapRoll(k, 71)+heapSlope*(in-1))
			inside = max(inside, in) // the deepest in any heap, as the shaders'
		}
	}
	lobes := cloudNoise(4*x+0.37, 4*y+0.61, 4*heapTile) + 0.5*cloudNoise(8*x+0.53, 8*y+0.29, 8*heapTile) - 0.75
	return top + heapLobes*lobes*min(max(1-inside, 0), 1)
}

func heapRoll(k, m float64) float64 { return mod289(m*k) / 289 }

func heaps(core, torn float64) float64 {
	return max(0.5+(core+heapBumps*(torn-0.5)-0.5)/cloudContrast, torn-heapFloor)
}

func wrap(v, period float64) float64 { return v - period*math.Floor(v/period) }

// Shade is how much of the sun the clouds take at the world point (x, y, z) under the sun at sun,
// 0 in the clear up to 1 under the thickest: each layer where the way to the sun crosses it, in
// one layer straight over the point, the high one only with HighShade.
func (w Weather) Shade(x, y, z float32, sun [3]float32) float32 {
	var ax, ay float32 // across a world unit up the way to the sun
	n := float32(math.Sqrt(float64(sun[0]*sun[0] + sun[1]*sun[1] + sun[2]*sun[2])))
	if n > 0 && !w.OneLayer {
		slant := max(sun[2]/n, lowSun) * n
		ax, ay = sun[0]/slant, sun[1]/slant
	}
	clear := float32(1)
	for _, l := range []Layer{Low, High} {
		var h float32
		if l == High {
			h = w.HighHeight(x, y) - z // the sheet's over the point, near enough
		} else {
			// the base over the point, then over where the way to the sun meets it
			base, _ := w.LowBand(x, y)
			if base > z {
				base, _ = w.LowBand(x+ax*(base-z), y+ay*(base-z))
			}
			h = base - z
		}
		if h <= 0 || l == High && !w.HighShade {
			continue // the ground over the layer, or high clouds shading nothing
		}
		clear *= 1 - w.cover(l, x+ax*h, y+ay*h)
	}
	return 1 - clear
}

// the sun lower than this casts the clouds' shadows as far as at it
const lowSun = 0.25

// how thick the clouds of the layer l are over the world point (x, y)
func (w Weather) cover(l Layer, x, y float32) float32 {
	m := w.Share(w.At(x, y), l)
	if l == High {
		m = w.HighAt(m, x, y)
	}
	if m.Cover <= 0 {
		return 0
	}
	return shade(w.cloud(l, x, y, m.Billow), m)
}

// FallsAt is the rain and the snow coming down at the world point (x, y), 0 to 1: its mass's,
// only as thick as the low clouds over it let fall, none from their gaps nor from the high ones.
func (w Weather) FallsAt(x, y float32) (rain, snow float32) {
	m := w.At(x, y)
	rain, snow = w.Falls(m)
	if rain+snow <= 0 {
		return 0, 0
	}
	low := w.Share(m, Low)
	if low.Cover <= 0 {
		return 0, 0
	}
	torn, heap := w.noise(x, y, low.Billow > 0)
	t := min(shade(mix(torn, heap, low.Billow), low)/fallUnder, 1)
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
