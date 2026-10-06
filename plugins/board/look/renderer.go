package look

import (
	"image/color"
	"math"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/internal/parallel"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/ground"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

var colorGridLine = color.RGBA{R: 20, G: 20, B: 20, A: 120}

// RenderState is a Renderer's live display toggles.
type RenderState struct {
	ShowGridLines bool
}

// ToggleShowGridLines flips whether grid lines are drawn.
func (r *RenderState) ToggleShowGridLines() { r.ShowGridLines = !r.ShowGridLines }

// Map is what a board is drawn by: how its cells lie on the screen (Look), what lies over them
// beyond their sprites (Dressing), how high they stand (Top) and whether the ground has heights at
// all (Heights). A board's Map is one, pricing a step besides (board.Map).
type Map interface {
	// Look is how the cells lie on the screen through a camera.
	Look() Look
	// Heights is the height of the ground at any point, for sight and navigation; nil on a flat map.
	Heights() ground.Heights
	// Dressing lays over the tiles what lies on them beyond their sprites; nil for nothing.
	Dressing() Dressing
	// Top is the height of c's corners as drawn — top-left, top-right, bottom-left, bottom-right —
	// and of its ground: zero on a flat map.
	Top(c cell.ID) (corners [4]float32, level float32)
}

// Board is what a Renderer reads of a board: its grid and shape, and its cells' kinds and changes.
// A board.Board is one.
type Board interface {
	grid.Grid
	// Shape is the grid's shape when it is one of grid.DefaultGrids.
	Shape() (grid.Shape, bool)
	// Kind is a cell's kind as whoever crosses it meets it, Bare its ground's.
	Kind(c cell.ID) cell.Kind
	Bare(c cell.ID) cell.Kind
	// CellVersion counts the changes to a cell, Changes those to them all.
	CellVersion(c cell.ID) uint64
	Changes() uint64
}

// Renderer is the render.Source of a Board's cells: each visible cell laid on the screen by the
// Map's Look, and the grid when it is on. Under a Parallel Dressing the tiles are dressed on
// several goroutines at once (Workers).
type Renderer struct {
	board   Board
	mapping Map           // what the board is drawn by: its Look and its Dressing
	camera  camera.Camera // the one of the frame being drawn
	atlas   render.AtlasSource
	cellW   float64
	cellH   float64
	square  bool
	state   RenderState
	shaded  map[cell.Name]render.MaterialID // the kinds worked out per pixel instead of drawn
	outline []geom.Vec
	quads   []camera.Quad // reused for the shaded kinds' quads
	tile    Tile
	// cells are the frame's visible cells in the order they are drawn, and workers the goroutines
	// sharing them, at most count: 0 as many as there are CPUs, 1 none
	cells   []cell.ID
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

	// space is the world the board lies in, for the still: how large, whether it wraps; zero for
	// none, its tiles composed every frame
	space world.SpaceCfg
	// still is the tiles of a flat map under an EvenLit dressing composed once in white (white
	// while composing), again when the board changes (stillAt); drawn this frame (stillOn) in
	// stillLight
	still      *render.Still
	stillAt    uint64
	stillOn    bool
	stillLight render.Light
	white      bool
	grid       gridLines // the still's
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

var _ render.Direct = (*Renderer)(nil)

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

// NewRenderer is a renderer of b's cells drawn from atlas by m, over the world space — zero for none,
// its tiles composed every frame: what board.Plugin.WithRenderer builds.
func NewRenderer(b Board, atlas render.AtlasSource, m Map, space world.SpaceCfg) *Renderer {
	w, h := b.CellBounds()
	shape, ok := b.Shape()
	r := &Renderer{board: b, atlas: atlas, cellW: w, cellH: h, square: ok && !shape.Hex, mapping: m, space: space}
	r.tile.r, r.tile.Atlas = r, atlas
	return r
}

// State is the renderer's live toggles: whether the grid is drawn.
func (l *Renderer) State() *RenderState { return &l.state }

func (l *Renderer) Init(*goke.SysInit) {}

// Shade is the per-pixel looks the board's Atlas declared, by the kind's name; their tiles are
// laid as live material quads every frame, never baked into the still, and skipped by the tiles'
// pass. A shaded kind takes no part in blending (give it no Spread).
func (l *Renderer) Shade(shaded map[cell.Name]render.MaterialID) { l.shaded = shaded }

// Workers sets how many goroutines at most share a frame's tiles under a Parallel Dressing: 0 as
// many as there are CPUs, 1 none.
func (l *Renderer) Workers(n int) { l.count = max(n, 0) }

// Compose hands the Map's Look every cell under cam, and the grid when it is on: on a square
// grid the Look outlines each tile along its own edges, on any other the cells' outlines are
// drawn as lines. Under a Parallel Dressing and a ParallelLook, with tiles enough, every tile is
// warmed first, then the tiles are shared out among goroutines, each drawing its run into a frame
// of its own, appended to f in order: the picture is the one goroutine would draw. A flat map under
// an EvenLit dressing seen from above is composed once instead (render.Still), again when the board
// changes, and drawn by Draw, the grid over it on the GPU.
func (l *Renderer) Compose(f *render.Frame, cam camera.Camera) {
	l.camera = cam
	d := l.dressing()
	m := l.mapping
	look := m.Look()
	if d != nil {
		d.Begin(f, cam)
	}
	if look == Nothing {
		return // the ground is drawn some other way: the dressing is readied for the frame, no more
	}
	if l.stillLight, l.stillOn = l.evenLight(m, d, cam); l.stillOn {
		l.composeStill(look, d)
		l.camera = cam
		l.materials(f)
		return
	}
	l.compose(f, look, d)
	l.materials(f)
}

// materialTier is where the shaded kinds' quads come: over the still and the tiles, under the
// grid's lines and whatever stands on the ground.
const materialTier = render.Ground + 5

// materials lays a live quad for every visible cell of a shaded kind — the material's inputs the
// cell's own: World its box, Custom its middle and half its width, Red 1.
func (l *Renderer) materials(f *render.Frame) {
	if len(l.shaded) == 0 {
		return
	}
	cam := l.camera
	l.eachVisible(func(c cell.ID) {
		m, ok := l.shaded[l.board.Kind(c).Name]
		if !ok {
			return
		}
		center := l.board.CellCenter(c)
		x0, y0 := float32(center.X-l.cellW/2), float32(center.Y-l.cellH/2)
		x1, y1 := float32(center.X+l.cellW/2), float32(center.Y+l.cellH/2)
		custom := [4]float32{(x0 + x1) / 2, (y0 + y1) / 2, (x1 - x0) / 2, 0}
		o := render.Overlay{
			Material: m,
			World:    render.Box(x0, y0, x1, y1),
			Red:      [4]float32{1, 1, 1, 1},
			Custom:   [4][4]float32{custom, custom, custom, custom},
		}
		l.quads = cam.ToScreenQuads(x0, y0, x1, y1, l.quads[:0])
		for _, q := range l.quads {
			f.Material(materialTier, 0, render.Corners{{q.X0, q.Y0}, {q.X1, q.Y0}, {q.X0, q.Y1}, {q.X1, q.Y1}}, &o)
		}
	})
}

// Tier is where the still comes: render.Ground, under all else.
func (l *Renderer) Tier() render.Tier { return render.Ground }

// Draw draws the still this frame composed through cam, as many times as a wrapping world shows
// it, and the grid over it when it is on; nothing where the tiles were composed into the frame.
func (l *Renderer) Draw(t render.Target, cam camera.Camera, u render.Uniforms) {
	if !l.stillOn || t.Screen == nil {
		return
	}
	zoom := cam.Zoom()
	x, y := cam.FromScreen(0, 0)
	w, h := cam.Viewport()
	ww, wh := float32(l.space.Width), float32(l.space.Height)
	xs, ys := []float32{x}, []float32{y}
	if l.space.Edges.WrapsX() && x+w/zoom > ww {
		xs = append(xs, x-ww)
	}
	if l.space.Edges.WrapsY() && y+h/zoom > wh {
		ys = append(ys, y-wh)
	}
	for _, sy := range ys {
		for _, sx := range xs {
			l.still.Draw(t.Screen, u, zoom, sx, sy, l.stillLight)
		}
	}
	if l.state.ShowGridLines && float32(min(l.cellW, l.cellH))*zoom >= MinGridCell {
		l.grid.draw(t, cam, u, l.board, l.space.Edges.WrapsX(), l.space.Edges.WrapsY())
	}
}

// evenLight is the light the still is drawn in this frame, and whether it is: a board in a world
// a plugin runs, a flat map, a camera looking straight down, the dressing none or lighting every
// tile alike.
func (l *Renderer) evenLight(m Map, d Dressing, cam camera.Camera) (render.Light, bool) {
	if l.space.Width == 0 || l.space.Height == 0 || m.Heights() != nil {
		return render.Light{}, false
	}
	if _, top := cam.Projection().(camera.TopDown); !top {
		return render.Light{}, false
	}
	if d == nil {
		return render.Light{1, 1, 1}, true
	}
	if e, ok := d.(EvenLit); ok {
		return e.EvenLight()
	}
	return render.Light{}, false
}

// composeStill composes the still anew where the board has changed since it was last.
func (l *Renderer) composeStill(look Look, d Dressing) {
	if l.still == nil {
		l.still = render.NewStill()
	}
	if l.stillAt == l.board.Changes()+1 {
		return
	}
	l.white = true
	l.still.Compose(float32(l.space.Width), float32(l.space.Height), func(f *render.Frame, cam camera.Camera) {
		l.camera = cam
		l.compose(f, look, d)
	})
	l.white = false
	l.stillAt = l.board.Changes() + 1
}

// compose hands look every cell under the renderer's camera, dressed by d.
func (l *Renderer) compose(f *render.Frame, look Look, d Dressing) {
	cam := l.camera
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
		l.eachVisible(func(c cell.ID) { l.outline = l.cell(f, look, &l.tile, c, l.outline) })
		return
	}
	l.cells = l.cells[:0]
	l.eachVisible(func(c cell.ID) {
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
func (l *Renderer) place(t *Tile, c cell.ID) geom.Vec {
	center := l.board.CellCenter(c)
	t.ID = c
	t.X0, t.Y0 = float32(center.X-l.cellW/2), float32(center.Y-l.cellH/2)
	t.X1, t.Y1 = float32(center.X+l.cellW/2), float32(center.Y+l.cellH/2)
	return center
}

// cell hands look cell c as tile t, and draws its grid lines where the grid is on and the tile is
// not outlined by the look, with outline as scratch; it gives the scratch back.
func (l *Renderer) cell(f *render.Frame, look Look, t *Tile, c cell.ID, outline []geom.Vec) []geom.Vec {
	if _, ok := l.shaded[l.board.Kind(c).Name]; ok {
		return outline // worked out per pixel instead (materials), never drawn as a tile
	}
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
func (l *Renderer) gridShown(c cell.ID, center geom.Vec) bool {
	if !l.state.ShowGridLines || l.white { // a still's grid is drawn over it on the GPU
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
func (l *Renderer) outlineLines(f *render.Frame, c cell.ID, outline []geom.Vec) []geom.Vec {
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
func (l *Renderer) topOf(c cell.ID) *cellTop {
	i, _ := l.board.Ordinal(c)
	t := &l.tops[i]
	if t.ver != 0 && t.seen == l.board.Changes() { // nothing on the board has changed since
		return t
	}
	v := l.board.CellVersion(c) + 1
	t.seen = l.board.Changes()
	if t.ver == v {
		return t
	}
	kind := l.board.Bare(c)
	t.sway, t.height, t.sprite, t.ver = float32(kind.Sway), float32(kind.Height), kind.SpriteID, v
	t.z, t.alt = l.mapping.Top(c)
	return t
}

// onScreen reports whether any of cell c — from the ground, or sea level below it, up to what stands
// on it — is drawn within the viewport, with room for a top leaning in the wind: in a view with
// depth the world rectangle under the screen holds cells beside it too.
func (l *Renderer) onScreen(c cell.ID) bool {
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
func (l *Renderer) eachVisible(fn func(c cell.ID)) {
	bounds := l.camera.Bounds()
	if l.square {
		if l.camera.Projection().Sorts() {
			// a view with depth: the bounds are the rectangle round the screen's diamond, twice
			// the cells on it
			l.board.CellsUnder(bounds, func(c cell.ID) {
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
			i, ok := l.board.Ordinal(c)
			if !ok || l.seen[i] == l.stamp {
				continue
			}
			l.seen[i] = l.stamp
			fn(c)
		}
	}
}

// dressing is the Dressing the board's tiles are dressed by, nil for none.
func (l *Renderer) dressing() Dressing { return l.mapping.Dressing() }

func abs32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}
