package board

import (
	"github.com/kjkrol/gram/plugins/board/cell"
	"math"

	"github.com/kjkrol/aabbworld/geom"
)

// squareGrid is a Grid of Width x Height square cells with 8-directional neighbors;
// a diagonal step costs √2 of an orthogonal one.
type squareGrid struct {
	Width, Height uint32
	CellSize      uint32
	WrapX, WrapY  bool
}

var _ Grid = (*squareGrid)(nil)

func newSquareGrid(width, height, cellSize uint32) *squareGrid {
	return &squareGrid{Width: width, Height: height, CellSize: cellSize}
}

func (g *squareGrid) idAt(x, y uint32) cell.ID {
	return cell.ID(uint64(y)*uint64(g.Width) + uint64(x))
}

func (g *squareGrid) cellXY(c cell.ID) (x, y uint32) {
	return uint32(uint64(c) % uint64(g.Width)), uint32(uint64(c) / uint64(g.Width))
}

func (g *squareGrid) Contains(c cell.ID) bool {
	if g.WrapX && g.WrapY {
		return true
	}
	x, y := g.cellXY(c)
	return uint64(c) < uint64(g.Width)*uint64(g.Height) && x < g.Width && y < g.Height
}

var squareDirs = [8][2]int64{
	{0, -1}, {0, 1}, {-1, 0}, {1, 0}, // orthogonal
	{-1, -1}, {1, -1}, {-1, 1}, {1, 1}, // diagonal
}

func (g *squareGrid) Neighbors(c cell.ID) []cell.ID {
	x, y := g.cellXY(c)
	out := make([]cell.ID, 0, 8)
	for _, d := range squareDirs {
		nx, okX := foldAxis(int64(x)+d[0], int64(g.Width), g.WrapX)
		ny, okY := foldAxis(int64(y)+d[1], int64(g.Height), g.WrapY)
		if !okX || !okY {
			continue
		}
		out = append(out, g.idAt(uint32(nx), uint32(ny)))
	}
	return out
}

func (g *squareGrid) CellCenter(c cell.ID) geom.Vec {
	x, y := g.cellXY(c)
	half := float64(g.CellSize) / 2
	return geom.NewVec(float64(x)*float64(g.CellSize)+half, float64(y)*float64(g.CellSize)+half)
}

func (g *squareGrid) CellAt(pos geom.Vec) (cell.ID, bool) {
	if g.CellSize == 0 {
		return 0, false
	}
	x, okX := foldAxis(int64(math.Floor(pos.X/float64(g.CellSize))), int64(g.Width), g.WrapX)
	y, okY := foldAxis(int64(math.Floor(pos.Y/float64(g.CellSize))), int64(g.Height), g.WrapY)
	if !okX || !okY {
		return 0, false
	}
	return g.idAt(uint32(x), uint32(y)), true
}

func (g *squareGrid) CellSpan() float32 { return float32(g.CellSize) }

func (g *squareGrid) CellBounds() (w, h float64) { return float64(g.CellSize), float64(g.CellSize) }

func (g *squareGrid) CellOutline(c cell.ID, dst []geom.Vec) []geom.Vec {
	x, y := g.cellXY(c)
	size := float64(g.CellSize)
	x0, y0 := float64(x)*size, float64(y)*size
	return append(dst, geom.NewVec(x0, y0), geom.NewVec(x0+size, y0), geom.NewVec(x0+size, y0+size), geom.NewVec(x0, y0+size))
}

// CellsUnder walks the columns and rows the box touches, edges included, each cell once however
// far the box reaches round a wrapping axis.
func (g *squareGrid) CellsUnder(box geom.AABB, fn func(c cell.ID)) {
	if g.CellSize == 0 {
		return
	}
	size := float64(g.CellSize)
	x0, x1, okX := spanOf(box.TopLeft.X, box.BottomRight.X, size, g.Width, g.WrapX)
	y0, y1, okY := spanOf(box.TopLeft.Y, box.BottomRight.Y, size, g.Height, g.WrapY)
	if !okX || !okY {
		return
	}
	for y := y0; y <= y1; y++ {
		fy, _ := foldAxis(y, int64(g.Height), g.WrapY)
		for x := x0; x <= x1; x++ {
			fx, _ := foldAxis(x, int64(g.Width), g.WrapX)
			fn(g.idAt(uint32(fx), uint32(fy)))
		}
	}
}

// spanOf is the cells lo..hi along one axis that the stretch from a to b touches: clamped to the
// board where the axis does not wrap, one lap at most where it does.
func spanOf(a, b, size float64, n uint32, wraps bool) (lo, hi int64, ok bool) {
	const eps = 1e-9
	lo, hi = int64(math.Floor((a-eps)/size)), int64(math.Floor((b+eps)/size))
	if wraps {
		return lo, min(hi, lo+int64(n)-1), true
	}
	lo, hi = max(lo, 0), min(hi, int64(n)-1)
	return lo, hi, lo <= hi
}

