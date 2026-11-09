package compose

// The ground's shade (GroundShade) baked for every shader of a frame to read: the shadows of the
// relief, from the heights the frame holds (GroundHeights), and those of the blocks standing on
// it, over level ground without them. It is baked anew as the light turns, a strip each sixteenth
// of a degree, each strip fading from the shade it held before into its own, and where a set of
// blocks changed, a few tiles a frame at once.

import (
	"embed"
	"fmt"
	"image"
	"image/color"
	"math"
	"strings"

	"github.com/kjkrol/gram/render/internal/gfx"
	"github.com/kjkrol/gram/render/internal/light"
)

//go:embed shaders/*.wgsl
var shaders embed.FS

// shaderFiles is the shaders' files named, one after another
func shaderFiles(names ...string) []byte {
	var b strings.Builder
	for _, n := range names {
		src, err := shaders.ReadFile("shaders/" + n)
		if err != nil {
			panic(fmt.Sprintf("render: %v", err))
		}
		b.Write(src)
		b.WriteString("\n")
	}
	return []byte(b.String())
}

var bakeShade = gfx.NewShaderWith("ground shade",
	shaderFiles("bake_ground.wgsl", "bake_shade.wgsl"),
	[]gfx.Uniform{
		{Name: "BakeCell", Size: 1},
		{Name: "ShadeSpan", Size: 1},
	})

var bakeBlocks = gfx.NewMeshShaderWith("blocks' shade",
	shaderFiles("bake_ground.wgsl", "bake_blocks.wgsl"),
	[]gfx.Uniform{
		{Name: "BakeCell", Size: 1},
		{Name: "ShadeSpan", Size: 1},
		{Name: "Strip", Size: 4},
	}).Instanced(2)

// pixels a cell the baked shade holds at most, and how wide the image may be
const (
	shadePx  = 8
	maxBaked = 8192
)

// how far the light turns, in radians, before the next strip is baked anew, so no strip lags a
// degree behind; a leap of a degree since the last strip bakes every strip anew, a strip a frame
const (
	shadeStrips = ShadeBands
	shadeStep   = math.Pi / 180 / shadeStrips
	shadeLeap   = math.Pi / 180
)

// the side of the tiles the shade is baked anew in where a set of blocks changed, in pixels, and
// how many of them a frame bakes at most
const (
	shadeTile     = 256
	tilesPerFrame = 8
)

// shaders/bake_ground.wgsl's: how wide a shadow's penumbra is per unit towards the light
const shadowSoft = 0.08

// the ground the shade is baked over, a solid whose shadow it bakes, and the solids of a part of
// the world (light.Lattice, Block, BlockSet)
type (
	Lattice  = light.Lattice
	Block    = light.Block
	BlockSet = light.BlockSet
)

// Baker bakes the shade of one ground.
type Baker struct {
	lat Lattice

	// the shade baked, and as it was before each band's last bake (GroundShade)
	baked     *Image
	was       *Image
	fresh     bool // neither filled yet
	shown     bool // baked once since they were made
	shade     GroundShade
	frames    uint64              // Plan calls
	bakedAt   [shadeStrips]uint64 // the frame each band was last baked in
	plan      bakePlan            // what the frame's Bake bakes
	planWay   [3]float32          // the light's way it is baked under
	shadeAt   uint64
	shadeSun  [3]float32 // the light's way the last strip was baked under
	strip     int
	hurry     int // strips left to bake one a frame since the light leapt
	coarse    bool
	baking    gfx.DrawTrianglesShaderOptions
	blocking  gfx.DrawMeshOptions
	bakeVerts []Vertex
	own       map[string][]float32 // the bake's own uniforms, a slice a name kept between bakes

	sets   []bakedSet // the blocks as last gathered, a set by its place in Lights.Casts
	tiles  [2]int     // how many tiles lie across the shade and down it
	queue  []int      // the tiles to bake anew, the longest waiting first
	queued []bool     // by tile
	near   []float32
}

// the rows of the shade a frame's Bake bakes, and its tiles
type bakePlan struct {
	from, to int
	tiles    []int
}

// a set of blocks as the baker last gathered it (BlockSet)
type bakedSet struct {
	version uint64
	box     [4]float32
	top     float32
	blocks  []Block
}

// Coarse has the shade baked half as fine a side from the next frame on, or as fine as ever
// again.
func (b *Baker) Coarse(on bool) { b.coarse = on }

// Coarsened reports whether the shade is baked coarse.
func (b *Baker) Coarsened() bool { return b.coarse }

