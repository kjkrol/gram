package look_test

import (
	"image/color"
	"testing"

	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/look"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

// white light on everything, as a light plugin without a sun gives it
type whiteLighting struct{}

func (whiteLighting) Join(*goke.QueryBuilder)                {}
func (whiteLighting) Begin(*render.Frame, camera.Camera)     {}
func (whiteLighting) At(x, y, z float32) render.Illumination { return render.WhiteLight }
func (whiteLighting) Of(*goke.Cursor, int, plane.AABB, world.Z,
	render.Appearance) render.Illumination {
	return render.WhiteLight
}

// hands the frame a distant light straight overhead, no ambient light, the ground's shade dark
// everywhere and, lamp set, a white lamp 10 up over (16, 16) reaching 100
type darkening struct {
	dark *render.Image
	lamp bool
}

func (*darkening) Init(*goke.SysInit) {}
func (d *darkening) Compose(f *render.Frame, _ camera.Camera) {
	f.Uniform("LightWay", 0, 0, 1)
	f.Uniform("LightStrength", 1)
	f.Uniform("LightColor", 1, 1, 1)
	f.Uniform("Ambience", 0, 0, 0)
	if d.lamp {
		f.Uniform("LampCount", 1)
		f.Uniform("Lamp0At", 16, 16, 10, 100)
		f.Uniform("Lamp0Color", 1, 1, 1)
	}
	f.GroundShade(render.GroundShade{Now: d.dark, Was: d.dark, Area: [4]float32{0, 0, 256, 256},
		Span: 10, Band: 1, Mix: [render.ShadeBands]float32{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1,
			1, 1, 1}})
}

func TestRenderer_ShadesTheStillAsTheFrameSaysWhereTheCellsTakeIt(t *testing.T) {
	needGPU(t)
	grass := cell.Kind{SpriteID: 1, Cost: 1, Allows: cell.Land}
	lava := cell.Kind{SpriteID: 2, Cost: 1, Allows: cell.Land, Takes: render.Shines}
	grid := grid.DefaultGrids{}.Square(8, 8, 32)
	brd, brdW := board.NewBoard(grid)
	brdW.SetAll(grass)
	brdW.Set(grid.At(6, 6), lava)
	atlas := render.NewAtlas()
	atlas.Add(grass.SpriteID, 4, render.Solid(color.RGBA{R: 200, G: 200, B: 200, A: 255}))
	atlas.Add(lava.SpriteID, 4, render.Solid(color.RGBA{R: 200, G: 200, B: 200, A: 255}))
	atlas.Close()
	space := world.SpaceCfg{Width: 256, Height: 256}
	dark := render.NewImage(1, 1)
	dark.Fill(color.RGBA{A: 255})
	cam := cameratest.NewFromSpace(space.Width, space.Height, 0)
	cam.SetViewport(256, 256)
	draw := func(r *look.Renderer, lamp bool) []byte {
		screen := render.NewImage(256, 256)
		render.NewComposer(&darkening{dark, lamp}, r).DrawWorld(screen, cam)
		pix := make([]byte, 4*256*256)
		screen.ReadPixels(pix)
		return pix
	}
	at := func(pix []byte, x, y int) byte { return pix[4*(y*256+x)] }
	unlit := draw(look.NewRenderer(brd, atlas, brd.Map(), space), false)
	if v := at(unlit, 16, 16); v < 195 {
		t.Fatalf("without a Lighting the grass is %d, want drawn as it is", v)
	}
	lighted := look.NewRenderer(brd, atlas, brd.Map(), space).
		WithLighting(func() world.Lighting { return whiteLighting{} })
	lit := draw(lighted, false)
	if v := at(lit, 16, 16); v > 10 {
		t.Errorf("under the frame's shade the grass is %d, want dark", v)
	}
	if v := at(lit, 6*32+16, 6*32+16); v < 195 {
		t.Errorf("under the frame's shade the lava, taking none, is %d, want as bright as drawn", v)
	}
	lamplit := draw(lighted, true)
	if near, far := at(lamplit, 16, 16), at(lamplit, 100, 200); near < 120 || far > 10 {
		t.Errorf("by the lamp the grass is %d and far from it %d, want it lit by the lamp alone "+
			"in the shade", near, far)
	}
}
