package bridges

import (
	"image/color"
	"math"

	"github.com/kjkrol/aabbworld/geom"
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

// renderer draws the bridges in hand as published once a frame
type renderer struct {
	p       *Plugin
	outline []geom.Vec
	drawn   render.Buffered[handed]
}

// handed is what the renderer draws: every bridge in hand marked, in the world, its cells'
// outlines handed.points[from:from+n]
type handed struct {
	all    []inHand
	points []geom.Vec
}

// inHand is a bridge in hand at its deck's level: the cell it is marked from, the one aimed at
// (n 0 for none) and, spanned, the span between their middles
type inHand struct {
	deck       float32
	fill, line color.RGBA
	cells      [2]outlined
	span       [4]geom.Vec
	middle     geom.Vec
	spanned    bool
}

type outlined struct {
	middle  geom.Vec
	from, n int32
}

// how far over its deck's level a bridge in hand lies among what lies there: over the routes,
// under the figures
const handLift = 0.9

var _ render.Source = (*renderer)(nil)

// Publish takes every bridge in hand marked, for the frame drawn next.
func (r *renderer) Publish() {
	b := r.drawn.Back()
	b.all, b.points = b.all[:0], b.points[:0]
	brd := r.p.board.Read(0).Board()
	for _, h := range r.p.hands {
		if !h.marked {
			continue
		}
		e := inHand{deck: float32(r.p.ground(h.t).level(h.from)), fill: standsFill,
			line: standsLine}
		if !h.aimed || h.err != nil {
			e.fill, e.line = refusedFill, refusedLine
		}
		e.cells[0] = r.outlined(b, h.from)
		if h.aimed {
			e.cells[1] = r.outlined(b, h.to)
			a, c := brd.CellCenter(h.from), brd.CellCenter(h.to)
			d := geom.NewVec(c.X-a.X, c.Y-a.Y)
			if length := math.Hypot(d.X, d.Y); length > 0 {
				half := h.t.spec.Width / 2
				n := geom.NewVec(-d.Y/length*half, d.X/length*half)
				e.span = [4]geom.Vec{
					geom.NewVec(a.X-n.X, a.Y-n.Y), geom.NewVec(c.X-n.X, c.Y-n.Y),
					geom.NewVec(a.X+n.X, a.Y+n.Y), geom.NewVec(c.X+n.X, c.Y+n.Y),
				}
				e.middle, e.spanned = geom.NewVec((a.X+c.X)/2, (a.Y+c.Y)/2), true
			}
		}
		b.all = append(b.all, e)
	}
}

// Swap has what was published last drawn.
func (r *renderer) Swap() { r.drawn.Swap() }

func (r *renderer) outlined(b *handed, c cell.ID) outlined {
	brd := r.p.board.Read(0).Board()
	r.outline = brd.CellOutline(c, r.outline[:0])
	o := outlined{middle: brd.CellCenter(c), from: int32(len(b.points)), n: int32(len(r.outline))}
	b.points = append(b.points, r.outline...)
	return o
}

func (r *renderer) Compose(f *render.Frame, cam camera.Camera) {
	d := r.drawn.Front()
	for _, h := range d.all {
		for _, c := range h.cells {
			if c.n > 0 {
				r.cell(f, cam, d.points[c.from:c.from+c.n], c.middle, h.deck, h.line)
			}
		}
		if !h.spanned {
			continue
		}
		var q render.Corners
		for i, p := range h.span {
			q[i][0], q[i][1] = cam.Project(float32(p.X), float32(p.Y), h.deck)
		}
		at := f.Lifted(float32(h.middle.X), float32(h.middle.Y), h.deck, handLift)
		f.Quad(at, q, [4]color.RGBA{h.fill, h.fill, h.fill, h.fill})
		for _, e := range [4][2]int{{0, 1}, {1, 3}, {3, 2}, {2, 0}} {
			f.Line(at, q[e[0]][0], q[e[0]][1], q[e[1]][0], q[e[1]][1], 2, h.line)
		}
	}
}

func (r *renderer) cell(f *render.Frame, cam camera.Camera, outline []geom.Vec, mid geom.Vec,
	deck float32, line color.RGBA) {
	at := f.Lifted(float32(mid.X), float32(mid.Y), deck, handLift)
	for i, p := range outline {
		n := outline[(i+1)%len(outline)]
		x0, y0 := cam.Project(float32(p.X), float32(p.Y), deck)
		x1, y1 := cam.Project(float32(n.X), float32(n.Y), deck)
		f.Line(at, x0, y0, x1, y1, 2, line)
	}
}
