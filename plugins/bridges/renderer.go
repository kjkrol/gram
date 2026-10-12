package bridges

import (
	"image/color"
	"math"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/render"
)

var (
	refusedFill = color.RGBA{R: 240, G: 200, B: 30, A: 90}
	refusedLine = color.RGBA{R: 240, G: 200, B: 30, A: 255}
	standsFill  = color.RGBA{R: 60, G: 200, B: 80, A: 90}
	standsLine  = color.RGBA{R: 60, G: 200, B: 80, A: 255}
)

// renderer draws the bridge in every player's hand as it would stand: its deck from head to head
// at the height it would run at, yellow where it may not stand, green where it may.
type renderer struct {
	p       *Plugin
	outline []geom.Vec
}

var _ render.Source = (*renderer)(nil)

func (r *renderer) Init(*goke.SysInit) {}

func (r *renderer) Compose(f *render.Frame, cam camera.Camera) {
	for _, h := range r.p.hands {
		if !h.marked {
			continue
		}
		g := r.p.ground(h.t)
		deck := float32(g.level(h.from))
		fill, line := standsFill, standsLine
		if !h.aimed || h.err != nil {
			fill, line = refusedFill, refusedLine
		}
		r.cell(f, cam, h.from, deck, line)
		if !h.aimed {
			continue
		}
		r.cell(f, cam, h.to, deck, line)
		a, b := r.p.board.Board().CellCenter(h.from), r.p.board.Board().CellCenter(h.to)
		d := geom.NewVec(b.X-a.X, b.Y-a.Y)
		length := math.Hypot(d.X, d.Y)
		if length == 0 {
			continue
		}
		half := h.t.spec.Width / 2
		n := geom.NewVec(-d.Y/length*half, d.X/length*half)
		var q render.Corners
		for i, p := range [4]geom.Vec{
			geom.NewVec(a.X-n.X, a.Y-n.Y), geom.NewVec(b.X-n.X, b.Y-n.Y),
			geom.NewVec(a.X+n.X, a.Y+n.Y), geom.NewVec(b.X+n.X, b.Y+n.Y),
		} {
			q[i][0], q[i][1] = cam.Project(float32(p.X), float32(p.Y), deck)
		}
		f.Quad(render.Marks, 0, q, [4]color.RGBA{fill, fill, fill, fill})
		for _, e := range [4][2]int{{0, 1}, {1, 3}, {3, 2}, {2, 0}} {
			f.Line(render.Marks, 0, q[e[0]][0], q[e[0]][1], q[e[1]][0], q[e[1]][1], 2, line)
		}
	}
}

// cell outlines c at the height deck.
func (r *renderer) cell(f *render.Frame, cam camera.Camera, c cell.ID, deck float32,
	line color.RGBA) {
	r.outline = r.p.board.Board().CellOutline(c, r.outline[:0])
	for i, p := range r.outline {
		n := r.outline[(i+1)%len(r.outline)]
		x0, y0 := cam.Project(float32(p.X), float32(p.Y), deck)
		x1, y1 := cam.Project(float32(n.X), float32(n.Y), deck)
		f.Line(render.Marks, 0, x0, y0, x1, y1, 2, line)
	}
}
