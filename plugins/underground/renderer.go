package underground

import (
	"embed"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/render"
)

//go:embed shaders
var shaders embed.FS

var tubeShader = render.NewMeshShaderWith("underground tubes",
	render.Files(shaders, "shaders/tube.wgsl"),
	[]render.Uniform{
		{Name: "TubeViewProj", Size: 16},
		{Name: "TubeEye", Size: 3},
		{Name: "TubeBend", Size: 1},
	}).Instanced(3)

// a stretch's floats and its vertices: a floor, a roof and four walls, two triangles each
const (
	tubeFloats   = 12
	tubeVertices = 36
)

// a mouth's frame by its tube's height: how thick it is and how far it stands out of the ground;
// and how far it runs in by its cell's span, past where the ground is cut away over the mouth
const (
	frameThick = 0.12
	frameOut   = 0.3
	frameIn    = 0.65
	// how much higher than a roof climbing in the frame stands: over the ground cut above it
	frameOver = 1.1
)

// the walls of a stretch standing, by bit: on its left and its right going along it, at its
// beginning and at its end; and whether it is a piece of the frame round a mouth
const (
	wallLeft = 1 << iota
	wallRight
	wallBegin
	wallEnd
	framed
)

// how many straight pieces a stretch turning in its cell is laid in
const bends = 4

// tubes draws what lies under the ground from inside, a stretch an instance, as published once
// a frame: every sunk floor of the board, whoever built it
type tubes struct {
	p *Plugin

	query *goke.Query
	floor goke.Comp[cell.Floor]
	on    goke.Comp[cell.On]

	// the stretches drawn and those written next, and one more than the board's changes each
	// holds
	bufs    [2][]float32
	ats     [2]uint64
	front   int
	opts    render.DrawMeshOptions
	own     map[string][]float32
	boxed   map[string]any
	outline []geom.Vec
}

var (
	_ render.Direct    = (*tubes)(nil)
	_ plugin.Bound     = (*tubes)(nil)
	_ plugin.Publisher = (*tubes)(nil)
)

func (r *tubes) Init(si *goke.SysInit) {
	r.query = si.NewQueryBuilder(&r.floor, &r.on).Build()
}

// Publish lays the stretches anew where the board's cells changed since the buffer was written.
func (r *tubes) Publish() {
	if r.query == nil {
		return
	}
	back := 1 - r.front
	if v := r.p.board.Read(0).Board().Changes() + 1; r.ats[back] != v {
		r.ats[back] = v
		r.bufs[back] = r.lay(r.bufs[back][:0])
	}
}

// Swap has what was published last drawn.
func (r *tubes) Swap() { r.front = 1 - r.front }

// lay appends to dst the stretches of every sunk floor: a band's from its cell's middle out to
// the edge each of its links crosses, a whole cell's across it
func (r *tubes) lay(dst []float32) []float32 {
	brd := r.p.board.Read(0).Board()
	g := brd.Grid
	for r.query.All(); r.query.Next(); {
		cur := r.query.Cursor()
		floors, ons := r.floor.Slice(cur), r.on.Slice(cur)
		for i, f := range floors {
			if !f.Sunk() {
				continue
			}
			c := ons[i].Cell
			m := g.CellCenter(c)
			_, level := brd.Map().Top(c)
			z := level + f.Height
			if f.Width <= 0 {
				dst = r.room(dst, c, f, m, z)
				continue
			}
			var legs []leg
			for way := range g.Directions() {
				if !f.Links.Has(way) {
					continue
				}
				step := g.Step(way)
				l := f32.Hypot(step.X, step.Y)
				lg := leg{way: way, u: geom.NewVec(step.X/l, step.Y/l), long: l, far: z,
					end: geom.NewVec(m.X+step.X/2, m.Y+step.Y/2), mouth: true}
				if n, ok := g.Toward(c, way); ok {
					nb := brd.Built(n)
					_, nl := brd.Map().Top(n)
					// it meets the floor it runs on to halfway, and at a mouth the ground
					// before it
					if nf := nb.Storey(f.Storey); nf.Sunk() {
						lg.far, lg.mouth = (z+nl+nf.Height)/2, false
					} else {
						lg.far = nl
					}
				}
				legs = append(legs, lg)
			}
			if len(legs) == 2 && legs[0].u.X*legs[1].u.X+legs[0].u.Y*legs[1].u.Y > -1+1e-3 {
				dst = bent(dst, legs[0], legs[1], m, z, f)
			} else {
				for _, lg := range legs {
					// the lamps are counted along the stretch's line, from where it crosses
					// the world's corner
					from := -(lg.end.X*lg.u.X + lg.end.Y*lg.u.Y)
					dst = append(dst, lg.end.X, lg.end.Y, lg.far, f.Width/2, m.X, m.Y, z, f.Clear,
						wallLeft|wallRight, from, r.mitre(g, f, lg.way), 0)
				}
			}
			for _, lg := range legs {
				if lg.mouth {
					dst = frame(dst, lg.end, lg.u, lg.far, (z-lg.far)/(lg.long/2),
						frameIn*lg.long, f)
				}
			}
		}
	}
	return dst
}

