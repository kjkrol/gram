package cameras_test

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
	p := topography.NewPlugin(w, b, topography.Config{Cell: 32, HeightUnit: 1})
	cams := cameras.NewPlugin(w)
	isometric := func() camera.Camera {
		cam := cams.New(p.Views(topography.Isometrically), camera.Config{})
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

// A click on the top of a kind standing on its cell lands on that cell, not on the ground behind
// it, isometrically and in perspective: the camera picks the top as it is drawn.
func TestPlugin_ThePickLandsOnTheTopOfAKindStandingOnItsCell(t *testing.T) {
	for _, perspective := range []bool{false, true} {
		w := topotest.NewWorld(0)
		b, grid := topotest.LevelBoard(w)
		p := topography.NewPlugin(w, b, topography.Config{Cell: 32, HeightUnit: 1, Perspective: perspective})
		wall := grid.CellIndex(2, 1)
		b.Res.Logic.Board.Set(wall, cell.Kind{Name: cell.Named("wall"), Cost: 1, Height: 30})
		cam := topotest.Camera(w, p)
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
		x, y, ok := picker.Pick(sx, sy)
		if c, in := grid.CellAt(geom.NewVec(float64(x), float64(y))); !ok || !in || c != wall {
			t.Errorf("perspective %v: the top of the wall picked (%v, %v) %v, cell %v, want the wall's cell %v", perspective, x, y, ok, c, wall)
		}
	}
}
