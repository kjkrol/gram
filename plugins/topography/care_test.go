package topography_test

import (
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	"github.com/kjkrol/gram/plugins/motion"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/topography"
	"github.com/kjkrol/gram/plugins/topography/internal/topotest"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/sector"
	"github.com/kjkrol/gram/render"
)

func TestPlugin_ShadesTheGroundWithFullCareAsFarAsTheConfigSays(t *testing.T) {
	w := topotest.NewWorld(0)
	b, _ := topotest.LevelBoard(w)
	p := topography.NewPlugin(w, b, nil, topography.Config{Care: 2})
	if p.Read(0).Care() != 2 {
		t.Errorf("full care within %d sectors, want the Config's 2", p.Read(0).Care())
	}
	for _, bd := range p.DefaultBindings() {
		if bd.Type() == reflect.TypeFor[topography.Care]() ||
			bd.Type() == reflect.TypeFor[topography.Haze]() {
			t.Errorf("%v is bound to %v, want the settings screen alone", bd.Type(),
				players.Written(bd.Trigger))
		}
	}
}

func TestReporter_SaysTheReachAndEachCareALine(t *testing.T) {
	w := topotest.NewWorld(0)
	b, _ := topotest.LevelBoard(w)
	p := topography.NewPlugin(w, b, nil, topography.Config{Care: 2})
	report := func() string {
		var got string
		p.Read(0).Reporter().Report(func(label, value string) { got = label + ": " + value })
		return got
	}
	want := "Ground: care within 2 sectors\neverything 0\nno shore 0\nflat mirror 0\nno shade 0" +
		"\nleast 0"
	if got := report(); got != want {
		t.Errorf("the panel says %q, want %q", got, want)
	}
	p.Write().SetCare(0)
	if got := report(); got != "Ground: care everywhere" {
		t.Errorf("with full care everywhere the panel says %q", got)
	}
}

func TestPlugin_CareSetsTheFullCaresReach(t *testing.T) {
	qw := topotest.NewQuasiWorld(t, false,
		func(*board.Units[topotest.Recruit], grid.Grid) []kind.Entry { return nil })
	caring := queueOf[topography.Care](t, qw.Topo)
	if qw.Topo.Read(0).Care() != 0 {
		t.Fatalf("full care within %d sectors at first, want everywhere", qw.Topo.Read(0).Care())
	}
	for _, want := range []int{3, 1, 0} {
		caring.Put(command.Nobody, topography.Care{Rings: want})
		qw.ECS.Tick(time.Second / 60)
		if got := qw.Topo.Read(0).Care(); got != want {
			t.Errorf("Care of %d: full care within %d sectors", want, got)
		}
	}
}

