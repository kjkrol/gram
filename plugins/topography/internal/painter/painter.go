package painter

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/atmosphere/air"
	"github.com/kjkrol/gram/plugins/atmosphere/sky"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/look"
	"github.com/kjkrol/gram/plugins/topography/internal/relief"
	"github.com/kjkrol/gram/render"
)

// Painter is the board's Dressing in relief: the light on the tiles and what lies on them, and
// the board painted flat for the ground drawn on the GPU.
type Painter struct {
	board   *board.Board
	relief  *relief.Relief
	sky     Sky
	styles  map[cell.Name]Style
	kinds   cell.Kinds
	heights bool
	square  bool
	sq      grid.Shape

	lighted sky.Sun
	weather air.Weather
	camera  camera.Camera
	cellW   float64
	cellH   float64
	cellPx  float32
	varies  bool // a perspective draws a world unit larger near than far
	tile    tile

	ways   []WayPiece
	blends []BlendPiece
	tops   []cellTop
	bakes  []cellBake
	// by corner of a square grid, good while the stamp is shoreStamp
	shores     []cornerShore
	shoreStamp uint32
	shoreFor   uint64
	wet        []bool
	wetAt      uint64
	albedo     *groundSheet
	newest     []uint64
	unpainted  []int
	canvas     render.Frame
	scratch    look.Tile
	bakeTile   tile
	coast      coast
}

var _ look.Dressing = (*Painter)(nil)

// Sky is the sky over the board as the painter reads it: the sun that lights the tiles and the
// weather.
type Sky interface {
	Sun() sky.Sun
	Air() air.Weather
}

// New is the painter of b over ground under the sky, the kinds looking as styles has them.
// heights says whether the ground stands in relief.
func New(b *board.Board, ground *relief.Relief, sky Sky, heights bool, styles map[cell.Name]Style) *Painter {
	w, h := b.CellBounds()
	sq, square := b.Shape()
	square = square && !sq.Hex
	return &Painter{board: b, relief: ground, sky: sky, styles: styles, heights: heights, square: square, sq: sq, cellW: w, cellH: h}
}

// WithKinds has the painter find the kinds its Styles name in kinds.
func (l *Painter) WithKinds(kinds cell.Kinds) *Painter {
	l.kinds = kinds
	return l
}

// Style sets how the kind named name looks in relief beyond its sprite.
func (l *Painter) Style(name string, s Style) { l.styles[cell.Named(name)] = s }

// StyleOf is how the kind named name looks, as Style set it.
func (l *Painter) StyleOf(name string) Style { return l.styles[cell.Named(name)] }

func (l *Painter) version() uint64 { return l.board.Version() + l.relief.Version() }

// sea level 0 off the board
func (l *Painter) beside(t *look.Tile, dx, dy int) [4]float32 {
	w, h := t.X1-t.X0, t.Y1-t.Y0
	x, y := (t.X0+t.X1)/2+float32(dx)*w, (t.Y0+t.Y1)/2+float32(dy)*h
	c, ok := l.board.CellAt(geom.NewVec(float64(x), float64(y)))
	if !ok {
		return [4]float32{}
	}
	return l.topOf(c).z
}

type tile struct {
	*look.Tile
	r          *Painter
	id         cell.ID
	memo       tileMemo
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
			t.pixels = float32(min(t.r.cellW, t.r.cellH)) * scale
		}
	}
	return t.pixels
}

func (l *Painter) inFront(cam camera.Camera, w render.World, z [4]float32) bool {
	if !l.varies {
		return true
	}
	for k, p := range w {
		if camera.ScaleAt(cam, p[0], p[1], z[k]) == 0 {
			return false
		}
	}
	return true
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
	l.lighted, l.weather = l.sky.Sun(), l.sky.Air()
	l.tables()
	l.cellPx = float32(min(l.cellW, l.cellH)) * cam.Zoom()
	w, h := cam.Viewport()
	x, y := cam.Unproject(w/2, 0, 0)
	far := camera.ScaleAt(cam, x, y, 0)
	x, y = cam.Unproject(w/2, h, 0)
	l.varies = far != camera.ScaleAt(cam, x, y, 0) || far != cam.Zoom()
	l.nextShores()
}

func (l *Painter) tables() {
	if n := l.board.CellCount(); len(l.tops) != n {
		l.tops, l.bakes = make([]cellTop, n), make([]cellBake, n)
	}
}

// Base is the sprite t's top is drawn in first: its own kind's, or the kind Under it round it.
func (l *Painter) Base(t *look.Tile) render.SpriteID { return l.tileOf(t).baseTop().sprite }

// Light is the light on t's top at its corners.
func (l *Painter) Light(t *look.Tile) render.Shade { return l.tileOf(t).Light() }

// FaceLight is the light on t's upright face looking dx, dy cells away.
func (l *Painter) FaceLight(t *look.Tile, dx, dy int) render.Light {
	return l.tileOf(t).FaceLight(dx, dy)
}

// Covers reports whether Dress lays over t's top anything that would hide its outline.
func (l *Painter) Covers(t *look.Tile) bool {
	b := l.bakeOf(l.tileOf(t))
	return len(b.blends) > 0 || len(b.ways) > 0
}

// Dress lays over t's top, drawn over the box x0..x1, y0..y1 lying at at, what lies on it: its
// water, the grounds round it running in, its outline where covered.
func (l *Painter) Dress(
	f *render.Frame, cam camera.Camera, t *look.Tile, x0, y0, x1, y1 float32, at render.At,
) {
	d, top := l.tileOf(t), f.Last()
	d.DrawSurface(f, x0, y0, x1, y1)
	d.DrawBlends(f, cam, at)
	if t.Outlined && l.Covers(t) {
		f.OutlineOn(top)
	}
}

// Across lays the way across t.
func (l *Painter) Across(f *render.Frame, cam camera.Camera, t *look.Tile) {
	l.tileOf(t).DrawWay(f, cam)
}

func (l *Painter) tileOf(t *look.Tile) *tile {
	if l.tile.Tile != t || l.tile.id != t.ID {
		l.tile = tile{Tile: t, r: l, id: t.ID}
	}
	return &l.tile
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
	r := l.relief.Corners(c)
	t.alt, t.sprite, t.ver = float32(r.Level()), kind.SpriteID, v
	t.shine, t.flow, t.spread, t.under, t.raised = style.Shine, style.Flow, style.Spread, style.Under, kind.Height > 0
	t.way = l.laneOf(l.board.Way(c))
	if l.square {
		t.ground = [4]float32(r)
	} else {
		t.ground = [4]float32{t.alt, t.alt, t.alt, t.alt}
	}
	for k := range t.z {
		t.z[k] = t.ground[k] + float32(kind.Height)
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

func (l *Painter) xy(c cell.ID) (x, y uint32) {
	x, y, _ = l.board.Coords(c)
	return x, y
}

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