// Plan settles what of the shade over lat the frame's Bake bakes under the light the way sun, with
// the sets of blocks casts standing on the ground, and how far each band has faded from the shade
// before into its own since it was last baked. It is the frame's shade, false before any is baked.
func (b *Baker) Plan(lat Lattice, sun [3]float32, casts []BlockSet) (GroundShade, bool) {
	b.frames++
	if b.Pending() { // planned, never drawn: what it would have baked is missing
		b.shadeAt = 0
	}
	b.plan.from, b.plan.to, b.plan.tiles = 0, 0, b.plan.tiles[:0]
	if lat.Cols < 2 || lat.Rows < 2 || lat.Cell <= 0 {
		b.shown = false
		return GroundShade{}, false
	}
	b.lat = lat
	ks := bakedScale(lat.Cols, lat.Rows, b.coarse)
	ws, h := ks*(lat.Cols-1), ks*(lat.Rows-1)
	if b.baked == nil || b.baked.Bounds().Dx() != ws || b.baked.Bounds().Dy() != h {
		if b.baked != nil {
			b.baked.Deallocate()
			b.was.Deallocate()
		}
		b.baked, b.was = gfx.NewImage(ws, h), gfx.NewImage(ws, h)
		b.fresh, b.shown, b.shadeAt = true, false, 0
		b.bakeVerts = make([]Vertex, 4)
		b.baking.Uniforms, b.own = map[string]any{}, map[string][]float32{}
		b.tiles = [2]int{(ws + shadeTile - 1) / shadeTile, (h + shadeTile - 1) / shadeTile}
		b.queue, b.queued = b.queue[:0], make([]bool, b.tiles[0]*b.tiles[1])
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
	turned := chord(way, b.shadeSun)
	b.gather(casts, way)
	span := b.shadeSpan()
	whole := b.shadeAt != lat.Version+1 || span != b.shade.Span
	if whole || turned > shadeLeap {
		// every strip is baked anew: the tiles waiting with them
		for _, t := range b.queue {
			b.queued[t] = false
		}
		b.queue = b.queue[:0]
	}
	switch {
	case whole:
		b.shadeAt, b.shadeSun, b.strip, b.hurry = lat.Version+1, way, 0, 0
		b.plan.from, b.plan.to = 0, h
		for k := range b.bakedAt {
			b.bakedAt[k] = b.frames
		}
	case turned > shadeLeap: // down from the top, each strip a frame behind the one over it
		b.hurry, b.strip = shadeStrips, 0
		fallthrough
	case b.hurry > 0 || turned >= shadeStep:
		b.shadeSun, b.hurry = way, max(b.hurry-1, 0)
		b.plan.from, b.plan.to = min(b.strip*per, h), min((b.strip+1)*per, h)
		b.bakedAt[b.strip] = b.frames
		b.strip = (b.strip + 1) % shadeStrips
	}
	if !b.shown { // the first shade baked shows at once
		for k := range b.bakedAt {
			b.bakedAt[k] = b.frames - shadeStrips
		}
	}
	k := min(tilesPerFrame, len(b.queue))
	b.plan.tiles = append(b.plan.tiles, b.queue[:k]...)
	for _, t := range b.plan.tiles {
		b.queued[t] = false
	}
	b.queue = b.queue[:copy(b.queue, b.queue[k:])]
	b.planWay = way
	b.shade.Span = span
	return b.shade, b.shown
}

// chord is how far apart the unit ways a and b are, about the angle between them in radians
func chord(a, b [3]float32) float64 {
	dx, dy, dz := float64(a[0]-b[0]), float64(a[1]-b[1]), float64(a[2]-b[2])
	return math.Sqrt(dx*dx + dy*dy + dz*dz)
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
func (b *Baker) Bake(u Uniforms) {
	if b.baked == nil {
		return
	}
	if lat := u.Get("GroundLattice"); b.lat.Span > 0 && (len(lat) < 4 || lat[3] < 0.5) {
		b.plan.from, b.plan.to, b.plan.tiles, b.shadeAt = 0, 0, b.plan.tiles[:0], 0
		return
	}
	if b.fresh {
		// the light everywhere: none of it held back, no shadow reaching up
		b.baked.Fill(color.RGBA{R: 255, B: 255, A: 255})
		b.was.Fill(color.RGBA{R: 255, B: 255, A: 255})
		b.fresh = false
	}
	y0, y1, tiles := b.plan.from, b.plan.to, b.plan.tiles
	b.plan.from, b.plan.to, b.plan.tiles = 0, 0, tiles[:0]
	if y0 >= y1 && len(tiles) == 0 {
		return
	}
	b.shown = true
	ws, h := b.baked.Bounds().Dx(), b.baked.Bounds().Dy()
	u.Into(b.baking.Uniforms)
	// the images baked into are never read while baked
	delete(b.baking.Uniforms, "GroundShade")
	delete(b.baking.Uniforms, "GroundShadeWas")
	b.set("BakeCell", b.lat.Cell)
	b.set("ShadeSpan", b.shade.Span)
	b.blocking.Uniforms = b.baking.Uniforms
	b.blocking.Vertices, b.blocking.Darkest = 6, true
	if y0 < y1 {
		rows0 := image.Rect(0, y0, ws, y1)
		b.was.SubImage(rows0).CopyFrom(b.baked.SubImage(rows0))
		per := (h + shadeStrips - 1) / shadeStrips
		for y := y0; y < y1; y += per {
			b.bakeRect(image.Rect(0, y, ws, min(y+per, y1)))
		}
	}
	// a tile of a band fading reads the shade it is baked over as the band's before
	for _, t := range tiles {
		at := b.tileRect(t)
		b.was.SubImage(at).CopyFrom(b.baked.SubImage(at))
		b.bakeRect(at)
	}
}

// bakeRect bakes the shade's pixels at, the relief's shadows and the blocks' over them
func (b *Baker) bakeRect(at image.Rectangle) {
	ws, h := float32(b.baked.Bounds().Dx()), float32(b.baked.Bounds().Dy())
	size := [2]float32{b.shade.Area[2], b.shade.Area[3]}
	from := [2]float32{float32(at.Min.X) / ws * size[0], float32(at.Min.Y) / h * size[1]}
	part := [2]float32{float32(at.Dx()) / ws * size[0], float32(at.Dy()) / h * size[1]}
	b.bakeInto(at, from, part)
	r := [4]float32{from[0], from[1], from[0] + part[0], from[1] + part[1]}
	if near := b.shading(r, b.planWay); len(near) > 0 {
		b.set("Strip", from[0], from[1], part[0], part[1])
		b.blocking.Instances = near
		b.baked.SubImage(at).DrawMesh(nil, bakeBlocks, &b.blocking)
	}
}

// tileRect is tile t's pixels
func (b *Baker) tileRect(t int) image.Rectangle {
	x, y := t%b.tiles[0]*shadeTile, t/b.tiles[0]*shadeTile
	return image.Rect(x, y, x+shadeTile, y+shadeTile).Intersect(b.baked.Bounds())
}

// set is the bake's uniform name, vals copied into the slice kept for it: a draw packs its
// uniforms before it returns, so one slice serves every strip
func (b *Baker) set(name string, vals ...float32) {
	u, ok := b.own[name]
	if !ok || len(u) != len(vals) {
		u = make([]float32, len(vals))
		b.own[name] = u
		b.baking.Uniforms[name] = u
	}
	copy(u, vals)
}

// Pending reports whether the frame's Plan left anything for its Bake to bake.
func (b *Baker) Pending() bool { return b.plan.from < b.plan.to || len(b.plan.tiles) > 0 }

// Drop lets the shade go: nothing to bake it over any more.
func (b *Baker) Drop() {
	if b.baked != nil {
		b.baked.Deallocate()
		b.was.Deallocate()
	}
	b.baked, b.was, b.shown, b.shadeAt = nil, nil, false, 0
	b.plan.from, b.plan.to, b.plan.tiles = 0, 0, b.plan.tiles[:0]
	b.sets, b.queue = b.sets[:0], b.queue[:0]
}

// Shade is the shade as last baked, for a test to read: the image and its area in the world.
func (b *Baker) Shade() GroundShade { return b.shade }

// Blocks is how many blocks the shade is baked with, as the sets last gathered hold them.
func (b *Baker) Blocks() int {
	n := 0
	for i := range b.sets {
		n += len(b.sets[i].blocks)
	}
	return n
}

// shadeSpan is how high over the ground a shadow may reach: up the relief, or to the top of the
// highest block over its lowest ground
func (b *Baker) shadeSpan() float32 {
	span := max(b.lat.Span, 1)
	for i := range b.sets {
		if len(b.sets[i].blocks) > 0 {
			span = max(span, b.sets[i].top-b.lat.Low)
		}
	}
	return span
}

// gather takes the sets of blocks as found, and has the tiles baked anew where the shadows of a
// set changed may have fallen before and may fall now, under the light the way sun
func (b *Baker) gather(casts []BlockSet, sun [3]float32) {
	for i := len(casts); i < len(b.sets); i++ {
		b.dirty(&b.sets[i], sun)
	}
	b.sets = b.sets[:min(len(b.sets), len(casts))]
	for len(b.sets) < len(casts) {
		b.sets = append(b.sets, bakedSet{})
	}
	for i := range casts {
		c, s := &casts[i], &b.sets[i]
		if c.Version == s.version {
			continue
		}
		b.dirty(s, sun)
		s.version, s.box, s.top = c.Version, c.Box, c.Top
		s.blocks = append(s.blocks[:0], c.Blocks...)
		b.dirty(s, sun)
	}
}

// dirty queues the tiles the shadows of s may fall on under the light the way sun
func (b *Baker) dirty(s *bakedSet, sun [3]float32) {
	if len(s.blocks) == 0 {
		return
	}
	r, size := b.reach(s.box, s.top, sun), b.shade.Area
	if !(r[2] >= 0 && r[0] <= size[2] && r[3] >= 0 && r[1] <= size[3]) {
		return
	}
	// tiles a world unit across and down
	tx := float32(b.baked.Bounds().Dx()) / size[2] / shadeTile
	ty := float32(b.baked.Bounds().Dy()) / size[3] / shadeTile
	x0, x1 := int(max(r[0], 0)*tx), min(int(min(r[2], size[2])*tx), b.tiles[0]-1)
	y0, y1 := int(max(r[1], 0)*ty), min(int(min(r[3], size[3])*ty), b.tiles[1]-1)
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			if t := y*b.tiles[0] + x; !b.queued[t] {
				b.queued[t] = true
				b.queue = append(b.queue, t)
			}
		}
	}
}