func TestSettings_TheCaresReachIsASliderToEverySectorAndTheCareShownASwitch(t *testing.T) {
	w := topotest.NewWorld(0)
	b, _ := topotest.LevelBoard(w)
	p := topography.NewPlugin(w, b, nil, topography.Config{Care: 2})
	settings := p.Settings()
	if len(settings) != 4 {
		t.Fatalf("%d settings, want the haze, the care following, the care's reach and the care "+
			"shown", len(settings))
	}
	haze, follows, reach, shown := settings[0].(control.Switch), settings[1].(control.Switch),
		settings[2].(control.Range), settings[3].(control.Switch)
	if haze.Section != control.Graphics || follows.Section != control.PictureCare ||
		reach.Section != control.PictureCare || shown.Section != control.PictureCare {
		t.Errorf("the sections are %q, %q, %q and %q, want the graphics and the care thrice",
			haze.Section, follows.Section, reach.Section, shown.Section)
	}
	if follows.Label != "Full care follows the frame budget" || follows.Get() ||
		follows.Set(true) != (topography.AdaptCare{On: true}) {
		t.Errorf("the following is %q (%v) building %v, want \"Full care follows the frame "+
			"budget\" off, AdaptCare{On: true}", follows.Label, follows.Get(), follows.Set(true))
	}
	if reach.Label != "Full care out to" || reach.Min != 1 || reach.Max != 8 || reach.Step != 1 {
		t.Errorf("the reach is %q from %v to %v by %v, want \"Full care out to\" 1 to 8 by 1",
			reach.Label, reach.Min, reach.Max, reach.Step)
	}
	if got := reach.Get(); got != 2 {
		t.Errorf("the reach reads %v, want the Config's 2", got)
	}
	for v, want := range map[float64]string{8: "every sector", 1: "a sector", 3: "3 sectors"} {
		if got := reach.Shows(v); got != want {
			t.Errorf("the reach shows %v as %q, want %q", v, got, want)
		}
	}
	if got := reach.Set(8); got != (topography.Care{Rings: 0}) {
		t.Errorf("the reach at the top builds %v, want Care{Rings: 0}: every sector", got)
	}
	if got := reach.Set(3); got != (topography.Care{Rings: 3}) {
		t.Errorf("the reach at 3 builds %v, want Care{Rings: 3}", got)
	}
	p.Write().SetCare(0)
	if got := reach.Get(); got != 8 {
		t.Errorf("the reach reads %v with every sector cared for, want the top", got)
	}
	p.Write().SetCare(12)
	if got := reach.Get(); got != 8 {
		t.Errorf("the reach reads %v with the care past the top, want the top", got)
	}
	if shown.Label != "Show the care" || shown.Get() {
		t.Errorf("the switch is %q (%v), want \"Show the care\" off", shown.Label, shown.Get())
	}
	if got := shown.Set(true); got != (topography.ShowCare{Shown: true}) {
		t.Errorf("the switch on builds %v, want ShowCare{Shown: true}", got)
	}
}

func TestPlugin_ShowCareThroughItsQueueShowsTheCare(t *testing.T) {
	qw := topotest.NewQuasiWorld(t, false,
		func(*board.Units[topotest.Recruit], grid.Grid) []kind.Entry { return nil })
	showing := queueOf[topography.ShowCare](t, qw.Topo)
	for _, shown := range []bool{true, false} {
		showing.Put(command.Nobody, topography.ShowCare{Shown: shown})
		qw.ECS.Tick(time.Second / 60)
		if qw.Topo.Read(0).CareShown() != shown {
			t.Errorf("ShowCare{%v} leaves the care shown %v", shown, qw.Topo.Read(0).CareShown())
		}
	}
}

func queueOf[C command.Command](t *testing.T, p *topography.Plugin) command.CommandQueue {
	t.Helper()
	for _, q := range p.Queues() {
		if q.Accepts() == reflect.TypeFor[C]() {
			return q
		}
	}
	t.Fatalf("no queue takes a %v", reflect.TypeFor[C]())
	return nil
}

// an anchor of the world's sectors standing still
type anchored struct{ x, y float32 }

func (a anchored) Anchors(at func(x, y float32)) { at(a.x, a.y) }