// leg is a way a floor runs out of its cell: its direction out of the middle and the step's
// length, the edge it crosses there and its floor's height at it, and whether it comes out of
// the ground there
type leg struct {
	way   int
	u     geom.Vec
	long  float32
	end   geom.Vec
	far   float32
	mouth bool
}

// bent appends to dst the stretch of f turning in its cell, from a's edge to b's through its
// middle m at z, in bends pieces along a curve: each meeting the next on the plane between
// them, and the first and the last cut square to the way out of the cell, as the stretches
// beyond them are
func bent(dst []float32, a, b leg, m geom.Vec, z float32, f cell.Floor) []float32 {
	var q [bends + 1]geom.Vec
	var h [bends + 1]float32
	for k := range bends + 1 {
		t := float32(k) / bends
		q[k] = geom.NewVec((1-t)*(1-t)*a.end.X+2*t*(1-t)*m.X+t*t*b.end.X,
			(1-t)*(1-t)*a.end.Y+2*t*(1-t)*m.Y+t*t*b.end.Y)
		h[k] = a.far + (z-a.far)*2*t
		if t > 0.5 {
			h[k] = z + (b.far-z)*(2*t-1)
		}
	}
	half := f.Width / 2
	toward := func(from, to geom.Vec) geom.Vec {
		d := geom.NewVec(to.X-from.X, to.Y-from.Y)
		l := max(f32.Hypot(d.X, d.Y), f32.Eps(1))
		return geom.NewVec(d.X/l, d.Y/l)
	}
	dot := func(u, v geom.Vec) float32 { return u.X*v.X + u.Y*v.Y }
	in := geom.NewVec(-a.u.X, -a.u.Y)
	// the lamps counted on from the stretch beyond a's edge
	from := dot(a.end, in)
	for k := range bends {
		d := toward(q[k], q[k+1])
		left := geom.NewVec(-d.Y, d.X)
		var begin, end float32
		if k == 0 {
			begin = -half * dot(left, in) / dot(d, in)
		} else {
			begin = -joint(d, toward(q[k], q[k-1]), half)
		}
		if k == bends-1 {
			end = half * dot(left, b.u) / dot(d, b.u)
		} else {
			end = joint(geom.NewVec(-d.X, -d.Y), toward(q[k+1], q[k+2]), half)
		}
		dst = append(dst, q[k].X, q[k].Y, h[k], half, q[k+1].X, q[k+1].Y, h[k+1], f.Clear,
			wallLeft|wallRight, from, end, begin)
		from += f32.Hypot(q[k+1].X-q[k].X, q[k+1].Y-q[k].Y)
	}
	return dst
}

// joint is how far short of the point two stretches meet at the left wall of the one going
// out along u ends, and its right wall past it, for it to meet the one going out along v on
// the plane between them, half wide; 0 where they run straight on
func joint(u, v geom.Vec, half float32) float32 {
	if along := u.X*v.X + u.Y*v.Y; along < 1-1e-3 {
		// the stretch is drawn coming in: its left is the normal of -u
		n := geom.NewVec(u.Y, -u.X)
		return half * (n.X*v.X + n.Y*v.Y) / (1 - along)
	}
	return 0
}

// mitre is how far short of its cell's middle the left wall of the stretch of f coming in
// along way ends, and its right wall past it, for the stretch to meet the one f turns into on
// the plane between them; 0 where f runs straight on
func (r *tubes) mitre(g grid.Grid, f cell.Floor, way int) float32 {
	other := -1
	for i := range g.Directions() {
		if i != way && f.Links.Has(i) {
			if other >= 0 {
				return 0 // a fork: its stretches meet as they come
			}
			other = i
		}
	}
	if other < 0 {
		return 0
	}
	s, o := g.Step(way), g.Step(other)
	ls, lo := f32.Hypot(s.X, s.Y), f32.Hypot(o.X, o.Y)
	// out of the middle along the stretch, and along the one it turns into
	return joint(geom.NewVec(s.X/ls, s.Y/ls), geom.NewVec(o.X/lo, o.Y/lo), f.Width/2)
}

