package board

import (
	"image/color"

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

// Renderer is the render.Source of Board's cells: their tiles on the Ground tier, and the grid over
// them when it is on.
type Renderer struct {
	board   *Board
	camera  camera.Camera // the one of the frame being drawn
	atlas   render.AtlasSource
	cellW   float64
	cellH   float64
	state   *RenderState
	outline []geom.Vec
	// seen marks by ordinal the cells a frame has visited on a grid walked by sampling; stamp is the frame's mark.
	seen  []uint32
	stamp uint32
	// tops holds by ordinal what a Compose has read of a cell, good while its stamp is topStamp.
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

// cellTop is a cell as one Compose reads it once: its corners with its kind standing on them,
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

var _ render.Source = (*Renderer)(nil)

// gridTier puts the lines of a grid other than square over every tile and under whatever stands on
// them.
const gridTier = render.Ground + 10

func newRenderer(board *Board, atlas render.AtlasSource, state *RenderState) *Renderer {
	w, h := board.CellBounds()
	return &Renderer{board: board, atlas: atlas, cellW: w, cellH: h, state: state}
}

func (l *Renderer) Init(*goke.SysInit) {}

// Compose hands f every cell under cam, and the grid when it is on: on a square grid each tile
// outlined along its own edges, on any other the cells' outlines as lines. From above a cell is its
// sprite over its box; through an isometric camera it is its top at the depth of its centre, so
// what stands on it follows it: raised by its kind's Height over the ground, sloped between the
// corner heights, with the two faces towards the viewer wherever it stands above the neighbour's
// top — a wall over grass, a raised edge over the sea.
func (l *Renderer) Compose(f *render.Frame, cam camera.Camera) {
	l.camera = cam
	_, l.relief = cam.Projection().(camera.Isometric)
	grid := l.gridShown()
	outlined, lines := grid && l.board.square != nil, grid && l.board.square == nil
	if !l.relief {
		l.eachVisible(func(c CellID) {
			center := l.board.CellCenter(c)
			x0, y0 := float32(center.X-l.cellW/2), float32(center.Y-l.cellH/2)
			x1, y1 := x0+float32(l.cellW), y0+float32(l.cellH)
			if sprite := l.board.kindOf(c).SpriteID; outlined {
				f.TileRect(render.Ground, 0, l.atlas, sprite, x0, y0, x1, y1)
			} else {
				f.SpriteRect(render.Ground, 0, l.atlas, sprite, x0, y0, x1, y1)
			}
			if lines {
				l.outlineFlat(f, c)
			}
		})
		return
	}
	l.nextTops()
	l.eachVisible(func(c CellID) {
		center := l.board.CellCenter(c)
		t := l.topOf(c)
		top, sprite := t.z, t.sprite
		x0, y0 := float32(center.X-l.cellW/2), float32(center.Y-l.cellH/2)
		x1, y1 := float32(center.X+l.cellW/2), float32(center.Y+l.cellH/2)
		depth := l.camera.Depth(float32(center.X), float32(center.Y), t.alt)
		// the face along x = x1 shows down to the top of the neighbour across it, likewise y = y1
		if fa, fb := l.neighbourTops(center.X+l.cellW, center.Y, 0, 2); top[1] > fa || top[3] > fb {
			f.Sprite(render.Ground, depth, l.atlas, sprite, l.face(x1, y0, x1, y1, top[1], top[3], fa, fb), shadeRight)
		}
		if fa, fb := l.neighbourTops(center.X, center.Y+l.cellH, 0, 1); top[2] > fa || top[3] > fb {
			f.Sprite(render.Ground, depth, l.atlas, sprite, l.face(x0, y1, x1, y1, top[2], top[3], fa, fb), shadeLeft)
		}
		if outlined {
			f.Tile(render.Ground, depth, l.atlas, sprite, l.sloped(x0, y0, x1, y1, top), slopeShade(top))
		} else {
			f.Sprite(render.Ground, depth, l.atlas, sprite, l.sloped(x0, y0, x1, y1, top), slopeShade(top))
		}
		if lines {
			l.outlineSloped(f, c, depth, x0, y0, x1, y1)
		}
	})
}

// gridShown reports whether the grid is on and its cells are large enough on screen to read.
func (l *Renderer) gridShown() bool {
	return l.state.ShowGridLines && float32(min(l.cellW, l.cellH))*l.camera.Zoom() >= minGridCell
}

// slopeShade lights a tile from the upper left: level at shadeLevel, brighter where it rises
// towards the light (up and to the left), darker where it falls away.
func slopeShade(top [4]float32) float32 {
	rise := (top[1] + top[3] - top[0] - top[2]) / 2 // along x, towards the right
	rise += (top[2] + top[3] - top[0] - top[1]) / 2 // along y, downwards
	return min(max(shadeLevel-shadePerUnit*rise, 0.45), 1)
}

// outlineSloped draws the two edges of c towards the viewer on the ground at its corner heights;
// the neighbours draw the other two.
func (l *Renderer) outlineSloped(f *render.Frame, c CellID, depth, x0, y0, x1, y1 float32) {
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
	// the edges border the two neighbours nearer the viewer: drawn after their tiles, else they halve them
	cx, cy := (x0+x1)/2, (y0+y1)/2
	depth = max(depth, l.camera.Depth(x1+(x1-x0)/2, cy, z[3]), l.camera.Depth(cx, y1+(y1-y0)/2, z[3]))
	f.Line(gridTier, depth, p[1][0], p[1][1], p[3][0], p[3][1], 1, colorGridLine)
	f.Line(gridTier, depth, p[2][0], p[2][1], p[3][0], p[3][1], 1, colorGridLine)
}

// outlineFlat draws c's outline from above, leaving out an edge that straddles a wrap seam: the
// cells either side draw its images.
func (l *Renderer) outlineFlat(f *render.Frame, c CellID) {
	l.outline = l.board.CellOutline(c, l.outline[:0])
	reach := float32(l.cellW+l.cellH) * l.camera.Zoom()
	for i, p := range l.outline {
		q := l.outline[(i+1)%len(l.outline)]
		ax, ay := l.camera.ToScreen(float32(p.X), float32(p.Y))
		bx, by := l.camera.ToScreen(float32(q.X), float32(q.Y))
		if abs32(bx-ax) > reach || abs32(by-ay) > reach {
			continue
		}
		f.Line(gridTier, 0, ax, ay, bx, by, 1, colorGridLine)
	}
}

// nextTops starts a Compose: every cell read before is read anew.
func (l *Renderer) nextTops() {
	if n := l.board.CellCount(); len(l.tops) != n {
		l.tops, l.topStamp = make([]cellTop, n), 0
	}
	if l.topStamp++; l.topStamp == 0 { // wrapped round: old reads would pass for new
		clear(l.tops)
		l.topStamp = 1
	}
}

// topOf is c as this Compose sees it, read from the board the first time it is asked for: the
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