func TestCareView_DrawsTheBoundariesBetweenTheCaresBandsOverTheGround(t *testing.T) {
	for _, tc := range []struct {
		name    string
		sectors int
		pieces  int
	}{
		// the middle of the screen over the sector (2, 2): full care a ring round it, one less
		// in the first row and column; the boundary between them 3 sectors' edges of 4 cells on
		// each of two sides
		{"two bands", 4, 24},
		// over the sector (3, 3): full care a ring round it, one less a ring round that, two
		// less in the first row and column; the full band's four sides of 3 sectors, the next
		// band's two of 5
		{"three bands", 6, 48 + 40},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// a world of sectors 128 a side (entities 4 across make buckets of 32), all in one
			// band round the middle, over a board covering it
			size := uint32(tc.sectors * 128)
			w := world.NewPlugin(world.Config{
				Space:    world.SpaceCfg{Width: size, Height: size},
				Entities: world.EntitiesCfg{MaxCount: 8, MinSize: 4, MaxSize: 4},
				Heights:  true,
				Sectors:  world.SectorsCfg{Buckets: 4, Bands: []sector.Band{{Reach: 1000}}},
			})
			w.Write().Anchor(anchored{float32(size) / 2, float32(size) / 2})
			m := motion.NewPlugin(w)
			cells := uint32(tc.sectors * 4)
			b := board.NewPlugin(grid.DefaultGrids{}.Square(cells, cells, 32),
				&cell.MultipleOccupancy{}, w).WithMotion(m)
			b.Write().Board().SetAll(cell.Kind{Name: cell.Named("grass"), Cost: 1, Allows: cell.Land})
			p := topography.NewPlugin(w, b, nil, topography.Config{Care: 1})
			ctx := topotest.NewInstallCtx()
			for _, plugin := range []interface {
				Install(plugin.Installer) error
			}{w, m, b, p} {
				if err := plugin.Install(ctx); err != nil {
					t.Fatal(err)
				}
			}
			sources := ctx.Sources()
			if len(sources) < 2 {
				t.Fatalf("the topography draws %v, want the ground and the care's view", sources)
			}
			careView := sources[len(sources)-2]
			ecs := ctx.ECS()
			ecs.Setup(ctx.Systems()...)
			ecs.SetPlan(func(rc goke.RunCtx, d time.Duration) {
				w.RunPlan(rc, d)
				m.RunPlan(rc, d)
				b.RunPlan(rc, d)
				p.RunPlan(rc, d)
				rc.Sync()
				w.Write().Clock().Replay(rc, d)
			})
			// the view sees every sector once the bands are laid
			cam := cameratest.NewFromSpace(size, size, 0)
			w.Write().NewView(cam)
			ecs.Tick(time.Second / 60)
			ecs.Tick(time.Second / 60)
			drawn := func() (pieces int, onTop bool) {
				var f render.Frame
				f.Reset(cam)
				careView.Compose(&f, cam)
				f.Each(func(at render.At, _ []render.Vertex) {
					pieces++
					onTop = onTop || at.Top()
				})
				return pieces, onTop
			}
			if pieces, _ := drawn(); pieces != 0 {
				t.Errorf("%d pieces with the care not shown, want none", pieces)
			}
			p.Write().ShowCare(true)
			pieces, onTop := drawn()
			if pieces != tc.pieces {
				t.Errorf("%d pieces with the care shown, want %d: the boundaries' segments a "+
					"cell each", pieces, tc.pieces)
			}
			if onTop {
				t.Error("a boundary's segment lies on top of the world, want it in the world")
			}
			p.Write().SetCare(0)
			if pieces, _ := drawn(); pieces != 0 {
				t.Errorf("%d pieces with every sector shaded with full care, want none", pieces)
			}
		})
	}
}

func TestPlugin_AsALeverOfTheFrameBudgetStepsTheCareDownTheLadderAndBackWhileAdaptive(
	t *testing.T) {
	w := topotest.NewWorld(0)
	b, _ := topotest.LevelBoard(w)
	p := topography.NewPlugin(w, b, nil, topography.Config{})
	if p.Write().Following() {
		t.Fatal("not adaptive, the care follows the frame budget")
	}
	p.Write().AdaptCare(true)
	if !p.Write().Following() {
		t.Fatal("adaptive, the care does not follow")
	}
	var cares []int
	for p.Write().Coarser() {
		cares = append(cares, p.Read(0).Care())
	}
	if want := []int{3, 2, 1}; !slices.Equal(cares, want) {
		t.Errorf("coarser from every sector the care went %v, want %v then no further", cares, want)
	}
	cares = nil
	for p.Write().Finer() {
		cares = append(cares, p.Read(0).Care())
	}
	if want := []int{2, 3, 0}; !slices.Equal(cares, want) {
		t.Errorf("finer from a sector the care went %v, want %v then no further", cares, want)
	}
	var _ cameras.Lever = p.Write()
}
