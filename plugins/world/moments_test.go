package world_test

import (
	"testing"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/clock"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/gram/rule/effect"
)

// tick is a step of the moments' test.
const tick = time.Second / 10

// heard is the command the tests' rules give: which rule.
type heard struct{ Rule string }

// heards is where the heard commands land, for the world to carry.
type heards struct{ control.Queue[heard] }

func (h *heards) Queues() []control.CommandQueue     { return []control.CommandQueue{&h.Queue} }
func (h *heards) DefaultBindings() []control.Binding { return nil }

// Rules of the clock's Moment, of a role the world plays, fire once at their time and every period after
// their offset, on the clock's time — at any tempo and never in the pause — and an effect one
// applies lands on the clock's entity, switching a phase on until it ends.
func TestMoments_TriggersFireOnTheClocksTimeAtAnyTempo(t *testing.T) {
	for _, tempo := range []float32{1, 4, 0.5} {
		w := world.NewPlugin(world.Config{
			Space:    world.SpaceCfg{Width: 400, Height: 400},
			Entities: world.EntitiesCfg{MaxCount: 4, MinSize: 10, MaxSize: 10},
		})
		night := w.Kinds().DefineTag[clock.Phase]("night")
		fx := w.Effects()
		fx.Define("dusk", effect.Spec{effect.Lasts(2 * tick), effect.Grant(night)})
		dusk := fx.Named("dusk")
		var orders heards
		if err := w.Carry(&orders); err != nil {
			t.Fatal(err)
		}
		// the world plays the role whose rules these are; a role it does not play stays silent
		w.Roles().Define("clockwork",
			rule.Then[clock.Moment]("once", rule.All, rule.If(clock.At(5*tick), rule.Order(heard{Rule: "once"}))),
			rule.Then[clock.Moment]("daily", rule.All, rule.If(clock.Every(4*tick, 2*tick), rule.Order(heard{Rule: "daily"}))),
			rule.Then[clock.Moment]("dusk", rule.All, rule.If(clock.At(3*tick), rule.Apply(dusk))),
		)
		w.Plays(w.Roles().Named("clockwork"))
		w.Roles().Define("unplayed", rule.Then[clock.Moment]("never", rule.All, rule.Order(heard{Rule: "never"})))
		unplayed := w.Roles().Named("unplayed")
		fired := map[string][]time.Duration{}
		var inNight []bool

		ctx := &installCtx{ecs: goke.New()}
		if err := w.Install(ctx); err != nil {
			t.Fatal(err)
		}
		if err := ctx.Deliver(append(w.Kinds().Played(), unplayed)...); err != nil {
			t.Fatal(err)
		}
		var systems []goke.System
		for _, produce := range ctx.pending {
			systems = append(systems, produce()...)
		}
		ctx.ecs.Setup(systems...)
		ctx.ecs.SetPlan(func(rc goke.RunCtx, d time.Duration) {
			w.RunPlan(rc, d)
			w.Clock().Simulate(rc, func(_ goke.RunCtx, step time.Duration) {
				inNight = append(inNight, w.Clock().In(night))
				now := w.Clock().Time() + step // the step's end, the Moment's Now
				orders.Drain(func(i control.Issued[heard]) { fired[i.Command.Rule] = append(fired[i.Command.Rule], now) })
			})
			w.Clock().Replay(rc, d)
		})
		for _, cmd := range map[float32][]any{4: {world.Faster{}, world.Faster{}}, 0.5: {world.Slower{}}}[tempo] {
			if !w.Carrier().Put(1, cmd) {
				t.Fatalf("the world carries no %T", cmd)
			}
		}
		ticks := int(12 / tempo)
		for range ticks {
			ctx.ecs.Tick(tick)
		}
		if never := fired["never"]; len(never) != 0 {
			t.Errorf("tempo %g: the rule of a role the world does not play fired at %v, want never", tempo, never)
		}
		once, daily := fired["once"], fired["daily"]
		if len(once) != 1 || once[0] != 5*tick {
			t.Errorf("tempo %g: the once rule fired at %v, want once at %v", tempo, once, 5*tick)
		}
		if len(daily) != 3 || daily[0] != 2*tick || daily[1] != 6*tick || daily[2] != 10*tick {
			t.Errorf("tempo %g: the daily rule fired at %v, want 2, 6 and 10 ticks", tempo, daily)
		}
		// cast in the step ending at 3 ticks, begun the step after, over by 2 ticks later
		nights := 0
		for _, in := range inNight {
			if in {
				nights++
			}
		}
		if nights < 1 || nights > 3 {
			t.Errorf("tempo %g: the night phase held for %d steps of %v, want about 2", tempo, nights, inNight)
		}
	}
}
