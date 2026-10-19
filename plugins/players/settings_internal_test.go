package players

import (
	"bytes"
	"encoding/gob"
	"strings"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/world"
)

type shade struct{ On bool }

func (shade) Command() {}

// tunable is a handler with one setting, a switch, and its state
type tunable struct {
	shades command.Queue[shade]
	on     bool
}

func (h *tunable) Name() string                       { return "gram.light" }
func (h *tunable) Queues() []command.CommandQueue     { return []command.CommandQueue{&h.shades} }
func (h *tunable) DefaultBindings() []control.Binding { return nil }
func (h *tunable) Settings() []control.Setting {
	return []control.Setting{control.Switch{
		Label: "Shadows",
		Get:   func() bool { return h.on },
		Set:   func(on bool) command.Command { return shade{On: on} },
	}}
}

func settingsRig(t *testing.T) (*Plugin, *tunable, *world.Plugin) {
	t.Helper()
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 100, Height: 100},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10},
	})
	h := &tunable{}
	p := NewPlugin(w, cameras.NewPlugin(w, cameras.Config{}), h)
	if err := p.Local("one").Bind(p.Defaults()...); err != nil {
		t.Fatal(err)
	}
	return p, h, w
}

func TestSettings_F10OpensTheScreenHoldingTheClockAndEscClosesIt(t *testing.T) {
	p, _, w := settingsRig(t)
	rt, comp := &engineStub{props: game.Props{ScreenWidth: 800, ScreenHeight: 600}}, &composed{}
	events := &control.InputEvents{}
	events.AddKeyEvent(control.KeyF10, control.ActionPress)
	p.Handle(events, rt, comp)
	if len(comp.names) != 1 || comp.names[0] != SettingsName || !w.Clock().State().Paused {
		t.Fatalf("F10 showed %v, clock paused %v; want the settings shown and the clock held",
			comp.names, w.Clock().State().Paused)
	}
	if rt.paused {
		t.Error("the settings paused the engine, want the clock alone: the plugins carry on")
	}
	events = &control.InputEvents{}
	events.AddKeyEvent(control.KeyEscape, control.ActionPress)
	p.settings.HandleEvents(events, rt, comp)
	if len(comp.names) != 0 || w.Clock().State().Paused {
		t.Errorf("Esc left %v shown, clock paused %v; want the settings hidden and the clock going",
			comp.names, w.Clock().State().Paused)
	}
}

func TestSettings_AClockPausedBeforeStaysPaused(t *testing.T) {
	p, _, w := settingsRig(t)
	w.Clock().TogglePause()
	rt, comp := &engineStub{props: game.Props{ScreenWidth: 800, ScreenHeight: 600}}, &composed{}
	p.settings.Open(rt, comp)
	p.settings.Close(rt, comp)
	if !w.Clock().State().Paused {
		t.Error("closing the settings let a clock go that was paused before them")
	}
}

func TestSettings_TheScreenListsEveryTunableAndItsControlsIssue(t *testing.T) {
	p, h, w := settingsRig(t)
	rt, comp := &engineStub{props: game.Props{ScreenWidth: 800, ScreenHeight: 600}}, &composed{}
	p.settings.Open(rt, comp)
	screen := geom.NewAABB(geom.NewVec(0, 0), geom.NewVec(800, 600))
	s := p.settings.screen
	s.Drawers()
	s.Lay(screen)
	box := s.Element("setting:Shadows")
	if box == nil {
		t.Fatal("no checkbox for the light's Shadows on the screen")
	}
	at := box.Box().TopLeft.Add(geom.NewVec(4, 4))
	events := &control.InputEvents{MousePos: at, ClickQueue: []control.ClickEvent{
		{Button: control.MouseButtonLeft, Action: control.ActionPress, Pos: at}}}
	p.settings.HandleEvents(events, rt, comp)
	var got []shade
	h.shades.Drain(func(i command.Issued[shade]) { got = append(got, i.Command) })
	if len(got) != 1 || !got[0].On {
		t.Fatalf("the checkbox issued %v, want the shadows on", got)
	}
	_ = w
}

func TestSettings_KeepRunningLetsTheClockGoAndIsSaved(t *testing.T) {
	p, _, w := settingsRig(t)
	rt, comp := &engineStub{props: game.Props{ScreenWidth: 800, ScreenHeight: 600}}, &composed{}
	p.settings.Open(rt, comp)
	if err := p.Issue(nil, KeepRunning{On: true}); err != nil {
		t.Fatal(err)
	}
	p.carry(rt, comp)
	if w.Clock().State().Paused || !p.Res.Settings.Running {
		t.Fatalf("running behind the settings: clock paused %v, Running %v", w.Clock().State().Paused,
			p.Res.Settings.Running)
	}
	var buf bytes.Buffer
	for _, v := range p.Serializable().Persisted() {
		if err := gob.NewEncoder(&buf).Encode(v); err != nil {
			t.Fatal(err)
		}
	}
	q, _, _ := settingsRig(t)
	for _, v := range q.Serializable().Persisted() {
		if err := gob.NewDecoder(&buf).Decode(v); err != nil {
			t.Fatal(err)
		}
	}
	q.Restore()
	if !q.Res.Settings.Running {
		t.Error("a game loaded forgot the settings ran behind the screen")
	}
	lines := ""
	for _, g := range p.shortcuts.groups() {
		if g.name == "General" {
			lines = strings.Join(g.lines, "\n")
		}
	}
	if !strings.Contains(lines, "F10") {
		t.Errorf("the general shortcuts %q, want F10 among them", lines)
	}
}
