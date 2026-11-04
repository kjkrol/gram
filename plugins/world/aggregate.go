package world

import (
	"image"
	"math"

	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/render"
)

// AggregateImage is the name the ground's shaders read [Read.Aggregate]'s image by, a frame image
// (render.RegisterImage) a renderer binds where it draws a sector at L3.
const AggregateImage = "Aggregate"

func init() { render.RegisterImage(AggregateImage) }

// Aggregates has the world sum the figures it draws at L3 into the image [Read.Aggregate] gives,
// for a renderer laying it over the ground it draws; a world no renderer asks it of draws nothing
// of them at L3 and spends nothing on them.
func (p *Plugin) Aggregates() {
	if g := p.module.sectors.grid; g != nil && p.renderer.aggregate.img == nil {
		p.renderer.aggregate = newAggregate(p.Res.Config, g.Side())
	}
}

// the most texels the figures' colour summed into the ground spans a side
const aggregateSide = 2048

// texels a side of a tile of the aggregate, a power of two: what is summed, cleared and put on
// the GPU at once
const (
	tileShift     = 6
	aggregateTile = 1 << tileShift
)

// aggregate is the figures' colour summed into the ground from L3, an image over the whole world
// perUnit texels a world unit, cols by rows, in tiles across of them a row; summed by the
// publisher into the back sums, the front put on the GPU as the frame is composed
type aggregate struct {
	img          *render.Image
	perUnit      float32
	cols, rows   int
	across       int
	wrapX, wrapY bool
	sums         render.Buffered[sums]
	summing      bool // some view publishing sees a sector at L3, a kind summed there
	fresh        bool // the front not yet put on the GPU
	// what the image holds: by tile the signature of its sums, 0 for clear, and the tiles not
	// clear; each tile's rectangle of the image, made as it is first written
	held  []uint64
	on    []int32
	rects []*render.Image
	// a tile's texels as its rectangle holds them, cut at the image's edge; one clear
	cut, zero []byte
}

// sums is a frame's colour summed into tiles of the aggregate: by tile its place among those
// written, one up, 0 for none; the tiles written, each one's texels row by row, premultiplied,
// aggregateTile a row past the image's edge too, and the signature of what was added to them
type sums struct {
	at     []int32
	tiles  []int32
	blocks [][]byte
	signs  []uint64
}

// newAggregate is the aggregate of a world cut into sectors side a side, empty: a texel of the
// ladder's Texel, a side over 64 for none, raised till the world spans aggregateSide at most
func newAggregate(cfg Config, side float32) aggregate {
	texel := cfg.LOD.Texel
	if texel <= 0 {
		texel = side / 64
	}
	w, h := float32(cfg.Space.Width), float32(cfg.Space.Height)
	texel = max(texel, max(w, h)/aggregateSide)
	cols := min(max(int(f32.Ceil(w/texel)), 1), aggregateSide)
	rows := min(max(int(f32.Ceil(h/texel)), 1), aggregateSide)
	across := (cols + aggregateTile - 1) / aggregateTile
	tiles := across * ((rows + aggregateTile - 1) / aggregateTile)
	g := aggregate{img: render.NewImage(cols, rows), perUnit: 1 / texel, cols: cols, rows: rows,
		across: across, wrapX: cfg.Space.Edges.WrapsX(), wrapY: cfg.Space.Edges.WrapsY(),
		held: make([]uint64, tiles), rects: make([]*render.Image, tiles),
		cut:  make([]byte, 4*aggregateTile*aggregateTile),
		zero: make([]byte, 4*aggregateTile*aggregateTile)}
	g.sums.Back().at = make([]int32, tiles)
	g.sums.Front().at = make([]int32, tiles)
	return g
}

// begin readies the back sums for a frame: no tile written
func (g *aggregate) begin() {
	if g.img == nil {
		return
	}
	s := g.sums.Back()
	for _, t := range s.tiles {
		s.at[t] = 0
	}
	s.tiles, s.signs = s.tiles[:0], s.signs[:0]
}

