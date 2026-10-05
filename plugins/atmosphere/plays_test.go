package atmosphere_test

import (
	"testing"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/game/stage"
	"github.com/kjkrol/gram/internal/engine"
	"github.com/kjkrol/gram/plugins/atmosphere"
	"github.com/kjkrol/gram/plugins/atmosphere/climate"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/gram/rule/effect"
)

// oneStage is a game of one Stage.
type oneStage struct{ stage game.Stage }

func (g oneStage) Props() game.Props { return game.Props{} }
func (g oneStage) Stages() (map[string]game.Stage, string) {
	return map[string]game.Stage{g.stage.Name(): g.stage}, g.stage.Name()
}

// The atmosphere is an entity of its own: a rule of the weather fires while the atmosphere plays
// its role, about that entity, so the effect it applies is a state of the atmosphere, not of the
// world; a role it does not play is silent.
func TestPlays_ARuleOfTheWeatherFiresForTheAtmospheresOwnEntity(t *testing.T) {
	for _, plays := range []bool{true, false} {
		var w *world.Plugin
		var a *atmosphere.Plugin
		var noted effect.Effect
		var ecs *goke.ECS
		st := stage.New("sky").
			Plugins(func(ctx game.Initializer) error {
				w = ctx.UseWorld(world.Config{Space: world.SpaceCfg{Width: 128, Height: 128}, Entities: world.EntitiesCfg{MaxCount: 4, MinSize: 1, MaxSize: 20}})
				a, ecs = atmosphere.NewPlugin(w, atmosphere.Config{}), ctx.ECS()
				return ctx.Use(a)
			}).
			Effects(func() {
				w.Effects().Define("noted", effect.Spec{})
				noted = w.Effects().Named("noted")
			}).
			Rules(func() {
				w.Roles().Define("weatherwise",
					rule.Then[climate.Weathering]("note the weather", rule.All, rule.Keep(noted)))
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
		if got := noted.On(a.Entity()); got != plays {
			t.Errorf("played by the atmosphere %v: its entity is noted %v", plays, got)
		}
		if noted.On(w.Entity()) {
			t.Errorf("played by the atmosphere %v: the world's entity is noted, want never", plays)
		}
	}
}
