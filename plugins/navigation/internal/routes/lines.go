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

// Lines are stretches of route gathered as the frame is composed and drawn on the GPU over the
// ground the frame drew.
type Lines struct {
	// four vec4s a stretch: its ends, its rectangle of the viewport, its colour, its width
	stretches []float32
	from      frame     // the camera's frame the rectangles are in
	placed    []float32 // the stretches with their rectangles in another frame
	opts      render.DrawMeshOptions
	own       map[string][]float32
}

// frame is where a camera's pixels lie in its shown frame: the part of it drawn, at k of its
// pixels
type frame struct {
	k   float32
	win camera.Window
}

func frameOf(cam camera.Camera) frame {
	k := camera.ResolutionOf(cam)
	if c, ok := cam.(camera.Cropping); ok {
		return frame{k, c.Window()}
	}
	w, h := cam.Viewport()
	return frame{k, camera.Window{W: w / k, H: h / k}}
}

// Reset drops the stretches gathered, for a frame composed anew.
func (g *Lines) Reset() { g.stretches = g.stretches[:0] }

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
	g.from = frameOf(cam)
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
	alpha := float32(line.A) / 255
	g.stretches = append(g.stretches,
		float32(a.X), float32(a.Y), float32(b.X), float32(b.Y),
		rect[0], rect[1], rect[2], rect[3],
		float32(line.R)/255*alpha, float32(line.G)/255*alpha, float32(line.B)/255*alpha, alpha,
		width, 0, 0, 0)
}

// Len is how many stretches are gathered.
func (g *Lines) Len() int { return len(g.stretches) / 16 }

// Draw lays the gathered stretches over the ground into the target, each in its colour and
// width, through cam: the camera they were gathered through, or one drawing another part of
// its frame at another resolution, as a picture drawn in layers does.
func (g *Lines) Draw(t render.Target, cam camera.Camera) {
	if t.Screen == nil || t.Ground == nil || len(g.stretches) == 0 {
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
	g.opts.Depth, g.opts.ReadDepth, g.opts.Instances = t.Depth, t.Ground, g.placedIn(cam)
	t.Screen.DrawMesh(nil, lining, &g.opts)
}

// placedIn is the stretches with their rectangles in cam's pixels: the stretches themselves
// where cam draws the frame they were gathered in
func (g *Lines) placedIn(cam camera.Camera) []float32 {
	to := frameOf(cam)
	if to == g.from {
		return g.stretches
	}
	vw, vh := cam.Viewport()
	g.placed = append(g.placed[:0], g.stretches...)
	for i := 4; i < len(g.placed); i += 16 {
		r := g.placed[i : i+4 : i+4]
		r[0] = min(max((r[0]/g.from.k+g.from.win.X-to.win.X)*to.k, 0), vw)
		r[2] = min(max((r[2]/g.from.k+g.from.win.X-to.win.X)*to.k, 0), vw)
		r[1] = min(max((r[1]/g.from.k+g.from.win.Y-to.win.Y)*to.k, 0), vh)
		r[3] = min(max((r[3]/g.from.k+g.from.win.Y-to.win.Y)*to.k, 0), vh)
	}
	return g.placed
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
