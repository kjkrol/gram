package cameras_test

import (
	"testing"

	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/lod"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/world"
)

func sectoredConfig() world.Config {
	return world.Config{
		Space:    world.SpaceCfg{Width: 1000, Height: 1000},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 10, MaxSize: 10},
		Sectors:  world.SectorsCfg{Buckets: 1},
	}
}

func TestDetail_GoesCoarserAfterTheLeversBeforeTheResolutionAndFinerAfterIt(t *testing.T) {
	d := &dial{at: 1, following: true}
	p := cameras.NewPlugin(world.NewPlugin(sectoredConfig()), cameras.Config{
		Levers:   []cameras.Lever{d},
		Tactical: cameras.Knobs{Strategy: cameras.Uniform},
	})
	cam := p.Write().New(cameras.FromAbove, camera.Config{})
	minimap := p.Write().New(cameras.FromAbove, camera.Config{Whole: true})
	for i, want := range []struct {
		factor float32
		dial   int
		shift  lod.Level
		res    float32
	}{
		{cameras.ResolutionStep, 0, 0, 1},
		{cameras.ResolutionStep, 0, 1, 1},
		{cameras.ResolutionStep, 0, 2, 1},
		{cameras.ResolutionStep, 0, 2, cameras.ResolutionStep},
		{1 / cameras.ResolutionStep, 1, 2, cameras.ResolutionStep},
		{1 / cameras.ResolutionStep, 2, 2, cameras.ResolutionStep},
		{1 / cameras.ResolutionStep, 3, 2, cameras.ResolutionStep},
		{1 / cameras.ResolutionStep, 3, 2, 1},
		{1 / cameras.ResolutionStep, 3, 1, 1},
		{1 / cameras.ResolutionStep, 3, 0, 1},
		{1 / cameras.ResolutionStep, 3, 0, 1},
	} {
		p.Redraw(want.factor)
		if d.at != want.dial || camera.LevelShiftOf(cam) != want.shift ||
			camera.ResolutionOf(cam) != want.res {
			t.Fatalf("step %d by %v: the dial at %d, the picture %v levels coarser at %v; want "+
				"%d, %v at %v", i+1, want.factor, d.at, camera.LevelShiftOf(cam),
				camera.ResolutionOf(cam), want.dial, want.shift, want.res)
		}
		if i == 3 {
			if got := report(p); got != "Picture: tactical\n0.85 of the pixels\nthe figures "+
				"sharp\nthe detail 2 levels coarser\nfollowing a budget of 60 FPS" {
				t.Errorf("shifted twice and stepped the telemetry says %q", got)
			}
			if got := camera.LevelShiftOf(minimap); got != 0 {
				t.Errorf("the minimap is drawn %v levels coarser, want as the world sees it", got)
			}
			late := p.Write().New(cameras.FromAbove, camera.Config{})
			if got := camera.LevelShiftOf(late); got != 2 {
				t.Errorf("a camera made while shifted is drawn %v levels coarser, want 2", got)
			}
			cam.(camera.Snapped).Snap()
			if got := camera.LevelShiftOf(camera.DrawnOf(cam)); got != 2 {
				t.Errorf("the frame's copy is drawn %v levels coarser, want the camera's 2", got)
			}
		}
	}
}

