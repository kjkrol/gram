package topography_test

import (
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/topography"
	"github.com/kjkrol/gram/plugins/topography/internal/topotest"
)

func TestPlugin_ViewsMakeIsometricCameras(t *testing.T) {
	w := topotest.NewWorld(0)
	b, _ := topotest.LevelBoard(w)
	p := topography.NewPlugin(w, b, nil, topography.Config{})
	cams := cameras.NewPlugin(w, cameras.Config{Turn: true, Tilt: true, Cell: 32, Cage: p.Read(0)})
	isometric := func() camera.Camera {
		cam := cams.Write().New(cameras.Isometrically, camera.Config{})
		cam.SetViewport(128, 64)
		cam.MoveTo(0, 0)
		return cam
	}
	for name, cam := range map[string]camera.Camera{"the first": isometric(), "another": isometric()} {
		if !cam.Projection().Sorts() || cam.Projection().Wraps() {
			t.Errorf("%s camera draws through %T, want the plugin's isometric projection", name, cam.Projection())
		}
		if sx, sy := cam.Project(0, 0, 10); sx == 0 && sy == 0 {
			t.Errorf("%s camera draws a height 10 at the screen's origin: heights are not lifted", name)
		}
		if vw, vh := cam.Viewport(); vw != 128 || vh != 64 {
			t.Errorf("%s camera is %v x %v, want the configured 128 x 64", name, vw, vh)
		}
	}
}

func TestPlugin_ThePickLandsOnTheTopOfAKindStandingOnItsCell(t *testing.T) {
	for _, perspective := range []bool{false, true} {
		w := topotest.NewWorld(0)
		b, grid := topotest.LevelBoard(w)
		p := topography.NewPlugin(w, b, nil, topography.Config{})
		wall := grid.At(2, 1)
		b.Write().Board().Set(wall, cell.Kind{Name: cell.Named("wall"), Cost: 1, Height: 30})
		cam := topotest.Camera(w, cameras.Config{
			Turn:        true,
			Tilt:        true,
			Perspective: perspective,
			Cell:        32,
			HeightUnit:  1,
			Cage:        p.Read(0),
		})
		picker, ok := cam.(camera.Picker)
		if !ok {
			t.Fatal("the topography's camera is no camera.Picker")
		}
		if perspective && !cam.(interface{ LookFrom(x, y, z float32) bool }).LookFrom(200, 200, 120) {
			t.Fatal("the perspective view is not reached")
		}
		centre := grid.CellCenter(wall)
		cam.CenterOn(centre.X, centre.Y, 30)
		sx, sy := cam.Project(float32(centre.X), float32(centre.Y), 30)
		x, y, _, ok := picker.Pick(sx, sy)
		if c, in := grid.CellAt(geom.NewVec(float32(x), float32(y))); !ok || !in || c != wall {
			t.Errorf("perspective %v: the top of the wall picked (%v, %v) %v, cell %v, want the wall's cell %v", perspective, x, y, ok, c, wall)
		}
	}
}

func TestCage_IsFreeOverTheGroundAndWhatStandsOnIt(t *testing.T) {
	w := topotest.NewWorld(0)
	b, grid := topotest.LevelBoard(w)
	p := topography.NewPlugin(w, b, nil, topography.Config{})
	hill := grid.At(1, 1)
	topotest.RaiseHills(p.Write().Relief(), grid, hill)
	wall := grid.At(2, 2)
	b.Write().Board().Set(wall, cell.Kind{Name: cell.Named("wall"), Cost: 1, Height: 30})
	top := float32(p.Write().Relief().At(grid.CellCenter(hill)))
	if top <= 0 {
		t.Fatalf("the hill stands %v high, want it raised", top)
	}
	at := func(c cell.ID) (float32, float32) {
		centre := grid.CellCenter(c)
		return float32(centre.X), float32(centre.Y)
	}
	hx, hy := at(hill)
	if p.Read(0).Free(hx, hy, top-1) || !p.Read(0).Free(hx, hy, top+1) {
		t.Errorf("over the hill %v high: free under %v, over %v; want solid under, free over",
			top, p.Read(0).Free(hx, hy, top-1), p.Read(0).Free(hx, hy, top+1))
	}
	wx, wy := at(wall)
	if p.Read(0).Free(wx, wy, 29) || !p.Read(0).Free(wx, wy, 31) {
		t.Errorf("by the wall 30 tall: free at 29 %v, at 31 %v; want solid, then free",
			p.Read(0).Free(wx, wy, 29), p.Read(0).Free(wx, wy, 31))
	}
	if low, high := p.Read(0).Span(); low > 0 || high < top {
		t.Errorf("the span is %v to %v, want the hill's %v within it", low, high, top)
	}
	if step, thin := p.Read(0).Grain(hx, hy); step != 0 || thin != 0 {
		t.Errorf("away from any deck the grain is %v, %v, want none", step, thin)
	}
}
