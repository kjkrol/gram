package billboards_test

import (
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/atmosphere/air"
	"github.com/kjkrol/gram/plugins/atmosphere/sky"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/topography"
	"github.com/kjkrol/gram/plugins/topography/internal/billboards"
	icameras "github.com/kjkrol/gram/plugins/topography/internal/cameras"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

// sheet is an AtlasSource of one sprite with no image behind it.
type sheet struct{}

func (sheet) Atlas() *render.Image                            { return nil }
func (sheet) UV(render.SpriteID) (sx0, sy0, sx1, sy1 float32) { return 0, 0, 8, 8 }
func (sheet) White() (u, v float32)                           { return 9, 9 }

// isometricIsland is a world with a level 4x4 board in relief, seen isometrically through its
// camera, 128 x 64.
func isometricIsland() (*world.Plugin, *topography.Plugin, camera.Camera) {
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 256, Height: 256},
		Entities: world.EntitiesCfg{MaxCount: 4, MinSize: 1, MaxSize: 20},
		Heights:  true,
	})
	b := board.NewPlugin(grid.DefaultGrids{}.Square(4, 4, 32), &cell.MultipleOccupancy{}, w)
	b.Res.Logic.Board.SetAll(cell.Kind{Cost: 1, Allows: cell.Land})
	p := topography.NewPlugin(w, b, topography.Config{Cell: 32, HeightUnit: 1})
	cam := cameras.NewPlugin(w).New(p.Views(topography.Isometrically), camera.Config{})
	cam.SetViewport(128, 64)
	cam.MoveTo(0, 0)
	return w, p, cam
}

// In relief an entity stands as a billboard drawn on the GPU: upright on its box's centre at its
// altitude, as wide as the box and as tall as its Z says, as long as the box without one; picked and
// outlined there. From above it lies over its box.
func TestBillboards_StandEntitiesUprightOnTheirCentre(t *testing.T) {
	w, _, cam := isometricIsland()
	cam.CenterOn(64, 64, 0)
	look := w.Look()
	box := plane.NewAABB(geom.NewVec(40, 40), 10, 10)
	stand := func(z world.Z) [][6]float32 {
		var f render.Frame
		f.Reset(cam)
		look.(world.DirectLook).Begin(cam)
		look.Sprite(&f, cam, box, z, sheet{}, render.Appearance{}, render.Light{1, 1, 1})
		return billboards.Stood(look)
	}
	if b := stand(world.Z{Altitude: 6}); len(b) != 1 || b[0] != [6]float32{45, 45, 6, 10, 10, 0} {
		t.Errorf("billboards %v, want one on (45, 45) at 6, 10 tall, 10 wide, upright", b)
	}
	if b := stand(world.Z{Altitude: 6, Height: 30}); len(b) != 1 || b[0][3] != 30 || b[0][4] != 10 {
		t.Errorf("billboards %v, want one 30 tall on its 10-wide box: as tall as its Z says", b)
	}
	drawn := look.Drawn(cam, box.AABB, world.Z{Altitude: 6})
	bx, by := cam.Project(45, 45, 6)
	if drawn[2][1] != by || (drawn[2][0]+drawn[3][0])/2 != bx {
		t.Errorf("drawn at %v, want the billboard standing on (%v, %v)", drawn, bx, by)
	}
	if fp := look.Footprint(cam, box.AABB, 6, nil); len(fp) != 1 || fp[0][0][1] == fp[0][1][1] {
		t.Errorf("footprint %v, want one diamond on the ground", fp)
	}
	// from above the world's own flat look: the sprite over its box
	icameras.Switch(cam)
	drawn = look.Drawn(cam, box.AABB, world.Z{Altitude: 6})
	if x0, y0 := cam.Project(40, 40, 0); drawn[0][0] != x0 || drawn[0][1] != y0 {
		t.Errorf("from above the entity is drawn at %v, want over its box's corner (%v, %v)", drawn[0], x0, y0)
	}
}

// fixedSky is a billboards.Sky of a fixed sun and weather.
type fixedSky struct {
	sun sky.Sun
	air air.Weather
}

func (s fixedSky) Sun() sky.Sun     { return s.sun }
func (s fixedSky) Air() air.Weather { return s.air }

// A billboard stands upright in the calm and leans with the wind what sways.
func TestBillboards_LeanWithTheWindWhatSways(t *testing.T) {
	w, p, cam := isometricIsland()
	cam.CenterOn(48, 48, 0)
	p.WithAtmosphere(fixedSky{sun: sky.DefaultSun, air: air.Weather{Wind: [2]float32{40, 0}}})

	look := w.Look()
	box := plane.NewAABB(geom.NewVec(40, 40), 10, 10)
	lean := func(sway float32) float32 {
		var f render.Frame
		f.Reset(cam)
		look.(world.DirectLook).Begin(cam)
		look.Sprite(&f, cam, box, world.Z{}, sheet{}, render.Appearance{Sway: sway}, render.Light{1, 1, 1})
		return billboards.Stood(look)[0][5]
	}
	if still, swaying := lean(0), lean(1); still != 0 || swaying == 0 {
		t.Errorf("a billboard's top leans %v unswaying, %v swaying; want upright and leaning", still, swaying)
	}
}
