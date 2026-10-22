package billboards_test

import (
	"math"
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
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	"github.com/kjkrol/gram/plugins/light"
	"github.com/kjkrol/gram/plugins/topography"
	"github.com/kjkrol/gram/plugins/topography/internal/billboards"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

type sheet struct{}

func (sheet) Atlas() *render.Image                            { return nil }
func (sheet) UV(render.SpriteID) (sx0, sy0, sx1, sy1 float32) { return 0, 0, 8, 8 }
func (sheet) White() (u, v float32)                           { return 9, 9 }

func isometricIsland() (*world.Plugin, *topography.Plugin, camera.Camera) {
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 256, Height: 256},
		Entities: world.EntitiesCfg{MaxCount: 4, MinSize: 1, MaxSize: 20},
		Heights:  true,
	})
	b := board.NewPlugin(grid.DefaultGrids{}.Square(4, 4, 32), &cell.MultipleOccupancy{}, w)
	b.Write().Board().SetAll(cell.Kind{Cost: 1, Allows: cell.Land})
	p := topography.NewPlugin(w, b, nil, topography.Config{})
	cam := cameras.NewPlugin(w, cameras.Config{
		Turn:       true,
		Tilt:       true,
		Cell:       32,
		HeightUnit: 1,
		Cage:       p.Read(0),
	}).New(cameras.Isometrically, camera.Config{})
	cam.SetViewport(128, 64)
	cam.MoveTo(0, 0)
	return w, p, cam
}

func TestBillboards_StandEntitiesUprightOnTheirCentre(t *testing.T) {
	w, _, cam := isometricIsland()
	cam.CenterOn(64, 64, 0)
	look := w.Read(0).Look()
	box := plane.NewAABB(geom.NewVec(40, 40), 10, 10)
	stand := func(z world.Z) [][6]float32 {
		var f render.Frame
		f.Reset(cam)
		look.(world.DirectLook).Begin(&render.Frame{}, cam)
		look.Sprite(&f, cam, box, z, sheet{}, render.Appearance{}, render.WhiteLight)
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
	cameratest.Turn(cam, -math.Pi/4)
	cameratest.Tilt(cam, math.Pi)
	drawn = look.Drawn(cam, box.AABB, world.Z{Altitude: 6})
	if x0, y0 := cam.Project(40, 40, 0); drawn[0][0] != x0 || drawn[0][1] != y0 {
		t.Errorf("from above the entity is drawn at %v, want over its box's corner (%v, %v)", drawn[0], x0, y0)
	}
}

type fixedSky struct {
	sun light.Distant
	air air.Weather
}

func (s fixedSky) Distant() light.Distant { return s.sun }
func (s fixedSky) Air() air.Weather       { return s.air }

func TestBillboards_LeanWithTheWindWhatSways(t *testing.T) {
	w, p, cam := isometricIsland()
	cam.CenterOn(48, 48, 0)
	p.WithAtmosphere(fixedSky{sun: sky.DefaultSun, air: air.Weather{Wind: [2]float32{40, 0}}})

	look := w.Read(0).Look()
	box := plane.NewAABB(geom.NewVec(40, 40), 10, 10)
	lean := func(sway float32) float32 {
		var f render.Frame
		f.Reset(cam)
		look.(world.DirectLook).Begin(&render.Frame{}, cam)
		look.Sprite(&f, cam, box, world.Z{}, sheet{}, render.Appearance{Sway: sway}, render.WhiteLight)
		return billboards.Stood(look)[0][5]
	}
	if still, swaying := lean(0), lean(1); still != 0 || swaying == 0 {
		t.Errorf("a billboard's top leans %v unswaying, %v swaying; want upright and leaning", still, swaying)
	}
}

func TestBillboards_LayADeckOnItsBeam(t *testing.T) {
	w, _, cam := isometricIsland()
	cam.CenterOn(64, 64, 0)
	look := w.Read(0).Look()
	var f render.Frame
	f.Reset(cam)
	look.(world.DirectLook).Begin(&render.Frame{}, cam)
	box := plane.NewAABB(geom.NewVec(40, 40), 16, 16)
	lie := world.Lie{Length: 30, Width: 12, Edged: true, Edge: 2}
	look.(world.LyingLook).Lay(&f, cam, box, geom.NewVec(1, 0), lie,
		world.Z{Altitude: 20, Height: 3}, sheet{}, render.Appearance{}, 0, render.WhiteLight)
	at := map[float32]int{}
	for _, k := range billboards.Lain(look) {
		at[k[2]]++
	}
	if at[20] != 12 || at[17] != 4 || at[22] != 4 || len(at) != 3 {
		t.Errorf("corners by height %v, want 12 at the deck (20), 4 at the beam's foot (17) "+
			"and 4 at the railings' top (22)", at)
	}
}

func TestBillboards_HazeFarOffOnlyWhenAsked(t *testing.T) {
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 4096, Height: 4096},
		Entities: world.EntitiesCfg{MaxCount: 4, MinSize: 1, MaxSize: 20},
		Heights:  true,
	})
	b := board.NewPlugin(grid.DefaultGrids{}.Square(128, 128, 32), &cell.MultipleOccupancy{}, w)
	b.Write().Board().SetAll(cell.Kind{Cost: 1, Allows: cell.Land})
	p := topography.NewPlugin(w, b, nil, topography.Config{})
	p.WithAtmosphere(fixedSky{sun: sky.DefaultSun, air: air.Weather{Sight: 300}})
	cam := cameras.NewPlugin(w, cameras.Config{
		Perspective: true,
		Cell:        32,
		Cage:        p.Read(0),
	}).New(cameras.Isometrically, camera.Config{})
	cam.SetViewport(320, 240)
	cam.CenterOn(2048, 2048, 0)
	cameratest.Switch(cam)
	look := w.Read(0).Look()
	haze := func() []float32 {
		var f render.Frame
		f.Reset(cam)
		look.(world.DirectLook).Begin(&render.Frame{}, cam)
		look.Sprite(&f, cam, plane.NewAABB(geom.NewVec(2043, 2043), 10, 10), world.Z{}, sheet{},
			render.Appearance{}, render.WhiteLight)
		return billboards.Hazed(look)
	}
	if got := haze(); len(got) != 1 || got[0] != 0 {
		t.Errorf("in perspective in thick air a billboard is hazed %v, want clear: not asked", got)
	}
	p.Write().SetHaze(true)
	if got := haze(); len(got) != 1 || got[0] <= 0 {
		t.Errorf("asked, in thick air a billboard is hazed %v, want some", got)
	}
}
