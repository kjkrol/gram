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
	"github.com/kjkrol/gram/plugins/world/view"
	"github.com/kjkrol/gram/render"
)

// RenderState is a Renderer's live display toggles and the line its grid is drawn in.
type RenderState struct {
	ShowGridLines bool
	// GridLine is the grid's colour, GridWidth its width in screen pixels; 0 draws no grid.
	GridLine  color.RGBA
	GridWidth float32
}

// Grid is the grid's line as the shaders take it, premultiplied, and its width in pixels; false
// while the grid is hidden or drawn in no line.
func (s *RenderState) Grid() (line [4]float32, width float32, shown bool) {
	if s == nil || !s.ShowGridLines || s.GridWidth <= 0 || s.GridLine.A == 0 {
		return line, 0, false
	}
	return premultiplied(s.GridLine), s.GridWidth, true
}

// Map is what a board is drawn by: how its cells lie on the screen, what lies over them beyond
// their sprites and how high they stand.
type Map interface {
	// Look is how the cells lie on the screen through a camera.
	Look() Look
	// Heights is the height of the ground at any point; nil on a flat map.
	Heights() ground.Heights
	// Dressing lays over the tiles what lies on them beyond their sprites; nil for nothing.
	Dressing() Dressing
	// Top is the height of c's corners as drawn (top-left, top-right, bottom-left, bottom-right)
	// and of its ground; zero on a flat map.
	Top(c cell.ID) (corners [4]float32, level float32)
}

// Board is what a Renderer reads of a board; a board.Board is one.
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
// Map's Look, and the grid when it is on.
type Renderer struct {
	board   Board
	mapping Map
	camera  camera.Camera
	// what a camera sees of the world's sectors, and what the frame's sees: the cells drawn are
	// those of the sectors seen alone, at their levels
	sees  func(camera.Camera) *view.View
	sight *view.View
	// what lights the tiles, nil for white light
	lighting func() world.Lighting
	atlas    render.AtlasSource
	cellW    float64
	cellH    float64
	square   bool
	state    RenderState
	shaded   map[cell.Name]render.MaterialID
	scratch  scratch
	quads    []camera.Quad
	tile     Tile
	cells    []cell.ID
	workers  []*tileWorker
	parts    []tilePart
	shares   int // the parts of this frame
	count    int // 0 all of the pool, 1 none
	pool     *parallel.Pool
	drawPart func(w, i int) // drawParted, bound once
	seen     []uint32
	stamp    uint32
	tops     []cellTop
	// the sides of every cell's outline on one of grid.DefaultGrids, nil on another grid
	rims []rim
	// gridded[i] is gridStamp where the cell of ordinal i draws its grid this frame
	gridded   []uint32
	gridStamp uint32
	// scaleVaries: a perspective draws a world unit larger in some places than others
	scaleVaries bool

	over       render.Frame   // what runs across the board, laid after every tile
	space      world.SpaceCfg // zero for none: the tiles composed every frame
	still      *render.Still
	stillAt    uint64
	stillOn    bool
	stillLight render.Light
	white      bool
	gridOn     bool // this frame's tiles draw the grid
	grid       gridLines
	shade      groundShade
	ground     GroundDressing // this frame's, nil for none
}

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

// how far over the ground what the board lays on it lies among what lies there: its materials,
// then its grid, under the sight and the routes
const (
	materialLift = 0.05
	gridLift     = 0.1
	// what a GroundDressing lays over tiles composed into the frame: over them, under the rest
	groundLift = 0.02
)

type tileWorker struct {
	tile    Tile
	scratch scratch
	dress   Dressing
	look    Look
}

// what a part of a frame's tiles draws, appended in the parts' order whoever drew it
type tilePart struct {
	frame render.Frame
	over  render.Frame
}

// what a goroutine drawing tiles keeps from one to the next
type scratch struct {
	outline []geom.Vec
	rims    []rim
}

// a side of a cell's outline: the direction it faces, its ends among the outline's corners, and
// whether the cell draws it even where the cell across draws its grid too
type rim struct {
	dir, a, b int
	owns      bool
}

// tilesPerWorker is the fewest tiles worth a goroutine of their own.
const tilesPerWorker = 64

// NewRenderer is a renderer of b's cells drawn from atlas by m, over the world space; a zero
// space composes the tiles every frame.
func NewRenderer(b Board, atlas render.AtlasSource, m Map, space world.SpaceCfg) *Renderer {
	w, h := b.CellBounds()
	shape, ok := b.Shape()
	r := &Renderer{
		board:   b,
		atlas:   atlas,
		cellW:   w,
		cellH:   h,
		square:  ok && !shape.Hex,
		mapping: m,
		space:   space,
	}
	r.tile.r, r.tile.Atlas = r, atlas
	r.drawPart = r.drawParted
	if ok && b.CellCount() > 0 {
		r.rims = rimsOf(b, b.At(0, 0), nil)
	}
	return r
}