// reach is the world rectangle (x0, y0, x1, y1) the shadows of what stands within box (its
// origin and size) up to top may fall on under the light the way sun, down to the lowest ground:
// where the blocks' shader draws them
func (b *Baker) reach(box [4]float32, top float32, sun [3]float32) [4]float32 {
	far := (top - b.lat.Low) / max(sun[2], 1e-3)
	spare := far*shadowSoft + b.lat.Cell
	cx, cy := -sun[0]*far, -sun[1]*far
	return [4]float32{
		box[0] + min(cx, 0) - spare,
		box[1] + min(cy, 0) - spare,
		box[0] + box[2] + max(cx, 0) + spare,
		box[1] + box[3] + max(cy, 0) + spare,
	}
}

// shading is the blocks whose shadows may fall on the world rectangle r (x0, y0, x1, y1) under the
// light the way sun, as the blocks' shader takes them, a set's skipped whole where its shadows fall
// wide of r
func (b *Baker) shading(r [4]float32, sun [3]float32) []float32 {
	b.near = b.near[:0]
	low, cell, z := b.lat.Low, b.lat.Cell, max(sun[2], 1e-3)
	for i := range b.sets {
		s := &b.sets[i]
		if len(s.blocks) == 0 || !meets(b.reach(s.box, s.top, sun), r) {
			continue
		}
		// reach's of each block, by hand: a call a block was a fifth of the time
		for _, bl := range s.blocks {
			ux, uy := float32(math.Abs(float64(bl.UX))), float32(math.Abs(float64(bl.UY)))
			far := (bl.Top - low) / z
			spare := far*shadowSoft + cell
			hx, hy := ux*bl.Along+uy*bl.Wide+spare, uy*bl.Along+ux*bl.Wide+spare
			cx, cy := -sun[0]*far, -sun[1]*far
			if bl.X+hx+max(cx, 0) >= r[0] && bl.X-hx+min(cx, 0) <= r[2] &&
				bl.Y+hy+max(cy, 0) >= r[1] && bl.Y-hy+min(cy, 0) <= r[3] {
				b.near = append(b.near, bl.X, bl.Y, bl.UX, bl.UY, bl.Along, bl.Wide, bl.Bottom,
					bl.Top)
			}
		}
	}
	return b.near
}

// meets reports whether the world rectangles q and r (x0, y0, x1, y1) meet
func meets(q, r [4]float32) bool {
	return q[2] >= r[0] && q[0] <= r[2] && q[3] >= r[1] && q[1] <= r[3]
}

// each pixel handed its point of the world in its custom values
func (b *Baker) bakeInto(at image.Rectangle, from, size [2]float32) {
	for i, c := range [4][2]float32{{0, 0}, {1, 0}, {0, 1}, {1, 1}} {
		b.bakeVerts[i] = Vertex{
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
	b.baked.SubImage(at).DrawTrianglesShader(b.bakeVerts, bakeQuad, bakeShade, &b.baking)
}

var bakeQuad = []uint16{0, 1, 2, 1, 2, 3}

func bakedScale(cols, rows int, coarse bool) int {
	shade := max(min(shadePx, maxBaked/max(cols-1, 1), maxBaked/max(rows-1, 1)), 2)
	if coarse {
		shade = max(shade/2, 2)
	}
	return shade
}
