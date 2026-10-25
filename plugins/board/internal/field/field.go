package field

import (
	"math"
	"slices"

	"github.com/kjkrol/aabbworld/collide"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/ground"
	"github.com/kjkrol/gram/plugins/board/internal/grids"
	"github.com/kjkrol/gram/plugins/board/internal/terrain"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/world"
)

var (
	_ ground.Cover    = (*Field)(nil)
	_ collision.Field = (*Field)(nil)
)

// Field is a board's ground for the others: what holds sight back (Walk) and what collision
// pushes colliders out of and never over (Solid, Overhang), read from the cells' terrain, for one
// goroutine at a time.
type Field struct {
	grid     grid.Grid
	square   *grids.Square // nil for any other grid
	cells    *terrain.Read
	altitude func(c cell.ID) float32
	veils    *[]veil // by ordinal, the board's: kept by Veil and Veils as the cells change
	boxes    []geom.AABB
	decks    []terrain.Deck
	seen     []cell.ID
}

// New is the ground of the board over g read through cells, its levels as altitude says.
func New(g grid.Grid, cells *terrain.Read, altitude func(c cell.ID) float32) *Field {
	sq, _ := g.(*grids.Square)
	veils := make([]veil, g.CellCount())
	return &Field{grid: g, square: sq, cells: cells, altitude: altitude, veils: &veils}
}

// Fork is the same ground read through cells and altitude of another goroutine, with scratch of
// its own.
func (f *Field) Fork(cells *terrain.Read, altitude func(c cell.ID) float32) *Field {
	return &Field{grid: f.grid, square: f.square, cells: cells, altitude: altitude, veils: f.veils}
}

// Version and Changes are the cells' counts: the cover's copies are kept by them.
func (f *Field) Version() uint64 { return f.cells.Version() }
func (f *Field) Changes() uint64 { return f.cells.Changes() }

// Walk is the cover of the board along a ray, the Cover contract: every cell whose kind has a
// Veil that dims an observer on blockers, in order, with the stretch of the ray inside it, its
// band (±Inf for a kind of no Height) and τ = 1 − Veil.
func (f *Field) Walk(origin, dir geom.Vec, length float32, blockers world.Layers, visit func(near, far, bottom, top, tau float32) bool) {
	if f.square != nil {
		f.walkSquare(origin, dir, length, blockers, visit)
		return
	}
	f.walkSteps(origin, dir, length, blockers, visit)
}

func (f *Field) covers(c cell.ID, blockers world.Layers) (bottom, top, tau float32, ok bool) {
	i, ok := f.cells.Ordinal(c)
	if !ok {
		return 0, 0, 0, false
	}
	v := &(*f.veils)[i]
	if !v.ok || !v.veils.Meets(blockers) {
		return 0, 0, 0, false
	}
	return v.bottom, v.top, v.tau, true
}

type veil struct {
	bottom, top, tau float32
	veils            world.Layers
	ok               bool
}

// Veil reads c's cover anew, the cell changed; on the goroutine writing the cells.
func (f *Field) Veil(c cell.ID) {
	i, ok := f.cells.Ordinal(c)
	if !ok {
		return
	}
	v := &(*f.veils)[i]
	k := f.cells.KindOf(c)
	if k == nil || k.Veil <= 0 {
		*v = veil{}
		return
	}
	*v = veil{bottom: f32.Inf(-1), top: f32.Inf(1), tau: 1 - min(k.Veil, 1), veils: world.Layers(k.Veils), ok: true}
	// a veil of no height stands at every height, as a wall of none does
	if k.Height > 0 {
		v.bottom = f.altitude(c)
		v.top = v.bottom + k.Height
	}
}

// Veils reads every cell's cover anew, all of them changed; on the goroutine writing the cells.
func (f *Field) Veils() { f.grid.EachCell(f.Veil) }

