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
		{Name: "TubeCover", Size: 4},
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
// beginning and at its end
const (
	wallLeft = 1 << iota
	wallRight
	wallBegin
	wallEnd
	// a road's ribbon runs along its floor: the stretch draws no road of its own
	bare
)

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
	kept    [2]*render.Instances
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
		if r.kept[back] != nil {
			r.kept[back].Release()
		}
		r.kept[back] = render.NewInstances(r.bufs[back])
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
			for way := range g.Directions() {
				if !f.Links.Has(way) {
					continue
				}
				step := g.Step(way)
				l := f32.Hypot(step.X, step.Y)
				u := geom.NewVec(step.X/l, step.Y/l)
				end, far, mouth := geom.NewVec(m.X+step.X/2, m.Y+step.Y/2), z, true
				if n, ok := g.Toward(c, way); ok {
					nb := brd.Built(n)
					_, nl := brd.Map().Top(n)
					// it meets the floor it runs on to halfway, and at a mouth the ground
					// before it
					if nf := nb.Storey(f.Storey); nf.Sunk() {
						far, mouth = (z+nl+nf.Height)/2, false
					} else {
						far = nl
					}
				}
				// the lamps are counted along the stretch's line, from where it crosses the
				// world's corner
				from := -(end.X*u.X + end.Y*u.Y)
				walls := float32(wallLeft | wallRight)
				if f.Lanes > 0 {
					walls += bare
				}
				dst = append(dst, end.X, end.Y, far, f.Width/2, m.X, m.Y, z, f.Clear,
					walls, from, r.mitre(g, f, way), 0)
				if mouth {
					dst = frame(dst, end, u, far, (z-far)/(l/2), frameIn*l, f)
				}
			}
		}
	}
	return dst
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
	u, v := geom.NewVec(s.X/ls, s.Y/ls), geom.NewVec(o.X/lo, o.Y/lo)
	if along := u.X*v.X + u.Y*v.Y; along < 1-1e-3 {
		// the stretch is drawn coming in: its left is the normal of -u
		n := geom.NewVec(u.Y, -u.X)
		return f.Width / 2 * (n.X*v.X + n.Y*v.Y) / (1 - along)
	}
	return 0
}

// frame appends to dst the frame round the mouth of f at end, the tunnel running in against u,
// its floor rising climb a unit: a jamb either side of the tube and a lintel over it, standing
// out of the ground and running in as far as the tube shows through the ground cut over it,
// level, from the lowest of the floor there to over the highest of the roof. Each is two
// pieces: the face round the mouth, as deep as it is thick, and behind it the cover the earth
// lies on.
func frame(dst []float32, end, u geom.Vec, floor, climb, in float32, f cell.Floor) []float32 {
	thick := frameThick * f.Clear
	out := frameOut * f.Clear
	// off the tube's own faces, which lie in the same planes
	gap := thick / 16
	low := floor + min(climb*in, 0)
	top := floor + f.Clear + thick + frameOver*max(climb*in, 0)
	const all = wallLeft | wallRight | wallBegin | wallEnd
	deep := min(thick, in)
	for part, run := range [2][2]float32{{-out, deep}, {deep, in}} {
		a := geom.NewVec(end.X-u.X*run[0], end.Y-u.Y*run[0])
		b := geom.NewVec(end.X-u.X*run[1], end.Y-u.Y*run[1])
		kind := float32(part + 1)
		for _, side := range [2]float32{-1, 1} {
			off := side * ((f.Width+thick)/2 + gap)
			dst = append(dst, a.X-u.Y*off, a.Y+u.X*off, low, thick/2,
				b.X-u.Y*off, b.Y+u.X*off, low, top-low, all, 0, 0, kind)
		}
		dst = append(dst, a.X, a.Y, floor+f.Clear+gap, f.Width/2+thick+gap,
			b.X, b.Y, floor+f.Clear+gap, top-floor-f.Clear-gap, all, 0, 0, kind)
	}
	return dst
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
	if t.Screen == nil || t.Depth == nil || len(r.bufs[r.front]) == 0 || r.kept[r.front] == nil {
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
	var cover [4]float32
	if r.p != nil {
		cover = r.p.cover
	}
	r.set("TubeCover", cover[:]...)
	r.opts.Depth, r.opts.WriteDepth = t.Depth, true
	r.opts.Vertices, r.opts.Kept = tubeVertices, r.kept[r.front]
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
