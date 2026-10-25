package painter

import (
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/look"
	"github.com/kjkrol/gram/plugins/topography/internal/relief"
	"github.com/kjkrol/gram/plugins/topography/internal/water"
	"github.com/kjkrol/gram/render"
)

// Painter is the board's Dressing in relief: the light on the tiles and what lies on them, and
// the board painted flat for the ground drawn on the GPU.
type Painter struct {
	board   Board
	ground  func() *relief.Relief
	styles  map[cell.Name]Style
	kinds   cell.Kinds
	heights bool
	square  bool
	sq      grid.Shape

	camera camera.Camera
	cellW  float32
	cellH  float32
	cellPx float32
	varies bool // a perspective draws a world unit larger near than far
	tile   tile

	tops         []cellTop
	wet          []bool
	wetAt        uint64
	albedo       *groundSheet
	details      *detailSheet
	detailsBaked uint64
	styleVersion uint64
	scratch      box
	bakeTile     tile
	lanes        lanes
	cells        cells
	baking       baking
	shoreMap     shoreMap
}

var (
	_ look.Dressing = (*Painter)(nil)
	_ look.Overlaid = (*Painter)(nil)
)

// Board is what the painter reads of a board: a look.Board, and each cell's way.
type Board interface {
	look.Board
	Way(c cell.ID) cell.Way
}

// New is the painter of b over the relief ground gives, the kinds looking as styles has them.
// heights says whether the ground stands in relief.
func New(b Board, ground func() *relief.Relief, heights bool,
	styles map[cell.Name]Style) *Painter {
	w, h := b.CellBounds()
	sq, square := b.Shape()
	square = square && !sq.Hex
	return &Painter{board: b, ground: ground, styles: styles, heights: heights, square: square,
		sq: sq, cellW: w, cellH: h}
}

// WithKinds has the painter find the kinds its Styles name in kinds.
func (l *Painter) WithKinds(kinds cell.Kinds) *Painter {
	l.kinds = kinds
	return l
}

// Style sets how the kind named name looks in relief beyond its sprite.
func (l *Painter) Style(name string, s Style) {
	l.styles[cell.Named(name)] = s
	l.styleVersion++
}

// StyleOf is how the kind named name looks, as Style set it.
func (l *Painter) StyleOf(name string) Style { return l.styles[cell.Named(name)] }

// box is a cell's tile over its box in the world
type box struct {
	ID             cell.ID
	X0, Y0, X1, Y1 float32
}

type tile struct {
	*box
	r          *Painter
	id         cell.ID
	base       *cellTop
	pixels     float32
	pixelsRead bool
}

// at the corner the camera draws largest: a tile near a perspective's eye keeps all its detail
// even where its middle lies behind the eye
func (t *tile) px() float32 {
	if !t.pixelsRead {
		t.pixels, t.pixelsRead = t.r.cellPx, true
		if cam := t.r.camera; cam != nil && t.r.varies {
			z := t.r.topOf(t.ID).z
			scale := float32(0)
			for k, p := range [4][2]float32{{t.X0, t.Y0}, {t.X1, t.Y0}, {t.X0, t.Y1}, {t.X1, t.Y1}} {
				scale = max(scale, camera.ScaleAt(cam, p[0], p[1], z[k]))
			}
			t.pixels = min(t.r.cellW, t.r.cellH) * scale
		}
	}
	return t.pixels
}

type cellTop struct {
	z      [4]float32 // the ground's corners with the kind's Height on them
	ground [4]float32
	alt    float32
	shine  float32
	flow   float32
	spread float32
	raised bool
	under  bool
	sprite render.SpriteID
	way    lane
	ver    uint64 // one more than the cell's version when read; 0 not read yet
	seen   uint64
}

// Begin readies the Painter for a frame through cam.
func (l *Painter) Begin(f *render.Frame, cam camera.Camera) {
	l.camera, l.tile = cam, tile{}
	l.tables()
	l.cellPx = min(l.cellW, l.cellH) * cam.Zoom()
	w, h := cam.Viewport()
	x, y := cam.Unproject(w/2, 0, 0)
	far := camera.ScaleAt(cam, x, y, 0)
	x, y = cam.Unproject(w/2, h, 0)
	l.varies = far != camera.ScaleAt(cam, x, y, 0) || far != cam.Zoom()
}

// Above readies the Painter for the board drawn whole from above, a world unit a pixel, as Begin
// readies it for a frame through a camera.
func (l *Painter) Above() {
	l.camera, l.tile = nil, tile{}
	l.tables()
	l.cellPx, l.varies = min(l.cellW, l.cellH), false
}

// Overlay is the still water glinting over c's top where its kind shines, off a square grid,
// whose water neither runs nor meets a shore; a square grid's the terrain paints.
func (l *Painter) Overlay(c cell.ID) (render.MaterialID, float32, bool) {
	if l.square {
		return 0, 0, false
	}
	shine, ok := l.cellTile(c).Shine()
	if !ok {
		return 0, 0, false
	}
	return water.SeaGlint(), shine, true
}

func (l *Painter) tables() {
	if n := l.board.CellCount(); len(l.tops) != n {
		l.tops = make([]cellTop, n)
	}
}

// Base is the sprite c's top is drawn in: its own kind's, or the kind Under it round it.
func (l *Painter) Base(c cell.ID, _ render.SpriteID) render.SpriteID {
	return l.cellTile(c).baseTop().sprite
}

func (l *Painter) topOf(c cell.ID) *cellTop {
	i, _ := l.ordinal(c)
	t := &l.tops[i]
	changes := l.board.Changes()
	if t.ver != 0 && t.seen == changes {
		return t
	}
	v := l.board.CellVersion(c) + 1
	t.seen = changes
	if t.ver == v {
		return t
	}
	kind := l.board.Kind(c)
	style := l.styles[kind.Name]
	r := l.ground().Corners(c)
	t.alt, t.sprite, t.ver = r.Level(), kind.SpriteID, v
	t.shine, t.flow, t.spread, t.under, t.raised = style.Shine, style.Flow, style.Spread, style.Under, kind.Height > 0
	t.way = l.laneOf(l.board.Way(c))
	if l.square {
		t.ground = [4]float32(r)
	} else {
		t.ground = [4]float32{t.alt, t.alt, t.alt, t.alt}
	}
	for k := range t.z {
		t.z[k] = t.ground[k] + kind.Height
	}
	return t
}

type lane struct {
	cell.Way
	shine, flow float32
	mix         render.SpriteID
	mixes       bool
	mixShine    float32
}

func (l *Painter) laneOf(w cell.Way) lane {
	s := l.styles[w.Kind.Name]
	ln := lane{Way: w, shine: s.Shine, flow: s.Flow}
	if s.MixWith != "" && l.kinds != nil {
		if k, ok := l.kinds.Get(s.MixWith); ok {
			ln.mix, ln.mixes, ln.mixShine = k.SpriteID, true, l.styles[k.Name].Shine
		}
	}
	return ln
}

func (l *Painter) ordinal(c cell.ID) (int, bool) { return l.board.Ordinal(c) }

func (l *Painter) cellAt(x, y int64) (cell.ID, bool) {
	fx, okX := fold(x, int64(l.sq.Cols), l.sq.WrapX)
	fy, okY := fold(y, int64(l.sq.Rows), l.sq.WrapY)
	if !okX || !okY {
		return 0, false
	}
	return l.board.At(uint32(fx), uint32(fy)), true
}

func fold(v, n int64, wrap bool) (int64, bool) {
	if wrap {
		return (v%n + n) % n, true
	}
	return v, v >= 0 && v < n
}
