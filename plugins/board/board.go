package board

import (
	"fmt"
	"math"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

// Board is a Grid and its terrain. Once the ECS is set up every cell is an entity carrying a
// [Plot] and a [Ground], and the board reads and writes them; before that, and for
// good on a board no ECS runs, it keeps a seed: a TerrainMap and the corner heights. It is also
// the world's Ground: on a square grid the ground runs between a cell's corners, on a hex grid a
// cell is level. Not safe for concurrent use.
type Board struct {
	Grid

	square   *squareGrid // the grid when it is square, for the ground's fast path; nil otherwise
	seed     *TerrainMap
	relief   []Relief // the seed's relief by ordinal; nil is level at 0
	cells    *cellStore
	version  uint64
	quasi3D  bool // the world has heights: cover spans the cells' bands
	climbing Climbing
	boxes    []geom.AABB // scratch for the boxes of a cell
}

// cellStore is where the cells' entities are: their ids by ordinal, and a query for each of the
// two components, so a read seeks only the column it needs.
type cellStore struct {
	ids    []uid.UID64
	plots  *goke.Query
	kinds  *goke.Query
	ways   *goke.Query
	plot   goke.Comp[Plot]
	ground goke.Comp[Ground]
	way    goke.Comp[Way]
}

var _ world.Ground = (*Board)(nil)
var _ Terrain = (*Board)(nil)

// NewBoard is a board over grid seeded with terrain.
func NewBoard(grid Grid, terrain *TerrainMap) *Board {
	sq, _ := grid.(*squareGrid)
	return &Board{Grid: grid, square: sq, seed: terrain, climbing: DefaultClimbing}
}

// bind hands the terrain over to the cell entities in st.
func (b *Board) bind(st *cellStore) {
	b.version += b.seed.Version()
	b.cells, b.seed, b.relief = st, nil, nil
}

// ordinal is Grid.Ordinal, straight from the id on a square grid, whose ids count row by row.
func (b *Board) ordinal(c CellID) (int, bool) {
	if sq := b.square; sq != nil {
		return int(c), uint64(c) < uint64(sq.Width)*uint64(sq.Height)
	}
	return b.Ordinal(c)
}

// groundOf is the i-th cell's Ground, in place.
func (b *Board) groundOf(i int) *Ground {
	st := b.cells
	if !st.kinds.SeekH(st.ids[i]) && !st.kinds.Seek(st.ids[i]) {
		panic(fmt.Sprintf("board: cell entity %d is gone", st.ids[i]))
	}
	return st.ground.At(st.kinds.Cursor())
}

// plotOf is the i-th cell's Plot, in place.
func (b *Board) plotOf(i int) *Plot {
	st := b.cells
	if !st.plots.SeekH(st.ids[i]) && !st.plots.Seek(st.ids[i]) {
		panic(fmt.Sprintf("board: cell entity %d is gone", st.ids[i]))
	}
	return st.plot.At(st.plots.Cursor())
}

// CellEntity is the entity of cell c; false off the board or before the ECS is set up.
func (b *Board) CellEntity(c CellID) (uid.UID64, bool) {
	i, ok := b.ordinal(c)
	if !ok || b.cells == nil {
		return 0, false
	}
	return b.cells.ids[i], true
}

// Kind is c's terrain kind as whoever crosses it meets it: its ground's, with a Way running across
// it deciding who may and what it costs (Way.Over); off the board, the zero kind admitting nobody.
func (b *Board) Kind(c CellID) CellKind {
	if b.cells == nil {
		return b.seed.Ways[c].Over(b.seed.Kind(c))
	}
	i, ok := b.ordinal(c)
	if !ok {
		return CellKind{}
	}
	return b.wayOf(i).Over(b.groundOf(i).Kind)
}

// Set assigns c's terrain kind, taking effect immediately.
func (b *Board) Set(c CellID, kind CellKind) {
	if b.set(c, kind) {
		b.version++
	}
}

// SetMany assigns kind to every cell in cells in one call.
func (b *Board) SetMany(cells []CellID, kind CellKind) {
	changed := false
	for _, c := range cells {
		changed = b.set(c, kind) || changed
	}
	if changed {
		b.version++
	}
}

func (b *Board) set(c CellID, kind CellKind) bool {
	if b.cells == nil {
		before := b.seed.Version()
		b.seed.Set(c, kind)
		return b.seed.Version() != before
	}
	i, ok := b.ordinal(c)
	if !ok {
		return false
	}
	g := b.groundOf(i)
	if g.Kind == kind {
		return false
	}
	g.Kind = kind
	return true
}

// SetAll resets every cell's terrain kind to kind.
func (b *Board) SetAll(kind CellKind) {
	if b.cells == nil {
		b.seed.SetAll(kind)
		return
	}
	st := b.cells
	for st.kinds.All(); st.kinds.Next(); {
		grounds := st.ground.Slice(st.kinds.Cursor())
		for i := range grounds {
			grounds[i].Kind = kind
		}
	}
	b.version++
}

// Version counts the changes to the terrain — kinds and heights, made through the board or by an
// effect on a cell's entity; it starts over with a load.
func (b *Board) Version() uint64 {
	if b.cells == nil {
		return b.version + b.seed.Version()
	}
	return b.version
}

// Relief is the ground height at c's corners; zero off the board.
func (b *Board) Relief(c CellID) Relief {
	i, ok := b.ordinal(c)
	if !ok {
		return Relief{}
	}
	return b.reliefAt(i)
}

func (b *Board) reliefAt(i int) Relief {
	if b.cells == nil {
		if b.relief == nil {
			return Relief{}
		}
		return b.relief[i]
	}
	return b.plotOf(i).Relief
}

// SetRelief puts c's corners at r's heights. On a square grid the neighbours meeting at a corner
// go with it: the ground has no vertical walls. A hex cell is level, at r's first corner.
func (b *Board) SetRelief(c CellID, r Relief) {
	if !b.sloped() {
		r = Relief{Corners: [4]float32{r.Corners[0], r.Corners[0], r.Corners[0], r.Corners[0]}}
		if b.setRelief(c, r) {
			b.version++
		}
		return
	}
	if b.setCorners(c, r) {
		b.version++
	}
}

// setCorners puts c's corners at r's heights and the neighbours' corners meeting them with them.
func (b *Board) setCorners(c CellID, r Relief) bool {
	x, y, ok := b.Coords(c)
	if !ok {
		return false
	}
	changed := false
	for k, d := range [4][2]int64{{0, 0}, {1, 0}, {0, 1}, {1, 1}} {
		if v, ok := b.foldVertex(int64(x)+d[0], int64(y)+d[1]); ok {
			changed = b.setHeightAt(v, float64(r.Corners[k])) || changed
		}
	}
	return changed
}

// seal brings the neighbours' corners to c's where they meet, so a relief written into c's Plot
// alone, by an effect, leaves no vertical wall; false where they met already.
func (b *Board) seal(c CellID) bool { return b.sloped() && b.setCorners(c, b.Relief(c)) }

func (b *Board) setRelief(c CellID, r Relief) bool {
	i, ok := b.ordinal(c)
	if !ok {
		return false
	}
	if b.cells == nil {
		if b.relief == nil {
			if r == (Relief{}) {
				return false
			}
			b.relief = make([]Relief, b.CellCount())
		}
		if b.relief[i] == r {
			return false
		}
		b.relief[i] = r
		return true
	}
	p := b.plotOf(i)
	if p.Relief == r {
		return false
	}
	p.Relief = r
	return true
}

// SetHeights raises every cell's ground to heights: sampled at its corners on a square grid, at
// its centre on a hex one.
func (b *Board) SetHeights(heights func(p geom.Vec) float64) {
	changed := false
	b.EachCell(func(c CellID) {
		changed = b.setRelief(c, b.sample(c, heights)) || changed
	})
	if changed {
		b.version++
	}
}

// sample is c's relief read off heights.
func (b *Board) sample(c CellID, heights func(p geom.Vec) float64) Relief {
	if !b.sloped() {
		h := float32(heights(b.CellCenter(c)))
		return Relief{Corners: [4]float32{h, h, h, h}}
	}
	w, h := b.CellBounds()
	o := b.CellCenter(c).Sub(geom.NewVec(w/2, h/2))
	var r Relief
	for k, d := range [4][2]float64{{0, 0}, {w, 0}, {0, h}, {w, h}} {
		r.Corners[k] = float32(heights(geom.NewVec(o.X+d[0], o.Y+d[1])))
	}
	return r
}

// sloped reports whether the ground runs between a cell's corners: a square grid's.
func (b *Board) sloped() bool { return b.square != nil }

// GroundAt is the ground height under p, 0 off the board: read between the cell's corners on a
// square grid, the cell's level elsewhere.
func (b *Board) GroundAt(p geom.Vec) float64 {
	sq := b.square
	if sq == nil {
		c, ok := b.CellAt(p)
		if !ok {
			return 0
		}
		return float64(b.Relief(c).Corners[0])
	}
	if sq.CellSize == 0 {
		return 0
	}
	size := float64(sq.CellSize)
	fx, fy := p.X/size, p.Y/size
	x0, y0 := math.Floor(fx), math.Floor(fy)
	x, okX := foldAxis(int64(x0), int64(sq.Width), sq.WrapX)
	y, okY := foldAxis(int64(y0), int64(sq.Height), sq.WrapY)
	if !okX || !okY {
		return 0
	}
	hs := b.reliefAt(int(y)*int(sq.Width) + int(x)).Corners
	u, v := fx-x0, fy-y0
	return (1-u)*(1-v)*float64(hs[0]) + u*(1-v)*float64(hs[1]) + (1-u)*v*float64(hs[2]) + u*v*float64(hs[3])
}

// Altitude is c's ground level, the mean of its corners.
func (b *Board) Altitude(c CellID) float64 { return b.Relief(c).Level() }

// Corners is the ground height at c's four corners — top-left, top-right, bottom-left,
// bottom-right — with c's column and row; false on a grid whose cells are level.
func (b *Board) Corners(c CellID) (hs [4]float64, x, y uint32, ok bool) {
	if !b.sloped() {
		return hs, 0, 0, false
	}
	x, y, ok = b.Coords(c)
	if !ok {
		return hs, 0, 0, false
	}
	r := b.Relief(c)
	for k := range hs {
		hs[k] = float64(r.Corners[k])
	}
	return hs, x, y, true
}

// At is GroundAt — the world.Ground contract.
func (b *Board) At(p geom.Vec) float64 { return b.GroundAt(p) }

// Step is how far apart sight samples the ground: the shorter side of a cell.
func (b *Board) Step() float64 {
	w, h := b.CellBounds()
	return min(w, h)
}

// Cell is an entity's current position on the board.
type Cell struct{ ID CellID }

// CellAABB is the size x size world rectangle centred on c.
func CellAABB(grid Grid, c CellID, size uint32) plane.AABB {
	center := grid.CellCenter(c)
	half := float64(size) / 2
	topLeft := geom.NewVec(center.X-half, center.Y-half)
	return plane.NewAABB(topLeft, float64(size), float64(size))
}

// Center returns pos's world-space center point.
func Center(pos world.Position) geom.Vec {
	return geom.NewVec(float64(pos.TopLeft.X)+float64(pos.Size.X)/2, float64(pos.TopLeft.Y)+float64(pos.Size.Y)/2)
}
