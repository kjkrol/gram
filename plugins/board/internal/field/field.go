package field

import (
	"math"
	"slices"

	"github.com/kjkrol/aabbworld/collide"
	"github.com/kjkrol/aabbworld/geom"
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
	_ ground.Readied  = (*Field)(nil)
	_ collision.Field = (*Field)(nil)
)

// Field is a board's ground for the others: what holds sight back (Walk, Ready) and what collision
// pushes colliders out of and never over (Solid, Overhang), read from the cells' terrain.
type Field struct {
	grid     grid.Grid
	square   *grids.Square // the grid when it is square, walked exactly; nil otherwise
	cells    *terrain.Cells
	heights  bool                    // the world has heights: cover and solid cells span the cells' bands
	altitude func(c cell.ID) float64 // a cell's ground level, the Map's
	boxes    []geom.AABB             // scratch for the boxes of a cell
	decks    []terrain.Deck          // scratch for the bands round a cell
	seen     []cell.ID               // the bands whose walls a walk has visited
	// veils holds by ordinal the cover of every cell (Ready), good while the cells' changes and
	// version are as they were
	veils        []veil
	veilsChanges uint64
	veilsVersion uint64
}

// New is the ground of the board over g with its cells, its levels as altitude says.
func New(g grid.Grid, cells *terrain.Cells, altitude func(c cell.ID) float64) *Field {
	sq, _ := g.(*grids.Square)
	return &Field{grid: g, square: sq, cells: cells, altitude: altitude}
}

// SetHeights says whether the world has heights, so cover and solid cells span the cells' bands.
func (f *Field) SetHeights(heights bool) { f.heights = heights }

// Walk is the cover of the board along a ray — the Cover contract: every cell whose kind has
// a Veil that dims an observer on blockers, in order, the stretch of the ray inside it, the band
// from the cell's level to Height above it (±Inf in a flat world) and τ = 1 − Veil. A square grid
// is walked exactly, cell by cell; any other grid in steps of a quarter cell.
func (f *Field) Walk(origin, dir geom.Vec, length float64, blockers world.Layers, visit func(near, far, bottom, top, tau float64) bool) {
	if f.square != nil {
		f.walkSquare(origin, dir, length, blockers, visit)
		return
	}
	f.walkSteps(origin, dir, length, blockers, visit)
}

// covers is the cover c puts on the ray for blockers: its band and τ, or false for none.
func (f *Field) covers(c cell.ID, blockers world.Layers) (bottom, top, tau float64, ok bool) {
	i, ok := f.cells.Ordinal(c)
	if !ok {
		return 0, 0, 0, false
	}
	f.Ready()
	v := &f.veils[i]
	if !v.ok || !v.veils.Meets(blockers) {
		return 0, 0, 0, false
	}
	return v.bottom, v.top, v.tau, true
}

// veil is the cover a cell puts on a ray: the band it covers and how see-through it is, for whom;
// ok false for a cell that covers nothing.
type veil struct {
	bottom, top, tau float64
	veils            world.Layers
	ok               bool
}

// Ready reads every cell's cover at once, when any cell has changed since — the Readied contract:
// walked from several goroutines at a time after it, the board reads nothing.
func (f *Field) Ready() {
	n := f.grid.CellCount()
	if len(f.veils) == n && f.veilsChanges == f.cells.Changes() && f.veilsVersion == f.cells.Version() {
		return
	}
	if len(f.veils) != n {
		f.veils = make([]veil, n)
	}
	f.veilsChanges, f.veilsVersion = f.cells.Changes(), f.cells.Version()
	f.grid.EachCell(func(c cell.ID) {
		i, ok := f.cells.Ordinal(c)
		if !ok {
			return
		}
		v := &f.veils[i]
		k := f.cells.KindOf(c)
		if k == nil || k.Veil <= 0 {
			*v = veil{}
			return
		}
		*v = veil{bottom: math.Inf(-1), top: math.Inf(1), tau: 1 - min(k.Veil, 1), veils: world.Layers(k.Veils), ok: true}
		if f.heights {
			v.bottom = f.altitude(c)
			v.top = v.bottom + k.Height
		}
	})
}

