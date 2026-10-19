package atmosphere

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/game/stage"
	"github.com/kjkrol/gram/internal/engine"
	"github.com/kjkrol/gram/plugins/atmosphere/backdrop"
	"github.com/kjkrol/gram/plugins/world"
)

type single struct{ stage game.Stage }

func (g single) Props() game.Props { return game.Props{} }
func (g single) Stages() (map[string]game.Stage, string) {
	return map[string]game.Stage{g.stage.Name(): g.stage}, g.stage.Name()
}

type painted func(d *backdrop.Dome)

var _ Sky = painted(nil)

func (s painted) Look(d *backdrop.Dome, _ *Plugin) { s(d) }

func skyStage(t *testing.T, figures func(a *Plugin), effects func(a *Plugin)) *Plugin {
	t.Helper()
	var a *Plugin
	st := stage.New("sky").
		World(world.Config{
			Space:    world.SpaceCfg{Width: 128, Height: 128},
			Entities: world.EntitiesCfg{MaxCount: 4, MinSize: 1, MaxSize: 20},
			Heights:  true,
		}).
		Plugins(func(w *world.Plugin) { a = NewPlugin(w, nil, Config{}) }).
		Figures(func() { figures(a) }).
		Effects(func() { effects(a) }).
		Update(func(goke.RunCtx, time.Duration) {})
	if err := engine.NewEngine(single{stage: st}).Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	return a
}

func TestSkies_ALaterSkyTakesThePartsItSetsAndLeavesTheRest(t *testing.T) {
	red := backdrop.Part{Source: []byte("fn SkyVault(s: SkyAt) -> vec3<f32> {\n" +
		"    return vec3<f32>(1.0, 0.0, 0.0);\n}\n")}
	var seen float32
	a := skyStage(t, func(a *Plugin) {
		a.Skies(
			painted(func(d *backdrop.Dome) { d.Vault, d.Moon.Size = red, 3 }),
			painted(func(d *backdrop.Dome) { seen, d.Vault = d.Moon.Size, backdrop.Part{} }))
	}, func(*Plugin) {})
	if seen != 3 {
		t.Errorf("the later sky looked at a moon of Size %v, want the earlier sky's 3", seen)
	}
	d := a.vault.Dome()
	if len(d.Vault.Source) != 0 || d.Moon.Size != 3 {
		t.Errorf("the sky drawn has a vault of %d bytes and a moon of Size %v, want the later "+
			"sky's none and the earlier's 3", len(d.Vault.Source), d.Moon.Size)
	}
	if !bytes.Equal(d.Sun.Source, backdrop.EarthSun().Source) {
		t.Error("a sun neither sky set is not the Earth's")
	}
}

func TestSkies_NoneIsTheEarths(t *testing.T) {
	a := skyStage(t, func(*Plugin) {}, func(*Plugin) {})
	d := a.vault.Dome()
	if !bytes.Equal(d.Vault.Source, backdrop.EarthVault().Source) ||
		len(d.Stars.Catalogue) == 0 {
		t.Error("without skies the atmosphere does not draw the Earth's")
	}
}

func TestSkies_AreNotedOnlyInTheFiguresLink(t *testing.T) {
	defer func() {
		if r, _ := recover().(string); !strings.Contains(r, "atmosphere: skies noted") {
			t.Errorf("skies noted in the Effects link: %q, want a panic", r)
		}
	}()
	skyStage(t, func(*Plugin) {}, func(a *Plugin) {
		a.Skies(painted(func(*backdrop.Dome) {}))
	})
}
