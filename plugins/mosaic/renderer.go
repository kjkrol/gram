package mosaic

import (
	"embed"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/render"
)

//go:embed shaders
var shaders embed.FS

var groundShader = render.NewMeshShaderWith("mosaic ground",
	render.Files(shaders, "shaders/ground.wgsl"),
	[]render.Uniform{
		{Name: "MosaicViewProj", Size: 16},
		{Name: "MosaicEye", Size: 3},
		{Name: "MosaicBend", Size: 1},
	}).Instanced(6)

// renderer draws the sectors' triangles, each sector's kept on the GPU till laid anew
type renderer struct {
	p     *Plugin
	shown map[int]*laid
	next  map[int]*laid
	// a batch's sectors laid, till the batch is whole
	staged map[int]*laid
	seen   []int
	opts   render.DrawMeshOptions
	own    map[string]any
	proj   []float32
	eye    []float32
	bend   []float32
}

// laid is a sector's triangles and what they cover: its cells reach past its rectangle
type laid struct {
	kept      *render.Instances
	box       geom.AABB
	low, high float32
}

func covering(triangles []float32) *laid {
	l := &laid{kept: render.NewInstances(triangles)}
	lo, hi := geom.NewVec(triangles[0], triangles[1]), geom.NewVec(triangles[0], triangles[1])
	l.low, l.high = triangles[2], triangles[2]
	for t := 0; t < len(triangles); t += triangleFloats {
		for v := t; v < t+12; v += 4 {
			x, y, z := triangles[v], triangles[v+1], triangles[v+2]
			lo, hi = geom.NewVec(min(lo.X, x), min(lo.Y, y)), geom.NewVec(max(hi.X, x), max(hi.Y, y))
			l.low, l.high = min(l.low, z), max(l.high, z)
		}
	}
	l.box = geom.NewAABB(lo, hi)
	return l
}

var (
	_ render.Direct    = (*renderer)(nil)
	_ plugin.Publisher = (*renderer)(nil)
)

// Publish follows the sectors' levels and lays the triangles of those that changed, shown
// together once all of a batch are laid.
func (r *renderer) Publish() {
	r.p.ground.step(r.p.worldPlugin.Read(0), func(i int, triangles []float32) {
		l := &laid{}
		if len(triangles) > 0 {
			l = covering(triangles)
		}
		r.staged[i] = l
	}, func() {
		for i, l := range r.staged {
			if old := r.next[i]; old != nil && old.kept != nil {
				old.kept.Release()
			}
			r.next[i] = l
			delete(r.staged, i)
		}
	})
}

// Swap has the triangles laid last drawn.
func (r *renderer) Swap() {
	for i, l := range r.next {
		if old := r.shown[i]; old != nil && old.kept != nil {
			old.kept.Release()
		}
		r.shown[i] = l
		delete(r.next, i)
	}
}

// Compose asks for the ground's pass while any sector is laid.
func (r *renderer) Compose(f *render.Frame, _ camera.Camera) {
	r.seen = r.seen[:0]
	for i, l := range r.shown {
		if l.kept != nil {
			r.seen = append(r.seen, i)
		}
	}
	if len(r.seen) > 0 {
		f.DrawOpaque()
	}
}

// Draw draws the sectors whose triangles cam shows into the target, under the frame's
// uniforms u.
func (r *renderer) Draw(t render.Target, cam camera.Camera, u render.Uniforms) {
	if t.Screen == nil || t.Depth == nil || len(r.seen) == 0 {
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
		r.proj, r.eye, r.bend = make([]float32, 16), make([]float32, 3), make([]float32, 1)
		r.own = map[string]any{"MosaicViewProj": r.proj, "MosaicEye": r.eye, "MosaicBend": r.bend}
		r.opts.Uniforms = map[string]any{}
	}
	u.Into(r.opts.Uniforms)
	copy(r.proj, tr.M[:])
	copy(r.eye, tr.Eye[:])
	r.bend[0] = tr.Bend
	for name, v := range r.own {
		r.opts.Uniforms[name] = v
	}
	hole, holed := camera.Inner(cam)
	r.opts.Depth, r.opts.WriteDepth, r.opts.Vertices = t.Depth, true, 3
	for _, i := range r.seen {
		l := r.shown[i]
		if !tr.Shows(l.box, 0, l.low-1, l.high+1, w, h, hole, holed) {
			continue
		}
		r.opts.Kept = l.kept
		t.Screen.DrawMesh(nil, groundShader, &r.opts)
	}
}
