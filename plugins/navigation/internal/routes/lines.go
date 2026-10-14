package routes

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

// a stretch an instance
var lining = render.NewMeshShaderWith("navigation routes", render.Files(shaders, "shaders/route.wgsl"), []render.Uniform{
	{Name: "Unproject", Size: 16}, {Name: "ViewSize", Size: 2}, {Name: "ViewAt", Size: 2},
}).Instanced(4)

// Tier is where the routes come: over the ground, under what stands on it.
const Tier = render.Ground + 60

// Lines are stretches of route gathered as the frame is composed and drawn on the GPU over the
// ground the frame drew.
type Lines struct {
	// four vec4s a stretch: its ends, its rectangle of the viewport, its colour, its width
	stretches []float32
	opts      render.DrawMeshOptions
	own       map[string][]float32
}

// New is a Lines with no stretches yet.
func New() *Lines {
	g := &Lines{own: map[string][]float32{}}
	g.opts = render.DrawMeshOptions{Vertices: 6, Uniforms: map[string]any{}}
	return g
}

// Add gathers the stretch from a to b over the ground groundAt gives, sampled every step; width
// is the line's, in pixels, line its colour.
func (g *Lines) Add(
	cam camera.Camera, a, b geom.Vec, groundAt func(geom.Vec) float32, step float64,
	width float32, line color.RGBA,
) {
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
		return
	}
	g.stretches = append(g.stretches,
		float32(a.X), float32(a.Y), float32(b.X), float32(b.Y),
		rect[0], rect[1], rect[2], rect[3],
		float32(line.R)/255, float32(line.G)/255, float32(line.B)/255, float32(line.A)/255,
		width, 0, 0, 0)
}

// Draw lays the gathered stretches over the ground into the target, each in its colour and
// width, and starts gathering anew.
func (g *Lines) Draw(t render.Target, cam camera.Camera) {
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
	g.set("Unproject", unproject[:]...)
	g.set("ViewSize", vw, vh)
	g.set("ViewAt", float32(at.X), float32(at.Y))
	g.opts.ReadDepth, g.opts.Instances = t.Depth, g.stretches
	t.Screen.DrawMesh(nil, lining, &g.opts)
}

// the uniform's slice is kept between frames, boxed once
func (g *Lines) set(name string, v ...float32) {
	s, ok := g.own[name]
	if !ok || len(s) != len(v) {
		s = make([]float32, len(v))
		g.own[name] = s
		g.opts.Uniforms[name] = s
	}
	copy(s, v)
}