func TestDetail_StillKeepsTheLevelsAndTheResolutionStepsAsBefore(t *testing.T) {
	w, p, start, tick := worldOf(t, sectoredConfig(),
		cameras.Config{Tactical: cameras.Knobs{Strategy: cameras.Uniform}})
	cam := p.Write().New(cameras.FromAbove, camera.Config{})
	start()
	p.Redraw(cameras.ResolutionStep)
	if camera.LevelShiftOf(cam) != 1 || !p.Read(0).DetailFollows() {
		t.Fatalf("following, a coarser step draws the picture %v levels coarser, want 1",
			camera.LevelShiftOf(cam))
	}
	w.Write().Carrier().Put(command.Nobody, cameras.Detail{Follows: false})
	tick()
	if camera.LevelShiftOf(cam) != 0 || p.Read(0).DetailFollows() {
		t.Fatalf("stilled, the picture is drawn %v levels coarser, want as the world sees it",
			camera.LevelShiftOf(cam))
	}
	p.Redraw(cameras.ResolutionStep)
	if camera.LevelShiftOf(cam) != 0 || camera.ResolutionOf(cam) != cameras.ResolutionStep {
		t.Errorf("stilled, a coarser step draws the picture %v levels coarser at %v, want the "+
			"resolution stepped alone", camera.LevelShiftOf(cam), camera.ResolutionOf(cam))
	}
	p.Redraw(1 / cameras.ResolutionStep)
	p.Redraw(1 / cameras.ResolutionStep)
	if camera.LevelShiftOf(cam) != 0 || camera.ResolutionOf(cam) != 1 {
		t.Errorf("stilled, finer steps leave the picture %v levels coarser at %v, want at every "+
			"pixel as the world sees it", camera.LevelShiftOf(cam), camera.ResolutionOf(cam))
	}
}

func TestDetail_FollowsNothingInAWorldOfNoSectors(t *testing.T) {
	p := cameras.NewPlugin(newWorld(),
		cameras.Config{Tactical: cameras.Knobs{Strategy: cameras.Uniform}})
	cam := p.Write().New(cameras.FromAbove, camera.Config{})
	if p.Read(0).DetailFollows() {
		t.Error("the detail follows the frame budget in a world of no sectors, drawn by no level")
	}
	p.Redraw(cameras.ResolutionStep)
	if camera.LevelShiftOf(cam) != 0 || camera.ResolutionOf(cam) != cameras.ResolutionStep {
		t.Errorf("a coarser step draws the picture %v levels coarser at %v, want the resolution "+
			"stepped", camera.LevelShiftOf(cam), camera.ResolutionOf(cam))
	}
	for _, s := range p.Settings() {
		if labelOf(s) == "Detail follows the frame budget" {
			t.Error("a world of no sectors lists the detail following the frame budget")
		}
	}
}

func TestSettings_TheDetailFollowingTheBudgetIsListedBesideItInAWorldOfSectors(t *testing.T) {
	settings := cameras.NewPlugin(world.NewPlugin(sectoredConfig()), cameras.Config{}).Settings()
	if got := labelOf(settings[3]); got != "Frame budget" {
		t.Fatalf("setting 4 is %q, want the frame budget", got)
	}
	detail, ok := settings[4].(control.Switch)
	if !ok || detail.Label != "Detail follows the frame budget" ||
		detail.Section != control.Picture {
		t.Fatalf("setting 5 is %#v, want the detail's switch under %q", settings[4],
			control.Picture)
	}
	if !detail.Get() || detail.Set(false) != (cameras.Detail{Follows: false}) {
		t.Errorf("the detail follows %v building %v, want on, Detail{Follows: false}",
			detail.Get(), detail.Set(false))
	}
}

func TestDetail_IsSavedAndALoadDrawsThePicturesAsTheWorldSeesThem(t *testing.T) {
	sectored := func() *cameras.Plugin {
		return cameras.NewPlugin(world.NewPlugin(sectoredConfig()),
			cameras.Config{Tactical: cameras.Knobs{Strategy: cameras.Uniform}})
	}
	still := sectored()
	still.Write().AdaptDetail(false)
	loaded := sectored()
	carried(t, still, loaded)
	if loaded.Read(0).DetailFollows() {
		t.Error("the detail follows the frame budget after a load that saved it still")
	}
	following := sectored()
	loaded = sectored()
	cam := loaded.Write().New(cameras.FromAbove, camera.Config{})
	loaded.Redraw(cameras.ResolutionStep)
	carried(t, following, loaded)
	if !loaded.Read(0).DetailFollows() || camera.LevelShiftOf(cam) != 0 {
		t.Errorf("after a load that saved it following the detail follows %v, the picture %v "+
			"levels coarser; want following, as the world sees it", loaded.Read(0).DetailFollows(),
			camera.LevelShiftOf(cam))
	}
}
