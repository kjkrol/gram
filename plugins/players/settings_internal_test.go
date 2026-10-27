package players

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"strings"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/telemetry"
	"github.com/kjkrol/gram/plugins/world"
)

type shade struct{ On bool }

func (shade) Command() {}

// tunable is a handler with two switches on one state: one of its own, one under Graphics
type tunable struct {
	shades command.Queue[shade]
	on     bool
}

func (h *tunable) Name() string                       { return "gram.light" }
func (h *tunable) Queues() []command.CommandQueue     { return []command.CommandQueue{&h.shades} }
func (h *tunable) DefaultBindings() []control.Binding { return nil }
func (h *tunable) Settings() []control.Setting {
	return []control.Setting{
		control.Switch{
			Label: "Lamps",
			Get:   func() bool { return h.on },
			Set:   func(on bool) command.Command { return shade{On: on} },
		},
		control.Switch{
			Label:   "Shadows",
			Section: control.Graphics,
			Get:     func() bool { return h.on },
			Set:     func(on bool) command.Command { return shade{On: on} },
		},
	}
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
	rt, comp := &engineStub{props: game.Props{ScreenWidth: 1920, ScreenHeight: 1080}}, &composed{}
	events := &control.InputEvents{}
	events.AddKeyEvent(control.KeyF10, control.ActionPress)
	p.Handle(events, rt, comp)
	if len(comp.names) != 1 || comp.names[0] != SettingsName || !w.Read(0).Clock().State().Paused {
		t.Fatalf("F10 showed %v, clock paused %v; want the settings shown and the clock held",
			comp.names, w.Read(0).Clock().State().Paused)
	}
	if rt.paused {
		t.Error("the settings paused the engine, want the clock alone: the plugins carry on")
	}
	events = &control.InputEvents{}
	events.AddKeyEvent(control.KeyEscape, control.ActionPress)
	p.settings.HandleEvents(events, rt, comp)
	if len(comp.names) != 0 || w.Read(0).Clock().State().Paused {
		t.Errorf("Esc left %v shown, clock paused %v; want the settings hidden and the clock going",
			comp.names, w.Read(0).Clock().State().Paused)
	}
}

func TestSettings_AClockPausedBeforeStaysPaused(t *testing.T) {
	p, _, w := settingsRig(t)
	w.Write().Clock().TogglePause()
	rt, comp := &engineStub{props: game.Props{ScreenWidth: 1920, ScreenHeight: 1080}}, &composed{}
	p.settings.Open(rt, comp)
	p.settings.Close(rt, comp)
	if !w.Read(0).Clock().State().Paused {
		t.Error("closing the settings let a clock go that was paused before them")
	}
}