// frame appends to dst the frame round the mouth of f at end, the tunnel running in against u,
// its floor rising climb a unit: a jamb either side of the tube and a lintel over it, standing
// out of the ground and running in as far as the tube shows through the ground cut over it,
// level, from the lowest of the floor there to over the highest of the roof
func frame(dst []float32, end, u geom.Vec, floor, climb, in float32, f cell.Floor) []float32 {
	thick := frameThick * f.Clear
	out := frameOut * f.Clear
	// off the tube's own faces, which lie in the same planes
	gap := thick / 16
	a, b := geom.NewVec(end.X+u.X*out, end.Y+u.Y*out), geom.NewVec(end.X-u.X*in, end.Y-u.Y*in)
	low := floor + min(climb*in, 0)
	top := floor + f.Clear + thick + frameOver*max(climb*in, 0)
	const faces = wallLeft | wallRight | wallBegin | wallEnd | framed
	for _, side := range [2]float32{-1, 1} {
		off := side * ((f.Width+thick)/2 + gap)
		dst = append(dst, a.X-u.Y*off, a.Y+u.X*off, low, thick/2,
			b.X-u.Y*off, b.Y+u.X*off, low, top-low, faces, 0, 0, 0)
	}
	return append(dst, a.X, a.Y, floor+f.Clear+gap, f.Width/2+thick+gap,
		b.X, b.Y, floor+f.Clear+gap, top-floor-f.Clear-gap, faces, 0, 0, 0)
}

// room appends to dst a whole cell's stretch, from its west edge to its east, a wall on every
// side no floor of its storey lies beyond
func (r *tubes) room(dst []float32, c cell.ID, f cell.Floor, m geom.Vec, z float32) []float32 {
	brd := r.p.board.Read(0).Board()
	g := brd.Grid
	half := g.CellSpan() / 2
	walls := float32(0)
	for way, wall := range map[[2]int]float32{
		{0, -1}: wallRight, {0, 1}: wallLeft, {-1, 0}: wallBegin, {1, 0}: wallEnd,
	} {
		open := false
		if d, ok := grid.Direction(g, way[0], way[1]); ok {
			if n, ok := g.Toward(c, d); ok {
				nb := brd.Built(n)
				open = nb.Storey(f.Storey).Sunk()
			}
		}
		if !open {
			walls += wall
		}
	}
	return append(dst, m.X-half, m.Y, z, half, m.X+half, m.Y, z, f.Clear, walls, m.X-half, 0, 0)
}

func (r *tubes) Compose(f *render.Frame, _ camera.Camera) {
	if len(r.bufs[r.front]) > 0 {
		f.DrawOpaque()
	}
}

// Draw draws the stretches into the target through cam under the frame's uniforms u.
func (r *tubes) Draw(t render.Target, cam camera.Camera, u render.Uniforms) {
	instances := r.bufs[r.front]
	if t.Screen == nil || t.Depth == nil || len(instances) == 0 {
		return
	}
	rays, ok := cam.(camera.Rays)
	if !ok {
		return
	}
	field, ok := rays.Rays()
	if !ok {
		return
	}
	w, h := cam.Viewport()
	tr, ok := camera.SceneTransform(field, w, h)
	if !ok {
		return
	}
	if r.own == nil {
		r.own, r.boxed, r.opts.Uniforms = map[string][]float32{}, map[string]any{}, map[string]any{}
	}
	u.Into(r.opts.Uniforms)
	r.set("TubeViewProj", tr.M[:]...)
	r.set("TubeEye", tr.Eye[:]...)
	r.set("TubeBend", tr.Bend)
	r.opts.Depth, r.opts.WriteDepth = t.Depth, true
	r.opts.Vertices, r.opts.Instances = tubeVertices, instances
	t.Screen.DrawMesh(nil, tubeShader, &r.opts)
}

func (r *tubes) set(name string, v ...float32) {
	u, ok := r.own[name]
	if !ok || len(u) != len(v) {
		u = make([]float32, len(v))
		r.own[name] = u
		r.boxed[name] = u
	}
	copy(u, v)
	r.opts.Uniforms[name] = r.boxed[name]
}
