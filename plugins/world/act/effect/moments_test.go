package effect_test

import (
	"testing"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/act"
	"github.com/kjkrol/gram/plugins/world/act/effect"
	"github.com/kjkrol/gram/plugins/world/clock"
)

// Triggers of the clock's Moment fire once at their time and every period after their offset, on
// the clock's time — at any tempo and never in the pause — and an effect one casts on the clock's
// entity switches a phase on until it ends.
func TestMoments_TriggersFireOnTheClocksTimeAtAnyTempo(t *testing.T) {
	for _, tempo := range []float32{1, 4, 0.5} {
		w := world.NewPlugin(world.Config{
			Space:    world.SpaceCfg{Width: 400, Height: 400},
			Entities: world.EntitiesCfg{MaxCount: 4, MinSize: 10, MaxSize: 10},
		})
		night := w.Kinds().DefineTag[clock.Phase]("night")
		fx := w.Effects()
		dusk := fx.Define("dusk", effect.Spec{effect.Lasts(2 * tick), effect.Grant(night)})
		var once, daily []time.Duration
		var r act.Reaction[clock.Moment]
		err := w.Hook(
			act.Trigger[clock.Moment]("once").Do(r.If(clock.At(5*tick),
				r.Run(func(_ plugin.Tick, m clock.Moment) { once = append(once, m.Now) }))),
			act.Trigger[clock.Moment]("daily").Do(r.If(clock.Every(4*tick, 2*tick),
				r.Run(func(_ plugin.Tick, m clock.Moment) { daily = append(daily, m.Now) }))),
			act.Trigger[clock.Moment]("dusk").Do(r.If(clock.At(3*tick),
				r.Run(func(t plugin.Tick, _ clock.Moment) { dusk.Cast(t.CmdBuf, w.Clock().Entity()) }))),
			// of no entity: casting on it fails, and nothing is cast on entity 0
			act.Trigger[clock.Moment]("nobody").Do(r.Apply(dusk)),
		)
		if err != nil {
			t.Fatal(err)
		}
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
			t.Errorf("tempo %g: the once trigger fired at %v, want once at %v", tempo, once, 5*tick)
		}
		if len(daily) != 3 || daily[0] != 2*tick || daily[1] != 6*tick || daily[2] != 10*tick {
			t.Errorf("tempo %g: the daily trigger fired at %v, want 2, 6 and 10 ticks", tempo, daily)
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