func (f *Field) walkSquare(origin, dir geom.Vec, length float64, blockers world.Layers, visit func(near, far, bottom, top, tau float64) bool) {
	sq := f.square
	size := float64(sq.CellSize)
	if size == 0 {
		return
	}
	cx, cy := int64(math.Floor(origin.X/size)), int64(math.Floor(origin.Y/size))
	sx, nx, dx := ddaAxis(origin.X, dir.X, cx, size)
	sy, ny, dy := ddaAxis(origin.Y, dir.Y, cy, size)
	for t := 0.0; t < length; {
		exit := min(nx, ny, length)
		if c, ok := sq.Fold(cx, cy); ok {
			if bottom, top, tau, ok := f.covers(c, blockers); ok && !visit(t, exit, bottom, top, tau) {
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

// ddaAxis is a ray's walk along one axis of a grid of cells size wide from cell c: the step, how
// far along the ray the next cell boundary is, and how far apart the boundaries are.
func ddaAxis(o, d float64, c int64, size float64) (step int64, next, delta float64) {
	switch {
	case d > 0:
		return 1, (float64(c+1)*size - o) / d, size / d
	case d < 0:
		return -1, (float64(c)*size - o) / d, -size / d
	}
	return 0, math.Inf(1), math.Inf(1)
}

func (f *Field) walkSteps(origin, dir geom.Vec, length float64, blockers world.Layers, visit func(near, far, bottom, top, tau float64) bool) {
	w, h := f.grid.CellBounds()
	step := min(w, h) / 4
	if step <= 0 {
		return
	}
	var cur cell.ID
	have, start := false, 0.0
	emit := func(end float64) bool {
		if !have {
			return true
		}
		bottom, top, tau, ok := f.covers(cur, blockers)
		return !ok || visit(start, end, bottom, top, tau)
	}
	for t := 0.0; ; t = min(t+step, length) {
		c, ok := f.grid.CellAt(geom.NewVec(origin.X+dir.X*t, origin.Y+dir.Y*t))
		if !ok || !have || c != cur {
			if !emit(t) {
				return
			}
			cur, have, start = c, ok, t
		}
		if t >= length {
			emit(length)
			return
		}
	}
}

// Solid is the solid ground of the board round box for an entity on layers spanning band — the
// collision.Field contract: every cell whose kind is Solid, keeps one of those layers out and
// stands in the band, and every raised deck over the whole cell, its slab over the entity's step,
// as its boxes. On a square grid a side is open where the neighbour across it is not solid for the
// entity, one under or over it among them, and the boxes lie in box's frame across a wrapping
// seam; any other grid gives its cell boxes open all round.
func (f *Field) Solid(layers world.Layers, band collision.Band, box geom.AABB, visit func(collide.FieldBox) bool) {
	sq := f.square
	if sq == nil {
		f.solidCells(layers, band, box, visit)
		return
	}
	size := float64(sq.CellSize)
	if size == 0 {
		return
	}
	x0, y0 := int64(math.Floor(box.TopLeft.X/size)), int64(math.Floor(box.TopLeft.Y/size))
	x1, y1 := int64(math.Ceil(box.BottomRight.X/size)), int64(math.Ceil(box.BottomRight.Y/size))
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
			fb := collide.FieldBox{Box: geom.NewAABBAt(geom.NewVec(float64(x)*size, float64(y)*size), size, size), Cell: uint64(c), Open: open}
			if !visit(fb) {
				return
			}
		}
	}
}

// Walls are the railings and the slabs of the decks running as bands round box, for an entity on
// layers spanning band — the collision.Field contract: a railing along each edge of a band, from
// its deck up its Height, keeping out an entity on a layer it Keeps; a raised band's slab, Width
// wide along it, over the entity's step. Their ends are free where the bridge ends or a head takes
// over. On a square grid they lie in box's frame across a wrapping seam.
func (f *Field) Walls(layers world.Layers, band collision.Band, box geom.AABB,
	visit func(collide.FieldWall) bool) {
	if !f.cells.Banded() {
		return
	}
	f.seen = f.seen[:0]
	if sq := f.square; sq != nil {
		size := float64(sq.CellSize)
		x0 := int64(math.Floor(box.TopLeft.X / size))
		y0 := int64(math.Floor(box.TopLeft.Y / size))
		x1 := int64(math.Ceil(box.BottomRight.X / size))
		y1 := int64(math.Ceil(box.BottomRight.Y / size))
		for y := y0; y < y1; y++ {
			for x := x0; x < x1; x++ {
				c, ok := sq.Fold(x, y)
				m := geom.NewVec((float64(x)+0.5)*size, (float64(y)+0.5)*size)
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

// wallsOf visits the walls of the bands on c and reaching into it, c's middle at m in the frame
// asked about; false when the visit ends the walk.
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
		if f.heights && fl.Height > 0 && fl.Thickness > 0 &&
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
			f.heights && r.Height > 0 && !railing.Meets(band) {
			continue
		}
		half := float64(f.grid.CellSpan()) / 32
		nx, ny := b.Y-a.Y, a.X-b.X
		l := math.Hypot(nx, ny)
		nx, ny = nx/l*fl.Width/2, ny/l*fl.Width/2
		for _, side := range [2]float64{-1, 1} {
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

// ends are a wall's ends, free as said.
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

// Overhang is how much of box, as area, lies over ground that does not take an entity on layers
// — cells whose kind allows none of them: water to a walker, a hole — the collision.Field
// contract: a push never makes it more. Off the board is none of it; the world's edges keep it.
func (f *Field) Overhang(layers world.Layers, box geom.AABB) float64 {
	sq := f.square
	if sq == nil {
		return f.overhangCells(layers, box)
	}
	size := float64(sq.CellSize)
	if size == 0 {
		return 0
	}
	var area float64
	x0, y0 := int64(math.Floor(box.TopLeft.X/size)), int64(math.Floor(box.TopLeft.Y/size))
	x1, y1 := int64(math.Ceil(box.BottomRight.X/size)), int64(math.Ceil(box.BottomRight.Y/size))
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			if c, ok := f.square.Fold(x, y); ok && !f.takes(c, layers) {
				area += overlap(box, geom.NewAABBAt(geom.NewVec(float64(x)*size, float64(y)*size), size, size))
			}
		}
	}
	return area
}

// overhangCells is Overhang over a grid other than square: the boxes of every cell under box.
func (f *Field) overhangCells(layers world.Layers, box geom.AABB) float64 {
	var area float64
	f.grid.CellsUnder(box, func(c cell.ID) {
		if f.takes(c, layers) {
			return
		}
		f.boxes = f.grid.CellBoxes(c, f.boxes[:0])
		for _, cb := range f.boxes {
			area += overlap(box, cb)
		}
	})
	return area
}

// takes reports whether c takes an entity on layers: its ground or a Floor over it allows one of
// them — any, for one on every plane (no layers) — and no Wall keeps it off.
func (f *Field) takes(c cell.ID, layers world.Layers) bool {
	k := f.cells.KindOf(c)
	if k == nil {
		return true
	}
	b := f.cells.Built(c)
	allows := (k.Allows | b.Floor.Allows) &^ b.Wall.Keeps
	return allows&cell.Domain(layers) != 0 || layers == 0 && allows != 0
}

// overlap is the area a and b share.
func overlap(a, b geom.AABB) float64 {
	w := min(a.BottomRight.X, b.BottomRight.X) - max(a.TopLeft.X, b.TopLeft.X)
	h := min(a.BottomRight.Y, b.BottomRight.Y) - max(a.TopLeft.Y, b.TopLeft.Y)
	if w <= 0 || h <= 0 {
		return 0
	}
	return w * h
}

// solidCells is Solid over a grid other than square: the boxes of every solid cell under box.
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

// stops reports whether c stops an entity on layers spanning band: its kind is solid for the
// layers, or a Wall over it keeps one of them off, and, where the world has heights, its band
// meets the entity's; or a raised deck's slab stands in the band over the entity's step.
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
	if !f.heights || fl.Thickness <= 0 || fl.Height <= 0 || fl.Width > 0 {
		return false
	}
	deck := f.altitude(c) + fl.Height
	return collision.Band{Bottom: deck - fl.Thickness, Top: deck}.Meets(band.Above())
}

// solidBand is the heights a wall height high stands in on c: from below up to height over its
// level, so nothing passes under a wall on a slope; Everywhere in a flat world, or for a wall of
// no height, which stands at every height as it does on the flat.
func (f *Field) solidBand(c cell.ID, height float64) collision.Band {
	if !f.heights || height <= 0 {
		return collision.Everywhere
	}
	return collision.Band{Bottom: math.Inf(-1), Top: f.altitude(c) + height}
}

// solidFor reports whether cells of k stop an entity on layers: Solid, keeping out a layer it is on.
func solidFor(k *cell.Kind, layers world.Layers) bool {
	return k != nil && k.Solid && world.Layers(^uint8(k.Allows)).Meets(layers)
}