// the walk sums t cell after cell, so in float64
func (f *Field) walkSquare(origin, dir geom.Vec, length float32, blockers world.Layers, visit func(near, far, bottom, top, tau float32) bool) {
	sq := f.square
	size := float64(sq.CellSize)
	if size == 0 {
		return
	}
	ox, oy, end := float64(origin.X), float64(origin.Y), float64(length)
	cx, cy := int64(math.Floor(ox/size)), int64(math.Floor(oy/size))
	sx, nx, dx := ddaAxis(ox, float64(dir.X), cx, size)
	sy, ny, dy := ddaAxis(oy, float64(dir.Y), cy, size)
	for t := 0.0; t < end; {
		exit := min(nx, ny, end)
		if c, ok := sq.Fold(cx, cy); ok {
			if bottom, top, tau, ok := f.covers(c, blockers); ok &&
				!visit(float32(t), float32(exit), bottom, top, tau) {
				return
			}
		}
		if nx < ny {
			cx, t, nx = cx+sx, nx, nx+dx
		} else {
			cy, t, ny = cy+sy, ny, ny+dy
		}
	}
}

func ddaAxis(o, d float64, c int64, size float64) (step int64, next, delta float64) {
	switch {
	case d > 0:
		return 1, (float64(c+1)*size - o) / d, size / d
	case d < 0:
		return -1, (float64(c)*size - o) / d, -size / d
	}
	return 0, math.Inf(1), math.Inf(1)
}

func (f *Field) walkSteps(origin, dir geom.Vec, length float32, blockers world.Layers, visit func(near, far, bottom, top, tau float32) bool) {
	w, h := f.grid.CellBounds()
	step := float64(min(w, h)) / 4
	if step <= 0 {
		return
	}
	var cur cell.ID
	have, start := false, float32(0)
	emit := func(end float32) bool {
		if !have {
			return true
		}
		bottom, top, tau, ok := f.covers(cur, blockers)
		return !ok || visit(start, end, bottom, top, tau)
	}
	ox, oy, end := float64(origin.X), float64(origin.Y), float64(length)
	for t := 0.0; ; t = min(t+step, end) {
		at := geom.NewVec(float32(ox+float64(dir.X)*t), float32(oy+float64(dir.Y)*t))
		c, ok := f.grid.CellAt(at)
		if !ok || !have || c != cur {
			if !emit(float32(t)) {
				return
			}
			cur, have, start = c, ok, float32(t)
		}
		if t >= end {
			emit(length)
			return
		}
	}
}

// Solid is the solid ground of the board round box for an entity on layers spanning band, the
// collision.Field contract: every cell that stops it, and every raised deck's slab over its step,
// as boxes. A side is open where the neighbour across it does not stop the entity; on a wrapping
// square grid the boxes lie in box's frame.
func (f *Field) Solid(layers world.Layers, band collision.Band, box geom.AABB, visit func(collide.FieldBox) bool) {
	sq := f.square
	if sq == nil {
		f.solidCells(layers, band, box, visit)
		return
	}
	size := float32(sq.CellSize)
	if size == 0 {
		return
	}
	x0, y0 := int64(f32.Floor(box.TopLeft.X/size)), int64(f32.Floor(box.TopLeft.Y/size))
	x1, y1 := int64(f32.Ceil(box.BottomRight.X/size)), int64(f32.Ceil(box.BottomRight.Y/size))
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			c, ok := f.square.Fold(x, y)
			if !ok || !f.stops(c, layers, band) {
				continue
			}
			var open collide.Sides
			for _, n := range [4]struct {
				dx, dy int64
				side   collide.Sides
			}{{-1, 0, collide.Left}, {1, 0, collide.Right}, {0, -1, collide.Top}, {0, 1, collide.Bottom}} {
				if nc, ok := f.square.Fold(x+n.dx, y+n.dy); !ok || !f.stops(nc, layers, band) {
					open |= n.side
				}
			}
			fb := collide.FieldBox{Box: geom.NewAABBAt(geom.NewVec(float32(x)*size, float32(y)*size), size, size), Cell: uint64(c), Open: open}
			if !visit(fb) {
				return
			}
		}
	}
}

