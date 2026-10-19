// Package bake bakes the light on the ground into an image for every shader of a frame to read
// (render.GroundShade): the shadows of the relief, from the heights the frame holds
// (render.GroundHeights), and those of the blocks standing on it, over level ground without
// them. The shade is baked anew as the light turns, a strip a frame, each strip fading from the
// shade it held before into its own.
package bake

import (
	"image"
	"image/color"
	"math"
	"slices"

	_ "github.com/kjkrol/gram/plugins/light/internal/distant" // the light in the shaders
	"github.com/kjkrol/gram/render"

	"embed"
)

//go:embed shaders/*.wgsl
var shaders embed.FS

var bakeShade = render.NewShaderWith("ground shade",
	render.Files(shaders, "shaders/ground.wgsl", "shaders/shade.wgsl"),
	[]render.Uniform{
		{Name: "BakeCell", Size: 1},
		{Name: "ShadeSpan", Size: 1},
	})

var bakeBlocks = render.NewMeshShaderWith("blocks' shade",
	render.Files(shaders, "shaders/ground.wgsl", "shaders/blocks.wgsl"),
	[]render.Uniform{
		{Name: "BakeCell", Size: 1},
		{Name: "ShadeSpan", Size: 1},
		{Name: "Strip", Size: 4},
	}).Instanced(2)

// pixels a cell the baked shade holds at most, and how wide the image may be
const (
	shadePx  = 8
	maxBaked = 8192
)

// dot products of the light's ways: a tenth of a degree turned bakes the shade anew in
// shadeStrips strips, one a frame; a degree leapt bakes all of it at once
const (
	shadeTurn   = 0.9999985
	shadeStrips = render.ShadeBands
	shadeLeap   = 0.99985
)

// shaders/ground.wgsl's: how wide a shadow's penumbra is per unit towards the light
const shadowSoft = 0.08

// Lattice is the ground the shade is baked over: Cols by Rows corners Cell world units apart
// from the origin, the lowest Low high and the highest Low+Span; Version counts the changes to
// the heights. Level ground is a Lattice of no Span.
type Lattice struct {
	Cols, Rows int
	Cell       float32
	Low, Span  float32
	Version    uint64
}

// Block is a solid whose shadow is baked: a rectangle round (X, Y) running (UX, UY), Along from
// its middle that way and Wide across, standing from Bottom up to Top.
type Block struct{ X, Y, UX, UY, Along, Wide, Bottom, Top float32 }

// Baker bakes the shade of one ground.
type Baker struct {
	lat Lattice

	// the shade baked, and as it was before each band's last bake (render.GroundShade)
	baked     *render.Image
	was       *render.Image
	fresh     bool // neither filled yet
	shown     bool // baked once since they were made
	shade     render.GroundShade
	frames    uint64                 // Plan calls
	bakedAt   [shadeStrips]uint64    // the frame each band was last baked in
	plan      struct{ from, to int } // the rows the frame's Bake bakes
	planWay   [3]float32             // the light's way they are baked under
	shadeAt   uint64
	shadeSun  [3]float32
	strip     int
	coarse    bool
	baking    render.DrawTrianglesShaderOptions
	blocking  render.DrawMeshOptions
	bakeVerts []render.Vertex

	blocks   []float32
	near     []float32
	blocksAt uint64 // a hash of the blocks as last found; 0 none
}

// New is a baker of the shade of nothing yet.
func New() *Baker { return &Baker{} }

// Coarse has the shade baked half as fine a side from the next frame on, or as fine as ever
// again.
func (b *Baker) Coarse(on bool) { b.coarse = on }

// Coarsened reports whether the shade is baked coarse.
func (b *Baker) Coarsened() bool { return b.coarse }