// add sums the straight colour c into the texels box covers, each by the share of it covered,
// premultiplied, each channel held at a whole: along an axis a box a texel long or less lies
// wholly in the texel under its middle
func (g *aggregate) add(box plane.AABB, c [4]uint8) {
	u0, v0 := box.TopLeft.X*g.perUnit, box.TopLeft.Y*g.perUnit
	u1, v1 := u0+box.Size.X*g.perUnit, v0+box.Size.Y*g.perUnit
	cv := cover{u0: u0, v0: v0, u1: u1, v1: v1}
	cv.x0, cv.nx = span(u0, u1)
	cv.y0, cv.ny = span(v0, v1)
	s := g.sums.Back()
	// a tile's sums are the same where what was added to them is: the box and the colour
	sign := mixed(uint64(math.Float32bits(u0))<<32|uint64(math.Float32bits(v0))) +
		mixed(uint64(math.Float32bits(box.Size.X))<<32|uint64(math.Float32bits(box.Size.Y))^
			uint64(c[0])<<56^uint64(c[1])<<48^uint64(c[2])<<40)
	nx, ny := cv.nx, cv.ny
	if nx <= wideTexels || ny <= wideTexels {
		g.weighed(s, &cv, 0, nx, 0, ny, c, sign)
		return
	}
	// the rim weighed, the first and last rows and the first and last columns between them, the
	// texels within wholly covered; a texel off the image's edge left out
	i0, i1 := inside(cv.x0, nx, g.cols, g.wrapX)
	j0, j1 := inside(cv.y0, ny, g.rows, g.wrapY)
	g.weighed(s, &cv, i0, i1, j0, min(j1, 1), c, sign)
	g.weighed(s, &cv, i0, i1, max(j0, ny-1), j1, c, sign)
	j0, j1 = max(j0, 1), min(j1, ny-1)
	g.weighed(s, &cv, i0, min(i1, 1), j0, j1, c, sign)
	g.weighed(s, &cv, max(i0, nx-1), i1, j0, j1, c, sign)
	g.whole(s, &cv, max(i0, 1), min(i1, nx-1), j0, j1, c, sign)
}

// past so many texels each way a box's sum weighs its rim alone
const wideTexels = 8

// cover is a box in texels, from (u0, v0) to (u1, v1), and the texels it covers: nx from x0 by
// ny from y0
type cover struct {
	u0, v0, u1, v1 float32
	x0, nx, y0, ny int
}

// weighed sums c into the texels [i0, i1) by [j0, j1) of those cv covers, each by the share of it
// covered, adding sign to each tile's signature as it comes to it
func (g *aggregate) weighed(s *sums, cv *cover, i0, i1, j0, j1 int, c [4]uint8, sign uint64) {
	r, gr, b := uint32(c[0]), uint32(c[1]), uint32(c[2])
	last := -1
	var block []byte
	for j := j0; j < j1; j++ {
		// the shares in 256ths, their products in 65536ths
		wy := uint32(along(cv.v0, cv.v1, cv.y0+j, cv.ny)*256 + 0.5)
		y := texelOf(cv.y0+j, g.rows, g.wrapY)
		row := y >> tileShift * g.across
		for i := i0; i < i1; i++ {
			w := uint32(along(cv.u0, cv.u1, cv.x0+i, cv.nx)*256+0.5) * wy
			if w < 128 {
				continue // under half a 255th
			}
			x := texelOf(cv.x0+i, g.cols, g.wrapX)
			if tile := row + x>>tileShift; tile != last {
				last = tile
				block = s.block(tile, sign)
			}
			at := 4 * ((y&(aggregateTile-1))<<tileShift | x&(aggregateTile-1))
			px := block[at : at+4 : at+4]
			px[0] = sat(px[0], (r*w+1<<15)>>16)
			px[1] = sat(px[1], (gr*w+1<<15)>>16)
			px[2] = sat(px[2], (b*w+1<<15)>>16)
			px[3] = sat(px[3], (255*w+1<<15)>>16)
		}
	}
}

// whole sums c into the texels [i0, i1) by [j0, j1) of those cv covers, each wholly covered, a
// run within a tile at a time
func (g *aggregate) whole(s *sums, cv *cover, i0, i1, j0, j1 int, c [4]uint8, sign uint64) {
	r, gr, b := uint32(c[0]), uint32(c[1]), uint32(c[2])
	for j := j0; j < j1; j++ {
		y := texelOf(cv.y0+j, g.rows, g.wrapY)
		row, from := y>>tileShift*g.across, 4*(y&(aggregateTile-1))<<tileShift
		for i := i0; i < i1; {
			x := texelOf(cv.x0+i, g.cols, g.wrapX)
			n := min(i1-i, aggregateTile-x&(aggregateTile-1), g.cols-x)
			at := from + 4*(x&(aggregateTile-1))
			px := s.block(row+x>>tileShift, sign)[at : at+4*n]
			for k := 0; k < len(px); k += 4 {
				q := px[k : k+4 : k+4]
				q[0] = sat(q[0], r)
				q[1] = sat(q[1], gr)
				q[2] = sat(q[2], b)
				q[3] = 255
			}
			i += n
		}
	}
}

