package landscape

import (
	"image"
	"slices"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/render"
)

// groundSheet is the board's atlas with, below it, what lies over the board's tiles painted once:
// the grounds running in and the ways, a cell every px pixels, row by row as on the board. From
// far a tile draws all of it as one piece of the sheet, over its top drawn from the same sheet.
type groundSheet struct {
	img   *ebiten.Image
	atlas render.AtlasSource // the board's, at the top-left
	top   int                // where the cells begin, below the atlas
	px    int                // how many pixels a cell spans
	// by ordinal: the bake a cell was painted from, its and its neighbours' newest; whether
	// anything lies painted over it
	painted []uint64
	dressed []bool
	seen    uint64 // one more than the board's count of changes when last brought up to date
}

var _ render.AtlasSource = (*groundSheet)(nil)

func (s *groundSheet) Atlas() *ebiten.Image                           { return s.img }
func (s *groundSheet) UV(id render.SpriteID) (x0, y0, x1, y1 float32) { return s.atlas.UV(id) }
func (s *groundSheet) White() (u, v float32)                          { return s.atlas.White() }

// cell is the part of the sheet cell x, y is painted in.
func (s *groundSheet) cell(x, y uint32) image.Rectangle {
	x0, y0 := int(x)*s.px, s.top+int(y)*s.px
	return image.Rect(x0, y0, x0+s.px, y0+s.px)
}

// The most pixels a cell spans on the ground sheet, the fewest worth painting, and how wide and tall
// a sheet may grow.
const (
	bakePx    = 16
	minBakePx = 4
	maxSheet  = 4096
)

// bakeCell is how many pixels a cell spans on screen under which a tile is dressed from the ground
// sheet: no finer than the sheet paints it.
const bakeCell = bakePx

// Sheet is the ground sheet made of the board's atlas while tiles are dressed from it (baking),
// brought up to date with the board; else the atlas itself.
func (l *dresser) Sheet(atlas render.AtlasSource) render.AtlasSource {
	if !l.baking {
		return atlas
	}
	if l.sheet == nil || l.sheet.atlas != atlas {
		l.sheet = l.newSheet(atlas)
	}
	if l.sheet == nil {
		l.baking = false // nothing to paint on
		return atlas
	}
	l.paint()
	return l.sheet
}

// newSheet is a ground sheet for the board over atlas, or nil where a cell would span too few
// pixels or the atlas has no sheet.
func (l *dresser) newSheet(atlas render.AtlasSource) *groundSheet {
	if atlas.Atlas() == nil {
		return nil
	}
	a := atlas.Atlas().Bounds().Size()
	cols, rows := int(l.sq.Cols), int(l.sq.Rows)
	px := min(bakePx, maxSheet/cols, (maxSheet-a.Y)/rows)
	if px < minBakePx {
		return nil
	}
	s := &groundSheet{atlas: atlas, top: a.Y, px: px,
		painted: make([]uint64, l.board.CellCount()), dressed: make([]bool, l.board.CellCount())}
	s.img = ebiten.NewImage(max(a.X, cols*px), a.Y+rows*px)
	s.img.DrawImage(atlas.Atlas(), nil)
	return s
}

// paint paints anew the cells whose grounds or ways, or their neighbours', have changed since: each
// alone while they are few, else the whole board at once.
func (l *dresser) paint() {
	s := l.sheet
	if s.seen == l.board.Changes()+1 {
		return
	}
	s.seen = l.board.Changes() + 1
	n := l.board.CellCount()
	if len(l.newest) != n {
		l.newest = make([]uint64, n)
	}
	for i := range n {
		c, _ := l.cellAt(int64(i%int(l.sq.Cols)), int64(i/int(l.sq.Cols)))
		l.newest[i] = l.bakeOf(l.cellTile(c)).ver
	}
	l.unpainted = l.unpainted[:0]
	for i := range n {
		v := uint64(0)
		l.around8(i, func(j int) { v = max(v, l.newest[j]) })
		if s.painted[i] != v {
			s.painted[i] = v
			l.unpainted = append(l.unpainted, i)
		}
	}
	if len(l.unpainted) > n/8 {
		l.paintAll()
		return
	}
	for _, i := range l.unpainted {
		l.paintCell(i)
	}
}

// around8 calls fn with the ordinal of the cell i and each of its neighbours on the board.
func (l *dresser) around8(i int, fn func(j int)) {
	cols := int64(l.sq.Cols)
	x, y := int64(i)%cols, int64(i)/cols
	for dy := int64(-1); dy <= 1; dy++ {
		for dx := int64(-1); dx <= 1; dx++ {
			if c, ok := l.cellAt(x+dx, y+dy); ok {
				j, _ := l.ordinal(c)
				fn(j)
			}
		}
	}
}

// cellTile is the tile of cell c as the board's renderer would hand it, for working out its bake.
func (l *dresser) cellTile(c board.CellID) *tile {
	center := l.board.CellCenter(c)
	l.scratch = board.Tile{ID: c,
		X0: float32(center.X - l.cellW/2), Y0: float32(center.Y - l.cellH/2),
		X1: float32(center.X + l.cellW/2), Y1: float32(center.Y + l.cellH/2)}
	l.bakeTile = tile{Tile: &l.scratch, r: l, id: c}
	return &l.bakeTile
}

