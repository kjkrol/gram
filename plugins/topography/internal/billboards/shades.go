package billboards

import (
	"math"

	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/atmosphere/sky"
	"github.com/kjkrol/gram/plugins/topography/internal/relief"
	"github.com/kjkrol/gram/render"
)

// shading lays the shadows of what stands over the ground the frame drew (shaders/shade.wgsl), a
// patch an instance.
var shading = render.NewMeshShaderWith("topography shades", render.Files(shaders, "shaders/shade.wgsl"), []render.Uniform{
	{Name: "Unproject", Size: 16}, {Name: "ViewSize", Size: 2}, {Name: "ViewAt", Size: 2},
}).Instanced(3)

// shades lays the shadows of what stands on a ground drawn on the GPU that lays none itself — the
// hex prisms — read from the frame's depth: each patch where the ground under it was drawn.
type shades struct {
	opts      render.DrawMeshOptions
	own       map[string][]float32
	instances []float32
}

// draw lays patches over the ground of relief r in the target through cam.
func (s *shades) draw(t render.Target, cam camera.Camera, r *relief.Relief, patches []sky.Patch) {
	if t.Screen == nil || t.Depth == nil || len(patches) == 0 {
		return
	}
	rays, ok := cam.(camera.Rays)
	if !ok {
		return
	}
	f, ok := rays.Rays()
	if !ok {
		return
	}
	w, h := cam.Viewport()
	tr, ok := camera.SceneTransform(f, w, h)
	if !ok {
		return
	}
	unproject, ok := tr.Unproject()
	if !ok {
		return
	}
	low, high := r.Extent()
	s.instances = s.instances[:0]
	for _, p := range patches {
		rect, ok := covers(tr, p, float32(low), float32(high)+shadeHeadroom, w, h)
		if !ok {
			continue
		}
		s.instances = append(s.instances, p.X, p.Y, p.UX, p.UY, p.Along, p.Wide, p.Fade, p.Veil, rect[0], rect[1], rect[2], rect[3])
	}
	if len(s.instances) == 0 {
		return
	}
	if s.own == nil {
		s.own, s.opts.Uniforms = map[string][]float32{}, map[string]any{}
	}
	at := t.Screen.Bounds().Min
	s.set("Unproject", unproject[:]...)
	s.set("ViewSize", w, h)
	s.set("ViewAt", float32(at.X), float32(at.Y))
	s.opts.Vertices, s.opts.ReadDepth, s.opts.Instances = 6, t.Depth, s.instances
	t.Screen.DrawMesh(nil, shading, &s.opts)
}

// shadeHeadroom is how far over the relief's highest ground, world units, a shadow may lie: on
// what stands there.
const shadeHeadroom = 64

// covers is the rectangle of the viewport w by h the patch p may cover through tr, its corners at
// every height from low to high, a little wider; the whole viewport where any lies behind the eye;
// false off the viewport.
func covers(tr camera.Transform, p sky.Patch, low, high, w, h float32) ([4]float32, bool) {
	x0, y0, x1, y1 := float32(math.Inf(1)), float32(math.Inf(1)), float32(math.Inf(-1)), float32(math.Inf(-1))
	for _, c := range [4][2]float32{{-p.Along, -p.Wide}, {p.Along, -p.Wide}, {-p.Along, p.Wide}, {p.Along, p.Wide}} {
		x, y := p.X+p.UX*c[0]-p.UY*c[1], p.Y+p.UY*c[0]+p.UX*c[1]
		for _, z := range [2]float32{low, high} {
			sx, sy, _, ok := tr.Apply(x, y, z, w, h)
			if !ok {
				return [4]float32{0, 0, w, h}, true
			}
			x0, y0, x1, y1 = min(x0, sx), min(y0, sy), max(x1, sx), max(y1, sy)
		}
	}
	r := [4]float32{max(x0-2, 0), max(y0-2, 0), min(x1+2, w), min(y1+2, h)}
	return r, r[0] < r[2] && r[1] < r[3]
}

// set hands the shader the uniform name as v, kept between frames and boxed once.
func (s *shades) set(name string, v ...float32) {
	u, ok := s.own[name]
	if !ok || len(u) != len(v) {
		u = make([]float32, len(v))
		s.own[name] = u
	}
	copy(u, v)
	s.opts.Uniforms[name] = u
}
