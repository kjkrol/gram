package control_test

import (
	"math"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/topography"
	"github.com/kjkrol/gram/plugins/world"
)

// isoCamera is a camera of a width x height world put in the isometric view.
func isoCamera(width, height uint32, cfg camera.Config) camera.Camera {
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: width, Height: height},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 100},
		Camera:   cfg,
		Quasi3D:  true,
	})
	b := board.NewPlugin(board.DefaultGrids{}.Square(width/32, height/32, 32), &board.MultipleOccupancy{}, w)
	topography.NewPlugin(w, b, topography.Config{Cell: 32, HeightUnit: 2, Isometric: true})
	return w.Camera()
}

// plateau is ground 12 high for x in [200, 300), 0 elsewhere.
func plateau(x, _ float32) float32 {
	if x >= 200 && x < 300 {
		return 12
	}
	return 0
}

func TestContext_WorldFollowsTheGround(t *testing.T) {
	cam := isoCamera(640, 640, camera.Config{ViewportWidth: 400, ViewportHeight: 300})
	cam.MoveTo(160, 160)
	sx, sy := cam.Project(250, 100, 12) // a point on the plateau, drawn 24 pixels above its ground
	screen := geom.NewVec(float64(sx), float64(sy))

	flat := control.Context{Camera: cam}
	if p := flat.World(screen); math.Abs(p.X-250) < 1 && math.Abs(p.Y-100) < 1 {
		t.Error("without Ground the click landed on the plateau point: the height was not ignored")
	}
	over := control.Context{Camera: cam, Ground: plateau}
	if p := over.World(screen); math.Abs(p.X-250) > 0.5 || math.Abs(p.Y-100) > 0.5 {
		t.Errorf("over Ground the click landed at %v, want the plateau point (250, 100)", p)
	}
	box := over.WorldBox(screen, geom.NewVec(screen.X+10, screen.Y))
	if box.TopLeft.X > 250 || box.BottomRight.X < 250 {
		t.Errorf("WorldBox %v does not span the plateau point", box)
	}
}