// paintAll paints the whole board anew: every cell's grounds, then every way over them, then every
// crossing over those.
func (l *dresser) paintAll() {
	s := l.sheet
	cells := s.img.SubImage(image.Rect(0, s.top, int(l.sq.Cols)*s.px, s.top+int(l.sq.Rows)*s.px)).(*ebiten.Image)
	cells.Clear()
	l.canvas.Reset(nil)
	clear(s.dressed)
	n := l.board.CellCount()
	for i := range n {
		b := &l.bakes[i]
		l.lay(i, b.blends, nil)
		s.dressed[i] = s.dressed[i] || len(b.blends) > 0
	}
	for i := range n {
		l.lay(i, nil, l.bakes[i].ways)
		l.markWays(i)
	}
	for i := range n {
		l.lay(i, nil, l.bakes[i].crossings)
	}
	render.Paint(cells, &l.canvas)
}

// paintCell paints cell i anew: its grounds and the ways and crossings of it and its neighbours
// reaching over it.
func (l *dresser) paintCell(i int) {
	s := l.sheet
	cols := int(l.sq.Cols)
	part := s.img.SubImage(s.cell(uint32(i%cols), uint32(i/cols))).(*ebiten.Image)
	part.Clear()
	l.canvas.Reset(nil)
	b := &l.bakes[i]
	s.dressed[i] = len(b.blends) > 0
	l.lay(i, b.blends, nil)
	l.around8(i, func(j int) {
		l.lay(j, nil, l.bakes[j].ways)
		if l.waysOver(j, i) {
			s.dressed[i] = true
		}
	})
	l.around8(i, func(j int) { l.lay(j, nil, l.bakes[j].crossings) })
	render.Paint(part, &l.canvas)
}

// lay adds to the canvas cell i's blends and ways, where they lie on the sheet, in even light, drawn
// from the board's atlas: a sheet cannot draw on itself.
func (l *dresser) lay(i int, blends []BlendPiece, ways []WayPiece) {
	s := l.sheet
	cols := int(l.sq.Cols)
	x0, y0 := float32((i%cols)*s.px), float32(s.top+(i/cols)*s.px)
	kx, ky := float32(s.px)/float32(l.cellW), float32(s.px)/float32(l.cellH)
	at := func(w render.World) render.Corners {
		var c render.Corners
		for k, p := range w {
			c[k] = [2]float32{x0 + p[0]*kx, y0 + p[1]*ky}
		}
		return c
	}
	for _, p := range blends {
		l.canvas.SpriteBlend(render.Ground, 0, s.atlas, p.Sprite, at(p.World), render.Even(1), p.Weight, p.Soft)
	}
	for _, p := range ways {
		p.draw(&l.canvas, render.Ground, 0, s.atlas, at(p.World), render.Even(1))
	}
}

// markWays marks dressed every cell cell i's ways and crossings reach over.
func (l *dresser) markWays(i int) {
	if len(l.bakes[i].ways) == 0 && len(l.bakes[i].crossings) == 0 {
		return
	}
	l.around8(i, func(j int) {
		if l.waysOver(i, j) {
			l.sheet.dressed[j] = true
		}
	})
}

// waysOver reports whether a piece of cell i's ways or crossings lies over cell j, i itself or a
// neighbour.
func (l *dresser) waysOver(i, j int) bool {
	cols := int(l.sq.Cols)
	// j's box as i's pieces are placed, from i's top-left; a step round a wrap folded back
	dx, dy := j%cols-i%cols, j/cols-i/cols
	dx, dy = unwrap(dx, cols), unwrap(dy, int(l.sq.Rows))
	w, h := float32(l.cellW), float32(l.cellH)
	bx0, by0 := float32(dx)*w, float32(dy)*h
	for _, p := range slices.Concat(l.bakes[i].ways, l.bakes[i].crossings) {
		px0, py0, px1, py1 := p.World[0][0], p.World[0][1], p.World[0][0], p.World[0][1]
		for _, q := range p.World[1:] {
			px0, py0, px1, py1 = min(px0, q[0]), min(py0, q[1]), max(px1, q[0]), max(py1, q[1])
		}
		if px0 < bx0+w && px1 > bx0 && py0 < by0+h && py1 > by0 {
			return true
		}
	}
	return false
}

// unwrap is d, a step between neighbours on an axis n long, as -1, 0 or 1 where it wraps round.
func unwrap(d, n int) int {
	switch {
	case d > 1:
		return d - n
	case d < -1:
		return d + n
	}
	return d
}

// dressBaked lays over tile t, its top drawn at corners, what lies over it as one piece of the
// ground sheet, where anything does.
func (l *dresser) dressBaked(f *render.Frame, t *tile, corners render.Corners, depth float32) {
	s := l.sheet
	i, _ := l.ordinal(t.ID)
	if !s.dressed[i] {
		return
	}
	cols := int(l.sq.Cols)
	r := s.cell(uint32(i%cols), uint32(i/cols))
	src := [4]float32{float32(r.Min.X), float32(r.Min.Y), float32(r.Max.X), float32(r.Max.Y)}
	f.SpritePart(render.Ground, depth, s, src, corners, t.Light())
	f.Fold(l.topOf(t.ID).z)
}
