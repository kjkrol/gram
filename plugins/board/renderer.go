package board

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/render"
)

var colorGridLine = color.RGBA{R: 20, G: 20, B: 20, A: 120}

// RenderState is the board renderer's live display toggles.
type RenderState struct {
	ShowGridLines bool
}

// ToggleShowGridLines flips whether grid lines are drawn.
func (r *RenderState) ToggleShowGridLines() { r.ShowGridLines = !r.ShowGridLines }

// Renderer draws Board's cells through a viewport's camera — list it before the entities layer in
// a Scene's Layers so terrain sits underneath, or hand it to render.NewSorted, where it submits
// each cell as a quad at the cell's altitude and depth (no grid lines there).
type Renderer struct {
	board   *Board
	camera  camera.Camera // the one of the frame being drawn
	atlas   render.AtlasSource
	cellW   float64
	cellH   float64
	state   *RenderState
	batch   *render.QuadBatch
	lines   *render.LineBatch
	outline []geom.Vec
	// seen marks by ordinal the cells a frame has visited on a grid walked by sampling; stamp is the frame's mark.
	seen  []uint32
	stamp uint32
	// tops holds by ordinal what a Submit has read of a cell, good while its stamp is topStamp.
	tops     []cellTop
	topStamp uint32
	// relief draws the sides of raised ground and of tall kinds: an isometric camera's view.
	relief bool
}

// Shades of a block's faces against its top: the side facing down-right and the one facing
// down-left, as if lit from the upper left; a level tile is drawn at shadeLevel, so a slope
// rising towards the light can be brighter and one falling away darker (shadePerUnit per world
// unit of rise across the cell).
const (
	shadeRight   = 0.72
	shadeLeft    = 0.55
	shadeLevel   = 0.92
	shadePerUnit = 0.012
)

// cellTop is a cell as one Submit reads it once: its corners with its kind standing on them,
// its ground level and its sprite.
type cellTop struct {
	z      [4]float32
	alt    float32
	sprite render.SpriteID
	stamp  uint32
}

// minGridCell is how many pixels a cell must span on screen for the grid to be drawn over it.
const minGridCell = 6

func abs32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

var _ render.Submitter = (*Renderer)(nil)
var _ render.Overlayer = (*Renderer)(nil)

func newRenderer(board *Board, atlas render.AtlasSource, state *RenderState) *Renderer {
	w, h := board.CellBounds()
	return &Renderer{board: board, atlas: atlas, cellW: w, cellH: h, state: state,
		batch: render.NewQuadBatch(atlas), lines: render.NewLineBatch()}
}

func (l *Renderer) Init(*goke.SysInit) {}

// look takes cam for the frame being drawn.
func (l *Renderer) look(cam camera.Camera) {
	l.camera = cam
	_, l.relief = cam.Projection().(camera.Isometric)
}

// DrawWorld draws the cells under cam, flat, with the grid lines when they are on.
func (l *Renderer) DrawWorld(screen *ebiten.Image, cam camera.Camera) {
	l.look(cam)
	l.batch.Reset(cam)
	l.lines.Reset()
	l.eachVisible(l.drawCell)
	l.batch.Flush(screen)
	l.lines.Flush(screen)
}

// gridShown reports whether the grid is on and its cells are large enough on screen to read.
func (l *Renderer) gridShown() bool {
	return l.state.ShowGridLines && float32(min(l.cellW, l.cellH))*l.camera.Zoom() >= minGridCell
}

// Submit hands every visible cell to sink at the depth of its centre, so the entities standing on
// it follow it: its top raised by its kind's Height over the ground, sloped between the corner
// heights the board gives, and, through an isometric camera, the two faces towards the viewer
// wherever that top stands above the neighbour's — a wall over grass, a raised edge over the sea.
func (l *Renderer) Submit(sink *render.Sink, cam camera.Camera) {
	l.look(cam)
	l.nextTops()
	l.eachVisible(func(c CellID) {
		center := l.board.CellCenter(c)
		t := l.topOf(c)
		alt, top, sprite := t.alt, t.z, t.sprite
		x0, y0 := float32(center.X-l.cellW/2), float32(center.Y-l.cellH/2)
		x1, y1 := float32(center.X+l.cellW/2), float32(center.Y+l.cellH/2)
		depth := l.camera.Depth(float32(center.X), float32(center.Y), alt)
		if l.relief {
			// the face along x = x1 shows down to the top of the neighbour across it, likewise y = y1
			if fa, fb := l.neighbourTops(center.X+l.cellW, center.Y, 0, 2); top[1] > fa || top[3] > fb {
				sink.Shaded(depth, l.atlas, sprite, l.face(x1, y0, x1, y1, top[1], top[3], fa, fb), shadeRight)
			}
			if fa, fb := l.neighbourTops(center.X, center.Y+l.cellH, 0, 1); top[2] > fa || top[3] > fb {
				sink.Shaded(depth, l.atlas, sprite, l.face(x0, y1, x1, y1, top[2], top[3], fa, fb), shadeLeft)
			}
		}
		shade := float32(1)
		if l.relief {
			shade = slopeShade(top)
		}
		sink.Shaded(depth, l.atlas, sprite, l.sloped(x0, y0, x1, y1, top), shade)
	})
}

