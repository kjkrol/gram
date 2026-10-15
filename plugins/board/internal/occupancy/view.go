package occupancy

import (
	"image/color"
	"math"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/uid"
)

// View draws the cells an Occupancy holds while shown: every hold its cell's outline lying over
// it at its level, in its holder's colour.
type View struct {
	grid      grid.Grid
	occupancy cell.Occupancy
	height    Height
	shown     bool

	frame   *render.Frame
	cam     camera.Camera
	at      cell.ID
	outline []geom.Vec
	cell    func(cell.ID)                      // inCell, bound once
	hold    func(uid.UID64, cell.Domain, bool) // drawHold, bound once
}

// Height is how high the point p of the cell c lies on the ground, or on the raised deck over c.
type Height func(c cell.ID, p geom.Vec, deck bool) float64

const (
	holdWidth = 2
	// of the way to the cell's middle; a deck's further in
	groundInset = 0.12
	deckInset   = 0.26
	// how far over its cell a hold lies among what lies there: over the grid, under the sight
	holdLift = 0.15
)

var _ render.Source = (*View)(nil)

// NewView is a View, hidden, of o's holds over g, each drawn at the height height says.
func NewView(g grid.Grid, o cell.Occupancy, height Height) *View {
	v := &View{grid: g, occupancy: o, height: height}
	v.cell, v.hold = v.inCell, v.drawHold
	return v
}

// Show shows the holds, or hides them.
func (v *View) Show(shown bool) { v.shown = shown }

// Shown reports whether the holds are shown.
func (v *View) Shown() bool { return v.shown }

func (v *View) Init(*goke.SysInit) {}

func (v *View) Compose(f *render.Frame, cam camera.Camera) {
	if !v.shown || v.occupancy == nil {
		return
	}
	v.frame, v.cam = f, cam
	v.grid.CellsUnder(cam.Bounds(), v.cell)
}

func (v *View) inCell(c cell.ID) {
	v.at = c
	v.occupancy.Holders(c, v.hold)
}

func (v *View) drawHold(e uid.UID64, _ cell.Domain, deck bool) {
	c := v.at
	v.outline = v.grid.CellOutline(c, v.outline[:0])
	mid := v.grid.CellCenter(c)
	inset, col := groundInset, holderColor(e)
	if deck {
		inset, col = deckInset, lighter(col)
	}
	z := float32(v.height(c, mid, deck))
	at := v.frame.Lifted(float32(mid.X), float32(mid.Y), z, holdLift)
	var x0, y0, xp, yp float32
	for i, p := range v.outline {
		p = geom.NewVec(p.X+(mid.X-p.X)*inset, p.Y+(mid.Y-p.Y)*inset)
		x, y := v.cam.Project(float32(p.X), float32(p.Y), float32(v.height(c, p, deck)))
		if i == 0 {
			x0, y0 = x, y
		} else {
			v.frame.Line(at, xp, yp, x, y, holdWidth, col)
		}
		xp, yp = x, y
	}
	if len(v.outline) > 1 {
		v.frame.Line(at, xp, yp, x0, y0, holdWidth, col)
	}
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
