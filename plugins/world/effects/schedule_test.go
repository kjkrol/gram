package effects_test

import (
	"testing"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/clock"
	"github.com/kjkrol/gram/plugins/world/effects"
)

// A schedule's entries come once at their moment and every period after their offset, on the
// clock's time — at any tempo and never in the pause — and an effect a point casts on the clock's
// entity switches a phase on until it ends.
func TestSchedule_EntriesComeOnTheClocksTimeAtAnyTempo(t *testing.T) {
	for _, tempo := range []float32{1, 4, 0.5} {
		w := world.NewPlugin(world.Config{
			Space:    world.SpaceCfg{Width: 400, Height: 400},
			Entities: world.EntitiesCfg{MaxCount: 4, MinSize: 10, MaxSize: 10},
		})
		night := w.Kinds().DefineTag[clock.Phase]("night")
		fx := w.Effects()
		dusk := fx.Define("dusk", effects.Spec{effects.Lasts(2 * tick), effects.Grant(night)})
		var once, daily []time.Duration
		fx.Schedule().At(5*tick, func(t plugin.Tick) { once = append(once, w.Clock().Time()+t.Dt) })
		fx.Schedule().Every(4*tick, 2*tick, func(t plugin.Tick) { daily = append(daily, w.Clock().Time()+t.Dt) })
		fx.Schedule().At(3*tick, func(t plugin.Tick) { fx.Cast(t.CmdBuf, w.Clock().Entity(), dusk) })
		var inNight []bool

		ctx := &installCtx{ecs: goke.New()}
		if err := w.Install(ctx); err != nil {
			t.Fatal(err)
		}
		var systems []goke.System
		for _, produce := range ctx.pending {
			systems = append(systems, produce()...)
		}
		ctx.ecs.Setup(systems...)
		ctx.ecs.SetPlan(func(rc goke.RunCtx, d time.Duration) {
			w.RunPlan(rc, d)
			w.Clock().Simulate(rc, func(goke.RunCtx, time.Duration) { inNight = append(inNight, w.Clock().In(night)) })
			w.Clock().Replay(rc, d)
		})
		if err := w.Clock().SetTempo(tempo); err != nil {
			t.Fatal(err)
		}
		ticks := int(12 / tempo)
		for range ticks {
			ctx.ecs.Tick(tick)
		}
		if len(once) != 1 || once[0] != 5*tick {
			t.Errorf("tempo %g: the once entry came at %v, want once at %v", tempo, once, 5*tick)
		}
		if len(daily) != 3 || daily[0] != 2*tick || daily[1] != 6*tick || daily[2] != 10*tick {
			t.Errorf("tempo %g: the daily entry came at %v, want 2, 6 and 10 ticks", tempo, daily)
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