// slopeShade lights a tile from the upper left: level at shadeLevel, brighter where it rises
// towards the light (up and to the left), darker where it falls away.
func slopeShade(top [4]float32) float32 {
	rise := (top[1] + top[3] - top[0] - top[2]) / 2 // along x, towards the right
	rise += (top[2] + top[3] - top[0] - top[1]) / 2 // along y, downwards
	return min(max(shadeLevel-shadePerUnit*rise, 0.45), 1)
}

// Overlay strokes the grid over a sorted picture, each cell's outline on the ground at its corner
// heights, when ShowGridLines is on.
func (l *Renderer) Overlay(screen *ebiten.Image, cam camera.Camera) {
	l.look(cam)
	if !l.gridShown() {
		return
	}
	l.lines.Reset()
	l.eachVisible(func(c CellID) {
		center := l.board.CellCenter(c)
		x0, y0 := float32(center.X-l.cellW/2), float32(center.Y-l.cellH/2)
		x1, y1 := float32(center.X+l.cellW/2), float32(center.Y+l.cellH/2)
		var z [4]float32
		if hs, _, _, ok := l.board.Corners(c); ok {
			for i := range z {
				z[i] = float32(hs[i])
			}
		} else {
			alt := float32(l.board.Altitude(c))
			z = [4]float32{alt, alt, alt, alt}
		}
		p := l.sloped(x0, y0, x1, y1, z)
		// the two edges towards the viewer; the neighbours draw the other two
		l.lines.Append(p[1][0], p[1][1], p[3][0], p[3][1], 1, colorGridLine)
		l.lines.Append(p[2][0], p[2][1], p[3][0], p[3][1], 1, colorGridLine)
	})
	l.lines.Flush(screen)
}

// nextTops starts a Submit: every cell read before is read anew.
func (l *Renderer) nextTops() {
	if n := l.board.CellCount(); len(l.tops) != n {
		l.tops, l.topStamp = make([]cellTop, n), 0
	}
	if l.topStamp++; l.topStamp == 0 { // wrapped round: old reads would pass for new
		clear(l.tops)
		l.topStamp = 1
	}
}

// topOf is c as this Submit sees it, read from the board the first time it is asked for: the
// ground's corners on a sloped grid, its level everywhere on a flat one, raised by the kind's Height.
func (l *Renderer) topOf(c CellID) *cellTop {
	i, _ := l.board.ordinal(c)
	t := &l.tops[i]
	if t.stamp == l.topStamp {
		return t
	}
	kind := l.board.kindOf(c)
	r := l.board.Relief(c)
	rise := float32(kind.Height)
	t.alt, t.sprite, t.stamp = float32(r.Level()), kind.SpriteID, l.topStamp
	if l.board.sloped() {
		for k := range t.z {
			t.z[k] = r.Corners[k] + rise
		}
	} else {
		t.z = [4]float32{t.alt + rise, t.alt + rise, t.alt + rise, t.alt + rise}
	}
	return t
}

// neighbourTops is the top of the cell at (x, y) at its corners a and b, the sea level 0 off the board.
func (l *Renderer) neighbourTops(x, y float64, a, b int) (float32, float32) {
	c, ok := l.board.CellAt(geom.NewVec(x, y))
	if !ok {
		return 0, 0
	}
	t := l.topOf(c)
	return t.z[a], t.z[b]
}

// corners projects the four corners of a world box at height z.
func (l *Renderer) corners(x0, y0, x1, y1, z float32) render.Corners {
	return l.sloped(x0, y0, x1, y1, [4]float32{z, z, z, z})
}

// sloped projects the four corners of a world box, each at its own height.
func (l *Renderer) sloped(x0, y0, x1, y1 float32, z [4]float32) render.Corners {
	var out render.Corners
	for i, p := range [4][2]float32{{x0, y0}, {x1, y0}, {x0, y1}, {x1, y1}} {
		out[i][0], out[i][1] = l.camera.Project(p[0], p[1], z[i])
	}
	return out
}

// face projects a wall from the edge (ax, ay)-(bx, by): tops topA and topB down to feet footA
// and footB.
func (l *Renderer) face(ax, ay, bx, by, topA, topB, footA, footB float32) render.Corners {
	var out render.Corners
	out[0][0], out[0][1] = l.camera.Project(ax, ay, topA)
	out[1][0], out[1][1] = l.camera.Project(bx, by, topB)
	out[2][0], out[2][1] = l.camera.Project(ax, ay, footA)
	out[3][0], out[3][1] = l.camera.Project(bx, by, footB)
	return out
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

func (l *Renderer) drawCell(c CellID) {
	center := l.board.CellCenter(c)
	x0, y0 := center.X-l.cellW/2, center.Y-l.cellH/2
	x1, y1 := center.X+l.cellW/2, center.Y+l.cellH/2

	l.batch.AppendQuad(float32(x0), float32(y0), float32(x1), float32(y1), l.board.Kind(c).SpriteID)

	if l.gridShown() {
		l.outline = l.board.CellOutline(c, l.outline[:0])
		for i, p := range l.outline {
			q := l.outline[(i+1)%len(l.outline)]
			ax, ay := l.camera.ToScreen(float32(p.X), float32(p.Y))
			bx, by := l.camera.ToScreen(float32(q.X), float32(q.Y))
			if reach := float32(l.cellW+l.cellH) * l.camera.Zoom(); abs32(bx-ax) > reach || abs32(by-ay) > reach {
				continue // the edge straddles a wrap seam; its images are drawn by the cells either side
			}
			l.lines.Append(ax, ay, bx, by, 1, colorGridLine)
		}
	}
}
