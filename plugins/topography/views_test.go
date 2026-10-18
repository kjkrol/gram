package topography_test

import (
	"math"
	"strings"
	"testing"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	"github.com/kjkrol/gram/plugins/topography"
	"github.com/kjkrol/gram/plugins/topography/internal/topotest"
	"github.com/kjkrol/gram/plugins/world"
)

func TestPlugin_TheStickLeansFromAboveToIsometric(t *testing.T) {
	// a world wide enough that no zoom floor holds the view back
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 2048, Height: 2048},
		Entities: world.EntitiesCfg{MaxCount: 4, MinSize: 1, MaxSize: 20},
		Heights:  true,
	})
	b := board.NewPlugin(grid.DefaultGrids{}.Square(64, 64, 32), &cell.MultipleOccupancy{}, w)
	p := topography.NewPlugin(w, b, topography.Config{})
	cam := cameras.NewPlugin(w, cameras.Config{
		Turn:       true,
		Tilt:       true,
		Cell:       32,
		TileW:      64,
		HeightUnit: 1,
		Cage:       p,
	}).New(cameras.FromAbove, camera.Config{})
	cam.SetViewport(128, 64)
	cam.MoveTo(0, 0)
	if cam.Projection().Sorts() {
		t.Fatal("a camera begun FromAbove looks isometrically")
	}
	x0, y0 := cam.Project(32, 32, 0)
	if x1, y1 := cam.Project(32, 32, 10); x0 != x1 || y0 != y1 {
		t.Errorf("from above a height is drawn %v, %v away from the ground, want not at all",
			x1-x0, y1-y0)
	}
	cam.CenterOn(1024, 1024, 0)
	before, _ := cam.Project(1024, 1024, 0)
	after, _ := cam.Project(1056, 1024, 0)
	cameratest.Turn(cam, math.Pi/4)
	cameratest.Tilt(cam, -math.Pi/3)
	if !cam.Projection().Sorts() {
		t.Fatal("turned an eighth and tilted 60° flatter the camera does not look isometrically")
	}
	vw, vh := cam.Viewport()
	if x, y := cam.Unproject(vw/2, vh/2, 0); math.Abs(float64(x-1024)) > 0.5 ||
		math.Abs(float64(y-1024)) > 0.5 {
		t.Errorf("leaning, the middle of the screen is over (%v, %v), want (1024, 1024)", x, y)
	}
	// one measure: the diamond is as wide as the cell's diagonal was from above
	left, _ := cam.Project(1024, 1056, 0)
	right, _ := cam.Project(1056, 1024, 0)
	if width, want := right-left, (after-before)*math.Sqrt2; math.Abs(float64(width-want)) > 0.5 {
		t.Errorf("leaning, a cell spans %v pixels, want its diagonal from above, %v", width, want)
	}
	_, top := cam.Project(1024, 1024, 0)
	_, bottom := cam.Project(1056, 1056, 0)
	if height := bottom - top; math.Abs(float64(2*height-(right-left))) > 0.5 {
		t.Errorf("leaning, a cell is %v wide and %v high, want a 2:1 diamond", right-left, height)
	}
	cameratest.Tilt(cam, math.Pi)
	if cam.Projection().Sorts() {
		t.Error("tilted back up the camera does not look straight down")
	}
}

func TestPlugin_RefusesAWrappingWorld(t *testing.T) {
	defer func() {
		if r, _ := recover().(string); !strings.Contains(r, "wraps") {
			t.Errorf("NewPlugin over a torus: %q, want a refusal", r)
		}
	}()
	w := topotest.NewWorld(aabbworld.Torus)
	b, _ := topotest.LevelBoard(w)
	topography.NewPlugin(w, b, topography.Config{})
}

func TestViews_EveryCameraBeginsAsItsStartSays(t *testing.T) {
	w := topotest.NewWorld(0)
	b, _ := topotest.LevelBoard(w)
	p := topography.NewPlugin(w, b, topography.Config{})
	cams := cameras.NewPlugin(w, cameras.Config{Turn: true, Tilt: true, Cell: 32, Cage: p})
	above := cams.New(cameras.FromAbove, camera.Config{})
	iso := cams.New(cameras.Isometrically, camera.Config{})
	if above.Projection().Sorts() {
		t.Error("the camera begun FromAbove looks isometrically")
	}
	if !iso.Projection().Sorts() {
		t.Error("the camera begun Isometrically looks from above")
	}
}
