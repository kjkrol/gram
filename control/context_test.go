package control_test

import (
	"math"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/topography"
	"github.com/kjkrol/gram/plugins/world"
)

func isoCamera(width, height uint32, screenW, screenH float32, heights func(geom.Vec) float64) camera.Camera {
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: width, Height: height},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 100},
		Heights:  true,
	})
	b := board.NewPlugin(grid.DefaultGrids{}.Square(width/32, height/32, 32), &cell.MultipleOccupancy{}, w)
	topo := topography.NewPlugin(w, b, nil, topography.Config{})
	topo.Write().Relief().SetHeights(heights)
	cam := cameras.NewPlugin(w, cameras.Config{
		Turn:       true,
		Tilt:       true,
		Cell:       32,
		HeightUnit: 2,
		Cage:       topo.Read(0),
	}).New(cameras.Isometrically, camera.Config{})
	cam.SetViewport(screenW, screenH)
	cam.MoveTo(0, 0)
	return cam
}

func plateau(p geom.Vec) float64 {
	if p.X >= 192 && p.X <= 320 {
		return 12
	}
	return 0
}

type hidden struct{ camera.Camera }

func TestContext_WorldAsksThePicker(t *testing.T) {
	cam := isoCamera(640, 640, 400, 300, plateau)
	if _, ok := cam.(camera.Picker); !ok {
		t.Fatal("the camera in relief is no camera.Picker")
	}
	cam.MoveTo(160, 160)
	sx, sy := cam.Project(250, 100, 12) // a point on the plateau, drawn 24 pixels above its ground
	screen := geom.NewVec(float64(sx), float64(sy))

	c := control.Context{Camera: cam}
	if p := c.World(screen); math.Abs(p.X-250) > 0.5 || math.Abs(p.Y-100) > 0.5 {
		t.Errorf("the click landed at %v, want the plateau point (250, 100)", p)
	}
	box := c.WorldBox(screen, geom.NewVec(screen.X+10, screen.Y))
	if box.TopLeft.X > 250 || box.BottomRight.X < 250 {
		t.Errorf("WorldBox %v does not span the plateau point", box)
	}
}

func TestContext_WorldThroughACameraThatPicksNothingIsTheGroundAtSeaLevel(t *testing.T) {
	iso := isoCamera(640, 640, 400, 300, func(geom.Vec) float64 { return 0 })
	iso.MoveTo(160, 160)
	cam := hidden{iso}
	sx, sy := cam.Project(250, 100, 12)
	screen := geom.NewVec(float64(sx), float64(sy))
	if p := (control.Context{Camera: cam}).World(screen); math.Abs(p.X-250) < 1 && math.Abs(p.Y-100) < 1 {
		t.Error("through a camera that picks nothing the click landed on the plateau point: the height was not ignored")
	}
}
