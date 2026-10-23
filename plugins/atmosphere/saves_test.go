package atmosphere_test

import (
	"math"
	"testing"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/game/stage"
	"github.com/kjkrol/gram/internal/engine"
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
		if math.Abs(float64(a.Cover-b.Cover)) > 1e-3 {
			t.Errorf("at %v the cover is %v, read back from a save %v", x, a.Cover, b.Cover)
		}
		clouded = clouded || a.Cover > 0.1
	}
	if !clouded {
		t.Error("a minute of weathers left no clouds in the sky to save")
	}
}