// CellBoxes is the cell's own square.
func (g *squareGrid) CellBoxes(c cell.ID, dst []geom.AABB) []geom.AABB {
	x, y := g.cellXY(c)
	size := float64(g.CellSize)
	topLeft := geom.NewVec(float64(x)*size, float64(y)*size)
	return append(dst, geom.NewAABB(topLeft, geom.NewVec(topLeft.X+size, topLeft.Y+size)))
}

func (g *squareGrid) EachCell(fn func(c cell.ID)) {
	for y := uint32(0); y < g.Height; y++ {
		for x := uint32(0); x < g.Width; x++ {
			fn(g.idAt(x, y))
		}
	}
}

func (g *squareGrid) SetWrap(x, y bool) { g.WrapX, g.WrapY = x, y }

func (g *squareGrid) Ordinal(c cell.ID) (int, bool) {
	x, y := g.cellXY(c)
	if x >= g.Width || y >= g.Height {
		return 0, false
	}
	return int(y)*int(g.Width) + int(x), true
}

func (g *squareGrid) CellCount() int { return int(g.Width) * int(g.Height) }

func (g *squareGrid) Coords(c cell.ID) (uint32, uint32, bool) {
	x, y := g.cellXY(c)
	return x, y, x < g.Width && y < g.Height
}

func (g *squareGrid) CellIndex(col, row uint32) (cell.ID, bool) {
	x, okX := foldAxis(int64(col), int64(g.Width), g.WrapX)
	y, okY := foldAxis(int64(row), int64(g.Height), g.WrapY)
	if !okX || !okY {
		return 0, false
	}
	return g.idAt(uint32(x), uint32(y)), true
}

// dxdy returns the wrap-aware column/row gap between a and b.
func (g *squareGrid) dxdy(a, b cell.ID) (dx, dy float64) {
	ax, ay := g.cellXY(a)
	bx, by := g.cellXY(b)
	width, height := uint32(0), uint32(0)
	if g.WrapX {
		width = g.Width
	}
	if g.WrapY {
		height = g.Height
	}
	return float64(wrapDistance1D(ax, bx, width)), float64(wrapDistance1D(ay, by, height))
}

// NeighborCost is 1 for an orthogonal step, √2 for a diagonal one.
func (g *squareGrid) NeighborCost(a, b cell.ID) float64 {
	dx, dy := g.dxdy(a, b)
	return math.Sqrt(dx*dx + dy*dy)
}

// Distance is octile distance.
func (g *squareGrid) Distance(a, b cell.ID) float64 {
	dx, dy := g.dxdy(a, b)
	if dx < dy {
		dx, dy = dy, dx
	}
	return dx + (math.Sqrt2-1)*dy
}

// DiagonalNeighbors returns the two cells flanking the corner between a and its diagonal b.
func (g *squareGrid) DiagonalNeighbors(a, b cell.ID) (c1, c2 cell.ID, ok bool) {
	ax, ay := g.cellXY(a)
	bx, by := g.cellXY(b)
	dx := axisDelta(ax, bx, g.Width, g.WrapX)
	dy := axisDelta(ay, by, g.Height, g.WrapY)
	if dx == 0 || dy == 0 {
		return 0, 0, false
	}
	c1, ok1 := g.CellIndex(uint32(wrapModI64(int64(ax)+dx, int64(g.Width))), ay)
	c2, ok2 := g.CellIndex(ax, uint32(wrapModI64(int64(ay)+dy, int64(g.Height))))
	return c1, c2, ok1 && ok2
}

// axisDelta is the single-step direction (-1/0/+1) from a to b along an axis of length size.
func axisDelta(a, b, size uint32, wraps bool) int64 {
	if a == b {
		return 0
	}
	if !wraps {
		if b > a {
			return 1
		}
		return -1
	}
	if wrapModI64(int64(b)-int64(a), int64(size)) == 1 {
		return 1
	}
	return -1
}

func absDiffU32(a, b uint32) uint32 {
	if a > b {
		return a - b
	}
	return b - a
}

// wrapDistance1D is the shorter way from a to b on an axis of length size; size 0 never wraps.
func wrapDistance1D(a, b, size uint32) uint32 {
	d := absDiffU32(a, b)
	if size == 0 {
		return d
	}
	if alt := size - d; alt < d {
		return alt
	}
	return d
}

// wrapModI64 folds v into [0,m) using true (non-negative) modulo.
func wrapModI64(v, m int64) int64 {
	v %= m
	if v < 0 {
		v += m
	}
	return v
}
