package grids

import (
	"fmt"
	"math"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/plugins/board/cell"
)

// Hex is a grid over pointy-top hexagonal cells, Width of them in each of Height rows, every
// other row half a cell east of the one over it, so the board keeps to a rectangle. A cell's id
// is its axial coordinates, its Coords its column and row.
type Hex struct {
	Width, Height uint32
	Size          float32
	WrapX, WrapY  bool
}

// NewHex is a hex grid of width x height cells of the given size, wrapping along no axis.
func NewHex(width, height uint32, size float32) *Hex {
	return &Hex{Width: width, Height: height, Size: size}
}

func packAxial(q, r int32) cell.ID {
	return cell.ID(uint64(uint32(q))<<32 | uint64(uint32(r)))
}

func unpackAxial(c cell.ID) (q, r int32) {
	return int32(uint32(c >> 32)), int32(uint32(c))
}

// colOf is the column of the axial (q, r): a row's cells begin half a row's count west of q 0
func colOf(q, r int32) int32 { return q + r>>1 }

func axialOf(col, row int32) (q, r int32) { return col - row>>1, row }

// foldAxial is the axial (q, r) as the cell of the board it is, across a seam where the grid
// wraps: folded by its column and row
func (g *Hex) foldAxial(q, r int32) (int32, int32, bool) {
	col, okC := foldAxis(int64(colOf(q, r)), int64(g.Width), g.WrapX)
	row, okR := foldAxis(int64(r), int64(g.Height), g.WrapY)
	fq, fr := axialOf(int32(col), int32(row))
	return fq, fr, okC && okR
}

// axial q, r: east, north-east, north-west, west, south-west, south-east; y runs down the screen
var hexDirs = [6][2]int32{{1, 0}, {1, -1}, {0, -1}, {-1, 0}, {-1, 1}, {0, 1}}

func (g *Hex) Directions() int { return len(hexDirs) }

func (g *Hex) Opposite(i int) int { return (i + 3) % 6 }

func (g *Hex) Offset(i int) (int, int) { return int(hexDirs[i][0]), int(hexDirs[i][1]) }

func (g *Hex) Step(i int) geom.Vec {
	dq, dr := float32(hexDirs[i][0]), float32(hexDirs[i][1])
	return geom.NewVec(g.Size*(f32.Sqrt(3)*dq+f32.Sqrt(3)/2*dr), g.Size*1.5*dr)
}

func (g *Hex) Corner(int) bool { return false }

func (g *Hex) Flanks(int) (int, int, bool) { return 0, 0, false }

// Toward is c's neighbour in the i-th of hexDirs, across a seam where the grid wraps; false off
// the grid or past the directions.
func (g *Hex) Toward(c cell.ID, i int) (cell.ID, bool) {
	if i < 0 || i >= len(hexDirs) {
		return 0, false
	}
	q, r := unpackAxial(c)
	nq, nr, ok := g.foldAxial(q+hexDirs[i][0], r+hexDirs[i][1])
	return packAxial(nq, nr), ok
}

func (g *Hex) Neighbors(c cell.ID) []cell.ID {
	q, r := unpackAxial(c)
	out := make([]cell.ID, 0, 6)
	for _, d := range hexDirs {
		if nq, nr, ok := g.foldAxial(q+d[0], r+d[1]); ok {
			out = append(out, packAxial(nq, nr))
		}
	}
	return out
}

// CellCenter is the world position of c's centre, with cell (0,0) fully in positive space.
func (g *Hex) CellCenter(c cell.ID) geom.Vec {
	q, r := unpackAxial(c)
	x := g.Size*(f32.Sqrt(3)*float32(q)+f32.Sqrt(3)/2*float32(r)) + g.Size
	y := g.Size*(1.5*float32(r)) + g.Size
	return geom.NewVec(x, y)
}

func (g *Hex) CellAt(pos geom.Vec) (cell.ID, bool) {
	if g.Size == 0 {
		return 0, false
	}
	x, y := pos.X-g.Size, pos.Y-g.Size
	qf := (f32.Sqrt(3)/3*x - 1.0/3*y) / g.Size
	rf := (2.0 / 3 * y) / g.Size
	q, r := axialRound(qf, rf)
	q, r, ok := g.foldAxial(q, r)
	if !ok {
		return 0, false
	}
	return packAxial(q, r), true
}

func (g *Hex) CellSpan() float32 { return g.Size }

// CellBounds is the hex's bounding box: √3·Size wide, 2·Size tall.
func (g *Hex) CellBounds() (w, h float32) { return f32.Sqrt(3) * g.Size, 2 * g.Size }

// CellOutline is the six corners of a pointy-top hex, clockwise from the top.
func (g *Hex) CellOutline(c cell.ID, dst []geom.Vec) []geom.Vec {
	center := g.CellCenter(c)
	for i := range 6 {
		a := -math.Pi/2 + float32(i)*math.Pi/3
		dst = append(dst, geom.NewVec(center.X+g.Size*f32.Cos(a), center.Y+g.Size*f32.Sin(a)))
	}
	return dst
}

