package look

import (
	"image/color"
	"math"
	"slices"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
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

// Board is what a Renderer reads of a board; a board.BoardRead is one.
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
	seen     []uint32
	stamp    uint32
	order    []int
	tops     []cellTop
	// the sides of every cell's outline on one of grid.DefaultGrids, nil on another grid
	rims []rim
	// gridded[i] is gridStamp where the cell of ordinal i draws its grid this frame
	gridded   []uint32
	gridStamp uint32
	// scaleVaries: a perspective draws a world unit larger in some places than others
	scaleVaries bool

	over    render.Frame   // what runs across the board, laid after every tile
	space   world.SpaceCfg // zero for none: the tiles composed into the frame every frame
	gridOn  bool           // this frame's tiles draw the grid
	grid    gridLines
	ground  GroundDressing // this frame's, nil for none
	tiles   tiles
	sectors seenSectors  // the sectors the frame's camera sees, as the shaders read them
	tiled   bool         // this frame's tiles are laid on the GPU
	tint    render.Light // the light every tile is lit in this frame
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

// what drawing tiles keeps from one to the next
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
	if l.tint, l.tiled = l.tiledBy(m, d, cam); l.tiled {
		l.ready(d)
		l.sectors.see(l.sight)
		f.DrawOpaque()
		if l.gridDrawn(cam) {
			f.DrawAt(f.OnGround(gridLift))
		}
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

// Draw lays the tiles on the GPU through cam in the opaque pass, under a Lighting lit per pixel by
// the frame's light, shade and lamps, what a GroundDressing lays over them, and the grid over
// them after; where the tiles were composed into the frame, only what the dressing lays.
func (l *Renderer) Draw(t render.Target, cam camera.Camera, u render.Uniforms) {
	if t.Screen == nil {
		return
	}
	if l.tiled {
		if t.Pass == render.SeeThrough {
			if l.gridDrawn(cam) {
				l.grid.draw(t, cam, u, l.board, &l.state, l.space.Edges.WrapsX(),
					l.space.Edges.WrapsY(), &l.sectors)
			}
			return
		}
		if v, ok := l.seeingOf(cam); ok {
			l.tiles.draw(t, v, u, l, l.tint, l.lit() != nil, &l.sectors)
		}
		if l.ground != nil {
			l.ground.DressGround(t, cam, u)
		}
		l.tiles.materials(t)
		return
	}
	if t.Pass == render.SeeThrough && l.ground != nil {
		l.ground.DressGround(t, cam, u)
	}
}

// ready brings the tile map up to the board as the Dressing d draws it
func (l *Renderer) ready(d Dressing) {
	sheet := l.atlas
	if d != nil {
		sheet = d.Sheet(l.atlas)
	}
	l.tile.Atlas, l.tile.dress = sheet, d
	l.nextTops()
	l.tiles.refresh(l, sheet)
}

// seeingOf is how cam sees the tiles; false for a camera without lines of sight
func (l *Renderer) seeingOf(cam camera.Camera) (seeing, bool) {
	rays, ok := cam.(camera.Rays)
	if !ok {
		return seeing{}, false
	}
	f, ok := rays.Rays()
	if !ok {
		return seeing{}, false
	}
	v := seeing{rays: f}
	v.w, v.h = cam.Viewport()
	v.x0, v.x1, v.y0, v.y1 = l.copies(cam)
	_, v.sways = l.mapping.Look().(Swaying)
	return v, true
}

// Whole draws the whole board from above into dst, px pixels a world unit, in light and under the
// uniforms u, none of it swaying; nothing of a board of no world.
func (l *Renderer) Whole(dst *render.Image, px float32, light render.Light, u render.Uniforms) {
	if l.space.Width == 0 || l.space.Height == 0 || px <= 0 {
		return
	}
	d := l.dressing()
	l.ready(d)
	if e, ok := d.(EvenLit); ok {
		if even, ok := e.EvenLight(); ok {
			for k := range light {
				light[k] *= even[k]
			}
		}
	}
	b := dst.Bounds()
	v := seeing{w: float32(b.Dx()), h: float32(b.Dy()), rays: camera.RayField{
		Origin: [3]float32{0, 0, wholeEye}, DX: [3]float32{1 / px, 0, 0},
		DY: [3]float32{0, 1 / px, 0}, Dir: [3]float32{0, 0, -1}}}
	var none seenSectors
	t := render.Target{Screen: dst}
	l.tiles.draw(t, v, u, l, light, false, &none)
	l.tiles.materials(t)
}

// Animated reports whether what Whole draws changes with the clock: some cells drawn by a
// material or under one.
func (l *Renderer) Animated() bool { return l.tiles.shaded > 0 || l.tiles.overlaid > 0 }

// how high over the board Whole looks down from
const wholeEye = 1 << 16

// tiledBy reports whether the tiles are laid on the GPU this frame through cam, and in what light:
// a flat map over a world, seen through a camera with lines of sight, its Dressing lighting every
// tile alike
func (l *Renderer) tiledBy(m Map, d Dressing, cam camera.Camera) (render.Light, bool) {
	if l.space.Width == 0 || l.space.Height == 0 || m.Heights() != nil {
		return render.Light{}, false
	}
	if _, ok := cam.(camera.Rays); !ok {
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
	_, _, shown := l.state.Grid()
	if l.gridOn = shown; l.gridOn {
		l.nextGrid()
	}
	l.cells = l.cells[:0]
	l.eachVisible(func(c cell.ID) {
		l.topOf(c)
		if l.gridOn && l.gridShown(c) {
			i, _ := l.board.Ordinal(c)
			l.gridded[i] = l.gridStamp
		}
		l.cells = append(l.cells, c)
	})
	f.Branch(&l.over)
	for _, c := range l.cells {
		l.cell(f, &l.over, look, &l.tile, c, &l.scratch)
	}
	f.Append(&l.over)
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
	if l.sight != nil {
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
	if l.sight == nil {
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
	l.order = l.order[:0]
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
			l.order = append(l.order, i)
		}
	}
	// in the order the cells are laid, row by row
	slices.Sort(l.order)
	shape, _ := l.board.Shape()
	for _, i := range l.order {
		fn(l.board.At(uint32(i)%shape.Cols, uint32(i)/shape.Cols))
	}
}

func (l *Renderer) dressing() Dressing { return l.mapping.Dressing() }

// the axial steps to a hex's neighbours, row by row as they are laid, and the steps round a
// square's cell
var (
	hexRound    = [6][2]int{{0, -1}, {1, -1}, {-1, 0}, {1, 0}, {-1, 1}, {0, 1}}
	squareRound = [8][2]int{{-1, -1}, {0, -1}, {1, -1}, {-1, 0}, {1, 0}, {-1, 1}, {0, 1}, {1, 1}}
)

// eachRound calls fn with each cell round c as the tiles shader reads them, false where there is
// none
func (l *Renderer) eachRound(c cell.ID, fn func(n cell.ID, ok bool)) {
	steps := squareRound[:]
	if !l.square {
		steps = hexRound[:]
	}
	for _, s := range steps {
		n, ok := l.step(c, s[0], s[1])
		fn(n, ok)
	}
}

// step is the cell da across and db down from c, as the grid's directions go
func (l *Renderer) step(c cell.ID, da, db int) (cell.ID, bool) {
	for d := range l.board.Directions() {
		if a, b := l.board.Offset(d); a == da && b == db {
			return l.board.Toward(c, d)
		}
	}
	return 0, false
}

func abs32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}