// Plan settles what of the shade over lat the frame's Bake bakes under the light the way sun, with
// blocks standing on the ground, and how far each band has faded from the shade before into its
// own since it was last baked. It is the frame's shade, false before any is baked.
func (b *Baker) Plan(lat Lattice, sun [3]float32, blocks []Block) (render.GroundShade, bool) {
	b.frames++
	if b.plan.from < b.plan.to { // planned, never drawn: what it would have baked is missing
		b.shadeAt = 0
	}
	b.plan.from, b.plan.to = 0, 0
	if lat.Cols < 2 || lat.Rows < 2 || lat.Cell <= 0 {
		b.shown = false
		return render.GroundShade{}, false
	}
	b.lat = lat
	ks := bakedScale(lat.Cols, lat.Rows, b.coarse)
	ws, h := ks*(lat.Cols-1), ks*(lat.Rows-1)
	if b.baked == nil || b.baked.Bounds().Dx() != ws || b.baked.Bounds().Dy() != h {
		if b.baked != nil {
			b.baked.Deallocate()
			b.was.Deallocate()
		}
		b.baked, b.was = render.NewImage(ws, h), render.NewImage(ws, h)
		b.fresh, b.shown, b.shadeAt = true, false, 0
		b.bakeVerts = make([]render.Vertex, 4)
		b.baking.Uniforms = map[string]any{}
	}
	per := (h + shadeStrips - 1) / shadeStrips
	b.shade.Now, b.shade.Was = b.baked, b.was
	b.shade.Area = [4]float32{0, 0, float32(lat.Cols-1) * lat.Cell, float32(lat.Rows-1) * lat.Cell}
	b.shade.Band = float32(per) / float32(h)
	defer b.fade()
	n := float32(math.Sqrt(float64(sun[0]*sun[0] + sun[1]*sun[1] + sun[2]*sun[2])))
	if n == 0 {
		return b.shade, b.shown
	}
	way := [3]float32{sun[0] / n, sun[1] / n, sun[2] / n}
	turned := way[0]*b.shadeSun[0] + way[1]*b.shadeSun[1] + way[2]*b.shadeSun[2]
	moved := b.gather(blocks)
	switch {
	case b.shadeAt != lat.Version+1 || turned < shadeLeap || moved:
		b.shadeAt, b.shadeSun, b.strip = lat.Version+1, way, 0
		b.plan.from, b.plan.to = 0, h
		for k := range b.bakedAt {
			b.bakedAt[k] = b.frames
		}
	case b.strip == 0 && turned > shadeTurn:
		return b.shade, b.shown
	default:
		if b.strip == 0 {
			b.shadeSun = way
		}
		b.plan.from, b.plan.to = min(b.strip*per, h), min((b.strip+1)*per, h)
		b.bakedAt[b.strip] = b.frames
		b.strip = (b.strip + 1) % shadeStrips
	}
	if !b.shown { // the first shade baked shows at once
		for k := range b.bakedAt {
			b.bakedAt[k] = b.frames - shadeStrips
		}
	}
	b.planWay = way
	b.shade.Span = b.shadeSpan()
	return b.shade, b.shown
}

// fade has each band fade from the shade before into its own over shadeStrips frames since it was
// baked.
func (b *Baker) fade() {
	for k, at := range b.bakedAt {
		b.shade.Mix[k] = min(float32(b.frames-at)/shadeStrips, 1)
	}
}

// Bake bakes the rows the frame's Plan said, under the light it planned, reading the ground's
// heights from the frame's uniforms u, keeping each band as it was before for the frame to fade
// from. Over a relief whose heights the frame does not hold yet it bakes nothing, and the next
// Plan plans the whole anew.
func (b *Baker) Bake(u render.Uniforms) {
	if b.baked == nil {
		return
	}
	if lat := u.Get("GroundLattice"); b.lat.Span > 0 && (len(lat) < 4 || lat[3] < 0.5) {
		b.plan.from, b.plan.to, b.shadeAt = 0, 0, 0
		return
	}
	if b.fresh {
		// the light everywhere: none of it held back, no shadow reaching up
		b.baked.Fill(color.RGBA{R: 255, B: 255, A: 255})
		b.was.Fill(color.RGBA{R: 255, B: 255, A: 255})
		b.fresh = false
	}
	y0, y1 := b.plan.from, b.plan.to
	b.plan.from, b.plan.to = 0, 0
	if y0 >= y1 {
		return
	}
	b.shown = true
	ws, h := b.baked.Bounds().Dx(), b.baked.Bounds().Dy()
	size := [2]float32{b.shade.Area[2], b.shade.Area[3]}
	rows0 := image.Rect(0, y0, ws, y1)
	b.was.SubImage(rows0).CopyFrom(b.baked.SubImage(rows0))
	u.Into(b.baking.Uniforms)
	// the images baked into are never read while baked
	delete(b.baking.Uniforms, "GroundShade")
	delete(b.baking.Uniforms, "GroundShadeWas")
	b.baking.Uniforms["BakeCell"] = []float32{b.lat.Cell}
	b.baking.Uniforms["ShadeSpan"] = []float32{b.shade.Span}
	b.blocking.Uniforms = b.baking.Uniforms
	b.blocking.Vertices, b.blocking.Darkest = 6, true
	per := (h + shadeStrips - 1) / shadeStrips
	for y := y0; y < y1; y += per {
		from := [2]float32{0, float32(y) / float32(h) * size[1]}
		part := [2]float32{size[0], float32(min(y+per, y1)-y) / float32(h) * size[1]}
		at := image.Rect(0, y, ws, min(y+per, y1))
		b.bakeInto(at, from, part)
		if near := b.shading(from[1], from[1]+part[1], b.planWay); len(near) > 0 {
			b.baking.Uniforms["Strip"] = []float32{from[0], from[1], part[0], part[1]}
			b.blocking.Instances = near
			b.baked.SubImage(at).DrawMesh(nil, bakeBlocks, &b.blocking)
		}
	}
}

// Pending reports whether the frame's Plan left anything for its Bake to bake.
func (b *Baker) Pending() bool { return b.plan.from < b.plan.to }

// Drop lets the shade go: nothing to bake it over any more.
func (b *Baker) Drop() {
	if b.baked != nil {
		b.baked.Deallocate()
		b.was.Deallocate()
	}
	b.baked, b.was, b.shown, b.shadeAt = nil, nil, false, 0
	b.plan.from, b.plan.to = 0, 0
}

