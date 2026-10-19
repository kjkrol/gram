package terrain

import (
	"math"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/world/view"
)

// span is a run of the ground's triangles' indices, shaded with care
type span struct{ first, count, care int }

// The cares the ground is shaded with (mesh.wgsl's CARE), the full down to the least, each
// dropping one more thing:
//
//	4  everything
//	3  the shore: no swell, surf or foam, no water running down the ways, no glint at a mouth
//	2  the waves and their glint: the still water a level mirror of the sky
//	1  the baked shade and the lamps: every cell takes every shade
//	0  the grid, the clouds' shadow and the water's picture: the albedo's flat colour
const (
	LeastCare = 0
	cares     = 5
	FullCare  = cares - 1
)

// a sector's triangles whole and coarse
type sectorSpans struct{ whole, coarse span }

// the cells [x0, x1) x [y0, y1) of the lattice
type cells struct{ x0, y0, x1, y1 int }

// the cells whose middles lie in box, of a lattice of cols x rows corners a cell apart
func cellsIn(box geom.AABB, cell float64, cols, rows int) cells {
	at := func(v float64, n int) int { return min(max(int(math.Ceil(v/cell-0.5)), 0), n-1) }
	return cells{at(box.TopLeft.X, cols), at(box.TopLeft.Y, rows), at(box.BottomRight.X, cols),
		at(box.BottomRight.Y, rows)}
}

// a split of the lattice's quads along the diagonal whose corners stand nearer, enc the heights
type lattice struct {
	cols int
	enc  []byte
}

func (l lattice) at(x, y int) uint32 { return uint32(y*l.cols + x) }

func (l lattice) height(i uint32) int { return int(l.enc[4*i])<<8 | int(l.enc[4*i+1]) }

func (l lattice) apart(a, b uint32) int {
	d := l.height(a) - l.height(b)
	if d < 0 {
		return -d
	}
	return d
}

// the quad of corners i0 top-left, i1 top-right, i2 bottom-left, i3 bottom-right, as two
// triangles
func (l lattice) quad(i0, i1, i2, i3 uint32, dst []uint32) []uint32 {
	if l.apart(i0, i3) < l.apart(i1, i2) {
		return append(dst, i0, i1, i3, i0, i3, i2)
	}
	return append(dst, i0, i1, i2, i1, i3, i2)
}

// every cell of c, two triangles each
func (l lattice) whole(c cells, dst []uint32) []uint32 {
	for y := c.y0; y < c.y1; y++ {
		for x := c.x0; x < c.x1; x++ {
			dst = l.quad(l.at(x, y), l.at(x+1, y), l.at(x, y+1), l.at(x+1, y+1), dst)
		}
	}
	return dst
}

// c with every second corner within: two cells by two a quad, and round them a ring of whole
// cells, stitched to them by fans, so the edge of c meets a whole neighbour without a crack
func (l lattice) coarse(c cells, dst []uint32) []uint32 {
	// within the ring, an even number of cells each way; an odd one leaves a second ring cell
	in := cells{c.x0 + 1, c.y0 + 1, c.x1 - 1, c.y1 - 1}
	in.x1 -= (in.x1 - in.x0) % 2
	in.y1 -= (in.y1 - in.y0) % 2
	if in.x1-in.x0 < 2 || in.y1-in.y0 < 2 {
		return l.whole(c, dst)
	}
	for y := in.y0; y < in.y1; y += 2 {
		for x := in.x0; x < in.x1; x += 2 {
			dst = l.quad(l.at(x, y), l.at(x+2, y), l.at(x, y+2), l.at(x+2, y+2), dst)
		}
	}
	for x := in.x0; x < in.x1; x += 2 {
		// above: the whole corners on the ring's outer edge, the coarse ones on the inner
		a, b, cc := l.at(x, in.y0-1), l.at(x+1, in.y0-1), l.at(x+2, in.y0-1)
		A, C := l.at(x, in.y0), l.at(x+2, in.y0)
		dst = append(dst, a, b, A, b, cc, C, b, C, A)
		// below
		A, C = l.at(x, in.y1), l.at(x+2, in.y1)
		a, b, cc = l.at(x, in.y1+1), l.at(x+1, in.y1+1), l.at(x+2, in.y1+1)
		dst = append(dst, A, C, b, A, b, a, C, cc, b)
	}
	for y := in.y0; y < in.y1; y += 2 {
		// left
		a, b, cc := l.at(in.x0-1, y), l.at(in.x0-1, y+1), l.at(in.x0-1, y+2)
		A, C := l.at(in.x0, y), l.at(in.x0, y+2)
		dst = append(dst, a, A, b, A, C, b, b, C, cc)
		// right
		A, C = l.at(in.x1, y), l.at(in.x1, y+2)
		a, b, cc = l.at(in.x1+1, y), l.at(in.x1+1, y+1), l.at(in.x1+1, y+2)
		dst = append(dst, A, a, b, A, b, C, C, b, cc)
	}
	// the ring's cells along no stitched side: its corners, and the second ring an odd side
	// leaves
	for y := c.y0; y < c.y1; y++ {
		for x := c.x0; x < c.x1; x++ {
			inside := x >= in.x0 && x < in.x1 && y >= in.y0 && y < in.y1
			stitched := x >= in.x0 && x < in.x1 && (y == in.y0-1 || y == in.y1) ||
				y >= in.y0 && y < in.y1 && (x == in.x0-1 || x == in.x1)
			if !inside && !stitched {
				dst = l.quad(l.at(x, y), l.at(x+1, y), l.at(x, y+1), l.at(x+1, y+1), dst)
			}
		}
	}
	return dst
}