// Walls are the railings and the slabs of the decks running as bands round box, for an entity on
// layers spanning band, the collision.Field contract. Their ends are free where the bridge ends or
// a head takes over.
func (f *Field) Walls(layers world.Layers, band collision.Band, box geom.AABB,
	visit func(collide.FieldWall) bool) {
	if !f.cells.Banded() {
		return
	}
	f.seen = f.seen[:0]
	if sq := f.square; sq != nil {
		size := float32(sq.CellSize)
		x0 := int64(f32.Floor(box.TopLeft.X / size))
		y0 := int64(f32.Floor(box.TopLeft.Y / size))
		x1 := int64(f32.Ceil(box.BottomRight.X / size))
		y1 := int64(f32.Ceil(box.BottomRight.Y / size))
		for y := y0; y < y1; y++ {
			for x := x0; x < x1; x++ {
				c, ok := sq.Fold(x, y)
				m := geom.NewVec((float32(x)+0.5)*size, (float32(y)+0.5)*size)
				if ok && !f.wallsOf(c, m, layers, band, visit) {
					return
				}
			}
		}
		return
	}
	done := false
	f.grid.CellsUnder(box, func(c cell.ID) {
		done = done || !f.wallsOf(c, f.grid.CellCenter(c), layers, band, visit)
	})
}

func (f *Field) wallsOf(c cell.ID, m geom.Vec, layers world.Layers, band collision.Band,
	visit func(collide.FieldWall) bool) bool {
	f.decks = f.cells.Bands(c, f.decks[:0])
	for _, d := range f.decks {
		if slices.Contains(f.seen, d.Cell) {
			continue
		}
		f.seen = append(f.seen, d.Cell)
		fl, top := d.Built.Floor, f.altitude(d.Cell)+d.Built.Floor.Height
		a, b, freeA, freeB, ok := f.cells.Ends(d.Cell, fl, geom.NewVec(m.X+d.Off.X, m.Y+d.Off.Y))
		if !ok {
			continue
		}
		open := ends(freeA, freeB)
		if fl.Height > 0 && fl.Thickness > 0 &&
			(collision.Band{Bottom: top - fl.Thickness, Top: top}).Meets(band.Above()) {
			slab := collide.FieldWall{
				A:    a,
				B:    b,
				Half: fl.Width / 2,
				Cell: uint64(d.Cell),
				Open: open,
			}
			if !visit(slab) {
				return false
			}
		}
		r := d.Built.Rail
		railing := collision.Band{Bottom: top, Top: top + r.Height}
		if r.Keeps == 0 || !world.Layers(r.Keeps).Meets(layers) ||
			r.Height > 0 && !railing.Meets(band) {
			continue
		}
		half := terrain.RailHalf(f.grid.CellSpan())
		nx, ny := b.Y-a.Y, a.X-b.X
		l := f32.Hypot(nx, ny)
		nx, ny = nx/l*fl.Width/2, ny/l*fl.Width/2
		for _, side := range [2]float32{-1, 1} {
			w := collide.FieldWall{
				A:    geom.NewVec(a.X+side*nx, a.Y+side*ny),
				B:    geom.NewVec(b.X+side*nx, b.Y+side*ny),
				Half: half,
				Cell: uint64(d.Cell),
				Open: open,
			}
			if !visit(w) {
				return false
			}
		}
	}
	return true
}

func ends(freeA, freeB bool) collide.Ends {
	var e collide.Ends
	if freeA {
		e |= collide.EndA
	}
	if freeB {
		e |= collide.EndB
	}
	return e
}

// Overhang is how much of box, as area, lies over ground that does not take an entity on layers,
// a deck taking one left out: the collision.Field contract, a push never makes it more. Off the
// board is none of it.
func (f *Field) Overhang(layers world.Layers, box geom.AABB) float32 {
	sq := f.square
	if sq == nil {
		return f.overhangCells(layers, box)
	}
	size := float32(sq.CellSize)
	if size == 0 {
		return 0
	}
	var area float32
	x0, y0 := int64(f32.Floor(box.TopLeft.X/size)), int64(f32.Floor(box.TopLeft.Y/size))
	x1, y1 := int64(f32.Ceil(box.BottomRight.X/size)), int64(f32.Ceil(box.BottomRight.Y/size))
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			if c, ok := f.square.Fold(x, y); ok {
				at := geom.NewVec(float32(x)*size, float32(y)*size)
				m := geom.NewVec(at.X+size/2, at.Y+size/2)
				area += f.bare(c, m, layers, box, geom.NewAABBAt(at, size, size))
			}
		}
	}
	return area
}