func (g *Hex) CellsUnder(box geom.AABB, fn func(c cell.ID)) { cellsUnder(g, box, fn) }

// boxes over each pointed end of a hex; more is a closer fit
const capStrips = 3

// CellBoxes covers the hex from outside: its middle band, then strips over each cap as wide as
// the hex is at the strip's wider edge.
func (g *Hex) CellBoxes(c cell.ID, dst []geom.AABB) []geom.AABB {
	center := g.CellCenter(c)
	s := g.Size
	halfW := f32.Sqrt(3) / 2 * s
	dst = append(
		dst,
		geom.NewAABB(
			geom.NewVec(center.X-halfW, center.Y-s/2),
			geom.NewVec(center.X+halfW, center.Y+s/2),
		),
	)
	h := s / 2 / capStrips
	for j := range capStrips {
		w := halfW * float32(j+1) / capStrips
		top := center.Y - s + float32(j)*h
		dst = append(
			dst,
			geom.NewAABB(geom.NewVec(center.X-w, top), geom.NewVec(center.X+w, top+h)),
		)
		bottom := center.Y + s - float32(j+1)*h
		dst = append(
			dst,
			geom.NewAABB(geom.NewVec(center.X-w, bottom), geom.NewVec(center.X+w, bottom+h)),
		)
	}
	return dst
}

func (g *Hex) EachCell(fn func(c cell.ID)) {
	for row := int32(0); row < int32(g.Height); row++ {
		for col := int32(0); col < int32(g.Width); col++ {
			fn(packAxial(axialOf(col, row)))
		}
	}
}

// SetWrap sets the axes the grid wraps along; rows alternate, so one of an odd Height wraps
// along x alone.
func (g *Hex) SetWrap(x, y bool) {
	if y && g.Height%2 != 0 {
		panic(fmt.Sprintf("board: a hex grid of %d rows cannot wrap along y: its rows alternate, "+
			"so the last would meet the first out of step", g.Height))
	}
	g.WrapX, g.WrapY = x, y
}

func (g *Hex) Ordinal(c cell.ID) (int, bool) {
	q, r := unpackAxial(c)
	col := colOf(q, r)
	if col < 0 || r < 0 || col >= int32(g.Width) || r >= int32(g.Height) {
		return 0, false
	}
	return int(r)*int(g.Width) + int(col), true
}

func (g *Hex) CellCount() int { return int(g.Width) * int(g.Height) }

// Coords is c's column and row.
func (g *Hex) Coords(c cell.ID) (uint32, uint32, bool) {
	q, r := unpackAxial(c)
	col := colOf(q, r)
	return uint32(col), uint32(r), col >= 0 && r >= 0 && col < int32(g.Width) && r < int32(g.Height)
}

// At is the cell in a column of a row.
func (g *Hex) At(col, row uint32) cell.ID {
	q, r := axialOf(int32(col), int32(row))
	fq, fr, ok := g.foldAxial(q, r)
	if !ok {
		panic(fmt.Sprintf("board: no cell at (%d, %d)", col, row))
	}
	return packAxial(fq, fr)
}

// NeighborCost is always 1 — every hex neighbor is equidistant in this axial model.
func (g *Hex) NeighborCost(a, b cell.ID) float64 { return 1 }

// DiagonalNeighbors always returns ok=false: hex neighbors share an edge, never a corner.
func (g *Hex) DiagonalNeighbors(a, b cell.ID) (c1, c2 cell.ID, ok bool) {
	return 0, 0, false
}

func hexCubeDistance(dq, dr int32) float64 {
	return (math.Abs(float64(dq)) + math.Abs(float64(dr)) + math.Abs(float64(dq+dr))) / 2
}

// Distance is hex (cube) distance, the shortest across every wrap period when toroidal.
func (g *Hex) Distance(a, b cell.ID) float64 {
	aq, ar := unpackAxial(a)
	bq, br := unpackAxial(b)
	dq, dr := aq-bq, ar-br

	width, height := int32(0), int32(0)
	if g.WrapX {
		width = int32(g.Width)
	}
	if g.WrapY {
		height = int32(g.Height)
	}
	best := math.Inf(1)
	for _, mq := range [3]int32{-width, 0, width} {
		for _, mr := range [3]int32{-height, 0, height} {
			// a board's height down is as many rows and half as many q back
			if d := hexCubeDistance(dq+mq-mr/2, dr+mr); d < best {
				best = d
			}
		}
	}
	return best
}

func axialRound(qf, rf float32) (int32, int32) {
	xf, zf := qf, rf
	yf := -xf - zf
	x, y, z := f32.Round(xf), f32.Round(yf), f32.Round(zf)
	dx, dy, dz := f32.Abs(x-xf), f32.Abs(y-yf), f32.Abs(z-zf)
	switch {
	case dx > dy && dx > dz:
		x = -y - z
	case dy > dz:
	default:
		z = -x - y
	}
	return int32(x), int32(z)
}
