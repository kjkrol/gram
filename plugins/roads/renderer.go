package roads

import (
	"image/color"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/roads/site"
	"github.com/kjkrol/gram/render"
)

var (
	refusedFill = color.RGBA{R: 240, G: 200, B: 30, A: 90}
	refusedLine = color.RGBA{R: 240, G: 200, B: 30, A: 255}
	liesFill    = color.RGBA{R: 60, G: 200, B: 80, A: 90}
	liesLine    = color.RGBA{R: 60, G: 200, B: 80, A: 255}
	gradedLine  = color.RGBA{R: 200, G: 160, B: 60, A: 255}
	blockedLine = color.RGBA{R: 230, G: 70, B: 50, A: 255}
)

// how far over the ground a road in hand lies among what lies there: its band over the routes
// as a bridge in hand, its outlines and points over the band, all under the figures
const (
	handLift = 0.9
	lineLift = 0.92
	markLift = 0.95
)

// renderer draws the roads in hand and those waiting for a step to be built, as published once
// a frame
type renderer struct {
	p       *Plugin
	outline []geom.Vec
	heights []float32
	moves   map[corner]float32
	drawn   render.Buffered[handed]
}

// handed is what the renderer draws, in the world: the roads' bands, the points marked and the
// outlines of the cells levelled
type handed struct {
	bands, marks []quad
	edges        []edge
}

// quad is a piece of a band or a point, its corners in Frame.Quad's order at their heights,
// lying among what lies at top
type quad struct {
	at     [4]geom.Vec
	z      [4]float32
	middle geom.Vec
	top    float32
	fill   color.RGBA
}

type edge struct {
	from, to geom.Vec
	zf, zt   float32
	line     color.RGBA
}

var (
	_ render.Source    = (*renderer)(nil)
	_ plugin.Publisher = (*renderer)(nil)
)

// Publish takes every road in hand and every road waiting to be built, for the frame drawn next.
func (r *renderer) Publish() {
	b := r.drawn.Back()
	b.bands, b.marks, b.edges = b.bands[:0], b.marks[:0], b.edges[:0]
	for _, h := range r.p.hands {
		r.hand(b, h)
	}
	for _, o := range r.p.Res.Orders {
		r.moves = r.p.movesOf(o.Estimate, r.moves)
		r.road(b, o.Estimate, r.moves, liesFill)
	}
}

// Swap has what was published last drawn.
func (r *renderer) Swap() { r.drawn.Swap() }

func (r *renderer) Compose(f *render.Frame, cam camera.Camera) {
	d := r.drawn.Front()
	for _, q := range d.bands {
		put(f, cam, q, handLift)
	}
	for _, e := range d.edges {
		x0, y0 := cam.Project(e.from.X, e.from.Y, e.zf)
		x1, y1 := cam.Project(e.to.X, e.to.Y, e.zt)
		at := f.Lifted((e.from.X+e.to.X)/2, (e.from.Y+e.to.Y)/2, max(e.zf, e.zt), lineLift)
		f.Edge(at, x0, y0, e.zf+lineLift, x1, y1, e.zt+lineLift, 2, e.line)
	}
	for _, q := range d.marks {
		put(f, cam, q, markLift)
	}
}

func put(f *render.Frame, cam camera.Camera, q quad, lift float32) {
	var s render.Corners
	for i, at := range q.at {
		s[i][0], s[i][1] = cam.Project(at.X, at.Y, q.z[i])
	}
	f.Quad(f.Lifted(q.middle.X, q.middle.Y, q.top, lift), s,
		[4]color.RGBA{q.fill, q.fill, q.fill, q.fill})
}

func (r *renderer) hand(b *handed, h *hand) {
	fill, line := liesFill, liesLine
	if h.err != nil || h.refused != "" {
		fill, line = refusedFill, refusedLine
	}
	switch {
	case h.planned:
		r.road(b, h.plan, h.moves, fill)
		for _, c := range h.plan.Graded {
			r.outlined(b, c, h.moves, gradedLine)
		}
		for _, c := range h.plan.Blocked {
			r.outlined(b, c, h.moves, blockedLine)
		}
	case len(h.cells) > 1:
		r.middles(b, h.cells, nil, fill)
	}
	if h.aimed && !h.planned {
		r.outlined(b, h.to, nil, refusedLine)
	}
	if h.piece {
		r.mark(b, h.at, h.moves, line)
	}
	for _, c := range h.points {
		r.mark(b, c, h.moves, line)
	}
}

