package look

import (
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/render"
)

var shadeShader = render.NewMeshShaderWith(
	"board shade",
	render.Files(shaders, "shaders/shade.wgsl"),
	[]render.Uniform{
		{Name: "ViewAt", Size: 2},
		{Name: "RayAt", Size: 3},
		{Name: "RayDX", Size: 3},
		{Name: "RayDY", Size: 3},
		{Name: "RayDir", Size: 3},
		{Name: "RayDDX", Size: 3},
		{Name: "RayDDY", Size: 3},
		{Name: "Cells", Size: 2},
		{Name: "CellSize", Size: 1},
	},
)

// groundShade lights the still, drawn as it is, by the frame's light per pixel: the distant
// light less the shadows the frame holds, and the lamps', as each cell takes them
type groundShade struct {
	opts  render.DrawMeshOptions
	own   map[string]any
	cells *render.Image // a pixel a cell of a square board, red its Taking; nil for every bit
	at    uint64        // one more than the board's count of changes the cells hold; 0 none
	buf   []byte
}

func (g *groundShade) draw(t render.Target, cam camera.Camera, u render.Uniforms, brd Board) {
	rays, ok := cam.(camera.Rays)
	if !ok {
		return
	}
	f, ok := rays.Rays()
	if !ok {
		return
	}
	if g.own == nil {
		g.own, g.opts.Uniforms = map[string]any{}, map[string]any{}
		g.opts.Multiply, g.opts.Vertices = true, 3
	}
	u.Into(g.opts.Uniforms)
	at := t.Screen.Bounds().Min
	g.set("ViewAt", float32(at.X), float32(at.Y))
	g.set("RayAt", f.Origin[:]...)
	g.set("RayDX", f.DX[:]...)
	g.set("RayDY", f.DY[:]...)
	g.set("RayDir", f.Dir[:]...)
	g.set("RayDDX", f.DDX[:]...)
	g.set("RayDDY", f.DDY[:]...)
	g.refresh(brd)
	g.opts.Images[0] = g.cells
	if g.cells == nil {
		g.set("Cells", 0, 0)
	}
	t.Screen.DrawMesh(nil, shadeShader, &g.opts)
}

// the cells' Takings as they stand, of a square board alone
func (g *groundShade) refresh(brd Board) {
	shape, ok := brd.Shape()
	if !ok || shape.Hex {
		g.cells = nil
		return
	}
	if v := brd.Changes() + 1; g.at != v {
		g.at = v
		cols, rows := int(shape.Cols), int(shape.Rows)
		if g.cells == nil || g.cells.Bounds().Dx() != cols || g.cells.Bounds().Dy() != rows {
			g.cells = render.NewImage(cols, rows)
		}
		g.buf = g.buf[:0]
		for y := range uint32(rows) {
			for x := range uint32(cols) {
				g.buf = append(g.buf, byte(takesOf(brd.Kind(brd.At(x, y)).Takes)), 0, 0, 255)
			}
		}
		g.cells.WritePixels(g.buf)
	}
	g.set("Cells", float32(shape.Cols), float32(shape.Rows))
	g.set("CellSize", float32(shape.Cell))
}

// the shade a cell takes, every bit for a kind saying none, no bit for one that Shines
func takesOf(t render.Taking) render.Taking {
	switch {
	case t == 0:
		return render.TakesAll
	case t&render.Shines != 0:
		return 0
	}
	return t
}

// kept between frames and boxed once
func (g *groundShade) set(name string, v ...float32) {
	u, ok := g.own[name]
	if s, _ := u.([]float32); !ok || len(s) != len(v) {
		u = make([]float32, len(v))
		g.own[name] = u
	}
	copy(u.([]float32), v)
	g.opts.Uniforms[name] = u
}