// inside is the run [lo, hi) of the n texels from first along an axis of size texels that lie on
// the image: every one where it wraps
func inside(first, n, size int, wraps bool) (lo, hi int) {
	if wraps {
		return 0, n
	}
	return min(max(-first, 0), n), max(min(size-first, n), 0)
}

// span is the first texel [lo, hi) covers along an axis and how many: one, that under its middle,
// where it is a texel long or less
func span(lo, hi float32) (first, n int) {
	if hi-lo <= 1 {
		return floor((lo + hi) / 2), 1
	}
	first = floor(lo)
	return first, -floor(-hi) - first
}

// floor is x floored without going through float64
func floor(x float32) int {
	i := int(x)
	if x < 0 && float32(i) != x {
		i--
	}
	return i
}

// along is the share of the texel at that [lo, hi) covers along an axis, of the n it covers: the
// whole of its length where it lies in one
func along(lo, hi float32, at, n int) float32 {
	if n == 1 {
		return hi - lo
	}
	return min(hi, float32(at+1)) - max(lo, float32(at))
}

// texelOf is the texel at among n along an axis: round it where the world wraps, else at its end
func texelOf(at, n int, wraps bool) int {
	switch {
	case at >= 0 && at < n:
		return at
	case wraps:
		if at %= n; at < 0 {
			at += n
		}
		return at
	}
	return min(max(at, 0), n-1)
}

func sat(a uint8, b uint32) uint8 { return uint8(min(uint32(a)+b, 255)) }

// block is tile's texels as s holds them, opened clear where none is written yet, sign added to
// its signature
func (s *sums) block(tile int, sign uint64) []byte {
	k := s.at[tile]
	if k == 0 {
		k = s.open(tile)
	}
	s.signs[k-1] += sign
	return s.blocks[k-1]
}

// open has tile written by s, its texels clear, and is its place among those written, one up
func (s *sums) open(tile int) int32 {
	n := len(s.tiles)
	if n == len(s.blocks) {
		s.blocks = append(s.blocks, make([]byte, 4*aggregateTile*aggregateTile))
	}
	clear(s.blocks[n])
	s.tiles = append(s.tiles, int32(tile))
	s.signs = append(s.signs, 0)
	s.at[tile] = int32(n + 1)
	return int32(n + 1)
}

// live reports whether the sums published last hold colour, as the image does once they are put
// on the GPU
func (g *aggregate) live() bool { return g.img != nil && len(g.sums.Front().tiles) > 0 }

// swap has the sums published last put on the GPU as the next frame is composed
func (g *aggregate) swap() {
	g.sums.Swap()
	g.fresh = true
}

// upload has the image hold the sums published last, once for them: the tiles whose sums differ
// from what it holds written, those summed no more cleared
func (g *aggregate) upload() {
	if g.img == nil || !g.fresh {
		return
	}
	g.fresh = false
	f := g.sums.Front()
	for _, t := range g.on {
		if f.at[t] == 0 {
			g.held[t] = 0
			g.write(int(t), g.zero)
		}
	}
	// the signature spares writing a tile whose figures stand still
	for k, t := range f.tiles {
		if sign := f.signs[k] | 1; g.held[t] != sign {
			g.held[t] = sign
			g.write(int(t), f.blocks[k])
		}
	}
	g.on = append(g.on[:0], f.tiles...)
}

func (g *aggregate) write(tile int, pix []byte) {
	r := g.rects[tile]
	if r == nil {
		x, y := tile%g.across*aggregateTile, tile/g.across*aggregateTile
		r = g.img.SubImage(image.Rect(x, y, min(x+aggregateTile, g.cols),
			min(y+aggregateTile, g.rows)))
		g.rects[tile] = r
	}
	w, h := r.Bounds().Dx(), r.Bounds().Dy()
	if w < aggregateTile {
		for y := range h {
			copy(g.cut[4*w*y:4*w*(y+1)], pix[4*aggregateTile*y:])
		}
		pix = g.cut
	}
	r.WritePixels(pix[:4*w*h])
}

// mixed is x's bits mixed (splitmix64's finaliser), so a share of any run of numbers is a share
// of the whole
func mixed(x uint64) uint64 {
	x = (x ^ x>>30) * 0xbf58476d1ce4e5b9
	x = (x ^ x>>27) * 0x94d049bb133111eb
	return x ^ x>>31
}