// the sides of c's outline, each owned by one of the two cells it lies between
func rimsOf(g grid.Grid, c cell.ID, out []rim) []rim {
	out = out[:0]
	for i := range g.Directions() {
		if g.Corner(i) {
			continue
		}
		a, b := grid.Rim(g, c, i)
		out = append(out, rim{dir: i, a: a, b: b, owns: i < g.Opposite(i)})
	}
	return out
}

// SetAtlas has the cells drawn from atlas from the next frame on.
func (l *Renderer) SetAtlas(atlas render.AtlasSource) {
	l.atlas, l.tile.Atlas = atlas, atlas
	l.stillAt = 0
	l.tops = l.tops[:0]
}

// State is the renderer's live toggles.
func (l *Renderer) State() *RenderState { return &l.state }

func (l *Renderer) Init(*goke.SysInit) {}

// Shade is the per-pixel looks the board's Atlas declared, by the kind's name. A shaded kind
// takes no part in blending.
func (l *Renderer) Shade(shaded map[cell.Name]render.MaterialID) { l.shaded = shaded }

// WithSeen has the cells drawn of the sectors a camera sees alone, as seen says, coarsely where it
// sees one coarsely, fading into the fog before its sight's end.
func (l *Renderer) WithSeen(seen func(camera.Camera) *view.View) *Renderer {
	l.sees = seen
	return l
}

// Workers sets how many goroutines of the pool at most share a frame's tiles under a Parallel
// Dressing: 0 all, 1 none.
func (l *Renderer) Workers(n int) { l.count = max(n, 0) }

// UsePool sets the goroutines a frame's tiles are shared among; nil, one.
func (l *Renderer) UsePool(p *parallel.Pool) { l.pool = p }

// Compose hands the Map's Look every cell under cam, and the grid when it is on; nothing before
// the renderer has an atlas.
func (l *Renderer) Compose(f *render.Frame, cam camera.Camera) {
	if l.atlas == nil {
		return
	}
	l.camera = cam
	l.sight = nil
	if l.sees != nil {
		if v := l.sees(cam); v.Grid() != nil {
			l.sight = v
		}
	}
	d := l.dressing()
	m := l.mapping
	look := m.Look()
	if d != nil {
		d.Begin(f, cam)
	}
	l.ground, _ = d.(GroundDressing)
	if look == Nothing {
		return // the dressing is readied for the frame, no more
	}
	if l.stillLight, l.stillOn = l.evenLight(m, d, cam); l.stillOn {
		l.composeStill(look, d)
		l.camera = cam
		f.DrawOpaque()
		if l.gridDrawn(cam) {
			f.DrawAt(f.OnGround(gridLift))
		}
		l.materials(f)
		return
	}
	l.compose(f, look, d)
	if l.ground != nil && m.Heights() == nil && !cam.Projection().Sorts() {
		f.DrawAt(f.OnGround(groundLift))
	}
	l.materials(f)
}

func (l *Renderer) gridDrawn(cam camera.Camera) bool {
	_, _, shown := l.state.Grid()
	return shown && float32(min(l.cellW, l.cellH))*cam.Zoom() >= MinGridCell
}

// the material's inputs: World the cell's box, Custom its middle and half its width, Red 1
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
		at := f.Lifted(float32(center.X), float32(center.Y), l.topOf(c).alt, materialLift)
		l.quads = cam.ToScreenQuads(x0, y0, x1, y1, l.quads[:0])
		for _, q := range l.quads {
			quad := render.Corners{{q.X0, q.Y0}, {q.X1, q.Y0}, {q.X0, q.Y1}, {q.X1, q.Y1}}
			f.Material(at, quad, &o)
		}
	})
}

// Draw draws the still through cam in the opaque pass, under a Lighting lit per pixel by the
// frame's light, shade and lamps, what a GroundDressing lays over it, and the grid over it
// after; where the tiles were composed into the frame, only what the dressing lays.
func (l *Renderer) Draw(t render.Target, cam camera.Camera, u render.Uniforms) {
	if t.Screen == nil {
		return
	}
	if !l.stillOn {
		if t.Pass == render.SeeThrough && l.ground != nil {
			l.ground.DressGround(t, cam, u)
		}
		return
	}
	if t.Pass == render.SeeThrough {
		if l.gridDrawn(cam) {
			l.grid.draw(t, cam, u, l.board, &l.state, l.space.Edges.WrapsX(),
				l.space.Edges.WrapsY())
		}
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
			l.still.Draw(t, cam, u, zoom, sx, sy, l.stillLight)
		}
	}
	if l.lit() != nil { // the still is drawn as it is: the light is laid over it per pixel
		l.shade.draw(t, cam, u, l.board)
	}
	if l.ground != nil {
		l.ground.DressGround(t, cam, u)
	}
}

