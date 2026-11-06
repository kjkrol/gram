package terrain

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/lod"
)

// span is a run of the ground's triangles' indices, drawn at a level
type span struct {
	first, count int
	level        lod.Level
}

// the skirt to the horizon is drawn as a sheet, the coarsest the ground draws
const skirtLevel = lod.L5

// a sector's triangles whole, coarse (every second corner) and coarser (every fourth)
type sectorSpans struct{ whole, coarse, coarser span }

// the cells [x0, x1) x [y0, y1) of the lattice
type cells struct{ x0, y0, x1, y1 int }

// the cells whose middles lie in box, of a lattice of cols x rows corners a cell apart
func cellsIn(box geom.AABB, cell float32, cols, rows int) cells {
	at := func(v float32, n int) int { return min(max(int(f32.Ceil(v/cell-0.5)), 0), n-1) }
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

// c with every step-th corner within: step cells by step a quad, and round them a ring stitched
// to them by fans, its outer edge every corner of c, so the edge of c meets a neighbour of any
// step without a crack; a c too small for step is laid at half of it
func (l lattice) coarse(c cells, step int, dst []uint32) []uint32 {
	if step < 2 {
		return l.whole(c, dst)
	}
	// within a ring of a cell, a multiple of step each way, what is left over split between the
	// ring's two sides
	in := cells{c.x0 + 1, c.y0 + 1, c.x1 - 1, c.y1 - 1}
	if in.x1-in.x0 < step || in.y1-in.y0 < step {
		return l.coarse(c, step/2, dst)
	}
	left, top := (in.x1-in.x0)%step, (in.y1-in.y0)%step
	in.x0, in.x1 = in.x0+left/2, in.x1-(left-left/2)
	in.y0, in.y1 = in.y0+top/2, in.y1-(top-top/2)
	for y := in.y0; y < in.y1; y += step {
		for x := in.x0; x < in.x1; x += step {
			dst = l.quad(l.at(x, y), l.at(x+step, y), l.at(x, y+step), l.at(x+step, y+step), dst)
		}
	}
	// each side a fan a coarse quad: its inner corners to every outer corner of c along it
	for x := in.x0; x < in.x1; x += step {
		dst = l.fan(x, in.y0, x+step, in.y0, x, c.y0, 1, 0, step, dst)
		dst = l.fan(x, in.y1, x+step, in.y1, x, c.y1, 1, 0, step, dst)
	}
	for y := in.y0; y < in.y1; y += step {
		dst = l.fan(in.x0, y, in.x0, y+step, c.x0, y, 0, 1, step, dst)
		dst = l.fan(in.x1, y, in.x1, y+step, c.x1, y, 0, 1, step, dst)
	}
	// each corner of the ring a fan from the corner of the coarse quads
	corner := func(ix, iy, ox, oy int) {
		dx, dy := sign(ox-ix), sign(iy-oy)
		for x := ix; x != ox; x += dx {
			dst = l.tri(ix, iy, x, oy, x+dx, oy, dst)
		}
		for y := oy; y != iy; y += dy {
			dst = l.tri(ix, iy, ox, y, ox, y+dy, dst)
		}
	}
	corner(in.x0, in.y0, c.x0, c.y0)
	corner(in.x1, in.y0, c.x1, c.y0)
	corner(in.x0, in.y1, c.x0, c.y1)
	corner(in.x1, in.y1, c.x1, c.y1)
	return dst
}

// the fan from the inner corners (ax, ay) and (bx, by) to the n+1 outer corners from (x, y) a
// (dx, dy) apart: the first n/2 edges between them to a, the rest to b, and a, b and the corner
// between the halves
func (l lattice) fan(ax, ay, bx, by, x, y, dx, dy, n int, dst []uint32) []uint32 {
	h := n / 2
	for k := range n {
		px, py := ax, ay
		if k >= h {
			px, py = bx, by
		}
		dst = l.tri(px, py, x+k*dx, y+k*dy, x+(k+1)*dx, y+(k+1)*dy, dst)
	}
	return l.tri(ax, ay, bx, by, x+h*dx, y+h*dy, dst)
}

// the triangle of the corners a, b and c, turned as a whole cell's triangles turn
func (l lattice) tri(ax, ay, bx, by, cx, cy int, dst []uint32) []uint32 {
	if (bx-ax)*(cy-ay)-(by-ay)*(cx-ax) < 0 {
		bx, by, cx, cy = cx, cy, bx, by
	}
	return append(dst, l.at(ax, ay), l.at(bx, by), l.at(cx, cy))
}

func sign(v int) int {
	if v < 0 {
		return -1
	}
	return 1
}

// the ground's triangles, by sector where the world has them, each whole, then each coarse, then
// each coarser, and the skirt after
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
			r.tris = l.whole(cellsIn(r.grid.Box(i), cell, cols, rows), r.tris)
			r.spans[i].whole = span{first: from, count: len(r.tris) - from}
		}
		for i := range n {
			from := len(r.tris)
			r.tris = l.coarse(cellsIn(r.grid.Box(i), cell, cols, rows), 2, r.tris)
			r.spans[i].coarse = span{first: from, count: len(r.tris) - from}
		}
		for i := range n {
			from := len(r.tris)
			r.tris = l.coarse(cellsIn(r.grid.Box(i), cell, cols, rows), 4, r.tris)
			r.spans[i].coarser = span{first: from, count: len(r.tris) - from}
		}
	}
	from := len(r.tris)
	r.tris = skirt(cols, rows, r.tris)
	r.skirted = span{first: from, count: len(r.tris) - from}
}

// the runs of the ground drawn through cam: the sectors its View sees, each at its level, its
// whole triangles short of Strategic, its coarse ones at it and its coarser ones beyond, joined
// where they follow one another at one level, and the skirt at skirtLevel; the whole mesh at
// Standard, its skirt too, in a world of no sectors, which has no levels
func (r *Renderer) runs(cam camera.Camera, dst []span) []span {
	r.counts = [lod.Levels]int{}
	if r.grid == nil {
		return append(dst, span{0, len(r.tris), lod.Standard})
	}
	v := r.cfg.Seen(cam)
	v.Sectors(func(i int, _ geom.AABB, level lod.Level) {
		s := r.spans[i].whole
		switch {
		case level > lod.Strategic:
			s = r.spans[i].coarser
		case level == lod.Strategic:
			s = r.spans[i].coarse
		}
		if s.count == 0 {
			return
		}
		s.level = level
		r.counts[level]++
		dst = append(dst, s)
	})
	skirt := r.skirted
	skirt.level = skirtLevel
	dst = append(dst, skirt)
	sortSpans(dst)
	joined := dst[:0]
	for _, s := range dst {
		if n := len(joined); n > 0 && joined[n-1].first+joined[n-1].count == s.first &&
			joined[n-1].level == s.level {
			joined[n-1].count += s.count
			continue
		}
		joined = append(joined, s)
	}
	return joined
}

func sortSpans(s []span) {
	for i := 1; i < len(s); i++ {
		for k := i; k > 0 && s[k].first < s[k-1].first; k-- {
			s[k], s[k-1] = s[k-1], s[k]
		}
	}
}
