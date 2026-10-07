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

// isoCamera is a camera of a width x height world put in the isometric view, over a relief
// raised by heights.
func isoCamera(width, height uint32, cfg camera.Config, heights func(geom.Vec) float64) camera.Camera {
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: width, Height: height},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 100},
		Heights:  true,
	})
	b := board.NewPlugin(grid.DefaultGrids{}.Square(width/32, height/32, 32), &cell.MultipleOccupancy{}, w)
	topo := topography.NewPlugin(w, b, topography.Config{Cell: 32, HeightUnit: 2})
	topo.Relief().SetHeights(heights)
	return cameras.NewPlugin(w).New(topo.Views(topography.Isometrically), cfg)
}

// plateau is ground 12 high for x in [192, 320], 0 elsewhere.
func plateau(p geom.Vec) float64 {
	if p.X >= 192 && p.X <= 320 {
		return 12
	}
	return 0
}

// hidden is a camera that keeps its Pick to itself.
type hidden struct{ camera.Camera }

func TestContext_WorldAsksThePicker(t *testing.T) {
	cam := isoCamera(640, 640, camera.Config{ViewportWidth: 400, ViewportHeight: 300}, plateau)
	if _, ok := cam.(camera.Picker); !ok {
		t.Fatal("the topography's camera is no camera.Picker")
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
	iso := isoCamera(640, 640, camera.Config{ViewportWidth: 400, ViewportHeight: 300}, func(geom.Vec) float64 { return 0 })
	iso.MoveTo(160, 160)
	cam := hidden{iso}
	sx, sy := cam.Project(250, 100, 12)
	screen := geom.NewVec(float64(sx), float64(sy))
	if p := (control.Context{Camera: cam}).World(screen); math.Abs(p.X-250) < 1 && math.Abs(p.Y-100) < 1 {
		t.Error("through a camera that picks nothing the click landed on the plateau point: the height was not ignored")
	}
}
