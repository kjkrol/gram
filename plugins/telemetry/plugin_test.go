package telemetry_test

import (
	"strings"
	"testing"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/plugins/telemetry"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/uid"
)

type stubReporter struct {
	inited bool
	label  string
}

func (r *stubReporter) Init(*goke.SysInit) { r.inited = true }
func (r *stubReporter) Report(line func(label, value string)) {
	line(r.label, "yes")
}

func newWorld() *world.Plugin {
	return world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 100, Height: 100},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10},
	})
}

func text(p *telemetry.Plugin) string {
	s, _ := p.Scenes()[0].(interface {
		Text(uid.UID64, bool) (string, bool)
	}).Text(0, false)
	return s
}

func TestPlugin_HasTheStandardLinesAndTakesMore(t *testing.T) {
	w := newWorld()
	p := telemetry.NewPlugin(w)
	if telemetry.Of(w) != p {
		t.Fatal("Of does not find the plugin made over the world")
	}
	got := text(p)
	for _, want := range []string{"FPS: ", "TPS: ", "Frame:\n  tick ", "\n  drawing ", "Entities: 0",
		"Game time: "} {
		if !strings.Contains(got, want) {
			t.Errorf("the panel says %q, want %q among its lines", got, want)
		}
	}
	r := &stubReporter{label: "Weather"}
	p.Line("Mood", func() string { return "fine" }).With(r)
	if got := text(p); !strings.HasSuffix(got, "Mood: fine\nWeather: yes") {
		t.Errorf("the panel says %q, want the game's line and the reporter's last", got)
	}
	p.Clear()
	if got, ok := p.Scenes()[0].(interface {
		Text(uid.UID64, bool) (string, bool)
	}).Text(0, false); ok || got != "" {
		t.Errorf("cleared, the panel says %q, want nothing", got)
	}
}

func TestPlugin_TheLayersInitialiseTheReportersAndCloseTheLines(t *testing.T) {
	w := newWorld()
	r := &stubReporter{label: "Weather"}
	p := telemetry.NewPlugin(w).With(r)
	layers := p.Scenes()[0].Drawers()
	ecs := goke.New()
	systems := make([]goke.System, 0, len(layers))
	for _, l := range layers {
		systems = append(systems, goke.SystemFn{OnInit: l.Init})
	}
	ecs.Setup(systems...)
	if !r.inited {
		t.Error("the reporter was not initialised by the scene's layers")
	}
	defer func() {
		if msg, _ := recover().(string); !strings.Contains(msg, "after the scene was made") {
			t.Errorf("panic %q; want one refusing a line after the scene was made", msg)
		}
	}()
	p.Line("late", func() string { return "" })
}

func TestPlugin_ToggleShowsAndHidesThePanelWithoutTakingTheInput(t *testing.T) {
	w := newWorld()
	p := telemetry.NewPlugin(w)
	stack, err := game.NewStack(append([]game.Scene{plain{}}, p.Scenes()...)...)
	if err != nil {
		t.Fatal(err)
	}
	c := stack.Composition()
	c.Show("main")
	if p.Shown(c) {
		t.Fatal("the panel starts shown, want hidden")
	}
	p.Toggle(c)
	if !p.Shown(c) || c.Active() != "main" {
		t.Errorf("toggled: shown %v, active %q; want shown with the game still active",
			p.Shown(c), c.Active())
	}
	p.Toggle(c)
	if p.Shown(c) {
		t.Error("toggled again, the panel is still shown")
	}
}

type plain struct{}

func (plain) Name() string                                                      { return "main" }
func (plain) Drawers() []render.Drawer                                          { return nil }
func (plain) HandleEvents(*control.InputEvents, game.Runtime, game.Composition) {}
func (plain) Focusable() bool                                                   { return true }
