package topography_test

import (
	"image/color"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	"github.com/kjkrol/gram/plugins/topography"
	"github.com/kjkrol/gram/plugins/topography/internal/topotest"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/render/gputest"
)

// straight down from 300 up, a screen pixel a world unit
type overhead struct{ camera.Camera }

func (overhead) Rays() (camera.RayField, bool) {
	return camera.RayField{
		Origin: [3]float32{0, 0, 300},
		DX:     [3]float32{1, 0, 0},
		DY:     [3]float32{0, 1, 0},
		Dir:    [3]float32{0, 0, -1},
	}, true
}

// the depth the meshes drawn before left, 24 bits of it in red, green and blue
var depthRead = render.NewMeshShaderWith("test depth read", []byte(`
struct Out { @builtin(position) clip: vec4<f32> }
@vertex
fn vs_main(@builtin(vertex_index) vid: u32) -> Out {
    var corners = array<vec2<f32>, 6>(
        vec2<f32>(-1.0, -1.0), vec2<f32>(1.0, -1.0), vec2<f32>(-1.0, 1.0),
        vec2<f32>(1.0, -1.0), vec2<f32>(1.0, 1.0), vec2<f32>(-1.0, 1.0),
    );
    return Out(vec4<f32>(corners[vid], 0.0, 1.0));
}
@fragment
fn fs_main(o: Out) -> @location(0) vec4<f32> {
    let v = u32(depthAt(o.clip.xy) * 16777215.0 + 0.5);
    return vec4<f32>(f32(v >> 16u), f32((v >> 8u) & 255u), f32(v & 255u), 255.0) / 255.0;
}
`), nil)

func TestHexes_DrawTheGroundTheReliefHoldsLitSmoothly(t *testing.T) {
	gputest.Need(t)
	const width, height = 320, 176
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: width, Height: height},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 4},
		Heights:  true,
	})
	g := grid.DefaultGrids{}.Hex(8, 6, 16)
	b := board.NewPlugin(g, &cell.MultipleOccupancy{}, w)
	b.Read(0).CellKinds().Define("grass", cell.Kind{Pace: 1, Allows: cell.Land})
	grass, _ := b.Read(0).CellKinds().Get("grass")
	b.Write().Board().SetAll(grass)
	b.Write().ShowGrid(false)
	p := topography.NewPlugin(w, b, nil, topography.Config{})
	a := b.NewAtlas(32)
	a.Add("grass", render.Solid(color.RGBA{R: 160, G: 160, B: 160, A: 255}))
	a.Close()
	relief := p.Write().Relief()
	relief.SetHeights(func(q geom.Vec) float32 {
		return 30 + 14*f32.Sin(q.X/37)*f32.Cos(q.Y/29)
	})
	ground := topotest.Drawn(t, p)[0].(render.Direct)
	cam := overhead{cameratest.NewFromSpace(width, height, 0)}
	w.Write().NewView(cam)
	screen := render.NewImage(width, height)
	depth := render.NewDepth().Readable()
	screen.ClearDepth(depth)
	u := render.UniformsOf(map[string]any{"LightWay": []float32{0.5, 0.3, 0.8},
		"LightStrength": []float32{1}, "LightColor": []float32{1, 1, 1},
		"Ambience": []float32{0.2, 0.2, 0.2}})
	ground.Draw(render.Target{Screen: screen, Depth: depth}, cam, u)
	lit := make([]byte, 4*width*height)
	screen.ReadPixels(lit)
	read := render.NewImage(width, height)
	read.DrawMesh(nil, depthRead, &render.DrawMeshOptions{ReadDepth: depth, Vertices: 6})
	deep := make([]byte, 4*width*height)
	read.ReadPixels(deep)

	f, _ := cam.Rays()
	tr, _ := camera.SceneTransform(f, width, height)
	inv, _ := tr.Unproject()
	// the cells all of whose neighbours lie on the board, away from the walls down its edge
	inner := func(q geom.Vec) bool {
		c, ok := g.CellAt(q)
		return ok && len(g.Neighbors(c)) == 6
	}
	checked := 0
	for y := range height {
		for x := range width {
			at := geom.NewVec(float32(x)+0.5, float32(y)+0.5)
			if !inner(at) {
				continue
			}
			k := 4 * (y*width + x)
			d := float32(int(deep[k])<<16|int(deep[k+1])<<8|int(deep[k+2])) / 16777215
			clip := [4]float32{2*at.X/width - 1, 1 - 2*at.Y/height, d, 1}
			var world [4]float32
			for r := range 4 {
				for c := range 4 {
					world[r] += inv[4*c+r] * clip[c]
				}
			}
			z, want := world[2]/world[3], relief.At(at)
			// a 24-bit depth over the scene's 110000 units of it is about a hundredth
			if f32.Abs(z-want) > 0.05 {
				t.Fatalf("the ground drawn at %v stands at %v, the relief's at %v", at, z, want)
			}
			// the light runs on from triangle to triangle: no step between neighbouring pixels
			if x+1 < width && inner(geom.NewVec(at.X+1, at.Y)) {
				if step := int(lit[k]) - int(lit[k+4]); step > 3 || step < -3 {
					t.Fatalf("the light steps from %d to %d at %v", lit[k], lit[k+4], at)
				}
			}
			checked++
		}
	}
	if checked < width*height/4 {
		t.Errorf("%d pixels of the ground checked, want most of the board's", checked)
	}
}