// Shade is the shade as last baked, for a test to read: the image and its area in the world.
func (b *Baker) Shade() render.GroundShade { return b.shade }

// Blocks is how many blocks the shade is baked with, a bridge's deck one however many pieces it
// is laid in.
func (b *Baker) Blocks() int { return len(b.blocks) / 8 }

// shadeSpan is how high over the ground a shadow may reach: up the relief, or to the top of the
// highest block over its lowest ground
func (b *Baker) shadeSpan() float32 {
	span := max(b.lat.Span, 1)
	for k := 7; k < len(b.blocks); k += 8 {
		span = max(span, b.blocks[k]-b.lat.Low)
	}
	return span
}

func (b *Baker) shading(y0, y1 float32, sun [3]float32) []float32 {
	b.near = b.near[:0]
	for k := 0; k+8 <= len(b.blocks); k += 8 {
		bl := b.blocks[k : k+8]
		far := (bl[7] - b.lat.Low) / sun[2]
		spare := far*shadowSoft + b.lat.Cell
		half := float32(math.Abs(float64(bl[3])))*bl[4] + float32(math.Abs(float64(bl[2])))*bl[5]
		cast := -sun[1] * far
		top, bottom := bl[1]-half+min(cast, 0)-spare, bl[1]+half+max(cast, 0)+spare
		if bottom >= y0 && top <= y1 {
			b.near = append(b.near, bl...)
		}
	}
	return b.near
}

// gather takes the blocks as found, joined; it reports whether they stand other than before
func (b *Baker) gather(found []Block) bool {
	at := hashed(found)
	if at == b.blocksAt {
		return false
	}
	b.blocksAt = at
	b.blocks = b.blocks[:0]
	for _, bl := range joined(slices.Clone(found)) {
		b.blocks = append(b.blocks, bl.X, bl.Y, bl.UX, bl.UY, bl.Along, bl.Wide, bl.Bottom, bl.Top)
	}
	return true
}

// never 0
func hashed(bs []Block) uint64 {
	h := uint64(14695981039346656037)
	for _, b := range bs {
		for _, v := range [8]float32{b.X, b.Y, b.UX, b.UY, b.Along, b.Wide, b.Bottom, b.Top} {
			h = (h ^ uint64(math.Float32bits(v))) * 1099511628211
		}
	}
	return max(h, 1)
}

// a bridge's deck is one block however many pieces it is laid in
func joined(bs []Block) []Block {
	for i := range bs {
		if bs[i].UX < 0 || bs[i].UX == 0 && bs[i].UY < 0 {
			bs[i].UX, bs[i].UY = -bs[i].UX, -bs[i].UY
		}
	}
	for again := true; again; {
		again = false
		for i := 0; i < len(bs); i++ {
			for j := i + 1; j < len(bs); j++ {
				if b, ok := join(bs[i], bs[j]); ok {
					bs[i], again = b, true
					bs = slices.Delete(bs, j, j+1)
					j--
				}
			}
		}
	}
	return bs
}

func join(a, b Block) (Block, bool) {
	const near = 0.01
	apart := func(x, y float32) bool { return math.Abs(float64(x-y)) > near }
	if apart(a.UX, b.UX) || apart(a.UY, b.UY) || apart(a.Wide, b.Wide) ||
		apart(a.Bottom, b.Bottom) || apart(a.Top, b.Top) {
		return a, false
	}
	dx, dy := b.X-a.X, b.Y-a.Y
	if apart(dx*a.UY-dy*a.UX, 0) {
		return a, false
	}
	s := dx*a.UX + dy*a.UY
	if s-b.Along > a.Along+near || s+b.Along < -a.Along-near {
		return a, false
	}
	lo, hi := min(-a.Along, s-b.Along), max(a.Along, s+b.Along)
	mid := (lo + hi) / 2
	a.X, a.Y, a.Along = a.X+a.UX*mid, a.Y+a.UY*mid, (hi-lo)/2
	return a, true
}

// each pixel handed its point of the world in its custom values
func (b *Baker) bakeInto(at image.Rectangle, from, size [2]float32) {
	for i, c := range [4][2]float32{{0, 0}, {1, 0}, {0, 1}, {1, 1}} {
		b.bakeVerts[i] = render.Vertex{
			DstX:    float32(at.Min.X) + c[0]*float32(at.Dx()),
			DstY:    float32(at.Min.Y) + c[1]*float32(at.Dy()),
			ColorR:  1,
			ColorG:  1,
			ColorB:  1,
			ColorA:  1,
			Custom0: from[0] + c[0]*size[0],
			Custom1: from[1] + c[1]*size[1],
		}
	}
	b.baked.SubImage(at).DrawTrianglesShader(b.bakeVerts, quad, bakeShade, &b.baking)
}

var quad = []uint16{0, 1, 2, 1, 2, 3}

func bakedScale(cols, rows int, coarse bool) int {
	shade := max(min(shadePx, maxBaked/max(cols-1, 1), maxBaked/max(rows-1, 1)), 2)
	if coarse {
		shade = max(shade/2, 2)
	}
	return shade
}
