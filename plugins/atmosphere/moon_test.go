package atmosphere_test

import (
	"testing"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/game/stage"
	"github.com/kjkrol/gram/internal/engine"
	"github.com/kjkrol/gram/plugins/atmosphere"
	"github.com/kjkrol/gram/plugins/atmosphere/calendar"
	"github.com/kjkrol/gram/plugins/atmosphere/sky"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/gram/rule/effect"
)

// night is a world under an atmosphere of cal, with whatever define and rules add, set up and
// ready to tick.
type night struct {
	w   *world.Plugin
	a   *atmosphere.Plugin
	ecs *goke.ECS
}

func newNight(t *testing.T, cal calendar.Config, define func(n *night), rules func(n *night)) *night {
	t.Helper()
	n := &night{}
	st := stage.New("night").
		Plugins(func(ctx game.Initializer) error {
			n.w = ctx.UseWorld(world.Config{Space: world.SpaceCfg{Width: 128, Height: 128}, Entities: world.EntitiesCfg{MaxCount: 4, MinSize: 1, MaxSize: 20}})
			n.a, n.ecs = atmosphere.NewPlugin(n.w, atmosphere.Config{Calendar: cal}), ctx.ECS()
			return ctx.Use(n.a)
		}).
		Effects(func() { define(n) }).
		Rules(func() {
			if rules != nil {
				rules(n)
			}
		}).
		Update(func(ctx goke.RunCtx, d time.Duration) {
			n.w.RunPlan(ctx, d)
			n.a.RunPlan(ctx, d)
		})
	if err := engine.NewEngine(oneStage{stage: st}).Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	return n
}

const tick = time.Second / 60

// The moon is a knob of the atmosphere: an effect on the atmosphere turns the light of the night
// to the colour it says, and its end gives the moon's own back.
func TestMoon_AnEffectOnTheAtmosphereColoursTheNight(t *testing.T) {
	red := render.Light{1, 0.2, 0.2}
	var blood effect.Effect
	n := newNight(t, calendar.Config{Start: time.Hour}, func(n *night) { // one in the morning
		n.w.Effects().Define("blood moon", effect.Spec{effect.Alter(func(m *sky.Moon) { m.Color = red })})
		blood = n.w.Effects().Named("blood moon")
	}, nil)
	n.ecs.Tick(tick)
	pale := n.a.Sun().Color
	if pale != sky.DefaultMoon().Color {
		t.Fatalf("the night is lit %v, want the moon's own %v", pale, sky.DefaultMoon().Color)
	}
	give := func(cmd any) {
		t.Helper()
		if !n.w.Commands().Put(control.Nobody, cmd) {
			t.Fatalf("the world carries no %T", cmd)
		}
		for range 3 {
			n.ecs.Tick(tick)
		}
	}
	give(rule.Cast(blood).On(n.a))
	if got := n.a.Sun().Color; got != red {
		t.Errorf("under the blood moon the night is lit %v, want %v", got, red)
	}
	give(rule.Lift(blood).On(n.a))
	if got := n.a.Sun().Color; got != pale {
		t.Errorf("the blood moon lifted, the night is lit %v, want the %v it was", got, pale)
	}
}

// The moon's rising is a moment of the atmosphere: its rules fire once a rise while the
// atmosphere plays their role, and Full picks the rises of a full moon — some, not all.
func TestMoonrise_FiresOnceARiseAndFullPicksTheFullOnes(t *testing.T) {
	var risen, full effect.Effect
	day := 2 * time.Second // 120 ticks; the moon of a GameYear goes round in four days
	n := newNight(t, calendar.Config{Day: day}, func(n *night) {
		n.w.Effects().Define("risen", effect.Spec{effect.Lasts(4 * tick)})
		risen = n.w.Effects().Named("risen")
		n.w.Effects().Define("full risen", effect.Spec{effect.Lasts(4 * tick)})
		full = n.w.Effects().Named("full risen")
	}, func(n *night) {
		n.a.Plays(rule.Role("moon watcher").Obeys(
			rule.Then[sky.Moonrise]("a rise", rule.All, rule.Apply(risen)),
			rule.Then[sky.Moonrise]("a full rise", rule.All, rule.If(sky.Moonrise.Full, rule.Apply(full)))))
	})
	rises, fulls := 0, 0
	wasRisen, wasFull := false, false
	for range 8 * 120 { // eight days: the moon twice round
		n.ecs.Tick(tick)
		r, f := risen.On(n.a.Entity()), full.On(n.a.Entity())
		if r && !wasRisen {
			rises++
		}
		if f && !wasFull {
			fulls++
		}
		wasRisen, wasFull = r, f
	}
	if rises < 5 || rises > 8 {
		t.Errorf("in eight days the moon rose %d times, want about six: once a day, a day lost each time round", rises)
	}
	if fulls < 2 || fulls >= rises {
		t.Errorf("of %d rises %d were of a full moon, want two at least — one each time round — and not all", rises, fulls)
	}
}
