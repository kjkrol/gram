package board

import (
	"image/color"
	"math"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/internal/parallel"
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
// board's Look, and the grid when it is on. Under a Parallel Dressing the tiles are dressed on
// several goroutines at once (Workers).
type Renderer struct {
	board   *Board
	mapping func() Map    // what the board is drawn by: its Look and its Dressing
	camera  camera.Camera // the one of the frame being drawn
	atlas   render.AtlasSource
	cellW   float64
	cellH   float64
	square  bool
	state   *RenderState
	outline []geom.Vec
	tile    Tile
	// cells are the frame's visible cells in the order they are drawn, and workers the goroutines
	// sharing them, at most count: 0 as many as there are CPUs, 1 none
	cells   []CellID
	workers []*tileWorker
	count   int
	// seen marks by ordinal the cells a frame has visited on a grid walked by sampling; stamp is the frame's mark.
	seen  []uint32
	stamp uint32
	// tops holds by ordinal what has been read of each cell, good while the cell stays as it was.
	tops []cellTop
	// scaleVaries is whether the frame's camera draws a world unit larger in some places than
	// others — a perspective — so each cell is measured where it lies
	scaleVaries bool
}

// cellTop is a cell as one Compose reads it once: its kind's sway and height, its sprite, and its
// corners and level as the Map draws them.
type cellTop struct {
	sway   float32
	height float32
	sprite render.SpriteID
	z      [4]float32 // the corners as drawn, what stands on the cell included
	alt    float32    // the ground's level
	ver    uint64     // one more than the cell's version when read; 0 not read yet
	seen   uint64     // the board's count of changes when last found as it was
}

// MinGridCell is how many pixels a cell must span on screen for the grid to be drawn over it.
const MinGridCell = 6

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

// tileWorker is one goroutine's share of a frame's tiles: the frame it draws them into, its tile
// and outline scratch, and the Dressing and Look it draws by.
type tileWorker struct {
	frame   render.Frame
	tile    Tile
	outline []geom.Vec
	dress   Dressing
	look    Look
}

// tilesPerWorker is the fewest tiles worth a goroutine of their own.
const tilesPerWorker = 64

func newRenderer(board *Board, atlas render.AtlasSource, state *RenderState, mapping func() Map) *Renderer {
	w, h := board.CellBounds()
	r := &Renderer{board: board, atlas: atlas, cellW: w, cellH: h, square: board.square != nil, state: state, mapping: mapping}
	r.tile.r, r.tile.Atlas = r, atlas
	return r
}

func (l *Renderer) Init(*goke.SysInit) {}

// Workers sets how many goroutines at most share a frame's tiles under a Parallel Dressing: 0 as
// many as there are CPUs, 1 none.
func (l *Renderer) Workers(n int) { l.count = max(n, 0) }

// Compose hands the Map's Look every cell under cam, and the grid when it is on: on a square
// grid the Look outlines each tile along its own edges, on any other the cells' outlines are
// drawn as lines. Under a Parallel Dressing and a ParallelLook, with tiles enough, every tile is
// warmed first, then the tiles are shared out among goroutines, each drawing its run into a frame
// of its own, appended to f in order: the picture is the one goroutine would draw.
func (l *Renderer) Compose(f *render.Frame, cam camera.Camera) {
	l.camera = cam
	d := l.dressing()
	look := l.mapping().Look()
	if d != nil {
		d.Begin(f, cam)
	}
	if look == Nothing {
		return // the ground is drawn some other way: the dressing is readied for the frame, no more
	}
	sheet := l.atlas
	if d != nil {
		sheet = d.Sheet(l.atlas)
	}
	l.tile.Atlas, l.tile.dress = sheet, d
	l.nextTops()
	w, h := cam.Viewport()
	x0, y0 := cam.Unproject(w/2, 0, 0)
	x1, y1 := cam.Unproject(w/2, h, 0)
	top := camera.ScaleAt(cam, x0, y0, 0)
	l.scaleVaries = top != camera.ScaleAt(cam, x1, y1, 0) || top != cam.Zoom()
	par, ok := d.(Parallel)
	pl, okLook := look.(ParallelLook)
	if !ok || !okLook || l.count == 1 {
		l.eachVisible(func(c CellID) { l.outline = l.cell(f, look, &l.tile, c, l.outline) })
		return
	}
	l.cells = l.cells[:0]
	l.eachVisible(func(c CellID) {
		l.topOf(c)
		l.place(&l.tile, c)
		par.Warm(&l.tile)
		l.cells = append(l.cells, c)
	})
	k := parallel.Workers(len(l.cells), tilesPerWorker, l.count)
	if k < 2 {
		for _, c := range l.cells {
			l.outline = l.cell(f, look, &l.tile, c, l.outline)
		}
		return
	}
	par.Ready()
	for len(l.workers) < k {
		tw := &tileWorker{}
		tw.tile.r = l
		l.workers = append(l.workers, tw)
	}
	for w, tw := range l.workers[:k] {
		tw.dress = par.Worker(w)
		tw.look = pl.Worker(w, tw.dress)
		tw.tile.Atlas, tw.tile.dress = sheet, tw.dress
		f.Branch(&tw.frame)
	}
	parallel.Run(k, len(l.cells), func(w, from, to int) {
		tw := l.workers[w]
		for _, c := range l.cells[from:to] {
			tw.outline = l.cell(&tw.frame, tw.look, &tw.tile, c, tw.outline)
		}
	})
	for _, tw := range l.workers[:k] {
		f.Append(&tw.frame)
	}
}

// place makes t cell c: its id and its box, whose centre it gives.
func (l *Renderer) place(t *Tile, c CellID) geom.Vec {
	center := l.board.CellCenter(c)
	t.ID = c
	t.X0, t.Y0 = float32(center.X-l.cellW/2), float32(center.Y-l.cellH/2)
	t.X1, t.Y1 = float32(center.X+l.cellW/2), float32(center.Y+l.cellH/2)
	return center
}

// cell hands look cell c as tile t, and draws its grid lines where the grid is on and the tile is
// not outlined by the look, with outline as scratch; it gives the scratch back.
func (l *Renderer) cell(f *render.Frame, look Look, t *Tile, c CellID, outline []geom.Vec) []geom.Vec {
	center := l.place(t, c)
	grid := l.gridShown(c, center)
	t.Outlined = grid && l.square
	look.Cell(f, l.camera, t)
	if grid && !l.square {
		return l.outlineLines(f, c, outline)
	}
	return outline
}

// gridShown reports whether the grid is on and cell c, its centre at center, is large enough on
// screen to read it: where it is drawn, as the camera draws a world unit there.
func (l *Renderer) gridShown(c CellID, center geom.Vec) bool {
	if !l.state.ShowGridLines {
		return false
	}
	scale := l.camera.Zoom()
	if l.scaleVaries {
		scale = camera.ScaleAt(l.camera, float32(center.X), float32(center.Y), l.topOf(c).alt)
	}
	return float32(min(l.cellW, l.cellH))*scale >= MinGridCell
}

// outlineLines draws c's outline on the ground at its level, leaving out an edge that straddles a
// wrap seam — the cells either side draw its images — each edge at the depth of its nearer end;
// outline is its scratch, given back.
func (l *Renderer) outlineLines(f *render.Frame, c CellID, outline []geom.Vec) []geom.Vec {
	alt := l.topOf(c).alt
	outline = l.board.CellOutline(c, outline[:0])
	reach := float32(l.cellW+l.cellH) * l.camera.Zoom()
	for i, p := range outline {
		q := outline[(i+1)%len(outline)]
		px, py, qx, qy := float32(p.X), float32(p.Y), float32(q.X), float32(q.Y)
		ax, ay := l.camera.Project(px, py, alt)
		bx, by := l.camera.Project(qx, qy, alt)
		if abs32(bx-ax) > reach || abs32(by-ay) > reach {
			continue
		}
		f.Line(gridTier, max(l.camera.Depth(px, py, alt), l.camera.Depth(qx, qy, alt)), ax, ay, bx, by, 1, colorGridLine)
	}
	return outline
}

// nextTops starts a Compose: the cells read before stay read as long as they do not change.
func (l *Renderer) nextTops() {
	if n := l.board.CellCount(); len(l.tops) != n {
		l.tops = make([]cellTop, n)
	}
}

// topOf is c as the board has it, read anew only when the cell has changed (Board.CellVersion).
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
	t.sway, t.height, t.sprite, t.ver = float32(kind.Sway), float32(kind.Height), kind.SpriteID, v
	t.z, t.alt = l.mapping().Top(c)
	return t
}

