package landscape

import (
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

// dresser is the board's Dressing the landscape lays: the light on the tiles and what lies on
// them, worked out of the board, the Styles of its kinds and the world's sun and weather.
type dresser struct {
	board   *board.Board
	sun     func() world.Sun
	styles  map[board.Name]Style
	kinds   board.CellKindDict // for the kinds Styles name
	quasi3D bool
	square  bool
	sq      board.SquareShape

	lighted world.Sun     // the sun of the frame being drawn
	lamp    world.Lamp    // lighted, ready to light every corner
	camera  camera.Camera // the one of the frame being drawn
	cellW   float64
	cellH   float64
	cellPx  float32 // how many pixels a cell spans in the frame being drawn
	tile    tile    // the tile being dressed

	ways   []WayPiece   // the pieces of the last tile's Way
	blends []BlendPiece // the last tile's Blends
	// tops holds by ordinal what has been read of each cell, good while the cell stays as it was;
	// bakes what has been worked out of it and the cells round it, good while they all do.
	tops  []cellTop
	bakes []cellBake
	// sunlit holds by ordinal how much sun reaches each corner of a cell's top, good while its
	// stamp is sunStamp: as long as neither the terrain nor the sun changes.
	shadows  bool
	sunlit   []cellSunlit
	sunStamp uint32
	sunFor   sunKey
	highest  float32 // the highest top a shadow may come from, measured when the shadows go stale
	stale    bool    // the shadows went stale this frame: highest is still to be measured
	// shores holds by corner of a square grid, row by row, the way to the shore from it, good
	// while its stamp is shoreStamp: as long as the terrain does not change.
	shores     []cornerShore
	shoreStamp uint32
	shoreFor   uint64
	// baking has the tiles dressed from sheet this frame: from far, on a square grid; newest,
	// unpainted, canvas and the scratch tile are what painting it works with
	baking    bool
	sheet     *groundSheet
	newest    []uint64
	unpainted []int
	canvas    render.Frame
	scratch   board.Tile
	bakeTile  tile
}

var _ board.Dressing = (*dresser)(nil)

func newDresser(b *board.Board, sun func() world.Sun, quasi3D bool, styles map[board.Name]Style) *dresser {
	w, h := b.CellBounds()
	sq, square := b.Square()
	return &dresser{board: b, sun: sun, styles: styles, quasi3D: quasi3D, square: square, sq: sq, cellW: w, cellH: h, shadows: true}
}

// tile is a board.Tile as the landscape dresses it, and what it has worked out of it already.
type tile struct {
	*board.Tile
	r    *dresser
	id   board.CellID
	memo tileMemo
}

// cellTop is a cell as the landscape reads it: its corners with its kind standing on them, the
// ground's corners under it, its level, its sprite, its kind's Style and the way across it.
type cellTop struct {
	z      [4]float32
	ground [4]float32
	alt    float32
	shine  float32
	flow   float32
	spread float32
	raised bool // something stands a Height over the ground
	under  bool
	sprite render.SpriteID
	way    lane   // what runs across the cell
	cross  lane   // and what crosses over that
	ver    uint64 // one more than the cell's version when read; 0 not read yet
	seen   uint64 // the board's count of changes when last found as it was
}

// Begin readies the dresser for a frame through cam: the sun, how near the eye is, and what has
// gone stale since the last.
func (l *dresser) Begin(f *render.Frame, cam camera.Camera) {
	l.camera, l.tile = cam, tile{}
	l.lighted = l.sun()
	l.lamp = l.lighted.Lamp()
	if n := l.board.CellCount(); len(l.tops) != n {
		l.tops, l.bakes = make([]cellTop, n), make([]cellBake, n)
	}
	l.nextSunlit()
	l.cellPx = float32(min(l.cellW, l.cellH)) * cam.Zoom()
	l.baking = l.square && l.cellPx < bakeCell
	l.nextShores()
}

// Base is the sprite t's top is drawn in first: its own kind's, or the kind Under it round it.
func (l *dresser) Base(t *board.Tile) render.SpriteID { return l.tileOf(t).baseTop().sprite }

// Light is the light on t's top at its corners.
func (l *dresser) Light(t *board.Tile) render.Shade { return l.tileOf(t).Light() }

// FaceLight is the light on t's upright face looking dx, dy cells away.
func (l *dresser) FaceLight(t *board.Tile, dx, dy int) render.Light {
	return l.tileOf(t).FaceLight(dx, dy)
}

// Covers reports whether Dress lays over t's top the grounds round it or a way — from far a piece
// of the ground sheet — which would hide its outline.
func (l *dresser) Covers(t *board.Tile) bool {
	d := l.tileOf(t)
	if l.baking {
		i, ok := l.ordinal(t.ID)
		return ok && l.sheet != nil && l.sheet.dressed[i]
	}
	b := l.bakeOf(d)
	return len(b.blends) > 0 || len(b.ways) > 0 || len(b.crossings) > 0
}

// Dress lays over t's top, drawn over the box x0..x1, y0..y1 at depth, what lies on it: its water,
// the grounds round it running in — from far with its way as one piece of the ground sheet — the
// clouds' shadows and, where it Covers an Outlined top, its outline over them, and then the way
// across it: laid in the order the frame draws them, so it needs no sorting.
func (l *dresser) Dress(f *render.Frame, cam camera.Camera, t *board.Tile, x0, y0, x1, y1, depth float32) {
	d, top := l.tileOf(t), f.Last()
	d.DrawSurface(f, x0, y0, x1, y1)
	if l.baking {
		z := l.topOf(t.ID).z
		var corners render.Corners
		for k, p := range [4][2]float32{{x0, y0}, {x1, y0}, {x0, y1}, {x1, y1}} {
			corners[k][0], corners[k][1] = cam.Project(p[0], p[1], z[k])
		}
		l.dressBaked(f, d, corners, depth)
	} else {
		d.DrawBlends(f, cam, depth)
	}
	OvercastOn(f, top, render.Box(x0, y0, x1, y1))
	if t.Outlined && l.Covers(t) {
		f.OutlineOn(top)
	}
	if !l.baking {
		d.DrawWay(f, cam, depth)
	}
}

// tileOf is t as the dresser dresses it, what it worked out of it kept while it is the same cell.
func (l *dresser) tileOf(t *board.Tile) *tile {
	if l.tile.Tile != t || l.tile.id != t.ID {
		l.tile = tile{Tile: t, r: l, id: t.ID}
	}
	return &l.tile
}

// topOf is c as the board has it, read anew only when the cell has changed (Board.CellVersion): the
// ground's corners on a square grid, its level everywhere on another, raised by its kind's Height.
func (l *dresser) topOf(c board.CellID) *cellTop {
	i, _ := l.ordinal(c)
	t := &l.tops[i]
	changes := l.board.Changes()
	if t.ver != 0 && t.seen == changes { // nothing on the board has changed since
		return t
	}
	v := l.board.CellVersion(c) + 1
	t.seen = changes
	if t.ver == v {
		return t
	}
	kind := l.board.Kind(c)
	style := l.styles[kind.Name]
	r := l.board.Relief(c)
	t.alt, t.sprite, t.ver = float32(r.Level()), kind.SpriteID, v
	t.shine, t.flow, t.spread, t.under, t.raised = style.Shine, style.Flow, style.Spread, style.Under, kind.Height > 0
	t.way, t.cross = l.laneOf(l.board.Way(c)), l.laneOf(l.board.Crossing(c).Way)
	if l.square {
		t.ground = r.Corners
	} else {
		t.ground = [4]float32{t.alt, t.alt, t.alt, t.alt}
	}
	for k := range t.z {
		t.z[k] = t.ground[k] + float32(kind.Height)
	}
	return t
}

// lane is what runs across a cell as the landscape draws it: a Way, and its kind's Style — how its
// water shines and runs — and the kind it turns into, where it mixes, and how that one shines.
type lane struct {
	board.Way
	shine, flow float32
	mix         render.SpriteID
	mixes       bool
	mixShine    float32
}

// laneOf is w with its kind's Style.
func (l *dresser) laneOf(w board.Way) lane {
	s := l.styles[w.Kind.Name]
	ln := lane{Way: w, shine: s.Shine, flow: s.Flow}
	if s.MixWith != "" && l.kinds != nil {
		if k, ok := l.kinds.Get(s.MixWith); ok {
			ln.mix, ln.mixes, ln.mixShine = k.SpriteID, true, l.styles[k.Name].Shine
		}
	}
	return ln
}

// ordinal is c's slot in a table of one per cell.
func (l *dresser) ordinal(c board.CellID) (int, bool) { return l.board.Ordinal(c) }

// xy is c's column and row on a square grid.
func (l *dresser) xy(c board.CellID) (x, y uint32) {
	x, y, _ = l.board.Coords(c)
	return x, y
}

// cellAt is the square grid's cell at column x, row y, folded where the grid wraps; false off it.
func (l *dresser) cellAt(x, y int64) (board.CellID, bool) {
	fx, okX := fold(x, int64(l.sq.Cols), l.sq.WrapX)
	fy, okY := fold(y, int64(l.sq.Rows), l.sq.WrapY)
	if !okX || !okY {
		return 0, false
	}
	return l.board.CellIndex(uint32(fx), uint32(fy))
}

// fold is v on an axis n long: wrapped round where it wraps; false off it where it does not.
func fold(v, n int64, wrap bool) (int64, bool) {
	if wrap {
		return (v%n + n) % n, true
	}
	return v, v >= 0 && v < n
}

// squareDirs are a square grid's directions in board.Links's order: north, south, west, east,
// north-west, north-east, south-west, south-east.
var squareDirs = [8][2]float32{{0, -1}, {0, 1}, {-1, 0}, {1, 0}, {-1, -1}, {1, -1}, {-1, 1}, {1, 1}}
