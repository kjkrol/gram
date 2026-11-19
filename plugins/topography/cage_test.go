package topography_test

import (
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/ground"
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
	for name, cam := range map[string]camera.Camera{"the first": isometric(),
		"another": isometric()} {
		if !cam.Projection().Sorts() || cam.Projection().Wraps() {
			t.Errorf(
				"%s camera draws through %T, want the plugin's isometric projection",
				name,
				cam.Projection(),
			)
		}
		if sx, sy := cam.Project(0, 0, 10); sx == 0 && sy == 0 {
			t.Errorf(
				"%s camera draws a height 10 at the screen's origin: heights are not lifted",
				name,
			)
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
		b.Write().Board().Set(wall, cell.Kind{Name: cell.Named("wall"), Pace: 1, Height: 30})
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
		if perspective &&
			!cam.(interface{ LookFrom(x, y, z float32) bool }).LookFrom(200, 200, 120) {
			t.Fatal("the perspective view is not reached")
		}
		centre := grid.CellCenter(wall)
		cam.CenterOn(centre.X, centre.Y, 30)
		sx, sy := cam.Project(float32(centre.X), float32(centre.Y), 30)
		x, y, _, ok := picker.Pick(sx, sy)
		if c, in := grid.CellAt(geom.NewVec(float32(x), float32(y))); !ok || !in || c != wall {
			t.Errorf(
				"perspective %v: the top of the wall picked (%v, %v) %v, cell %v, want the "+
					"wall's cell %v",
				perspective,
				x,
				y,
				ok,
				c,
				wall,
			)
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
	b.Write().Board().Set(wall, cell.Kind{Name: cell.Named("wall"), Pace: 1, Height: 30})
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

func TestCage_TheSeaIsSolidUpToItsLevelOverTheGroundUnderIt(t *testing.T) {
	w := topotest.NewWorld(0)
	b, grid := topotest.LevelBoard(w)
	p := topography.NewPlugin(w, b, nil, topography.Config{})
	sea := grid.At(2, 1)
	p.Write().Relief().SetHeights(ground.MeanOfCells(grid, func(c cell.ID) float32 {
		if c == sea {
			return -24
		}
		return 0
	}))
	b.Write().Board().Set(sea, cell.Kind{Name: cell.Named("sea"), Pace: 1, Allows: cell.Water})
	centre := grid.CellCenter(sea)
	x, y := float32(centre.X), float32(centre.Y)
	if bottom := p.Write().Relief().At(centre); bottom >= -1 {
		t.Fatalf("the harbour's floor lies at %v, want it sunk", bottom)
	}
	if p.Read(0).Free(x, y, -1) || !p.Read(0).Free(x, y, 1) {
		t.Errorf("over the sea: free under its level %v, over it %v; want solid, then free",
			p.Read(0).Free(x, y, -1), p.Read(0).Free(x, y, 1))
	}
}

func TestCage_IsFreeInARoomStandingOnTheGroundWhereverTheGroundLies(t *testing.T) {
	w := topotest.NewWorld(0)
	b, grid := topotest.LevelBoard(w)
	p := topography.NewPlugin(w, b, nil, topography.Config{})
	// the ground climbs 12 northward across the room's cell (1, 1), its floor 4 up, its roof 10
	// over that: dug into the slope by its northern half
	p.Write().Relief().SetHeights(func(q geom.Vec) float32 { return (64 - q.Y) * 12 / 32 })
	room := grid.At(1, 1)
	rooms := board.NewEntities(b, board.Shape{Size: 16}, func(c cell.ID) cell.ID { return c })
	rooms.Define("room", comp.Load(func(c cell.ID) cell.Floor {
		return cell.Floor{Allows: cell.Land, Height: 4 - p.Write().Relief().Altitude(c),
			Storey: 1, Clear: 10, Standing: true}
	}))
	w.Write().Seed(rooms.Named("room").Entry(room))
	ctx := topotest.NewInstallCtx()
	for _, install := range []func() error{
		func() error { return w.Install(ctx) },
		func() error { return b.Install(ctx) },
		func() error { return p.Install(ctx) },
	} {
		if err := install(); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Populate(); err != nil {
		t.Fatal(err)
	}
	ctx.ECS().Setup(ctx.Systems()...)
	ctx.ECS().SetPlan(func(rc goke.RunCtx, d time.Duration) {
		w.RunPlan(rc, d)
		b.RunPlan(rc, d)
		p.RunPlan(rc, d)
		rc.Sync()
		w.Write().Clock().Replay(rc, d)
	})
	ctx.ECS().Tick(time.Second / 60)
	cage := p.Read(0)
	for name, c := range map[string]struct {
		x, y, z float32
		free    bool
	}{
		"in the room under the slope's ground": {48, 36, 8, true},
		"in the room over its floor":           {48, 60, 5, true},
		"under its floor":                      {48, 60, 2, false},
		"under the ground beside it":           {80, 36, 5, false},
	} {
		if got := cage.Free(c.x, c.y, c.z); got != c.free {
			t.Errorf("%s at (%v, %v, %v): free %v, want %v", name, c.x, c.y, c.z, got, c.free)
		}
	}
}
