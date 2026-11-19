package world

import (
	"cmp"
	"fmt"
	"image/color"
	"math"
	"slices"

	"github.com/kjkrol/gram/internal/f32"
)

// Shape is what a Part of a Body is.
type Shape uint8

const (
	// Spheroid is a ball squashed or drawn out along the line from a Part's From to its To, Wide
	// across in its middle: a head, a globe, a mound.
	Spheroid Shape = iota + 1
	// Cone is one cut short, Wide across at a Part's From and Tip across at its To: a post,
	// an arm, a skirt.
	Cone
)

// Body is a kind's shape: its Parts in world units off the middle of its foot, X across, Y
// deep and Z up, within a box Wide a side and Tall high, each in one of its Paints. A view in
// relief stands it whole near the camera and as a card of it further off (Atlas.AddBody).
type Body struct {
	Wide   float32
	Tall   float32
	Parts  []Part
	Paints []Paint
}

// Part is a piece of a Body from From to To, as its Shape says, in the Paint of that number of
// its Body's.
type Part struct {
	Shape Shape
	From  [3]float32
	To    [3]float32
	Wide  float32
	Tip   float32
	Paint int
}

// Paint is what a Part is painted in: its Colour, dotted with Lit and with Dots where it has
// them, specks of it catching the light and specks of what else it carries.
type Paint struct {
	Colour color.RGBA
	Lit    color.RGBA
	Dots   color.RGBA
}

// The most Parts and Paints a Body holds, and how many sides it is seen from as a card.
const (
	MaxParts  = 12
	MaxPaints = 4
	BodyViews = 8
)

// a paint's texels in the bodies' table and a part's, and those before a row's parts: the
// row's own and its paints'
const (
	paintTexels = 3
	partTexels  = 3
	rowTexels   = 1 + paintTexels*MaxPaints
	tableWide   = rowTexels + partTexels*MaxParts
)

// Check panics where the body, of's, does not hold together: no part, too many, a part of no
// shape or width, in no paint, or reaching past the body's box, which its cards are as wide as.
func (b Body) Check(of string) {
	switch {
	case b.Wide <= 0 || b.Tall <= 0:
		panic(fmt.Sprintf("world: the body of %s needs a Wide and a Tall", of))
	case len(b.Parts) == 0 || len(b.Parts) > MaxParts:
		panic(fmt.Sprintf("world: the body of %s has %d parts, 1 to %d it may", of,
			len(b.Parts), MaxParts))
	case len(b.Paints) == 0 || len(b.Paints) > MaxPaints:
		panic(fmt.Sprintf("world: the body of %s has %d paints, 1 to %d it may", of,
			len(b.Paints), MaxPaints))
	}
	// within a float's last bits: the shares of a height seldom sum to it whole
	half, high := b.Wide/2+f32.Eps(b.Wide), b.Tall+f32.Eps(b.Tall)
	for i, p := range b.Parts {
		if p.Shape != Spheroid && p.Shape != Cone || p.Wide <= 0 || p.Tip < 0 {
			panic(fmt.Sprintf("world: the body of %s: part %d is no spheroid nor cone as wide as "+
				"anything", of, i))
		}
		if p.Paint < 0 || p.Paint >= len(b.Paints) {
			panic(fmt.Sprintf("world: the body of %s: part %d is in paint %d of %d", of, i,
				p.Paint, len(b.Paints)))
		}
		tip := p.Tip
		if p.Shape == Spheroid {
			tip = p.Wide
		}
		for k, at := range [2][3]float32{p.From, p.To} {
			reach := max(f32.Abs(at[0]), f32.Abs(at[1])) + [2]float32{p.Wide, tip}[k]/2
			if reach > half || at[2] < b.Tall-high || at[2] > high {
				panic(fmt.Sprintf("world: the body of %s: part %d, %v wide at %v, reaches past "+
					"half its width %v either way or its height %v", of, i,
					[2]float32{p.Wide, tip}[k], at, b.Wide, b.Tall))
			}
		}
	}
}

// topFirst is the parts from the highest down: seen from above the nearer come first, and what
// they hide of the rest is not worked out
func topFirst(parts []Part) []Part {
	out := slices.Clone(parts)
	slices.SortStableFunc(out, func(a, b Part) int {
		return cmp.Compare(b.From[2]+b.To[2], a.From[2]+a.To[2])
	})
	return out
}

// row is the body's row of the table its shaders read (body.wgsl), tableWide texels: how many
// parts it has; each paint in three texels, its colour, its lit dots' and its other dots', the
// last two clear where it has none; then each part in three, from the highest down: where its
// From stands and how wide half of it is there, the same of its To, its shape and its paint.
// Across and deep count in the body's widths, -1 to 1, up in its heights, 0 to 1, a half width
// in its widths.
func (b Body) row() []byte {
	out := make([]byte, 4*tableWide)
	put := func(texel int, v ...float32) {
		for k, f := range v {
			out[4*texel+k] = byte(min(max(f, 0), 1)*255 + 0.5)
		}
	}
	paint := func(texel int, c color.RGBA) {
		put(texel, float32(c.R)/255, float32(c.G)/255, float32(c.B)/255, float32(c.A)/255)
	}
	put(0, float32(len(b.Parts))/255, 0, 0, 1)
	for k, p := range b.Paints {
		paint(1+paintTexels*k, p.Colour)
		paint(2+paintTexels*k, p.Lit)
		paint(3+paintTexels*k, p.Dots)
	}
	at := func(v [3]float32, wide float32) []float32 {
		return []float32{(v[0]/b.Wide + 1) / 2, (v[1]/b.Wide + 1) / 2, v[2] / b.Tall,
			wide / 2 / b.Wide}
	}
	for slot, p := range topFirst(b.Parts) {
		tip := p.Tip
		if p.Shape == Spheroid {
			tip = p.Wide
		}
		texel := rowTexels + partTexels*slot
		put(texel, at(p.From, p.Wide)...)
		put(texel+1, at(p.To, tip)...)
		put(texel+2, float32(p.Shape)/255, float32(p.Paint)/255, 0, 1)
	}
	return out
}