func (l *Renderer) evenLight(m Map, d Dressing, cam camera.Camera) (render.Light, bool) {
	if l.space.Width == 0 || l.space.Height == 0 || m.Heights() != nil {
		return render.Light{}, false
	}
	if _, top := cam.Projection().(camera.TopDown); !top {
		return render.Light{}, false
	}
	even := render.Light{1, 1, 1}
	if d != nil {
		e, ok := d.(EvenLit)
		if !ok {
			return render.Light{}, false
		}
		if even, ok = e.EvenLight(); !ok {
			return render.Light{}, false
		}
	}
	return even, true
}

// the Lighting as it stands, nil for none
func (l *Renderer) lit() world.Lighting {
	if l.lighting == nil {
		return nil
	}
	return l.lighting()
}

// WithLighting has the tiles lit as lighting says, the light plugin's.
func (l *Renderer) WithLighting(lighting func() world.Lighting) *Renderer {
	l.lighting = lighting
	return l
}

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
	split := ok && okLook && l.count != 1 && l.pool.Size() > 1
	_, _, shown := l.state.Grid()
	if l.gridOn = shown && !l.white; l.gridOn { // a still's grid is drawn over it on the GPU
		l.nextGrid()
	}
	l.cells = l.cells[:0]
	l.eachVisible(func(c cell.ID) {
		l.topOf(c)
		if l.gridOn && l.gridShown(c) {
			i, _ := l.board.Ordinal(c)
			l.gridded[i] = l.gridStamp
		}
		if split {
			l.place(&l.tile, c)
			par.Warm(&l.tile)
		}
		l.cells = append(l.cells, c)
	})
	k := 1
	if split {
		k = l.pool.Parts(len(l.cells), tilesPerWorker, l.count)
	}
	if k < 2 {
		f.Branch(&l.over)
		for _, c := range l.cells {
			l.cell(f, &l.over, look, &l.tile, c, &l.scratch)
		}
		f.Append(&l.over)
		return
	}
	par.Ready()
	n := l.pool.Size()
	for len(l.workers) < n {
		tw := &tileWorker{}
		tw.tile.r = l
		l.workers = append(l.workers, tw)
	}
	for w, tw := range l.workers[:n] {
		tw.dress = par.Worker(w)
		tw.look = pl.Worker(w, tw.dress)
		tw.tile.Atlas, tw.tile.dress = sheet, tw.dress
	}
	for len(l.parts) < k {
		l.parts = append(l.parts, tilePart{})
	}
	for i := range l.parts[:k] {
		f.Branch(&l.parts[i].frame)
		f.Branch(&l.parts[i].over)
	}
	l.shares = k
	b := l.pool.Begin(l.drawPart, nil)
	b.Add(k)
	b.Wait()
	for i := range l.parts[:k] {
		f.Append(&l.parts[i].frame)
	}
	for i := range l.parts[:k] {
		f.Append(&l.parts[i].over)
	}
}

// drawParted lays part i of the visible tiles on worker w
func (l *Renderer) drawParted(w, i int) {
	tw, p := l.workers[w], &l.parts[i]
	from, to := parallel.Part(len(l.cells), l.shares, i)
	for _, c := range l.cells[from:to] {
		l.cell(&p.frame, &p.over, tw.look, &tw.tile, c, &tw.scratch)
	}
}

func (l *Renderer) place(t *Tile, c cell.ID) geom.Vec {
	center := l.board.CellCenter(c)
	t.ID = c
	t.X0, t.Y0 = float32(center.X-l.cellW/2), float32(center.Y-l.cellH/2)
	t.X1, t.Y1 = float32(center.X+l.cellW/2), float32(center.Y+l.cellH/2)
	return center
}

// the tile and what lies on it go to f, what runs across the board to over; a material's cell
// has only its grid drawn
func (l *Renderer) cell(f, over *render.Frame, look Look, t *Tile, c cell.ID, s *scratch) {
	t.Coarse, t.Fog = false, 0
	if l.sight != nil && !l.white {
		middle := l.board.CellCenter(c)
		t.Coarse = l.sight.Level(middle.X, middle.Y) != view.Exact
		t.Fog = l.sight.Fog(middle.X, middle.Y)
	}
	if _, ok := l.shaded[l.board.Kind(c).Name]; !ok {
		l.place(t, c)
		look.Cell(f, l.camera, t)
		if t.dress != nil && !t.Coarse {
			t.dress.Across(over, l.camera, t)
		}
	}
	if l.gridOn && !t.Coarse && l.griddedAt(c) {
		l.gridLines(f, c, s)
	}
}

