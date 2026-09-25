package control_test

import (
	"math"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
)

// plateau is ground 12 high for x in [200, 300), 0 elsewhere.
func plateau(x, _ float32) float32 {
	if x >= 200 && x < 300 {
		return 12
	}
	return 0
}

func TestContext_WorldFollowsTheGround(t *testing.T) {
	cam := camera.NewFromSpaceWithConfig(640, 640, 0, camera.Config{ViewportWidth: 400, ViewportHeight: 300, Projection: camera.Isometric{Cell: 32, HeightUnit: 2}})
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