func TestSettings_TheScreenListsEveryTunableAndItsControlsIssue(t *testing.T) {
	p, h, w := settingsRig(t)
	rt, comp := &engineStub{props: game.Props{ScreenWidth: 1920, ScreenHeight: 1080}}, &composed{}
	p.settings.Open(rt, comp)
	screen := geom.NewAABB(geom.NewVec(0, 0), geom.NewVec(1920, 1080))
	s := p.settings.screen
	s.Drawers()
	s.Lay(screen)
	box := s.Element("setting:Picture/Graphics/Shadows")
	if box == nil {
		t.Fatal("no checkbox for the light's Shadows on the screen")
	}
	for _, name := range []string{
		"setting:Picture/Tactical/Resolution",
		"setting:Picture/Riding/Resolution",
	} {
		if s.Element(name) == nil {
			t.Errorf("no %s on the screen: a label in two sections is two settings", name)
		}
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

func TestSettings_SectionsComeFirstCentredAndAMovedSettingSitsUnderItsSubheading(t *testing.T) {
	p, _, _ := settingsRig(t)
	rt, comp := &engineStub{props: game.Props{ScreenWidth: 1920, ScreenHeight: 1080}}, &composed{}
	p.settings.Open(rt, comp)
	s := p.settings.screen
	s.Drawers()
	s.Lay(geom.NewAABB(geom.NewVec(0, 0), geom.NewVec(1920, 1080)))
	boxOf := func(name string) geom.AABB {
		t.Helper()
		e := s.Element(name)
		if e == nil {
			t.Fatalf("no %s on the screen", name)
		}
		return e.Box()
	}
	picture, players := boxOf("section:Picture"), boxOf("section:Players")
	if players.TopLeft.X < picture.TopLeft.X ||
		players.TopLeft.X == picture.TopLeft.X && players.TopLeft.Y <= picture.TopLeft.Y {
		t.Errorf("Picture at %v, Players at %v; want the shared section first", picture.TopLeft,
			players.TopLeft)
	}
	graphics := boxOf("section:Picture/Graphics")
	shadows := boxOf("setting:Picture/Graphics/Shadows")
	// the column the subheading lies in, whichever the height broke it into
	var column geom.AABB
	for i := 0; ; i++ {
		e := s.Element(fmt.Sprintf("settings.column.%d", i))
		if e == nil {
			t.Fatalf("Graphics at %v lies in no column", graphics)
		}
		if column = e.Box(); within(graphics, column) {
			break
		}
	}
	if shadows.TopLeft.Y < graphics.BottomRight.Y || !within(shadows, column) {
		t.Errorf("Graphics at %v, Shadows at %v, the column %v; want the moved setting under "+
			"its subheading in one column", graphics, shadows, column)
	}
	title := boxOf("settings.title")
	if centre := (title.TopLeft.X + title.BottomRight.X) / 2; f32.Abs(centre-960) > 1 {
		t.Errorf("the title's centre at x %v, want 960", centre)
	}
}

func within(a, in geom.AABB) bool {
	return a.TopLeft.X >= in.TopLeft.X && a.TopLeft.Y >= in.TopLeft.Y &&
		a.BottomRight.X <= in.BottomRight.X && a.BottomRight.Y <= in.BottomRight.Y
}

func TestSettings_KeepRunningLetsTheClockGoAndIsSaved(t *testing.T) {
	p, _, w := settingsRig(t)
	rt, comp := &engineStub{props: game.Props{ScreenWidth: 1920, ScreenHeight: 1080}}, &composed{}
	p.settings.Open(rt, comp)
	if err := p.write.Issue(nil, KeepRunning{On: true}); err != nil {
		t.Fatal(err)
	}
	p.carry(rt, comp)
	if w.Read(0).Clock().State().Paused || !p.Res.Settings.Running {
		t.Fatalf("running behind the settings: clock paused %v, Running %v",
			w.Read(0).Clock().State().Paused,
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
	for _, g := range p.controls.groups() {
		if g.name == "General" {
			lines = g.text()
		}
	}
	if !strings.Contains(lines, "F10") {
		t.Errorf("the general controls %q, want F10 among them", lines)
	}
}

func TestSettings_TheTelemetryPanelShownAndWhatItShows(t *testing.T) {
	p, _, w := settingsRig(t)
	tp := telemetry.NewPlugin(w)
	rt, comp := &engineStub{}, &composed{}
	p.carry(rt, comp)
	var shown control.Switch
	var shows control.Choice
	for _, s := range p.Settings() {
		switch s := s.(type) {
		case control.Switch:
			if s.Label == "Telemetry panel" {
				shown = s
			}
		case control.Choice:
			if s.Label == "Telemetry panel shows" {
				shows = s
			}
		}
	}
	if shown.Set == nil || shows.Set == nil {
		t.Fatal("no switch of the telemetry panel and choice of what it shows among the settings")
	}
	give := func(cmd command.Command) {
		t.Helper()
		if err := p.write.Issue(nil, cmd); err != nil {
			t.Fatal(err)
		}
		p.carry(rt, comp)
	}
	give(shows.Set(1))
	if tp.Read(0).Shown(comp) || tp.Read(0).Mode() != telemetry.Extended || shows.Get() != 1 {
		t.Errorf("everything chosen: shown %v, mode %v, the choice %d; want hidden, extended, 1",
			tp.Read(0).Shown(comp), tp.Read(0).Mode(), shows.Get())
	}
	give(shown.Set(true))
	if !shown.Get() || tp.Read(0).Mode() != telemetry.Extended {
		t.Errorf("switched on: shown %v in mode %v, want shown, still extended", shown.Get(),
			tp.Read(0).Mode())
	}
	give(shows.Set(0))
	if !tp.Read(0).Shown(comp) || tp.Read(0).Mode() != telemetry.Basic || shows.Get() != 0 {
		t.Errorf("the basics chosen: shown %v, mode %v, the choice %d; want shown, basic, 0",
			tp.Read(0).Shown(comp), tp.Read(0).Mode(), shows.Get())
	}
}
