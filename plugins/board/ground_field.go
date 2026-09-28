package board

import (
	"math"

	"github.com/kjkrol/aabbworld/collide"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/world"
)

var _ world.Cover = (*Board)(nil)
var _ world.Field = (*Board)(nil)

// Walk is the cover of the board along a ray — the world.Cover contract: every cell whose kind has
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
	k := b.kindOf(c)
	if k == nil || k.Veil <= 0 || !world.Layers(k.Veils).Meets(blockers) {
		return 0, 0, 0, false
	}
	bottom, top = math.Inf(-1), math.Inf(1)
	if b.quasi3D {
		bottom = b.altitude(c)
		top = bottom + k.Height
	}
	return bottom, top, 1 - min(k.Veil, 1), true
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

// Solid is the solid ground of the board round box for an entity on layers — the world.Field
// contract: every cell whose kind is Solid and keeps one of those layers out, as its boxes. On a
// square grid a side is open where the neighbour across it is not solid for the entity, and the
// boxes lie in box's frame across a wrapping seam; any other grid gives its cell boxes open all
// round.
func (b *Board) Solid(layers world.Layers, box geom.AABB, visit func(world.FieldBox) bool) {
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
			fb := world.FieldBox{Box: geom.NewAABBAt(geom.NewVec(float64(x)*size, float64(y)*size), size, size), Cell: uint64(c), Open: open}
			if !visit(fb) {
				return
			}
		}
	}
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
func (b *Board) solidCells(layers world.Layers, box geom.AABB, visit func(world.FieldBox) bool) {
	const all = collide.Left | collide.Right | collide.Top | collide.Bottom
	done := false
	b.CellsUnder(box, func(c CellID) {
		if done || !solidFor(b.kindOf(c), layers) {
			return
		}
		b.boxes = b.CellBoxes(c, b.boxes[:0])
		for _, cb := range b.boxes {
			if !visit(world.FieldBox{Box: cb, Cell: uint64(c), Open: all}) {
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
