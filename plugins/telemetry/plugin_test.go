package telemetry_test

import (
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

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
	telemetry.Refresh(p, 0)
	if telemetry.Of(w) != p {
		t.Fatal("Of does not find the plugin made over the world")
	}
	p.SetMode(telemetry.Extended)
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

func TestPanel_TextIsRefreshedFourTimesASecond(t *testing.T) {
	w := newWorld()
	reads := 0
	p := telemetry.NewPlugin(w).Clear().Line("Reads", func() string {
		reads++
		return strconv.Itoa(reads)
	})
	p.SetMode(telemetry.Extended)
	if got := text(p); got != "Reads: 1" {
		t.Fatalf("the panel says %q, want its line read once", got)
	}
	telemetry.Aged(p, 100*time.Millisecond)
	if got := text(p); got != "Reads: 1" {
		t.Errorf("read 100 ms on, the panel says %q, want the text kept", got)
	}
	telemetry.Aged(p, 150*time.Millisecond)
	if got := text(p); got != "Reads: 2" {
		t.Errorf("read 250 ms on, the panel says %q, want its line read anew", got)
	}
}

func TestPlugin_ShowsTheBasicsAloneUntilExtended(t *testing.T) {
	w := newWorld()
	p := telemetry.NewPlugin(w).Line("Mood", func() string { return "fine" }).
		With(&stubReporter{label: "Weather"})
	telemetry.Refresh(p, 0)
	if p.Mode() != telemetry.Basic {
		t.Fatalf("the panel starts in mode %v, want the basics", p.Mode())
	}
	labels := func() []string {
		var out []string
		for l := range strings.SplitSeq(text(p), "\n") {
			if label, _, ok := strings.Cut(l, ":"); ok && !strings.HasPrefix(l, " ") {
				out = append(out, label)
			}
		}
		return out
	}
	basics := []string{"FPS", "TPS", "Frame", "Entities"}
	if got := labels(); !slices.Equal(got, basics) {
		t.Errorf("in the basic mode the panel's lines are %q, want %q", got, basics)
	}
	p.SetMode(telemetry.Extended)
	every := append(slices.Clone(basics), "Game time", "Mood", "Weather")
	if got := labels(); !slices.Equal(got, every) {
		t.Errorf("in the extended mode the panel's lines are %q, want %q", got, every)
	}
	telemetry.Refresh(p, time.Hour)
	p.SetMode(telemetry.Basic)
	if got := labels(); !slices.Equal(got, basics) {
		t.Errorf("the mode set back, the panel's lines are %q, want the basics at once", got)
	}
}

func TestPlugin_TheModeIsSavedWithTheGame(t *testing.T) {
	saved := telemetry.NewPlugin(newWorld())
	saved.SetMode(telemetry.Extended)
	loaded := telemetry.NewPlugin(newWorld())
	from, into := saved.Serializable().Persisted(), loaded.Serializable().Persisted()
	*into[0].(*[]byte) = append([]byte(nil), *from[0].(*[]byte)...)
	loaded.Restore()
	if loaded.Mode() != telemetry.Extended || !loaded.Res.Settings.Extended {
		t.Errorf("loaded, the panel's mode is %v, want the extended one saved", loaded.Mode())
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