// road lays e's band out of every cell's middle along its links, or from middle to middle where
// e has no links
func (r *renderer) road(b *handed, e site.Estimate, moves map[corner]float32, fill color.RGBA) {
	if len(e.Links) != len(e.Cells) {
		r.middles(b, e.Cells, moves, fill)
		return
	}
	g := r.p.board.Read(0).Board().Grid
	half := r.p.width() / 2
	for k, c := range e.Cells {
		links := e.Links[k]
		if links == 0 {
			continue
		}
		middle, z := r.level(c, moves)
		for i := range g.Directions() {
			if !links.Has(i) {
				continue
			}
			s := g.Step(i)
			exit := geom.NewVec(middle.X+s.X/2, middle.Y+s.Y/2)
			r.band(b, middle, exit, z, r.rim(exit, g.Corner(i)), half, fill)
		}
	}
}

func (r *renderer) middles(b *handed, cells []cell.ID, moves map[corner]float32,
	fill color.RGBA) {
	half := r.p.width() / 2
	var from geom.Vec
	var zf float32
	for k, c := range cells {
		to, zt := r.level(c, moves)
		if k > 0 {
			r.band(b, from, to, zf, zt, half, fill)
		}
		from, zf = to, zt
	}
}

func (r *renderer) band(b *handed, from, to geom.Vec, zf, zt, half float32, fill color.RGBA) {
	d := geom.NewVec(to.X-from.X, to.Y-from.Y)
	length := f32.Hypot(d.X, d.Y)
	if length == 0 {
		return
	}
	n := geom.NewVec(-d.Y/length*half, d.X/length*half)
	b.bands = append(b.bands, quad{
		at: [4]geom.Vec{
			geom.NewVec(from.X-n.X, from.Y-n.Y),
			geom.NewVec(to.X-n.X, to.Y-n.Y),
			geom.NewVec(from.X+n.X, from.Y+n.Y),
			geom.NewVec(to.X+n.X, to.Y+n.Y),
		},
		z:      [4]float32{zf, zt, zf, zt},
		middle: geom.NewVec((from.X+to.X)/2, (from.Y+to.Y)/2),
		top:    max(zf, zt),
		fill:   fill,
	})
}

func (r *renderer) outlined(b *handed, c cell.ID, moves map[corner]float32, line color.RGBA) {
	r.level(c, moves)
	for k, from := range r.outline {
		n := (k + 1) % len(r.outline)
		b.edges = append(b.edges, edge{
			from: from,
			to:   r.outline[n],
			zf:   r.heights[k],
			zt:   r.heights[n],
			line: line,
		})
	}
}

// mark lays a point of the road at c's middle, a tenth of a cell either side of it
func (r *renderer) mark(b *handed, c cell.ID, moves map[corner]float32, line color.RGBA) {
	m, z := r.level(c, moves)
	s := r.p.board.Read(0).Board().CellSpan() / 10
	b.marks = append(b.marks, quad{
		at: [4]geom.Vec{
			geom.NewVec(m.X-s, m.Y-s),
			geom.NewVec(m.X+s, m.Y-s),
			geom.NewVec(m.X-s, m.Y+s),
			geom.NewVec(m.X+s, m.Y+s),
		},
		z:      [4]float32{z, z, z, z},
		middle: m,
		top:    z,
		fill:   line,
	})
}

// level has the renderer hold c's outline and the heights its corners lie at once moves are
// made: c's middle and the height there, the mean of its corners'
func (r *renderer) level(c cell.ID, moves map[corner]float32) (geom.Vec, float32) {
	r.outline, r.heights = r.p.heightsOf(c, moves, r.outline, r.heights)
	sum := float32(0)
	for _, h := range r.heights {
		sum += h
	}
	return r.p.board.Read(0).Board().CellCenter(c), sum / float32(len(r.heights))
}

// rim is the height at p, where a way leaves the cell last levelled: the corner nearest it, or
// the mean of the two nearest, the ends of the edge it crosses
func (r *renderer) rim(p geom.Vec, corner bool) float32 {
	a, b := -1, -1
	var da, db float32
	for k, at := range r.outline {
		d := f32.Hypot(at.X-p.X, at.Y-p.Y)
		switch {
		case a < 0 || d < da:
			a, b, da, db = k, a, d, da
		case b < 0 || d < db:
			b, db = k, d
		}
	}
	if corner || b < 0 {
		return r.heights[a]
	}
	return (r.heights[a] + r.heights[b]) / 2
}