// onScreen reports whether any of cell c — from the ground, or sea level below it, up to what stands
// on it — is drawn within the viewport, with room for a top leaning in the wind: in a view with
// depth the world rectangle under the screen holds cells beside it too.
func (l *Renderer) onScreen(c CellID) bool {
	top := l.topOf(c)
	center := l.board.CellCenter(c)
	x0, y0 := float32(center.X-l.cellW/2), float32(center.Y-l.cellH/2)
	x1, y1 := float32(center.X+l.cellW/2), float32(center.Y+l.cellH/2)
	low, high := float32(0), top.z[0]
	for _, z := range top.z {
		low, high = min(low, z-top.height), max(high, z)
	}
	minX, minY := float32(math.Inf(1)), float32(math.Inf(1))
	maxX, maxY := float32(math.Inf(-1)), float32(math.Inf(-1))
	for _, z := range [2]float32{low, high} {
		for _, p := range [4][2]float32{{x0, y0}, {x1, y0}, {x0, y1}, {x1, y1}} {
			sx, sy := l.camera.Project(p[0], p[1], z)
			minX, maxX = min(minX, sx), max(maxX, sx)
			minY, maxY = min(minY, sy), max(maxY, sy)
		}
	}
	w, h := l.camera.Viewport()
	return maxX > -offScreen && minX < w+offScreen && maxY > -offScreen && minY < h+offScreen
}

// offScreen is how many pixels past the viewport's edge a cell still counts as drawn: what leans
// or stands over it may reach in.
const offScreen = 48

// eachVisible calls fn once for every cell under the camera's bounds: on a square grid straight
// from the rows and columns, on any other by sampling every half cell.
func (l *Renderer) eachVisible(fn func(c CellID)) {
	bounds := l.camera.Bounds()
	if l.board.square != nil {
		if l.camera.Projection().Sorts() {
			// a view with depth: the bounds are the rectangle round the screen's diamond, twice
			// the cells on it
			l.board.CellsUnder(bounds, func(c CellID) {
				if l.onScreen(c) {
					fn(c)
				}
			})
			return
		}
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
func (l *Renderer) dressing() Dressing { return l.mapping().Dressing() }

// NewRenderer is a renderer of brd's cells drawn from atlas by m — nil for the simple map: what
// WithRenderer builds, for a board no plugin runs.
func NewRenderer(brd *Board, atlas render.AtlasSource, m Map) *Renderer {
	if m == nil {
		m = brd.Map()
	}
	return newRenderer(brd, atlas, &RenderState{}, func() Map { return m })
}
