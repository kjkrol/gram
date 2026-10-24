package atmosphere_test

import (
	"testing"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/game/stage"
	"github.com/kjkrol/gram/internal/engine"
	"github.com/kjkrol/gram/plugins/atmosphere"
	"github.com/kjkrol/gram/plugins/atmosphere/air"
	"github.com/kjkrol/gram/plugins/atmosphere/climate"
	"github.com/kjkrol/gram/plugins/atmosphere/climate/weather"
	"github.com/kjkrol/gram/plugins/world"
)

func TestRead_DrawnAirIsTheWeatherAsLastPublished(t *testing.T) {
	lasting := [2]time.Duration{time.Hour, time.Hour}
	grey := weather.State{Name: "grey", Clouds: [2]float32{0.9, 0.9}, Billow: [2]float32{1, 1},
		Lasts: lasting}
	clear := weather.State{Name: "clear", Lasts: lasting}
	var w *world.Plugin
	var a *atmosphere.Plugin
	var ecs *goke.ECS
	st := stage.New("sky").
		World(world.Config{Space: world.SpaceCfg{Width: 640, Height: 480},
			Entities: world.EntitiesCfg{MaxCount: 4, MinSize: 1, MaxSize: 20}, Heights: true}).
		Plugins(func(made *world.Plugin) {
			w, a = made, atmosphere.NewPlugin(made, nil, atmosphere.Config{
				Climate: climate.Config{Weathers: []weather.State{grey, clear}, Start: "grey"}})
		}).
		Effects(func(ctx game.Initializer) { ecs = ctx.ECS() }).
		Update(func(ctx goke.RunCtx, d time.Duration) {
			w.RunPlan(ctx, d)
			a.RunPlan(ctx, d)
		})
	if err := engine.NewEngine(oneStage{stage: st}).Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	run := func(d time.Duration) {
		for range int(d / (time.Second / 60)) {
			ecs.Tick(time.Second / 60)
		}
	}
	cover := func(w air.Weather) float32 { return w.At(320, 240).Cover }
	r := a.Read(0)
	run(time.Second)
	if r.DrawnAir().Dome != r.Air().Dome || cover(r.DrawnAir()) < 0.8 {
		t.Fatalf("before any publish the drawn weather covers %v, want the live grey one",
			cover(r.DrawnAir()))
	}
	frame := func() {
		a.Publish()
		a.Swap()
	}
	frame()
	was := cover(r.DrawnAir())
	if r.DrawnAir().Dome == r.Air().Dome || was != cover(r.Air()) {
		t.Fatalf("published, the drawn weather covers %v on the live dome %v; want the live %v on "+
			"a copy", was, r.DrawnAir().Dome == r.Air().Dome, cover(r.Air()))
	}
	a.Write().Climate().Set("clear")
	run(3 * time.Second)
	if cover(r.Air()) > was/2 {
		t.Fatalf("set clear, the air still covers %v", cover(r.Air()))
	}
	if got := cover(r.DrawnAir()); got != was {
		t.Errorf("cleared after the frame was published, the drawn weather covers %v, want %v "+
			"till the next", got, was)
	}
	frame()
	if got := cover(r.DrawnAir()); got != cover(r.Air()) {
		t.Errorf("published again, the drawn weather covers %v, want the live %v", got,
			cover(r.Air()))
	}
	// the buffer drawn next held the grey sky, two frames ago: it catches up with nothing new
	frame()
	if got := cover(r.DrawnAir()); got != cover(r.Air()) {
		t.Errorf("the buffer two frames old covers %v, want it caught up: %v", got, cover(r.Air()))
	}
}