// nextGrid starts a frame's marks of the cells drawing their grid
func (l *Renderer) nextGrid() {
	if n := l.board.CellCount(); len(l.gridded) != n {
		l.gridded, l.gridStamp = make([]uint32, n), 0
	}
	if l.gridStamp++; l.gridStamp == 0 { // wrapped round: old marks would pass for new
		clear(l.gridded)
		l.gridStamp = 1
	}
}

func (l *Renderer) griddedAt(c cell.ID) bool {
	i, ok := l.board.Ordinal(c)
	return ok && l.gridded[i] == l.gridStamp
}

func (l *Renderer) gridShown(c cell.ID) bool {
	scale := l.camera.Zoom()
	if l.scaleVaries {
		center := l.board.CellCenter(c)
		scale = camera.ScaleAt(l.camera, float32(center.X), float32(center.Y), l.topOf(c).alt)
	}
	return float32(min(l.cellW, l.cellH))*scale >= MinGridCell
}

// gridLines draws the sides of c the cell across leaves to it, each side of the board once
func (l *Renderer) gridLines(f *render.Frame, c cell.ID, s *scratch) {
	alt := l.topOf(c).alt
	s.outline = l.board.CellOutline(c, s.outline[:0])
	rims := l.rims
	if rims == nil {
		s.rims = rimsOf(l.board, c, s.rims)
		rims = s.rims
	}
	zoom := l.camera.Zoom()
	reach := float32(l.cellW+l.cellH) * zoom
	line, width := l.state.GridLine, l.state.GridWidth
	for _, e := range rims {
		if n, ok := l.board.Toward(c, e.dir); ok && !e.owns && l.griddedAt(n) {
			continue
		}
		p, q := s.outline[e.a], s.outline[e.b]
		px, py, qx, qy := float32(p.X), float32(p.Y), float32(q.X), float32(q.Y)
		ax, ay := l.camera.Project(px, py, alt)
		bx, by := l.camera.Project(qx, qy, alt)
		at := f.Lifted((px+qx)/2, (py+qy)/2, alt, gridLift)
		if abs32(bx-ax) <= reach && abs32(by-ay) <= reach {
			f.Line(at, ax, ay, bx, by, width, line)
			continue
		}
		// its ends on either side of a wrap seam, which only a camera from above has: both
		// images of it
		dx, dy := (qx-px)*zoom, (qy-py)*zoom
		f.Line(at, ax, ay, ax+dx, ay+dy, width, line)
		f.Line(at, bx-dx, by-dy, bx, by, width, line)
	}
}

func (l *Renderer) nextTops() {
	if n := l.board.CellCount(); len(l.tops) != n {
		l.tops = make([]cellTop, n)
	}
}

func (l *Renderer) topOf(c cell.ID) *cellTop {
	i, _ := l.board.Ordinal(c)
	t := &l.tops[i]
	if t.ver != 0 && t.seen == l.board.Changes() {
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

// pixels past the viewport's edge a cell still counts as drawn: what stands on it may reach in
const offScreen = 48

// the cells under the camera: of the sectors it sees where the world has them, each cell in the
// sector its middle lies in
func (l *Renderer) eachVisible(fn func(c cell.ID)) {
	bounds := l.camera.Bounds()
	if l.sight == nil || l.white {
		l.eachIn(bounds, fn)
		return
	}
	grid := l.sight.Grid()
	l.sight.Sectors(func(i int, box geom.AABB, _ view.Level) {
		part, ok := overlap(bounds, box)
		if !ok {
			return
		}
		l.eachIn(part, func(c cell.ID) {
			if middle := l.board.CellCenter(c); grid.Of(middle.X, middle.Y) == i {
				fn(c)
			}
		})
	})
}

func overlap(a, b geom.AABB) (geom.AABB, bool) {
	lo := geom.NewVec(max(a.TopLeft.X, b.TopLeft.X), max(a.TopLeft.Y, b.TopLeft.Y))
	hi := geom.NewVec(min(a.BottomRight.X, b.BottomRight.X), min(a.BottomRight.Y, b.BottomRight.Y))
	return geom.NewAABB(lo, hi), lo.X < hi.X && lo.Y < hi.Y
}

func (l *Renderer) eachIn(bounds geom.AABB, fn func(c cell.ID)) {
	if l.square {
		if l.camera.Projection().Sorts() {
			// the bounds are the rectangle round the screen's diamond, twice the cells on it
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

func (l *Renderer) dressing() Dressing { return l.mapping.Dressing() }

func abs32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}
