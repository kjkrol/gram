package look

import (
	"embed"
	"image/color"

	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/render"
)

//go:embed shaders/*.wgsl
var shaders embed.FS

var gridShader = render.NewMeshShaderWith("board grid", render.Files(shaders, "shaders/grid.wgsl"), []render.Uniform{
	{Name: "ViewAt", Size: 2}, {Name: "RayAt", Size: 3}, {Name: "RayDX", Size: 3}, {Name: "RayDY", Size: 3},
	{Name: "RayDir", Size: 3}, {Name: "RayDDX", Size: 3}, {Name: "RayDDY", Size: 3},
	{Name: "GridHex", Size: 1}, {Name: "GridCell", Size: 1}, {Name: "GridCells", Size: 2}, {Name: "GridWrap", Size: 2}, {Name: "GridLine", Size: 4},
})

type gridLines struct {
	opts render.DrawMeshOptions
	own  map[string][]float32
}

func (g *gridLines) draw(t render.Target, cam camera.Camera, u render.Uniforms, brd Board, wrapX, wrapY bool) {
	rays, ok := cam.(camera.Rays)
	if !ok {
		return
	}
	f, ok := rays.Rays()
	if !ok {
		return
	}
	if g.own == nil {
		g.own, g.opts.Uniforms = map[string][]float32{}, map[string]any{}
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
	shape, ok := brd.Shape()
	if !ok {
		return
	}
	g.set("GridHex", flag(shape.Hex))
	g.set("GridCell", float32(shape.Cell))
	g.set("GridCells", float32(shape.Cols), float32(shape.Rows))
	g.set("GridWrap", flag(wrapX), flag(wrapY))
	c := premultiplied(colorGridLine)
	g.set("GridLine", c[:]...)
	g.opts.Vertices = 3
	t.Screen.DrawMesh(nil, gridShader, &g.opts)
}

func (g *gridLines) set(name string, v ...float32) {
	s, ok := g.own[name]
	if !ok || len(s) != len(v) {
		s = make([]float32, len(v))
		g.own[name] = s
	}
	copy(s, v)
	g.opts.Uniforms[name] = s
}

func flag(b bool) float32 {
	if b {
		return 1
	}
	return 0
}

func premultiplied(c color.RGBA) [4]float32 {
	return [4]float32{float32(c.R) / 255, float32(c.G) / 255, float32(c.B) / 255, float32(c.A) / 255}
}
