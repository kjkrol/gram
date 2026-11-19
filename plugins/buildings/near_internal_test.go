package buildings

import (
	"image/color"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/render/gputest"
)

// straight down from 200 up, a screen pixel 4 world units
type above struct{ camera.Camera }

func (above) Rays() (camera.RayField, bool) {
	return camera.RayField{Origin: [3]float32{0, 0, 200}, DX: [3]float32{4, 0, 0},
		DY: [3]float32{0, 4, 0}, Dir: [3]float32{0, 0, -1}}, true
}

// riding a unit, behind it
type behind struct{ above }

func (behind) Fasten(camera.Fastening) {}

func (behind) Fastening() camera.Fastening {
	return camera.Fastening{How: camera.Behind, Entity: 1 << 40}
}

// a ground of nothing but its depth, for the buildings to stand in
type depthOnly struct{}

func (depthOnly) Compose(f *render.Frame, _ camera.Camera)           { f.DrawOpaque() }
func (depthOnly) Draw(render.Target, camera.Camera, render.Uniforms) {}

var blueTiles = color.RGBA{R: 40, G: 60, B: 200, A: 255}

// a house of two cells, (3, 3) and (4, 3), its floor 10 up on level ground, its roof blue; the
// room bared or not, seen through cam: the pixel over its middle
func housePixel(t *testing.T, cam camera.Camera, bared bool) color.RGBA {
	t.Helper()
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 256, Height: 256},
		Entities: world.EntitiesCfg{MaxCount: 8, MinSize: 1, MaxSize: 64},
		Heights:  true,
	})
	brd := board.NewPlugin(grid.DefaultGrids{}.Square(8, 8, 32), &cell.MultipleOccupancy{}, w)
	p := NewPlugin(w, brd)
	spec := Spec{Storey: 12, Floors: 1, Walls: color.RGBA{R: 200, G: 200, B: 200, A: 255},
		Tiles: blueTiles, Bay: 8, Door: cell.Gap{Width: 4, Height: 8},
		Parts: []Part{{Roof: Hip, Pitch: 0.5}}}
	at := spec.of(Row{Mid: [2]float32{128, 112}, Size: [2]float32{64, 32}})
	h := house{spec: spec, at: at, floor: 10, doorAt: geom.NewVec(112, 128)}
	level := func(geom.Vec) float32 { return 10 }
	parts := map[int][]float32{0: lay(nil, spec, at, level, 0.5, true, &h)}
	kept, ranges := render.PackInstances(solidFloats, parts, map[int][]float32{})
	inner := inside(spec, at, h)
	inner[15] = 1
	inside, in := render.PackInstances(innerFloats, map[int][]float32{0: inner[:]})
	p.solids.shown = &stock{kept: kept, fine: ranges[0], coarse: ranges[1],
		spans: map[int][2]float32{0: {10, 40}}, inside: inside, inner: in[0],
		rooms: []laidRoom{{sector: 0, inner: inner, lamp: 1}}}
	if bared {
		p.solids.frames[p.solids.frame].bared = []int{0}
	}
	c := render.NewComposer(depthOnly{}, p.solids, p.near)
	c.Light(func() *render.Lights {
		return &render.Lights{Sun: render.Distant{Dir: [3]float32{0.3, 0.2, 1}, Strength: 1,
			Ambient: 0.4}}
	})
	screen := render.NewImage(64, 64)
	for range 2 {
		c.DrawWorld(screen, cam)
	}
	pix := make([]byte, 4*64*64)
	screen.ReadPixels(pix)
	i := 4 * (28*64 + 32) // (128, 112)
	return color.RGBA{R: pix[i], G: pix[i+1], B: pix[i+2], A: pix[i+3]}
}

func blue(c color.RGBA) bool { return int(c.B) > int(c.R)+60 && int(c.B) > int(c.G)+60 }

func TestNear_ARoomAnUncoveredEntityStandsInHasWhatLiesOverItGiveWay(t *testing.T) {
	gputest.Need(t)
	cam := above{cameratest.NewFromSpace(64, 64, 0)}
	if got := housePixel(t, cam, false); !blue(got) {
		t.Errorf("over a room bared by none the house shows %v, want its blue roof", got)
	}
	if got := housePixel(t, cam, true); blue(got) || got.A == 0 {
		t.Errorf("over a room bared the house shows %v, want its floor through the roof", got)
	}
	riding := behind{above{cameratest.NewFromSpace(64, 64, 0)}}
	if got := housePixel(t, riding, true); !blue(got) {
		t.Errorf("riding a unit, over a room bared the house shows %v, want its roof whole", got)
	}
}
