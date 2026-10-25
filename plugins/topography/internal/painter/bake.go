package painter

import (
	"github.com/kjkrol/gram/plugins/board/cell"
	public "github.com/kjkrol/gram/plugins/topography/painter"
	"github.com/kjkrol/gram/render"
)

// groundSheet is the board painted flat: its albedo, the water's layers in quadrants beside it,
// the detail of each pixel's kind, px and wpx pixels a cell, and what the terrain reads of each
// cell's detail
type groundSheet struct {
	img   *render.Image
	atlas render.AtlasSource
	top   int
	px    int
	// nil where the water would not fit
	water *render.Image
	wpx   int
	kinds *render.Image
	// by cell, the detail of its kind, or Dressed where its pixels hold more than one
	cells   []byte
	cellsAt uint64
	seen    uint64 // the cells' count of changes it was painted at; 0 never
	styled  uint64
}

// Dressed marks a cell whose pixels hold more than one kind in CellDetails.
const Dressed = public.Dressed

const (
	bakePx    = 16
	minBakePx = 4
	maxSheet  = 4096
)

// Surface is the whole board painted flat out of atlas, px pixels a cell, and its water, wpx
// pixels a cell, brought up to date with the board on the GPU: the cells round those that
// changed painted anew, all of them when many did. It is nil off a square grid, where a cell
// would span too few pixels or the atlas has no sheet; the water is nil where it would not fit.
func (l *Painter) Surface(atlas render.AtlasSource) (albedo, water *render.Image, px, wpx int) {
	if !l.square {
		return nil, nil, 0, 0
	}
	if l.albedo == nil || l.albedo.atlas != atlas {
		l.albedo = l.newSheet(atlas)
	}
	s := l.albedo
	if s == nil {
		return nil, nil, 0, 0
	}
	l.tables()
	l.refresh(atlas)
	g := &l.cells
	if s.seen != g.at || s.styled != l.styleVersion {
		l.bake(s, atlas, l.toPaint(s.seen != g.prev || s.styled != l.styleVersion))
		s.seen, s.styled = g.at, l.styleVersion
		l.noteCells(s)
	}
	return s.img, s.water, s.px, s.wpx
}

// Wet is, by cell of a square grid, row by row, whether water may lie on it, and the board's
// count of changes it holds for, 1 more. Call it after Surface.
func (l *Painter) Wet() ([]bool, uint64) {
	at := l.board.Changes() + 1
	if !l.square || l.wetAt == at && len(l.wet) == l.board.CellCount() {
		return l.wet, l.wetAt
	}
	n := l.board.CellCount()
	own := make([]bool, n)
	cols := int(l.sq.Cols)
	for i := range n {
		c, ok := l.cellAt(int64(i%cols), int64(i/cols))
		if !ok {
			continue
		}
		own[i] = l.base(c).shine > 0 || l.wets(c)
	}
	l.wet = make([]bool, n)
	for i := range n {
		l.around8(i, func(j int) { l.wet[i] = l.wet[i] || own[j] })
	}
	l.wetAt = at
	return l.wet, l.wetAt
}

// wets reports whether water lies on the way across c: it shines, or turns into a kind that does
func (l *Painter) wets(c cell.ID) bool {
	w := l.topOf(c).way
	return w.Runs() && l.heights && (w.shine > 0 || w.mixShine > 0)
}

func (l *Painter) newSheet(atlas render.AtlasSource) *groundSheet {
	if atlas.Atlas() == nil {
		return nil
	}
	cols, rows := int(l.sq.Cols), int(l.sq.Rows)
	px := min(bakePx, maxSheet/cols, maxSheet/rows)
	if px < minBakePx {
		return nil
	}
	s := &groundSheet{atlas: atlas, px: px}
	s.img = render.NewImage(cols*px, rows*px)
	if wpx := min(px, maxSheet/(2*cols), maxSheet/(2*rows)); wpx >= minBakePx {
		s.water, s.wpx = render.NewImage(2*cols*wpx, 2*rows*wpx), wpx
	}
	s.kinds = render.NewImage(cols*px, rows*px)
	return s
}

func (l *Painter) noteCells(s *groundSheet) {
	n := l.board.CellCount()
	if len(s.cells) != n {
		s.cells = make([]byte, n)
	}
	cols := int(l.sq.Cols)
	for i := range n {
		s.cells[i] = Dressed
		if c, ok := l.cellAt(int64(i%cols), int64(i/cols)); ok && !l.dressed(c) {
			s.cells[i] = l.detailOf(l.base(c).sprite)
		}
	}
	s.cellsAt++
}

// dressed reports whether more than c's own kind may lie on its top: the grounds round it
// running in, where it runs into them and a neighbour is of another sprite or lies otherwise
// under the rest, or a way across it or a cell beside it, which may reach in
func (l *Painter) dressed(c cell.ID) bool {
	mine := l.topOf(c)
	if mine.way.Runs() {
		return true
	}
	blends := !mine.raised && (mine.under || mine.spread > 0)
	for d := range l.board.Directions() {
		n, ok := l.board.Toward(c, d)
		if !ok {
			continue
		}
		t := l.topOf(n)
		if t.way.Runs() || blends && (t.sprite != mine.sprite || t.under != mine.under) {
			return true
		}
	}
	return false
}

func (l *Painter) around8(i int, fn func(j int)) {
	cols := int64(l.sq.Cols)
	c, ok := l.cellAt(int64(i)%cols, int64(i)/cols)
	if !ok {
		return
	}
	fn(i)
	for d := range l.board.Directions() {
		if n, ok := l.board.Toward(c, d); ok {
			j, _ := l.ordinal(n)
			fn(j)
		}
	}
}

func (l *Painter) cellTile(c cell.ID) *tile {
	center := l.board.CellCenter(c)
	l.scratch = box{ID: c,
		X0: center.X - l.cellW/2, Y0: center.Y - l.cellH/2,
		X1: center.X + l.cellW/2, Y1: center.Y + l.cellH/2}
	l.bakeTile = tile{box: &l.scratch, r: l, id: c}
	return &l.bakeTile
}
