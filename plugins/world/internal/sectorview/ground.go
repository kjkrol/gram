package sectorview

import (
	"embed"
	"image/color"

	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/render"
)

//go:embed shaders/*.wgsl
var shaders embed.FS

var laying = render.NewMeshShaderWith("world sectors",
	render.Files(shaders, "shaders/sectors.wgsl"),
	[]render.Uniform{
		{Name: "Unproject", Size: 16}, {Name: "ViewSize", Size: 2}, {Name: "ViewAt", Size: 2},
		{Name: "Sectors", Size: 4}, {Name: "World", Size: 4}, {Name: "Line", Size: 4},
	})

// Grid is the world's sectors as Ground lays them: side a side in world units, Cols across and
// Rows down, over a world Width x Height wrapping as WrapX and WrapY say.
type Grid struct {
	Side          float32
	Cols, Rows    int
	Width, Height float32
	WrapX, WrapY  bool
}

// Ground lays the sectors over the ground a frame drew, each in its colour, its edge stronger.
type Ground struct {
	grid Grid
	// the colours, a texel a sector, by camera: each composes and draws in turn
	tints map[camera.Camera]*tints
	opts  render.DrawMeshOptions
	own   map[string][]float32
}

type tints struct {
	img     *render.Image
	pix     []byte
	written bool
}

// New is a Ground over grid.
func New(grid Grid) *Ground {
	g := &Ground{grid: grid, tints: map[camera.Camera]*tints{}, own: map[string][]float32{}}
	g.opts = render.DrawMeshOptions{Vertices: 3, Uniforms: map[string]any{}}
	return g
}

// Paint has cam draw sector i in colors[i], opaque; one transparent is not drawn.
func (g *Ground) Paint(cam camera.Camera, colors []color.RGBA) {
	t := g.tints[cam]
	if t == nil {
		t = &tints{img: render.NewImage(g.grid.Cols, g.grid.Rows),
			pix: make([]byte, 4*g.grid.Cols*g.grid.Rows)}
		g.tints[cam] = t
	}
	changed := !t.written
	for i, c := range colors {
		px := t.pix[4*i : 4*i+4]
		if px[0] != c.R || px[1] != c.G || px[2] != c.B || px[3] != c.A {
			px[0], px[1], px[2], px[3] = c.R, c.G, c.B, c.A
			changed = true
		}
	}
	if changed {
		t.img.WritePixels(t.pix)
		t.written = true
	}
}

// Line is how a sector is laid: its edge Width pixels wide at Edge of its colour, its inside at
// Inside.
type Line struct{ Width, Inside, Edge float32 }

// Draw lays the sectors cam was painted with over the ground of t as line says.
func (g *Ground) Draw(t render.Target, cam camera.Camera, line Line) {
	tn := g.tints[cam]
	if t.Screen == nil || t.Ground == nil || tn == nil {
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
	gr := g.grid
	g.set("Unproject", unproject[:]...)
	g.set("ViewSize", vw, vh)
	g.set("ViewAt", float32(at.X), float32(at.Y))
	g.set("Sectors", gr.Side, float32(gr.Cols), float32(gr.Rows), 0)
	g.set("World", gr.Width, gr.Height, flag(gr.WrapX), flag(gr.WrapY))
	g.set("Line", line.Width, line.Inside, line.Edge, 0)
	g.opts.Depth = t.Depth
	g.opts.ReadDepth = t.Ground
	g.opts.Images = [4]*render.Image{tn.img, nil, nil, nil}
	t.Screen.DrawMesh(nil, laying, &g.opts)
}

func flag(b bool) float32 {
	if b {
		return 1
	}
	return 0
}

// the uniform's slice is kept between frames, boxed once
func (g *Ground) set(name string, v ...float32) {
	s, ok := g.own[name]
	if !ok || len(s) != len(v) {
		s = make([]float32, len(v))
		g.own[name] = s
		g.opts.Uniforms[name] = s
	}
	copy(s, v)
}
