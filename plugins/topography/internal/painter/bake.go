package painter

import (
	"image"

	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/topography/internal/water"
	public "github.com/kjkrol/gram/plugins/topography/painter"
	"github.com/kjkrol/gram/render"
)

type groundSheet struct {
	img   *render.Image
	atlas render.AtlasSource
	top   int
	px    int
	based bool
	// nil where the water would not fit
	water   *render.Image
	wpx     int
	wcanvas [water.Layers]render.Frame
	// nil but for the board's own sheet: the detail each pixel's kind carries, in red
	kinds   *render.Image
	kcanvas render.Frame
	painted []uint64
	dressed []bool
	// by cell, the detail of its kind, or Dressed where its pixels hold more than one
	cells   []byte
	cellsAt uint64
	seen    uint64
	styled  uint64
}

// Dressed marks a cell whose pixels hold more than one kind in CellDetails.
const Dressed = public.Dressed

var _ render.AtlasSource = (*groundSheet)(nil)

func (s *groundSheet) Atlas() *render.Image                           { return s.img }
func (s *groundSheet) UV(id render.SpriteID) (x0, y0, x1, y1 float32) { return s.atlas.UV(id) }
func (s *groundSheet) White() (u, v float32)                          { return s.atlas.White() }

func (s *groundSheet) cell(x, y uint32) image.Rectangle {
	x0, y0 := int(x)*s.px, s.top+int(y)*s.px
	return image.Rect(x0, y0, x0+s.px, y0+s.px)
}

const (
	bakePx    = 16
	minBakePx = 4
	maxSheet  = 4096
)

const bakeCell = bakePx

// Sheet is the atlas itself: the tiles are dressed only to be composed once, from above.
func (l *Painter) Sheet(atlas render.AtlasSource) render.AtlasSource { return atlas }

// Surface is the whole board painted flat out of atlas, px pixels a cell, and its water, wpx
// pixels a cell, brought up to date with the board. It is nil off a square grid, where a cell
// would span too few pixels or the atlas has no sheet; the water is nil where it would not fit.
func (l *Painter) Surface(atlas render.AtlasSource) (albedo, water *render.Image, px, wpx int) {
	if !l.square {
		return nil, nil, 0, 0
	}
	if l.albedo == nil || l.albedo.atlas != atlas {
		l.albedo = l.newSheet(atlas, true)
	}
	if l.albedo == nil {
		return nil, nil, 0, 0
	}
	l.tables()
	l.paint(l.albedo)
	return l.albedo.img, l.albedo.water, l.albedo.px, l.albedo.wpx
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
		own[i] = l.base(c).shine > 0 || wets(l.bakes[i].ways)
	}
	l.wet = make([]bool, n)
	for i := range n {
		l.around8(i, func(j int) { l.wet[i] = l.wet[i] || own[j] })
	}
	l.wetAt = at
	return l.wet, l.wetAt
}

func wets(ways []WayPiece) bool {
	for _, p := range ways {
		if p.Shine > 0 || p.MixShine > 0 {
			return true
		}
	}
	return false
}

func (l *Painter) newSheet(atlas render.AtlasSource, based bool) *groundSheet {
	if atlas.Atlas() == nil {
		return nil
	}
	a := atlas.Atlas().Bounds().Size()
	if based {
		a = image.Point{}
	}
	cols, rows := int(l.sq.Cols), int(l.sq.Rows)
	px := min(bakePx, maxSheet/cols, (maxSheet-a.Y)/rows)
	if px < minBakePx {
		return nil
	}
	s := &groundSheet{atlas: atlas, top: a.Y, px: px, based: based,
		painted: make([]uint64, l.board.CellCount()), dressed: make([]bool, l.board.CellCount())}
	s.img = render.NewImage(max(a.X, cols*px), a.Y+rows*px)
	if !based {
		s.img.DrawImage(atlas.Atlas(), nil)
	}
	if wpx := min(px, maxSheet/(2*cols), maxSheet/(2*rows)); based && wpx >= minBakePx {
		s.water, s.wpx = render.NewImage(2*cols*wpx, 2*rows*wpx), wpx
	}
	if based {
		s.kinds = render.NewImage(cols*px, rows*px)
	}
	return s
}

func (l *Painter) paint(s *groundSheet) {
	if s.seen == l.board.Changes()+1 && s.styled == l.styleVersion {
		return
	}
	restyled := s.styled != l.styleVersion
	s.seen, s.styled = l.board.Changes()+1, l.styleVersion
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
	if restyled || len(l.unpainted) > n/8 {
		l.paintAll(s)
	} else {
		for _, i := range l.unpainted {
			l.paintCell(s, i)
		}
	}
	l.noteCells(s)
}

func (l *Painter) noteCells(s *groundSheet) {
	n := l.board.CellCount()
	if len(s.cells) != n {
		s.cells = make([]byte, n)
	}
	cols := int(l.sq.Cols)
	for i := range n {
		s.cells[i] = Dressed
		if c, ok := l.cellAt(int64(i%cols), int64(i/cols)); ok && !s.dressed[i] {
			s.cells[i] = l.detailOf(l.base(c).sprite)
		}
	}
	s.cellsAt++
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
		X0: float32(center.X - l.cellW/2), Y0: float32(center.Y - l.cellH/2),
		X1: float32(center.X + l.cellW/2), Y1: float32(center.Y + l.cellH/2)}
	l.bakeTile = tile{box: &l.scratch, r: l, id: c}
	return &l.bakeTile
}

