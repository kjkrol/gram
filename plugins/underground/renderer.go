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

// the walls of a stretch standing, by bit: on its left and its right going along it, at its
// beginning and at its end
const (
	wallLeft = 1 << iota
	wallRight
	wallBegin
	wallEnd
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
			for way := range g.Directions() {
				if !f.Links.Has(way) {
					continue
				}
				step := g.Step(way)
				end, far := geom.NewVec(m.X+step.X/2, m.Y+step.Y/2), z
				if n, ok := g.Toward(c, way); ok {
					nb := brd.Built(n)
					if nf := nb.Storey(f.Storey); nf.Sunk() {
						_, nl := brd.Map().Top(n)
						far = (z + nl + nf.Height) / 2
					}
				}
				// the lamps are counted along the stretch's line, from where it crosses the
				// world's corner
				l := f32.Hypot(step.X, step.Y)
				from := -(end.X*step.X + end.Y*step.Y) / l
				dst = append(dst, end.X, end.Y, far, f.Width/2, m.X, m.Y, z, f.Clear,
					wallLeft|wallRight, from, 0, 0)
			}
		}
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
