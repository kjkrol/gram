package underground

import (
	"image/color"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

var (
	refusedFill = color.RGBA{R: 240, G: 200, B: 30, A: 90}
	refusedLine = color.RGBA{R: 240, G: 200, B: 30, A: 255}
	liesFill    = color.RGBA{R: 60, G: 200, B: 80, A: 90}
	liesLine    = color.RGBA{R: 60, G: 200, B: 80, A: 255}
	courseLine  = color.RGBA{R: 235, G: 225, B: 190, A: 170}
	unitMark    = color.RGBA{R: 250, G: 240, B: 200, A: 230}
)

// the marks' levels over the world: the courses under the routes, the units in them over those
const (
	courseLevel = 0.3
	unitLevel   = 0.4
	// how far over its floor's level a tunnel in hand lies among what lies there
	handLift = 0.9
)

// marker draws what of the structures a player sees from over the ground, as published once a
// frame: the tunnel in hand, and where the setting says the courses of what lies under the
// ground and the units in it, as marks over the world
type marker struct {
	p *Plugin

	units *goke.Query
	base  goke.Comp[world.Base]
	z     goke.Comp[world.Z]
	mover goke.Comp[unit.Mover]

	drawn   render.Buffered[marked]
	outline []geom.Vec
}

// marked is what the marker draws: the courses' stretches on the ground over them, the units
// under the ground, and every tunnel in hand
type marked struct {
	at      uint64 // one more than the board's changes the courses hold
	shown   bool
	courses []course
	units   []geom.Vec
	heights []float32 // the ground over each of units
	hands   []inHand
	points  []geom.Vec
}

// a stretch of a course: its ends on the ground over it
type course struct{ a, b [3]float32 }

// inHand is a tunnel in hand at its first head's level: the cell it is marked from, the one
// aimed at (n 0 for none), its outlines marked.points[from:from+n], and the strip between them
type inHand struct {
	level      float32
	fill, line color.RGBA
	cells      [2]outlined
	strip      [4]geom.Vec
	middle     geom.Vec
	aimed      bool
}

type outlined struct {
	middle  geom.Vec
	from, n int32
}

var (
	_ render.Source    = (*marker)(nil)
	_ plugin.Bound     = (*marker)(nil)
	_ plugin.Publisher = (*marker)(nil)
)

func (r *marker) Init(si *goke.SysInit) {
	r.units = si.NewQueryBuilder(&r.base, &r.z, &r.mover).Build()
}

// Publish takes the tunnels in hand, the courses as the board has them and the units under the
// ground, for the frame drawn next.
func (r *marker) Publish() {
	b := r.drawn.Back()
	brd := r.p.board.Read(0).Board()
	heights := r.p.board.Read(0).Heights()
	b.shown = !r.p.Res.Settings.HideCourses && heights != nil
	if v := brd.Changes() + 1; b.shown && b.at != v {
		b.at = v
		b.courses = b.courses[:0]
		r.p.tubes.Publish()
		tubes := r.p.tubes.bufs[1-r.p.tubes.front]
		for k := 0; k+tubeFloats <= len(tubes); k += tubeFloats {
			t := tubes[k : k+tubeFloats]
			if t[8] != wallLeft|wallRight || t[11] != 0 {
				continue // a chamber's cell, a mouth's frame
			}
			a, c := geom.NewVec(t[0], t[1]), geom.NewVec(t[4], t[5])
			b.courses = append(b.courses, course{
				a: [3]float32{a.X, a.Y, heights.At(a)},
				b: [3]float32{c.X, c.Y, heights.At(c)},
			})
		}
	}
	b.units, b.heights = b.units[:0], b.heights[:0]
	if b.shown && brd.Sunk() && r.units != nil {
		for r.units.All(); r.units.Next(); {
			cur := r.units.Cursor()
			bases, zs := r.base.Slice(cur), r.z.Slice(cur)
			for i := range bases {
				at := bases[i].Pos.Center()
				if _, on, ok := brd.LevelFor(at, zs[i].Altitude, zs[i].Reach()); !ok ||
					on == cell.OnGround {
					continue
				}
				if top := heights.At(at); zs[i].Altitude+zs[i].Height < top {
					b.units, b.heights = append(b.units, at), append(b.heights, top)
				}
			}
		}
	}
	b.hands, b.points = b.hands[:0], b.points[:0]
	for _, h := range r.p.hands {
		if !h.marked {
			continue
		}
		e := inHand{level: r.p.ground(h.t).level(h.from), fill: liesFill, line: liesLine}
		if !h.aimed || h.err != nil {
			e.fill, e.line = refusedFill, refusedLine
		}
		e.cells[0] = r.outlined(b, h.from)
		if h.aimed {
			e.cells[1] = r.outlined(b, h.to)
			a, c := brd.CellCenter(h.from), brd.CellCenter(h.to)
			d := geom.NewVec(c.X-a.X, c.Y-a.Y)
			if length := f32.Hypot(d.X, d.Y); length > 0 {
				half := h.t.spec.Width / 2
				n := geom.NewVec(-d.Y/length*half, d.X/length*half)
				e.strip = [4]geom.Vec{
					geom.NewVec(a.X-n.X, a.Y-n.Y), geom.NewVec(c.X-n.X, c.Y-n.Y),
					geom.NewVec(a.X+n.X, a.Y+n.Y), geom.NewVec(c.X+n.X, c.Y+n.Y),
				}
				e.middle, e.aimed = geom.NewVec((a.X+c.X)/2, (a.Y+c.Y)/2), true
			}
		}
		b.hands = append(b.hands, e)
	}
}

// Swap has what was published last drawn.
func (r *marker) Swap() { r.drawn.Swap() }

func (r *marker) outlined(b *marked, c cell.ID) outlined {
	brd := r.p.board.Read(0).Board()
	r.outline = brd.CellOutline(c, r.outline[:0])
	o := outlined{middle: brd.CellCenter(c), from: int32(len(b.points)), n: int32(len(r.outline))}
	b.points = append(b.points, r.outline...)
	return o
}

func (r *marker) Compose(f *render.Frame, cam camera.Camera) {
	d := r.drawn.Front()
	for _, h := range d.hands {
		for _, c := range h.cells {
			if c.n > 0 {
				r.cell(f, cam, d.points[c.from:c.from+c.n], c.middle, h.level, h.line)
			}
		}
		if !h.aimed {
			continue
		}
		var q render.Corners
		for i, p := range h.strip {
			q[i][0], q[i][1] = cam.Project(p.X, p.Y, h.level)
		}
		at := f.Lifted(h.middle.X, h.middle.Y, h.level, handLift)
		f.Quad(at, q, [4]color.RGBA{h.fill, h.fill, h.fill, h.fill})
	}
	// from under the ground its marks would lie over the tunnel one rides in
	if !d.shown || under(cam, d) {
		return
	}
	for _, c := range d.courses {
		if !cam.Visible(geom.NewAABB(
			geom.NewVec(min(c.a[0], c.b[0]), min(c.a[1], c.b[1])),
			geom.NewVec(max(c.a[0], c.b[0]), max(c.a[1], c.b[1])))) {
			continue
		}
		x0, y0 := cam.Project(c.a[0], c.a[1], c.a[2])
		x1, y1 := cam.Project(c.b[0], c.b[1], c.b[2])
		// broken: the middle half of each stretch
		f.Line(render.OnTop(courseLevel), x0+(x1-x0)*0.25, y0+(y1-y0)*0.25,
			x0+(x1-x0)*0.75, y0+(y1-y0)*0.75, 2, courseLine)
	}
	for i, u := range d.units {
		x, y := cam.Project(u.X, u.Y, d.heights[i])
		q := render.Corners{{x - 3, y - 3}, {x + 3, y - 3}, {x - 3, y + 3}, {x + 3, y + 3}}
		f.Quad(render.OnTop(unitLevel), q, [4]color.RGBA{unitMark, unitMark, unitMark, unitMark})
	}
}

// under says whether cam's eye lies under the ground: riding a unit in a tunnel
func under(cam camera.Camera, d *marked) bool {
	eyed, ok := cam.(interface {
		Eye() (x, y, z float32, ok bool)
	})
	if !ok {
		return false
	}
	x, y, z, ok := eyed.Eye()
	if !ok {
		return false
	}
	for i, u := range d.units {
		if f32.Hypot(u.X-x, u.Y-y) < 64 && z < d.heights[i] {
			return true
		}
	}
	return false
}

func (r *marker) cell(f *render.Frame, cam camera.Camera, outline []geom.Vec, mid geom.Vec,
	level float32, line color.RGBA) {
	at := f.Lifted(mid.X, mid.Y, level, handLift)
	for i, p := range outline {
		n := outline[(i+1)%len(outline)]
		x0, y0 := cam.Project(p.X, p.Y, level)
		x1, y1 := cam.Project(n.X, n.Y, level)
		f.Line(at, x0, y0, x1, y1, 2, line)
	}
}
