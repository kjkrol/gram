package board

import (
	"math"

	"github.com/kjkrol/aabbworld/collide"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/world"
)

// Walk is the cover of the board along a ray — the Cover contract: every cell whose kind has
// a Veil that dims an observer on blockers, in order, the stretch of the ray inside it, the band
// from the cell's level to Height above it (±Inf in a flat world) and τ = 1 − Veil. A square grid
// is walked exactly, cell by cell; any other grid in steps of a quarter cell.
func (b *Board) Walk(origin, dir geom.Vec, length float64, blockers world.Layers, visit func(near, far, bottom, top, tau float64) bool) {
	if b.square != nil {
		b.walkSquare(origin, dir, length, blockers, visit)
		return
	}
	b.walkSteps(origin, dir, length, blockers, visit)
}

// covers is the cover c puts on the ray for blockers: its band and τ, or false for none.
func (b *Board) covers(c CellID, blockers world.Layers) (bottom, top, tau float64, ok bool) {
	i, ok := b.ordinal(c)
	if !ok {
		return 0, 0, 0, false
	}
	b.Ready()
	v := &b.veils[i]
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
func (b *Board) Ready() {
	n := b.Grid.CellCount()
	if len(b.veils) == n && b.veilsChanges == b.Changes() && b.veilsVersion == b.Version() {
		return
	}
	if len(b.veils) != n {
		b.veils = make([]veil, n)
	}
	b.veilsChanges, b.veilsVersion = b.Changes(), b.Version()
	b.Grid.EachCell(func(c CellID) {
		i, ok := b.ordinal(c)
		if !ok {
			return
		}
		v := &b.veils[i]
		k := b.kindOf(c)
		if k == nil || k.Veil <= 0 {
			*v = veil{}
			return
		}
		*v = veil{bottom: math.Inf(-1), top: math.Inf(1), tau: 1 - min(k.Veil, 1), veils: world.Layers(k.Veils), ok: true}
		if b.heights {
			v.bottom = b.altitude(c)
			v.top = v.bottom + k.Height
		}
	})
}

func (b *Board) walkSquare(origin, dir geom.Vec, length float64, blockers world.Layers, visit func(near, far, bottom, top, tau float64) bool) {
	sq := b.square
	size := float64(sq.CellSize)
	if size == 0 {
		return
	}
	cx, cy := int64(math.Floor(origin.X/size)), int64(math.Floor(origin.Y/size))
	sx, nx, dx := ddaAxis(origin.X, dir.X, cx, size)
	sy, ny, dy := ddaAxis(origin.Y, dir.Y, cy, size)
	for t := 0.0; t < length; {
		exit := min(nx, ny, length)
		if x, okX := foldAxis(cx, int64(sq.Width), sq.WrapX); okX {
			if y, okY := foldAxis(cy, int64(sq.Height), sq.WrapY); okY {
				if bottom, top, tau, ok := b.covers(sq.idAt(uint32(x), uint32(y)), blockers); ok && !visit(t, exit, bottom, top, tau) {
					return
				}
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

func (b *Board) walkSteps(origin, dir geom.Vec, length float64, blockers world.Layers, visit func(near, far, bottom, top, tau float64) bool) {
	w, h := b.CellBounds()
	step := min(w, h) / 4
	if step <= 0 {
		return
	}
	var cur CellID
	have, start := false, 0.0
	emit := func(end float64) bool {
		if !have {
			return true
		}
		bottom, top, tau, ok := b.covers(cur, blockers)
		return !ok || visit(start, end, bottom, top, tau)
	}
	for t := 0.0; ; t = min(t+step, length) {
		c, ok := b.CellAt(geom.NewVec(origin.X+dir.X*t, origin.Y+dir.Y*t))
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

// Solid is the solid ground of the board round box for an entity on layers — the collision.Field
// contract: every cell whose kind is Solid and keeps one of those layers out, as its boxes. On a
// square grid a side is open where the neighbour across it is not solid for the entity, and the
// boxes lie in box's frame across a wrapping seam; any other grid gives its cell boxes open all
// round.
func (b *Board) Solid(layers world.Layers, box geom.AABB, visit func(collide.FieldBox) bool) {
	sq := b.square
	if sq == nil {
		b.solidCells(layers, box, visit)
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
			c, ok := b.squareCell(x, y)
			if !ok || !solidFor(b.kindOf(c), layers) {
				continue
			}
			var open collide.Sides
			for _, n := range [4]struct {
				dx, dy int64
				side   collide.Sides
			}{{-1, 0, collide.Left}, {1, 0, collide.Right}, {0, -1, collide.Top}, {0, 1, collide.Bottom}} {
				if nc, ok := b.squareCell(x+n.dx, y+n.dy); !ok || !solidFor(b.kindOf(nc), layers) {
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

// Overhang is how much of box, as area, lies over ground that does not take an entity on layers
// — cells whose kind allows none of them: water to a walker, a hole — the collision.Field
// contract: a push never makes it more. Off the board is none of it; the world's edges keep it.
func (b *Board) Overhang(layers world.Layers, box geom.AABB) float64 {
	sq := b.square
	if sq == nil {
		return b.overhangCells(layers, box)
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
			if c, ok := b.squareCell(x, y); ok && !takes(b.kindOf(c), layers) {
				area += overlap(box, geom.NewAABBAt(geom.NewVec(float64(x)*size, float64(y)*size), size, size))
			}
		}
	}
	return area
}

// overhangCells is Overhang over a grid other than square: the boxes of every cell under box.
func (b *Board) overhangCells(layers world.Layers, box geom.AABB) float64 {
	var area float64
	b.CellsUnder(box, func(c CellID) {
		if takes(b.kindOf(c), layers) {
			return
		}
		b.boxes = b.CellBoxes(c, b.boxes[:0])
		for _, cb := range b.boxes {
			area += overlap(box, cb)
		}
	})
	return area
}

// takes reports whether cells of k take an entity on layers: they allow one of them — any, for
// one on every plane (no layers).
func takes(k *CellKind, layers world.Layers) bool {
	return k == nil || k.Allows&Domain(layers) != 0 || layers == 0 && k.Allows != 0
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

// squareCell is the cell at column x, row y of the square grid, folded across a wrapping seam.
func (b *Board) squareCell(x, y int64) (CellID, bool) {
	sq := b.square
	fx, okX := foldAxis(x, int64(sq.Width), sq.WrapX)
	fy, okY := foldAxis(y, int64(sq.Height), sq.WrapY)
	if !okX || !okY {
		return 0, false
	}
	return sq.idAt(uint32(fx), uint32(fy)), true
}

// solidCells is Solid over a grid other than square: the boxes of every solid cell under box.
func (b *Board) solidCells(layers world.Layers, box geom.AABB, visit func(collide.FieldBox) bool) {
	const all = collide.Left | collide.Right | collide.Top | collide.Bottom
	done := false
	b.CellsUnder(box, func(c CellID) {
		if done || !solidFor(b.kindOf(c), layers) {
			return
		}
		b.boxes = b.CellBoxes(c, b.boxes[:0])
		for _, cb := range b.boxes {
			if !visit(collide.FieldBox{Box: cb, Cell: uint64(c), Open: all}) {
				done = true
				return
			}
		}
	})
}

// solidFor reports whether cells of k stop an entity on layers: Solid, keeping out a layer it is on.
func solidFor(k *CellKind, layers world.Layers) bool {
	return k != nil && k.Solid && world.Layers(^uint8(k.Allows)).Meets(layers)
}

// kindOf is Kind without the copy, nil off the board; the pointer is good until the cells change.
func (b *Board) kindOf(c CellID) *CellKind {
	if b.cells == nil {
		k := b.seed.Kind(c)
		return &k
	}
	i, ok := b.ordinal(c)
	if !ok {
		return nil
	}
	return &b.groundOf(i).Kind
}
