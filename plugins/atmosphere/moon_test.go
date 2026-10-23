package atmosphere_test

import (
	"testing"
	"time"

	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/act"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/ceremony/cond"
	"github.com/kjkrol/ceremony/effect"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/game/stage"
	"github.com/kjkrol/gram/internal/engine"
	"github.com/kjkrol/gram/plugins/atmosphere"
	"github.com/kjkrol/gram/plugins/atmosphere/calendar"
	"github.com/kjkrol/gram/plugins/atmosphere/sky"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

type night struct {
	w   *world.Plugin
	a   *atmosphere.Plugin
	ecs *goke.ECS
}

func newNight(t *testing.T, cfg atmosphere.Config, define func(n *night),
	rules func(n *night)) *night {
	t.Helper()
	n := &night{}
	st := stage.New("night").
		World(world.Config{Space: world.SpaceCfg{Width: 128, Height: 128}, Entities: world.EntitiesCfg{MaxCount: 4, MinSize: 1, MaxSize: 20}, Heights: true}).
		Plugins(func(w *world.Plugin) { n.w, n.a = w, atmosphere.NewPlugin(w, nil, cfg) }).
		Effects(func(ctx game.Initializer) { n.ecs = ctx.ECS(); define(n) }).
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

func TestMoon_AGamesOwnLightsTheNightFromTheStart(t *testing.T) {
	red := render.Light{1, 0.2, 0.2}
	cfg := atmosphere.Config{Calendar: calendar.Config{Start: time.Hour}} // one in the morning
	cfg.Sky.Moon = sky.Moon{Color: red, Strength: 0.5, Face: render.Light{1, 1, 1}}
	n := newNight(t, cfg, func(*night) {}, nil)
	for range 3 {
		n.ecs.Tick(tick)
	}
	if got := n.a.Read(0).Sun().Color; got != red {
		t.Errorf("under the game's own moon the night is lit %v, want %v", got, red)
	}
}

func TestMoon_AnEffectOnTheAtmosphereColoursTheNight(t *testing.T) {
	red := render.Light{1, 0.2, 0.2}
	var blood effect.Effect
	early := atmosphere.Config{Calendar: calendar.Config{Start: time.Hour}} // one in the morning
	n := newNight(t, early, func(n *night) {
		n.w.Write().Effects().Define("blood moon",
			effect.Spec{effect.Alter(func(m *sky.Moon) { m.Color = red })})
		blood = n.w.Write().Effects().Named("blood moon")
	}, nil)
	n.ecs.Tick(tick)
	pale := n.a.Read(0).Sun().Color
	if pale != sky.DefaultMoon().Color {
		t.Fatalf("the night is lit %v, want the moon's own %v", pale, sky.DefaultMoon().Color)
	}
	give := func(cmd command.Command) {
		t.Helper()
		if !n.w.Write().Carrier().Put(command.Nobody, cmd) {
			t.Fatalf("the world carries no %T", cmd)
		}
		for range 3 {
			n.ecs.Tick(tick)
		}
	}
	give(ceremony.Put(blood).On(n.a))
	if got := n.a.Read(0).Sun().Color; got != red {
		t.Errorf("under the blood moon the night is lit %v, want %v", got, red)
	}
	give(ceremony.Take(blood).On(n.a))
	if got := n.a.Read(0).Sun().Color; got != pale {
		t.Errorf("the blood moon lifted, the night is lit %v, want the %v it was", got, pale)
	}
}

func TestMoonrise_FiresOnceARiseAndFullPicksTheFullOnes(t *testing.T) {
	var risen, full effect.Effect
	day := 2 * time.Second // 120 ticks; the moon of a GameYear goes round in four days
	cfg := atmosphere.Config{Calendar: calendar.Config{DayRealTime: day}}
	n := newNight(t, cfg, func(n *night) {
		n.w.Write().Effects().Define("risen", effect.Spec{effect.Lasts(4 * tick)})
		risen = n.w.Write().Effects().Named("risen")
		n.w.Write().Effects().Define("full risen", effect.Spec{effect.Lasts(4 * tick)})
		full = n.w.Write().Effects().Named("full risen")
	}, func(n *night) {
		n.w.Roles().Define("moon watcher",
			act.Branch[sky.Moonrise]{Name: "a rise", When: cond.Self.If(cond.Always), Then: act.Self.Put(risen)},
			act.Branch[sky.Moonrise]{
				Name: "a full rise",
				When: cond.Self.If(sky.FullMoon()),
				Then: act.Self.Put(full),
			})
		n.a.Plays(n.w.Roles().Named("moon watcher"))
	})
	rises, fulls := 0, 0
	wasRisen, wasFull := false, false
	for range 8 * 120 { // eight days: the moon twice round
		n.ecs.Tick(tick)
		r, f := n.w.Read(0).Effects().Has(n.a.Entity(),
			risen), n.w.Read(0).Effects().Has(n.a.Entity(), full)
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
