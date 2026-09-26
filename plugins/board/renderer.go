package board

import (
	"image/color"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

var colorGridLine = color.RGBA{R: 20, G: 20, B: 20, A: 120}

// RenderState is the board renderer's live display toggles.
type RenderState struct {
	ShowGridLines bool
}

// ToggleShowGridLines flips whether grid lines are drawn.
func (r *RenderState) ToggleShowGridLines() { r.ShowGridLines = !r.ShowGridLines }

// Renderer is the render.Source of Board's cells: each visible cell laid on the screen by the
// board's Look, and the grid when it is on.
type Renderer struct {
	board   *Board
	look    func() Look
	sun     func() world.Sun
	weather func() world.Weather // the air over the world; nil, a calm clear day
	dressed func() Dressing      // what lays the tiles' light and dresses them; nil for none
	camera  camera.Camera        // the one of the frame being drawn
	atlas   render.AtlasSource
	cellW   float64
	cellH   float64
	state   *RenderState
	outline []geom.Vec
	tile    Tile
	// seen marks by ordinal the cells a frame has visited on a grid walked by sampling; stamp is the frame's mark.
	seen  []uint32
	stamp uint32
	// tops holds by ordinal what has been read of each cell, good while the cell stays as it was.
	tops []cellTop
}

// cellTop is a cell as one Compose reads it once: its corners with its kind standing on them, the
// ground's corners under it, its ground level and its sprite.
type cellTop struct {
	z      [4]float32
	ground [4]float32
	alt    float32
	sway   float32
	sprite render.SpriteID
	ver    uint64 // one more than the cell's version when read; 0 not read yet
	seen   uint64 // the board's count of changes when last found as it was
}

// minGridCell is how many pixels a cell must span on screen for the grid to be drawn over it.
const minGridCell = 6

func abs32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

var _ render.Source = (*Renderer)(nil)

// gridTier puts the lines of a grid other than square over every tile and under whatever stands on
// them.
const gridTier = render.Ground + 10

func newRenderer(board *Board, atlas render.AtlasSource, state *RenderState, look func() Look, sun func() world.Sun) *Renderer {
	w, h := board.CellBounds()
	r := &Renderer{board: board, atlas: atlas, cellW: w, cellH: h, state: state, look: look, sun: sun}
	r.tile.r, r.tile.Atlas = r, atlas
	return r
}

func (l *Renderer) Init(*goke.SysInit) {}

// Compose hands the board's Look every cell under cam, and the grid when it is on: on a square
// grid the Look outlines each tile along its own edges, on any other the cells' outlines are
// drawn as lines.
func (l *Renderer) Compose(f *render.Frame, cam camera.Camera) {
	l.camera = cam
	f.Daylight(l.sun().Daylight())
	if l.weather != nil {
		f.Weather(l.weather().Frame())
	}
	l.tile.Atlas = l.atlas
	if d := l.dressing(); d != nil {
		d.Begin(f, cam)
		l.tile.Atlas = d.Sheet(l.atlas)
	}
	look := l.look()
	grid := l.gridShown()
	square := l.board.square != nil
	l.tile.Outlined = grid && square
	l.nextTops()
	l.eachVisible(func(c CellID) {
		center := l.board.CellCenter(c)
		t := &l.tile
		t.ID = c
		t.X0, t.Y0 = float32(center.X-l.cellW/2), float32(center.Y-l.cellH/2)
		t.X1, t.Y1 = float32(center.X+l.cellW/2), float32(center.Y+l.cellH/2)
		look.Cell(f, cam, t)
		if grid && !square {
			l.outlineLines(f, c)
		}
	})
}

// gridShown reports whether the grid is on and its cells are large enough on screen to read.
func (l *Renderer) gridShown() bool {
	return l.state.ShowGridLines && float32(min(l.cellW, l.cellH))*l.camera.Zoom() >= minGridCell
}

// outlineLines draws c's outline on the ground at its level, leaving out an edge that straddles a
// wrap seam — the cells either side draw its images — each edge at the depth of its nearer end.
func (l *Renderer) outlineLines(f *render.Frame, c CellID) {
	alt := float32(l.board.Altitude(c))
	l.outline = l.board.CellOutline(c, l.outline[:0])
	reach := float32(l.cellW+l.cellH) * l.camera.Zoom()
	for i, p := range l.outline {
		q := l.outline[(i+1)%len(l.outline)]
		px, py, qx, qy := float32(p.X), float32(p.Y), float32(q.X), float32(q.Y)
		ax, ay := l.camera.Project(px, py, alt)
		bx, by := l.camera.Project(qx, qy, alt)
		if abs32(bx-ax) > reach || abs32(by-ay) > reach {
			continue
		}
		f.Line(gridTier, max(l.camera.Depth(px, py, alt), l.camera.Depth(qx, qy, alt)), ax, ay, bx, by, 1, colorGridLine)
	}
}

// nextTops starts a Compose: the cells read before stay read as long as they do not change.
func (l *Renderer) nextTops() {
	if n := l.board.CellCount(); len(l.tops) != n {
		l.tops = make([]cellTop, n)
	}
}

// topOf is c as the board has it, read anew only when the cell has changed (Board.CellVersion): the
// ground's corners on a sloped grid, its level everywhere on a flat one, raised by the kind's Height.
func (l *Renderer) topOf(c CellID) *cellTop {
	i, _ := l.board.ordinal(c)
	t := &l.tops[i]
	if t.ver != 0 && t.seen == l.board.changes { // nothing on the board has changed since
		return t
	}
	v := l.board.CellVersion(c) + 1
	t.seen = l.board.changes
	if t.ver == v {
		return t
	}
	kind := l.board.kindOf(c)
	r := l.board.Relief(c)
	rise := float32(kind.Height)
	t.alt, t.sway, t.sprite, t.ver = float32(r.Level()), float32(kind.Sway), kind.SpriteID, v
	if l.board.sloped() {
		t.ground = r.Corners
	} else {
		t.ground = [4]float32{t.alt, t.alt, t.alt, t.alt}
	}
	for k := range t.z {
		t.z[k] = t.ground[k] + rise
	}
	return t
}

// eachVisible calls fn once for every cell under the camera's bounds: on a square grid straight
// from the rows and columns, on any other by sampling every half cell.
func (l *Renderer) eachVisible(fn func(c CellID)) {
	bounds := l.camera.Bounds()
	if l.board.square != nil {
		l.board.CellsUnder(bounds, fn)
		return
	}
	step := min(l.cellW, l.cellH) / 2
	if step <= 0 {
		step = 1
	}
	if n := l.board.CellCount(); len(l.seen) != n {
		l.seen, l.stamp = make([]uint32, n), 0
	}
	if l.stamp++; l.stamp == 0 { // wrapped round: old marks would pass for new
		clear(l.seen)
		l.stamp = 1
	}
	for y := bounds.TopLeft.Y; y < bounds.BottomRight.Y+step; y += step {
		for x := bounds.TopLeft.X; x < bounds.BottomRight.X+step; x += step {
			c, ok := l.board.CellAt(geom.NewVec(x, y))
			if !ok {
				continue
			}
			i, ok := l.board.ordinal(c)
			if !ok || l.seen[i] == l.stamp {
				continue
			}
			l.seen[i] = l.stamp
			fn(c)
		}
	}
}

// dressing is the Dressing the board's tiles are dressed by, nil for none.
func (l *Renderer) dressing() Dressing {
	if l.dressed == nil {
		return nil
	}
	return l.dressed()
}

// NewRenderer is a renderer of brd's cells drawn from atlas as look lays them, in sun's light for
// the frame, dressed by dressing — nil for none: what WithRenderer builds, for a board no plugin
// runs.
func NewRenderer(brd *Board, atlas render.AtlasSource, look Look, sun func() world.Sun, dressing Dressing) *Renderer {
	r := newRenderer(brd, atlas, &RenderState{}, func() Look { return look }, sun)
	r.dressed = func() Dressing { return dressing }
	return r
}