// the ground's triangles, by sector where the world has them, each whole and then each coarse,
// and the skirt after
func (r *Renderer) lay(cols, rows int, cell float32, enc []byte) {
	l := lattice{cols: cols, enc: enc}
	r.tris = r.tris[:0]
	if r.grid == nil {
		r.tris = l.whole(cells{0, 0, cols - 1, rows - 1}, r.tris)
	} else {
		n := r.grid.Len()
		r.spans = append(r.spans[:0], make([]sectorSpans, n)...)
		for i := range n {
			from := len(r.tris)
			r.tris = l.whole(cellsIn(r.grid.Box(i), float64(cell), cols, rows), r.tris)
			r.spans[i].whole = span{first: from, count: len(r.tris) - from}
		}
		for i := range n {
			from := len(r.tris)
			r.tris = l.coarse(cellsIn(r.grid.Box(i), float64(cell), cols, rows), r.tris)
			r.spans[i].coarse = span{first: from, count: len(r.tris) - from}
		}
	}
	from := len(r.tris)
	r.tris = skirt(cols, rows, r.tris)
	r.skirted = span{first: from, count: len(r.tris) - from}
}

// the runs of the ground drawn through cam: the sectors its View sees at their levels, each
// with its ring's care, joined where they follow one another with one care, and the skirt; the
// whole mesh, with full care, in a world of no sectors
func (r *Renderer) runs(cam camera.Camera, dst []span) []span {
	r.counts = [cares]int{}
	if r.grid == nil {
		return append(dst, span{0, len(r.tris), FullCare})
	}
	careOf := r.CareOf(cam)
	v := r.cfg.Seen(cam)
	v.Sectors(func(i int, _ geom.AABB, level view.Level) {
		s := r.spans[i].whole
		if level != view.Exact {
			s = r.spans[i].coarse
		}
		if s.count == 0 {
			return
		}
		s.care = careOf(i)
		r.counts[s.care]++
		dst = append(dst, s)
	})
	skirt := r.skirted
	skirt.care = careOf(-1)
	dst = append(dst, skirt)
	sortSpans(dst)
	joined := dst[:0]
	for _, s := range dst {
		if n := len(joined); n > 0 && joined[n-1].first+joined[n-1].count == s.first &&
			joined[n-1].care == s.care {
			joined[n-1].count += s.count
			continue
		}
		joined = append(joined, s)
	}
	return joined
}

// CareOf is the care sector i is shaded with through cam, the skirt's for i < 0: full within
// Care rings of the sector under the middle of the screen or under the eye, one less every Care
// rings farther, down to the least; full everywhere with no care set, in a world of no sectors
// or through a camera knowing no middle.
func (r *Renderer) CareOf(cam camera.Camera) func(i int) int {
	full := func(int) int { return FullCare }
	f, ok := cam.(camera.Focused)
	if r.care <= 0 || !ok || r.cfg.Seen == nil {
		return full
	}
	g := r.cfg.Seen(cam).Grid()
	if g == nil {
		return full
	}
	x, y, _ := f.Focus()
	middle := g.Of(float64(x), float64(y))
	eye := middle
	if e, ok := cam.(camera.Eyed); ok {
		if x, y, _, ok := e.Eye(); ok {
			eye = g.Of(float64(x), float64(y))
		}
	}
	cols := g.Cols()
	return func(i int) int {
		if i < 0 {
			return LeastCare
		}
		d := min(rings(i, middle, cols), rings(i, eye, cols))
		band := 0
		if d > 0 {
			band = (d - 1) / r.care
		}
		return max(FullCare-band, 0)
	}
}

// how many rings of sectors out from b sector a lies, on a grid cols wide
func rings(a, b, cols int) int {
	dx, dy := a%cols-b%cols, a/cols-b/cols
	return max(dx, -dx, dy, -dy)
}

func sortSpans(s []span) {
	for i := 1; i < len(s); i++ {
		for k := i; k > 0 && s[k].first < s[k-1].first; k-- {
			s[k], s[k-1] = s[k-1], s[k]
		}
	}
}