func (f *Field) overhangCells(layers world.Layers, box geom.AABB) float32 {
	var area float32
	f.grid.CellsUnder(box, func(c cell.ID) {
		m := f.grid.CellCenter(c)
		f.boxes = f.grid.CellBoxes(c, f.boxes[:0])
		for _, cb := range f.boxes {
			area += f.bare(c, m, layers, box, cb)
		}
	})
	return area
}

func (f *Field) bare(c cell.ID, m geom.Vec, layers world.Layers, box, cb geom.AABB) float32 {
	r, ok := meet(box, cb)
	k := f.cells.KindOf(c)
	if !ok || k == nil {
		return 0
	}
	b := f.cells.Built(c)
	allows := k.Allows
	if b.Floor.Width == 0 {
		allows |= b.Floor.Allows
	}
	if admits(allows&^b.Wall.Keeps, layers) {
		return 0
	}
	area := (r.BottomRight.X - r.TopLeft.X) * (r.BottomRight.Y - r.TopLeft.Y)
	if !f.cells.Banded() {
		return area
	}
	f.decks = f.cells.Bands(c, f.decks[:0])
	for _, d := range f.decks {
		if admits(d.Built.Floor.Allows&^b.Wall.Keeps, layers) {
			area -= f.cells.BandArea(d.Built.Floor, geom.NewVec(m.X+d.Off.X, m.Y+d.Off.Y), r)
		}
	}
	return max(area, 0)
}

// no layers: an entity on every plane
func admits(allows cell.Domain, layers world.Layers) bool {
	return allows&cell.Domain(layers) != 0 || layers == 0 && allows != 0
}

func meet(a, b geom.AABB) (geom.AABB, bool) {
	tl := geom.NewVec(max(a.TopLeft.X, b.TopLeft.X), max(a.TopLeft.Y, b.TopLeft.Y))
	br := geom.NewVec(min(a.BottomRight.X, b.BottomRight.X), min(a.BottomRight.Y, b.BottomRight.Y))
	return geom.NewAABB(tl, br), br.X > tl.X && br.Y > tl.Y
}

func overlap(a, b geom.AABB) float32 {
	w := min(a.BottomRight.X, b.BottomRight.X) - max(a.TopLeft.X, b.TopLeft.X)
	h := min(a.BottomRight.Y, b.BottomRight.Y) - max(a.TopLeft.Y, b.TopLeft.Y)
	if w <= 0 || h <= 0 {
		return 0
	}
	return w * h
}

func (f *Field) solidCells(layers world.Layers, band collision.Band, box geom.AABB, visit func(collide.FieldBox) bool) {
	const all = collide.Left | collide.Right | collide.Top | collide.Bottom
	done := false
	f.grid.CellsUnder(box, func(c cell.ID) {
		if done || !f.stops(c, layers, band) {
			return
		}
		f.boxes = f.grid.CellBoxes(c, f.boxes[:0])
		for _, cb := range f.boxes {
			if !visit(collide.FieldBox{Box: cb, Cell: uint64(c), Open: all}) {
				done = true
				return
			}
		}
	})
}

func (f *Field) stops(c cell.ID, layers world.Layers, band collision.Band) bool {
	k := f.cells.KindOf(c)
	if solidFor(k, layers) && f.solidBand(c, k.Height).Meets(band) {
		return true
	}
	b := f.cells.Built(c)
	keeps := world.Layers(b.Wall.Keeps)
	if keeps != 0 && keeps.Meets(layers) && f.solidBand(c, b.Wall.Height).Meets(band) {
		return true
	}
	fl := b.Floor
	if fl.Thickness <= 0 || fl.Height <= 0 || fl.Width > 0 {
		return false
	}
	deck := f.altitude(c) + fl.Height
	return collision.Band{Bottom: deck - fl.Thickness, Top: deck}.Meets(band.Above())
}

// from below up, so nothing passes under a wall on a slope; a wall of no height stands at every
// height
func (f *Field) solidBand(c cell.ID, height float32) collision.Band {
	if height <= 0 {
		return collision.Everywhere
	}
	return collision.Band{Bottom: f32.Inf(-1), Top: f.altitude(c) + height}
}

func solidFor(k *cell.Kind, layers world.Layers) bool {
	return k != nil && k.Solid && world.Layers(^uint8(k.Allows)).Meets(layers)
}
