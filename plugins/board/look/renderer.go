package look

import (
	"image/color"

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

// Renderer is the render.Direct of a Board's cells laid on the GPU, flat over the world as the
// Map's Look lays them, and the grid over them when it is on.
type Renderer struct {
	board   Board
	mapping Map
	// what a camera sees of the world's sectors, and what the frame's sees: the cells drawn are
	// those of the sectors seen alone, at their levels
	sees  func(camera.Camera) *view.View
	sight *view.View
	// what lights the tiles, nil for white light
	lights func() *render.Lights
	atlas  render.AtlasSource
	cellW  float32
	cellH  float32
	square bool
	state  RenderState
	// the toggles as the frame drawn holds them, the live ones before any was published
	states    render.Buffered[RenderState]
	published bool
	shaded    map[cell.Name]render.MaterialID
	tops      []cellTop

	space   world.SpaceCfg // zero for none: nothing drawn
	dress   Dressing       // this frame's, nil for none
	grid    gridLines
	ground  GroundDressing // this frame's, nil for none
	tiles   tiles
	sectors seenSectors // the sectors the frame's camera sees, as the shaders read them
	tiled   bool        // this frame's tiles are laid on the GPU
}

type cellTop struct {
	sway   float32
	height float32
	sprite render.SpriteID
	ver    uint64 // one more than the cell's version when read; 0 not read yet
	seen   uint64 // the board's count of changes when last found as it was
}

// MinGridCell is how many pixels a cell must span on screen for the grid to be drawn over it.
const MinGridCell = 6

var _ render.Direct = (*Renderer)(nil)

// NewRenderer is a renderer of b's cells drawn from atlas by m, over the world space; nothing is
// drawn of a zero space.
func NewRenderer(b Board, atlas render.AtlasSource, m Map, space world.SpaceCfg) *Renderer {
	w, h := b.CellBounds()
	shape, ok := b.Shape()
	return &Renderer{board: b, atlas: atlas, cellW: w, cellH: h, square: ok && !shape.Hex,
		mapping: m, space: space}
}

// how far over the ground the grid lies among what lies there, under the sight and the routes
const gridLift = 0.1

// SetAtlas has the cells drawn from atlas from the next frame on.
func (l *Renderer) SetAtlas(atlas render.AtlasSource) {
	l.atlas = atlas
	l.tops = l.tops[:0]
}

// State is the renderer's live toggles.
func (l *Renderer) State() *RenderState { return &l.state }

// Publish hands the frame drawn next the toggles as they stand.
func (l *Renderer) Publish() { *l.states.Back() = l.state }

// Swap has the toggles published last drawn.
func (l *Renderer) Swap() {
	l.states.Swap()
	l.published = true
}

// drawn is the toggles the frame drawn holds
func (l *Renderer) drawn() *RenderState {
	if !l.published {
		return &l.state
	}
	return l.states.Front()
}

// Shade is the per-pixel looks the board's Atlas declared, by the kind's name. A shaded kind
// takes no part in blending.
func (l *Renderer) Shade(shaded map[cell.Name]render.MaterialID) { l.shaded = shaded }

// WithSeen has the cells drawn of the sectors a camera sees alone, as seen says, coarsely where it
// sees one coarsely, fading into the fog before its sight's end.
func (l *Renderer) WithSeen(seen func(camera.Camera) *view.View) *Renderer {
	l.sees = seen
	return l
}

// Compose readies the Map's Dressing for the frame through cam and asks for the tiles to be laid
// in the opaque pass and the grid over them when it is on; nothing before the renderer has an
// atlas, of a Look laying Nothing or of no world.
func (l *Renderer) Compose(f *render.Frame, cam camera.Camera) {
	l.tiled = false
	if l.atlas == nil {
		return
	}
	l.sight = nil
	if l.sees != nil {
		if v := l.sees(cam); v.Grid() != nil {
			l.sight = v
		}
	}
	d := l.dressing()
	if d != nil {
		d.Begin(f, cam)
	}
	l.ground, _ = d.(GroundDressing)
	if l.mapping.Look() == Nothing || l.space.Width == 0 || l.space.Height == 0 {
		return // the dressing is readied for the frame, no more
	}
	l.tiled = true
	l.ready(d)
	l.sectors.see(l.sight)
	f.DrawOpaque()
	if l.gridDrawn(cam) {
		f.DrawAt(f.OnGround(gridLift))
	}
}

func (l *Renderer) gridDrawn(cam camera.Camera) bool {
	_, _, shown := l.drawn().Grid()
	return shown && min(l.cellW, l.cellH)*cam.Zoom() >= MinGridCell
}

// Draw lays the tiles on the GPU through cam in the opaque pass, under a Lighting lit per pixel by
// the frame's light, shade and lamps, then what a GroundDressing lays over them and the cells
// drawn by their material, and the grid over them after; nothing through a camera without lines
// of sight.
func (l *Renderer) Draw(t render.Target, cam camera.Camera, u render.Uniforms) {
	if t.Screen == nil || !l.tiled {
		return
	}
	if t.Pass == render.SeeThrough {
		if l.gridDrawn(cam) {
			l.grid.draw(t, cam, u, l.board, l.drawn(), l.space.Edges.WrapsX(),
				l.space.Edges.WrapsY(), &l.sectors)
		}
		return
	}
	v, ok := l.seeingOf(cam)
	if !ok {
		return
	}
	l.tiles.draw(t, v, u, l, render.Light{1, 1, 1}, l.lit() != nil, &l.sectors)
	if l.ground != nil {
		l.ground.DressGround(t, cam, u)
	}
	l.tiles.materials(t)
}

// ready brings the tile map up to the board as the Dressing d draws it
func (l *Renderer) ready(d Dressing) {
	l.dress = d
	l.nextTops()
	l.tiles.refresh(l, l.atlas)
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
	v.sways = l.mapping.Look() == Swaying
	return v, true
}

// Whole draws the whole board from above into dst, px pixels a world unit, in light and under the
// uniforms u, none of it swaying; nothing of a board of no world.
func (l *Renderer) Whole(dst *render.Image, px float32, light render.Light, u render.Uniforms) {
	if l.space.Width == 0 || l.space.Height == 0 || px <= 0 {
		return
	}
	l.ready(l.dressing())
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

// the lights as they stand, nil for none
func (l *Renderer) lit() *render.Lights {
	if l.lights == nil {
		return nil
	}
	return l.lights()
}

// WithLights has the tiles lit by the lights lights gives, the light plugin's.
func (l *Renderer) WithLights(lights func() *render.Lights) *Renderer {
	l.lights = lights
	return l
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
	t.sway, t.height, t.sprite, t.ver = kind.Sway, kind.Height, kind.SpriteID, v
	return t
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
