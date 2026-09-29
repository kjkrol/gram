package topography

import (
	"image"
	"slices"

	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/topography/terrain"
	"github.com/kjkrol/gram/render"
)

// groundSheet is the board's cells painted once, px pixels a cell, row by row as on the board from
// (0, top) of img: the tiles' sheet — the board's atlas with, below it, what lies over the tiles,
// the grounds running in and the ways, which from far a tile draws as one piece over its top
// drawn from the same sheet — or, based, the board's albedo: every cell's base under all that,
// for the ground drawn on the GPU (topography/terrain), with the board's water beside it.
type groundSheet struct {
	img   *render.Image
	atlas render.AtlasSource // the board's, at the top-left of the tiles' sheet
	top   int                // where the cells begin, below the atlas
	px    int                // how many pixels a cell spans
	based bool               // every cell's base is painted under what lies over it
	// water is a based sheet's water, wpx pixels a cell in the layers terrain.WaterLayers
	// names, each a quadrant; wcanvas gathers a layer's pieces; nil where the water would not fit
	water   *render.Image
	wpx     int
	wcanvas [terrain.WaterLayers]render.Frame
	// by ordinal: the bake a cell was painted from, its and its neighbours' newest; whether
	// anything lies painted over it
	painted []uint64
	dressed []bool
	seen    uint64 // one more than the board's count of changes when last brought up to date
}

var _ render.AtlasSource = (*groundSheet)(nil)

func (s *groundSheet) Atlas() *render.Image                           { return s.img }
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

// Sheet is the ground sheet made of the board's atlas while tiles may be dressed from it
// (sheeted), brought up to date with the board; else the atlas itself. The sheet holds the atlas
// as it is, so a tile drawn in full takes its sprites from it the same.
func (l *dresser) Sheet(atlas render.AtlasSource) render.AtlasSource {
	if !l.sheeted {
		return atlas
	}
	if l.sheet == nil || l.sheet.atlas != atlas {
		l.sheet = l.newSheet(atlas, false)
	}
	if l.sheet == nil {
		l.sheeted = false // nothing to paint on
		return atlas
	}
	l.paint(l.sheet)
	return l.sheet
}

// Surface is the whole board painted flat out of atlas, px pixels a cell — every cell's base with
// the grounds running in, the ways and the crossings over it — and its water, wpx pixels a cell
// (terrain.WaterLayers), brought up to date with the board, for the ground drawn on the GPU;
// nil off a square grid, where a cell would span too few pixels or the atlas has no sheet; no
// water where it would not fit.
func (l *dresser) Surface(atlas render.AtlasSource) (albedo, water *render.Image, px, wpx int) {
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

// Wet is, by cell of a square grid, row by row, whether water may lie on it — its own base shines,
// or its or a neighbour's ways or crossings run with water or glint where they turn into it — and
// the board's count of changes it holds for, 1 more: for the ground drawn on the GPU to leave the
// water out of the dry. Call it after Surface.
func (l *dresser) Wet() ([]bool, uint64) {
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
		own[i] = l.base(c).shine > 0 || wets(l.bakes[i].ways) || wets(l.bakes[i].crossings)
	}
	l.wet = make([]bool, n)
	for i := range n {
		l.around8(i, func(j int) { l.wet[i] = l.wet[i] || own[j] })
	}
	l.wetAt = at
	return l.wet, l.wetAt
}

// wets reports whether any of ways runs with water or glints where it turns into it.
func wets(ways []WayPiece) bool {
	for _, p := range ways {
		if p.Shine > 0 || p.MixShine > 0 {
			return true
		}
	}
	return false
}

// newSheet is a ground sheet for the board over atlas — the tiles', or based, the board's albedo —
// or nil where a cell would span too few pixels or the atlas has no sheet.
func (l *dresser) newSheet(atlas render.AtlasSource, based bool) *groundSheet {
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
	return s
}

// paint paints anew on s the cells whose grounds or ways, or their neighbours', have changed
// since: each alone while they are few, else the whole board at once.
func (l *dresser) paint(s *groundSheet) {
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
		l.paintAll(s)
		return
	}
	for _, i := range l.unpainted {
		l.paintCell(s, i)
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

// paintAll paints the whole board anew on s: every cell's base where s is based, every cell's
// grounds, then every way over them, then every crossing over those.
func (l *dresser) paintAll(s *groundSheet) {
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
	for i := range n {
		l.lay(s, i, nil, l.bakes[i].crossings)
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
		for i := range n {
			l.layWater(s, i, nil, l.bakes[i].crossings)
		}
		for q := range s.wcanvas {
			render.Paint(s.water.SubImage(s.waterLayer(q)), &s.wcanvas[q])
		}
	}
}

// paintCell paints cell i anew on s: its base where s is based, its grounds and the ways and
// crossings of it and its neighbours reaching over it.
func (l *dresser) paintCell(s *groundSheet, i int) {
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
	l.around8(i, func(j int) { l.lay(s, j, nil, l.bakes[j].crossings) })
	render.Paint(part, &l.canvas)
	if s.water != nil {
		l.resetWater(s)
		l.layWaterBase(s, i)
		l.layWater(s, i, b.blends, nil)
		l.around8(i, func(j int) { l.layWater(s, j, nil, l.bakes[j].ways) })
		l.around8(i, func(j int) { l.layWater(s, j, nil, l.bakes[j].crossings) })
		for q := range s.wcanvas {
			part := s.water.SubImage(s.waterCell(q, i%cols, i/cols))
			part.Clear()
			render.Paint(part, &s.wcanvas[q])
		}
	}
}

// layBase adds to the canvas cell i's base — the sprite its top is drawn in first, the sea under a
// coast — over its whole cell on s, in even light.
func (l *dresser) layBase(s *groundSheet, i int) {
	cols := int(l.sq.Cols)
	c, ok := l.cellAt(int64(i%cols), int64(i/cols))
	if !ok {
		return
	}
	r := s.cell(uint32(i%cols), uint32(i/cols))
	x0, y0, x1, y1 := float32(r.Min.X), float32(r.Min.Y), float32(r.Max.X), float32(r.Max.Y)
	l.canvas.Sprite(render.Ground, 0, s.atlas, l.base(c).sprite, render.Corners{{x0, y0}, {x1, y0}, {x0, y1}, {x1, y1}}, render.Even(1))
}

// lay adds to the canvas cell i's blends and ways, where they lie on s, in even light, drawn from
// the board's atlas: a sheet cannot draw on itself.
func (l *dresser) lay(s *groundSheet, i int, blends []BlendPiece, ways []WayPiece) {
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

// markWays marks dressed on s every cell cell i's ways and crossings reach over.
func (l *dresser) markWays(s *groundSheet, i int) {
	if len(l.bakes[i].ways) == 0 && len(l.bakes[i].crossings) == 0 {
		return
	}
	l.around8(i, func(j int) {
		if l.waysOver(i, j) {
			s.dressed[j] = true
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
	z := l.topOf(t.ID).z
	hazed(f, l.camera, l.weather, [4][3]float32{{t.X0, t.Y0, z[0]}, {t.X1, t.Y0, z[1]}, {t.X0, t.Y1, z[2]}, {t.X1, t.Y1, z[3]}})
	f.Fold(z)
}