func (l *Painter) paintAll(s *groundSheet) {
	cells := s.img.SubImage(image.Rect(0, s.top, int(l.sq.Cols)*s.px, s.top+int(l.sq.Rows)*s.px))
	cells.Clear()
	l.canvas.Reset(nil)
	clear(s.dressed)
	n := l.board.CellCount()
	for i := range n {
		b := &l.bakes[i]
		if s.based {
			l.layBase(s, i)
		}
		l.lay(s, i, b.blends, nil)
		s.dressed[i] = s.dressed[i] || len(b.blends) > 0
	}
	for i := range n {
		l.lay(s, i, nil, l.bakes[i].ways)
		l.markWays(s, i)
	}
	render.Paint(cells, &l.canvas)
	if s.water != nil {
		s.water.Clear()
		l.resetWater(s)
		for i := range n {
			l.layWaterBase(s, i)
			l.layWater(s, i, l.bakes[i].blends, nil)
		}
		for i := range n {
			l.layWater(s, i, nil, l.bakes[i].ways)
		}
		for q := range s.wcanvas {
			render.Paint(s.water.SubImage(s.waterLayer(q)), &s.wcanvas[q])
		}
	}
	if s.kinds != nil {
		s.kinds.Clear()
		s.kcanvas.Reset(nil)
		for i := range n {
			l.layKindBase(s, i)
			l.layKinds(s, i, l.bakes[i].blends, nil)
		}
		for i := range n {
			l.layKinds(s, i, nil, l.bakes[i].ways)
		}
		render.Paint(s.kinds, &s.kcanvas)
	}
}

func (l *Painter) paintCell(s *groundSheet, i int) {
	cols := int(l.sq.Cols)
	part := s.img.SubImage(s.cell(uint32(i%cols), uint32(i/cols)))
	part.Clear()
	l.canvas.Reset(nil)
	b := &l.bakes[i]
	s.dressed[i] = len(b.blends) > 0
	if s.based {
		l.layBase(s, i)
	}
	l.lay(s, i, b.blends, nil)
	l.around8(i, func(j int) {
		l.lay(s, j, nil, l.bakes[j].ways)
		if l.waysOver(j, i) {
			s.dressed[i] = true
		}
	})
	render.Paint(part, &l.canvas)
	if s.water != nil {
		l.resetWater(s)
		l.layWaterBase(s, i)
		l.layWater(s, i, b.blends, nil)
		l.around8(i, func(j int) { l.layWater(s, j, nil, l.bakes[j].ways) })
		for q := range s.wcanvas {
			part := s.water.SubImage(s.waterCell(q, i%cols, i/cols))
			part.Clear()
			render.Paint(part, &s.wcanvas[q])
		}
	}
	if s.kinds != nil {
		s.kcanvas.Reset(nil)
		l.layKindBase(s, i)
		l.layKinds(s, i, b.blends, nil)
		l.around8(i, func(j int) { l.layKinds(s, j, nil, l.bakes[j].ways) })
		part := s.kinds.SubImage(s.cell(uint32(i%cols), uint32(i/cols)))
		part.Clear()
		render.Paint(part, &s.kcanvas)
	}
}

func (l *Painter) layBase(s *groundSheet, i int) {
	cols := int(l.sq.Cols)
	c, ok := l.cellAt(int64(i%cols), int64(i/cols))
	if !ok {
		return
	}
	r := s.cell(uint32(i%cols), uint32(i/cols))
	x0, y0, x1, y1 := float32(r.Min.X), float32(r.Min.Y), float32(r.Max.X), float32(r.Max.Y)
	corners := render.Corners{{x0, y0}, {x1, y0}, {x0, y1}, {x1, y1}}
	l.canvas.Sprite(render.At{}, s.atlas, l.base(c).sprite, corners, render.Even(1))
}

// drawn from the board's atlas: a sheet cannot draw on itself
func (l *Painter) lay(s *groundSheet, i int, blends []BlendPiece, ways []WayPiece) {
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
		l.canvas.SpriteBlend(render.At{}, s.atlas, p.Sprite, at(p.World), render.Even(1), p.Weight,
			p.Soft)
	}
	for _, p := range ways {
		p.draw(&l.canvas, render.At{}, s.atlas, at(p.World), render.Even(1))
	}
}

func (l *Painter) markWays(s *groundSheet, i int) {
	if len(l.bakes[i].ways) == 0 {
		return
	}
	l.around8(i, func(j int) {
		if l.waysOver(i, j) {
			s.dressed[j] = true
		}
	})
}

func (l *Painter) waysOver(i, j int) bool {
	cols := int(l.sq.Cols)
	dx, dy := j%cols-i%cols, j/cols-i/cols
	dx, dy = unwrap(dx, cols), unwrap(dy, int(l.sq.Rows))
	w, h := float32(l.cellW), float32(l.cellH)
	bx0, by0 := float32(dx)*w, float32(dy)*h
	for _, p := range l.bakes[i].ways {
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

func unwrap(d, n int) int {
	switch {
	case d > 1:
		return d - n
	case d < -1:
		return d + n
	}
	return d
}