// topAt is how high over its foot the middle of what the body shows from above stands: its
// parts' middles, each by how much of the level it covers
func (b Body) topAt() float32 {
	var high, all float32
	for _, p := range b.Parts {
		tip := p.Tip
		if p.Shape == Spheroid {
			tip = p.Wide
		}
		across := (p.Wide + tip) / 2
		covers := across * (across + f32.Hypot(p.To[0]-p.From[0], p.To[1]-p.From[1]))
		high += covers * (p.From[2] + p.To[2]) / 2
		all += covers
	}
	if all == 0 {
		return b.Tall / 2
	}
	return high / all
}

// reach is how far the part reaches along the unit vector u, from and to
func (p Part) reach(u [3]float64) (from, to float64) {
	var a, b, long float64
	for k := range 3 {
		a += float64(p.From[k]) * u[k]
		b += float64(p.To[k]) * u[k]
		long += float64(p.To[k]-p.From[k]) * float64(p.To[k]-p.From[k])
	}
	// how much of u runs along the part, a part of no length standing up
	along := u[2]
	if long = math.Sqrt(long); long > 0 {
		along = (b - a) / long
	}
	across := math.Sqrt(max(1-along*along, 0))
	if p.Shape == Spheroid {
		half := math.Hypot(long/2*along, float64(p.Wide)/2*across)
		return (a+b)/2 - half, (a+b)/2 + half
	}
	wide, tip := float64(p.Wide)/2*across, float64(p.Tip)/2*across
	return min(a-wide, b-tip), max(a+wide, b+tip)
}

// steepest is how steeply an eye looks down at a body for its card to lie whatever it shows,
// radians: a standing card follows the eye no further (body.wgsl's shapeSteepest, the slope).
const steepest = 70 * math.Pi / 180

// steep is how steeply an eye looks down at the body for its card from above to show it truer
// than a standing one photographed from elevation (untrue), the drop of its way a step along
// the level, squared; steepest at the most.
func (b Body) steep(elevation float64) float32 {
	const step = math.Pi / 180
	e := elevation + step
	for ; e < steepest; e += step {
		if standing, lying := b.untrue(elevation, e); lying < standing {
			break
		}
	}
	slope := math.Tan(min(e, steepest))
	return float32(slope * slope)
}

// untrue is how far off the body's own outline its cards show it to an eye looking down from
// e: each part's reach up the picture against where the body itself shows it, by how wide the
// part is, over the sides the standing cards are taken from. A standing card, photographed
// from elevation, stands grown about topAt; the one from above is held up there, facing the
// eye; neither shows what of it lies under the ground.
func (b Body) untrue(elevation, e float64) (standing, lying float64) {
	top := float64(b.topAt())
	grown := b.grown(elevation, e)
	s0, c0 := math.Sincos(elevation)
	se, ce := math.Sincos(e)
	stood := func(reach float64) float64 { return max(top+(reach/c0-top)*grown, 0) * ce }
	lain := func(reach float64) float64 { return max(top*ce+reach, -top*se*se/ce) }
	for k := range BodyViews {
		sh, ch := math.Sincos(float64(k) * 2 * math.Pi / BodyViews)
		for _, p := range b.Parts {
			from, to := p.reach([3]float64{ch * se, sh * se, ce})
			sfrom, sto := p.reach([3]float64{ch * s0, sh * s0, c0})
			lfrom, lto := p.reach([3]float64{ch, sh, 0})
			wide := float64(p.Wide)
			if p.Shape == Cone {
				wide = float64(p.Wide+p.Tip) / 2
			}
			standing += wide * (math.Abs(stood(sfrom)-from) + math.Abs(stood(sto)-to))
			lying += wide * (math.Abs(lain(lfrom)-from) + math.Abs(lain(lto)-to))
		}
	}
	return standing, lying
}

// grown is how many times as tall a card of the body photographed from elevation shows on an
// upright plane, about topAt, for an eye looking down from e (body.wgsl's shapeRow, the same).
func (b Body) grown(elevation, e float64) float64 {
	across := float64(b.Wide) * float64(b.Wide) / 4
	half := float64(b.Tall) - float64(b.topAt())
	up := max(half*half, 1e-3)
	was := math.Tan(elevation)
	steep := math.Tan(min(max(e, elevation), steepest))
	return math.Sqrt((up + across*steep*steep) / (up + across*was*was))
}

// mean is the colour the body is far off, its parts' paints by how much of it each is
func (b Body) mean() color.RGBA {
	var red, green, blue, all float32
	for _, p := range b.Parts {
		long := f32.Hypot(f32.Hypot(p.To[0]-p.From[0], p.To[1]-p.From[1]), p.To[2]-p.From[2])
		share := max(long, p.Wide) * (p.Wide + p.Tip) / 2
		if p.Shape == Spheroid {
			share = max(long, p.Wide) * p.Wide
		}
		c := b.Paints[p.Paint].Colour
		red, green, blue = red+share*float32(c.R), green+share*float32(c.G), blue+share*float32(c.B)
		all += share
	}
	if all == 0 {
		return color.RGBA{}
	}
	return color.RGBA{R: uint8(red / all), G: uint8(green / all), B: uint8(blue / all), A: 255}
}
