package occupancy

import (
	"image/color"
	"math"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/world/view"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/uid"
)

// View draws the cells an Occupancy holds while shown, as published once a frame: every hold its
// cell's outline lying over it at its level, in its holder's colour.
type View struct {
	grid      grid.Grid
	occupancy cell.Occupancy
	height    Height
	views     func() []*view.View
	shown     bool
	width     float32
	drawn     render.Buffered[holds]

	back    *holds
	visited []uint64 // the cells published, by ordinal
	at      cell.ID
	outline []geom.Vec
	cell    func(cell.ID)                      // inCell, bound once
	once    func(cell.ID)                      // inCell once a cell, bound once
	hold    func(uid.UID64, cell.Domain, bool) // takeHold, bound once
}

// holds are what the View draws: every hold, its points holds.points[from:from+n] round it in the
// world, and how wide
type holds struct {
	all    []hold
	points [][3]float32
	width  float32
}

type hold struct {
	mid     [3]float32
	color   color.RGBA
	from, n int32
}

// Height is how high the point p of the cell c lies on the ground, or on the raised deck over c.
type Height func(c cell.ID, p geom.Vec, deck bool) float64

const (
	// of the way to the cell's middle; a deck's further in
	groundInset = 0.12
	deckInset   = 0.26
	// how far over its cell a hold lies among what lies there: over the grid, under the sight
	holdLift = 0.15
)

var _ render.Source = (*View)(nil)

// NewView is a View, hidden, of o's holds over g under what the cameras' views see (every cell
// with none), each drawn at the height height says, in no line till Width.
func NewView(g grid.Grid, o cell.Occupancy, height Height, views func() []*view.View) *View {
	v := &View{grid: g, occupancy: o, height: height, views: views}
	v.cell, v.once, v.hold = v.inCell, v.inCellOnce, v.takeHold
	return v
}

// Width sets how many pixels wide a hold is outlined.
func (v *View) Width(px float32) { v.width = px }

// Show shows the holds, or hides them.
func (v *View) Show(shown bool) { v.shown = shown }

// Shown reports whether the holds are shown.
func (v *View) Shown() bool { return v.shown }

// Publish takes every hold of the cells under what the cameras see, for the frame drawn next.
func (v *View) Publish() {
	b := v.drawn.Back()
	b.all, b.points, b.width = b.all[:0], b.points[:0], v.width
	if !v.shown || v.occupancy == nil {
		return
	}
	v.back = b
	var views []*view.View
	if v.views != nil {
		views = v.views()
	}
	if len(views) == 0 {
		v.grid.EachCell(v.cell)
		return
	}
	if n := (v.grid.CellCount() + 63) / 64; len(v.visited) != n {
		v.visited = make([]uint64, n)
	}
	clear(v.visited)
	for _, seen := range views {
		v.grid.CellsUnder(seen.Bounds, v.once)
	}
}

// Swap has what was published last drawn.
func (v *View) Swap() { v.drawn.Swap() }

func (v *View) Compose(f *render.Frame, cam camera.Camera) {
	d := v.drawn.Front()
	for _, h := range d.all {
		at := f.Lifted(h.mid[0], h.mid[1], h.mid[2], holdLift)
		pts := d.points[h.from : h.from+h.n]
		var x0, y0, xp, yp float32
		for i, p := range pts {
			x, y := cam.Project(p[0], p[1], p[2])
			if i == 0 {
				x0, y0 = x, y
			} else {
				f.Line(at, xp, yp, x, y, d.width, h.color)
			}
			xp, yp = x, y
		}
		if len(pts) > 1 {
			f.Line(at, xp, yp, x0, y0, d.width, h.color)
		}
	}
}

func (v *View) inCellOnce(c cell.ID) {
	if k, ok := v.grid.Ordinal(c); ok {
		if v.visited[k/64]&(1<<(k%64)) != 0 {
			return
		}
		v.visited[k/64] |= 1 << (k % 64)
	}
	v.inCell(c)
}

func (v *View) inCell(c cell.ID) {
	v.at = c
	v.occupancy.Holders(c, v.hold)
}

func (v *View) takeHold(e uid.UID64, _ cell.Domain, deck bool) {
	c, b := v.at, v.back
	v.outline = v.grid.CellOutline(c, v.outline[:0])
	mid := v.grid.CellCenter(c)
	inset, col := groundInset, holderColor(e)
	if deck {
		inset, col = deckInset, lighter(col)
	}
	h := hold{mid: [3]float32{float32(mid.X), float32(mid.Y), float32(v.height(c, mid, deck))},
		color: col, from: int32(len(b.points)), n: int32(len(v.outline))}
	for _, p := range v.outline {
		p = geom.NewVec(p.X+(mid.X-p.X)*inset, p.Y+(mid.Y-p.Y)*inset)
		b.points = append(b.points, [3]float32{float32(p.X), float32(p.Y),
			float32(v.height(c, p, deck))})
	}
	b.all = append(b.all, h)
}

// hues a golden angle apart by index, so neighbours differ
func holderColor(e uid.UID64) color.RGBA {
	h := math.Mod(float64(e.Index())*0.618033988749895, 1) * 6
	x := 1 - math.Abs(math.Mod(h, 2)-1)
	var r, g, b float64
	switch int(h) {
	case 0:
		r, g = 1, x
	case 1:
		r, g = x, 1
	case 2:
		g, b = 1, x
	case 3:
		g, b = x, 1
	case 4:
		r, b = x, 1
	default:
		r, b = 1, x
	}
	return color.RGBA{R: uint8(60 + 195*r), G: uint8(60 + 195*g), B: uint8(60 + 195*b), A: 255}
}

func lighter(c color.RGBA) color.RGBA {
	return color.RGBA{R: c.R/2 + 128, G: c.G/2 + 128, B: c.B/2 + 128, A: c.A}
}
