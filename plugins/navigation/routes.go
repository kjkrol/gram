package navigation

import (
	"embed"
	"image/color"
	"math"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/render"
)

//go:embed shaders/*.wgsl
var shaders embed.FS

// lining lays the routes over the ground (shaders/route.wgsl), a stretch an instance.
var lining = render.NewMeshShaderWith("navigation routes", render.Files(shaders, "shaders/route.wgsl"), []render.Uniform{
	{Name: "Unproject", Size: 16}, {Name: "ViewSize", Size: 2}, {Name: "ViewAt", Size: 2},
	{Name: "LineColor", Size: 4}, {Name: "LineWidth", Size: 1},
}).Instanced(2)

// RouteTier puts the routes drawn on the GPU over the ground and under what stands on it, read
// from the depth the ground alone left.
const RouteTier = render.Ground + 60

// routes are the stretches of the routes of a world with heights drawn on the GPU: gathered as the
// frame is composed, each with the part of the viewport it may cover, and laid over the ground the
// frame's meshes drew, read from their depth.
type routes struct {
	stretches []float32 // two vec4s a stretch: its ends, and its rectangle of the viewport
	opts      render.DrawMeshOptions
	own       map[string][]float32
}

func newRoutes() *routes {
	g := &routes{own: map[string][]float32{}}
	g.opts = render.DrawMeshOptions{Vertices: 6, Uniforms: map[string]any{}}
	return g
}

// add gathers the stretch from a to b over the ground through cam, the ground groundAt gives
// sampled every step to find the part of the viewport it may cover — all of it where any of the
// stretch lies behind the eye — a little wider than width pixels.
func (g *routes) add(cam camera.Camera, a, b geom.Vec, groundAt func(geom.Vec) float32, step float64, width float32) {
	w, h := cam.Viewport()
	x0, y0, x1, y1 := float32(math.Inf(1)), float32(math.Inf(1)), float32(math.Inf(-1)), float32(math.Inf(-1))
	pieces := 1
	if length := math.Hypot(b.X-a.X, b.Y-a.Y); step > 0 && length > step {
		pieces = int(math.Ceil(length / step))
	}
	behind := false
	for i := 0; i <= pieces; i++ {
		t := float64(i) / float64(pieces)
		p := geom.NewVec(a.X+(b.X-a.X)*t, a.Y+(b.Y-a.Y)*t)
		z := groundAt(p)
		if camera.ScaleAt(cam, float32(p.X), float32(p.Y), z) == 0 {
			behind = true
			break
		}
		sx, sy := cam.Project(float32(p.X), float32(p.Y), z)
		x0, y0, x1, y1 = min(x0, sx), min(y0, sy), max(x1, sx), max(y1, sy)
	}
	pad := width + 4
	rect := [4]float32{max(x0-pad, 0), max(y0-pad, 0), min(x1+pad, w), min(y1+pad, h)}
	if behind {
		rect = [4]float32{0, 0, w, h}
	}
	if rect[0] >= rect[2] || rect[1] >= rect[3] {
		return // off the viewport
	}
	g.stretches = append(g.stretches, float32(a.X), float32(a.Y), float32(b.X), float32(b.Y), rect[0], rect[1], rect[2], rect[3])
}

// draw lays the gathered stretches over the ground into the target through cam, in line and width
// pixels wide, and starts gathering anew.
func (g *routes) draw(t render.Target, cam camera.Camera, line color.RGBA, width float32) {
	defer func() { g.stretches = g.stretches[:0] }()
	if t.Screen == nil || t.Depth == nil || len(g.stretches) == 0 {
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
	vw, vh := cam.Viewport()
	tr, ok := camera.SceneTransform(f, vw, vh)
	if !ok {
		return
	}
	unproject, ok := tr.Unproject()
	if !ok {
		return
	}
	at := t.Screen.Bounds().Min
	c := [4]float32{float32(line.R) / 255, float32(line.G) / 255, float32(line.B) / 255, float32(line.A) / 255}
	g.set("Unproject", unproject[:]...)
	g.set("ViewSize", vw, vh)
	g.set("ViewAt", float32(at.X), float32(at.Y))
	g.set("LineColor", c[:]...)
	g.set("LineWidth", width)
	g.opts.ReadDepth, g.opts.Instances = t.Depth, g.stretches
	t.Screen.DrawMesh(nil, lining, &g.opts)
}

// set hands the shader the uniform name as v, kept between frames and boxed once.
func (g *routes) set(name string, v ...float32) {
	s, ok := g.own[name]
	if !ok || len(s) != len(v) {
		s = make([]float32, len(v))
		g.own[name] = s
		g.opts.Uniforms[name] = s
	}
	copy(s, v)
}
