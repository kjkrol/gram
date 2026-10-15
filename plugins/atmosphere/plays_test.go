package atmosphere_test

import (
	"testing"
	"time"

	"github.com/kjkrol/ceremony/act"
	"github.com/kjkrol/ceremony/cond"
	"github.com/kjkrol/ceremony/effect"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/game/stage"
	"github.com/kjkrol/gram/internal/engine"
	"github.com/kjkrol/gram/plugins/atmosphere"
	"github.com/kjkrol/gram/plugins/atmosphere/climate"
	"github.com/kjkrol/gram/plugins/world"
)

type oneStage struct{ stage game.Stage }

func (g oneStage) Props() game.Props { return game.Props{} }
func (g oneStage) Stages() (map[string]game.Stage, string) {
	return map[string]game.Stage{g.stage.Name(): g.stage}, g.stage.Name()
}

func TestPlays_ARuleOfTheWeatherFiresForTheAtmospheresOwnEntity(t *testing.T) {
	for _, plays := range []bool{true, false} {
		var w *world.Plugin
		var a *atmosphere.Plugin
		var noted effect.Effect
		var ecs *goke.ECS
		st := stage.New("sky").
			World(world.Config{Space: world.SpaceCfg{Width: 128, Height: 128}, Entities: world.EntitiesCfg{MaxCount: 4, MinSize: 1, MaxSize: 20}, Heights: true}).
			Plugins(func(made *world.Plugin) { w, a = made, atmosphere.NewPlugin(made, atmosphere.Config{}) }).
			Effects(func(ctx game.Initializer) {
				ecs = ctx.ECS()
				w.Effects().Define("noted", effect.Spec{})
				noted = w.Effects().Named("noted")
			}).
			Rules(func() {
				w.Roles().Define("weatherwise",
					act.Branch[climate.Weathering]{
						Name: "note the weather",
						When: cond.Self.If(cond.Always),
						Then: act.Self.Hold(noted),
					})
				weatherwise := w.Roles().Named("weatherwise")
				if plays {
					a.Plays(weatherwise)
				} else {
					w.Plays(weatherwise) // delivered, but the weather is not the world's
				}
			}).
			Update(func(ctx goke.RunCtx, d time.Duration) {
				w.RunPlan(ctx, d)
				a.RunPlan(ctx, d)
			})
		if err := engine.NewEngine(oneStage{stage: st}).Init(); err != nil {
			t.Fatalf("Init: %v", err)
		}
		for range 3 {
			ecs.Tick(time.Second / 60)
		}
		if a.Entity() == w.Entity() {
			t.Fatalf("the atmosphere's entity is the world's, %d", w.Entity())
		}
		if got := w.Effects().Has(a.Entity(), noted); got != plays {
			t.Errorf("played by the atmosphere %v: its entity is noted %v", plays, got)
		}
		if w.Effects().Has(w.Entity(), noted) {
			t.Errorf("played by the atmosphere %v: the world's entity is noted, want never", plays)
		}
	}
}
