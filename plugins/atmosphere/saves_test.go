package atmosphere_test

import (
	"bytes"
	"encoding/gob"
	"testing"
	"time"

	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/game/stage"
	"github.com/kjkrol/gram/internal/engine"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/plugins/atmosphere"
	"github.com/kjkrol/gram/plugins/atmosphere/climate"
	"github.com/kjkrol/gram/plugins/atmosphere/climate/weather"
	"github.com/kjkrol/gram/plugins/world"
)

func TestAtmosphere_TheAirMassesOutlastASaveThroughTheEngine(t *testing.T) {
	t.Chdir(t.TempDir())
	cfg := atmosphere.Config{Climate: climate.Config{Weathers: []weather.State{
		{Name: "clear", Wind: [2]float32{40, 40},
			Lasts: [2]time.Duration{time.Second, time.Second}},
		{Name: "cloudy", Wind: [2]float32{40, 40}, Clouds: [2]float32{0.9, 0.9},
			Lasts: [2]time.Duration{time.Second, time.Second}},
	}, Start: "clear", Horizon: 2000}}
	type sky struct {
		a   *atmosphere.Plugin
		ecs *goke.ECS
	}
	run := func() (*engine.Engine, *sky) {
		s := &sky{}
		st := stage.New("sky").
			World(world.Config{Space: world.SpaceCfg{Width: 640, Height: 480},
				Entities: world.EntitiesCfg{MaxCount: 4, MinSize: 1, MaxSize: 20}, Heights: true}).
			Plugins(func(w *world.Plugin) { s.a = atmosphere.NewPlugin(w, nil, cfg) }).
			Effects(func(ctx game.Initializer) { s.ecs = ctx.ECS() }).
			Update(func(ctx goke.RunCtx, d time.Duration) { s.a.RunPlan(ctx, d) })
		return engine.NewEngine(oneStage{stage: st}), s
	}
	eng, was := run()
	if err := eng.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	// weathers as long as the wind of 40 takes to cross the world, a cloudy one now and then
	for range 60 * 60 {
		was.ecs.Tick(time.Second / 60)
	}
	if err := eng.Save(""); err != nil {
		t.Fatalf("Save: %v", err)
	}
	back, now := run()
	if err := back.Load("", ""); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := back.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	was.ecs.Tick(time.Second / 60)
	now.ecs.Tick(time.Second / 60)
	clouded := false
	// from the horizon the wind blows from, where the clouds are born, over the world
	for x := float32(-6000); x < 1600; x += 40 {
		a, b := was.a.Read(0).Air().At(x, 240), now.a.Read(0).Air().At(x, 240)
		if f32.Abs(float32(a.Cover-b.Cover)) > 1e-3 {
			t.Errorf("at %v the cover is %v, read back from a save %v", x, a.Cover, b.Cover)
		}
		clouded = clouded || a.Cover > 0.1
	}
	if !clouded {
		t.Error("a minute of weathers left no clouds in the sky to save")
	}
}

func shadeSwitches(t *testing.T, a *atmosphere.Plugin) (ground, figures, high control.Switch) {
	t.Helper()
	labelled := map[string]control.Switch{}
	for _, s := range a.Settings() {
		if sw, ok := s.(control.Switch); ok {
			labelled[sw.Label] = sw
		}
	}
	ground = labelled["Clouds shade the ground"]
	figures = labelled["Clouds shade the figures"]
	high = labelled["High clouds' shadows"]
	if ground.Get == nil || figures.Get == nil || high.Get == nil {
		t.Fatal("the settings lack a switch of the clouds' shadows")
	}
	return ground, figures, high
}

func TestCloudShadows_SwitchedOnTheSettingsScreenOutlastASave(t *testing.T) {
	t.Chdir(t.TempDir())
	type sky struct {
		a   *atmosphere.Plugin
		ecs *goke.ECS
	}
	run := func() (*engine.Engine, *sky) {
		s := &sky{}
		var w *world.Plugin
		st := stage.New("sky").
			World(world.Config{
				Space:    world.SpaceCfg{Width: 128, Height: 128},
				Entities: world.EntitiesCfg{MaxCount: 4, MinSize: 1, MaxSize: 20},
				Heights:  true,
			}).
			Plugins(func(made *world.Plugin) {
				w, s.a = made, atmosphere.NewPlugin(made, nil, atmosphere.Config{})
			}).
			Effects(func(ctx game.Initializer) { s.ecs = ctx.ECS() }).
			Update(func(ctx goke.RunCtx, d time.Duration) {
				w.RunPlan(ctx, d)
				s.a.RunPlan(ctx, d)
			})
		return engine.NewEngine(oneStage{stage: st}), s
	}
	eng, was := run()
	if err := eng.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	ground, figures, high := shadeSwitches(t, was.a)
	if !ground.Get() || !figures.Get() || !high.Get() {
		t.Fatalf("the clouds start shading the ground %v, the figures %v, with the high ones %v; "+
			"want all", ground.Get(), figures.Get(), high.Get())
	}
	for _, cmd := range []command.Command{ground.Set(false), high.Set(false)} {
		if err := eng.Issue(cmd); err != nil {
			t.Fatal(err)
		}
	}
	was.ecs.Tick(time.Second / 60)
	want := atmosphere.CloudShadows{Figures: true}
	if got := was.a.Read(0).CloudShadows(); got != want || ground.Get() || !figures.Get() ||
		high.Get() {
		t.Errorf("switched off on the ground and for the high clouds, the shadows fall %+v and "+
			"the switches say %v, %v, %v; want %+v", got, ground.Get(), figures.Get(), high.Get(),
			want)
	}
	if w := was.a.Read(0).Air(); w.ShadeGround || !w.ShadeFigures || w.HighShade {
		t.Errorf("the weather shades the ground %v, the figures %v, with the high clouds %v; "+
			"want the figures alone", w.ShadeGround, w.ShadeFigures, w.HighShade)
	}
	if err := eng.Save(""); err != nil {
		t.Fatalf("Save: %v", err)
	}
	back, now := run()
	if err := back.Load("", ""); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := back.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if got := now.a.Read(0).CloudShadows(); got != want {
		t.Errorf("loaded, the clouds' shadows fall %+v, want %+v as saved", got, want)
	}
}

func TestCloudShadows_AllFallFromASaveMadeBeforeTheyWereSaved(t *testing.T) {
	// the atmosphere's Settings as a save held them before the clouds' shadows were among them
	type before struct {
		Frozen         bool
		Hour           float32
		Clouds         atmosphere.CloudLook
		CloudReach     float32
		CloudCoarse    int
		HighShade      bool
		AdaptiveClouds bool
	}
	var saved bytes.Buffer
	err := gob.NewEncoder(&saved).Encode(before{Clouds: atmosphere.LowBodies, HighShade: true})
	if err != nil {
		t.Fatal(err)
	}
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 128, Height: 128},
		Entities: world.EntitiesCfg{MaxCount: 4, MinSize: 1, MaxSize: 20},
		Heights:  true,
	})
	a := atmosphere.NewPlugin(w, nil, atmosphere.Config{})
	loading := a.Serializable().Persisted()
	*loading[0].(*[]byte) = saved.Bytes()
	a.Restore()
	all := atmosphere.CloudShadows{Ground: true, Figures: true, High: true}
	if got := a.Read(0).CloudShadows(); got != all ||
		a.Read(0).ShownClouds() != atmosphere.LowBodies {
		t.Errorf("loaded from a save of before, the shadows fall %+v and the clouds look %v; want "+
			"all of them, as the game starts, and the look saved, the low as bodies", got,
			a.Read(0).ShownClouds())
	}
}
