package players

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/plugin/section"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/telemetry"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

func TestWritten_SpellsATriggerAsAHelpScreenDoes(t *testing.T) {
	for _, c := range []struct {
		trigger control.Trigger
		want    string
	}{
		{control.KeyPress{Key: control.KeyW, Mods: control.Mods{Shift: true}}, "Shift+W"},
		{control.KeyPress{Key: control.KeyBracketRight}, "]"},
		{control.KeyPress{Key: control.KeyEscape, Mods: control.Mods{Shift: true}}, "Shift+Esc"},
		{control.KeyHeld{Key: control.KeyQ}, "Q (held)"},
		{control.ButtonPress{Button: control.MouseButtonRight, Mods: control.Mods{Shift: true}.Holding(control.KeyS)}, "Shift+S+right click"},
		{control.Drag{Button: control.MouseButtonLeft, Mods: control.Mods{}.Holding(control.KeyL)}, "L+left drag"},
		{control.ButtonHeld{Button: control.MouseButtonMiddle}, "middle drag"},
		{control.Wheel{}, "wheel"},
		{control.CursorAtEdge{}, "cursor at an edge"},
	} {
		if got := Written(c.trigger); got != c.want {
			t.Errorf("Written(%#v) = %q, want %q", c.trigger, got, c.want)
		}
	}
}

func TestShortcuts_ListTheBindingsByPluginAndTheScenesKeys(t *testing.T) {
	w := world.NewPlugin(world.Config{Space: world.SpaceCfg{Width: 100, Height: 100}, Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10}})
	cams := cameras.NewPlugin(w)
	p := NewPlugin(w, cams)
	if err := p.Local("one").Bind(p.Defaults()...); err != nil {
		t.Fatal(err)
	}
	s := p.OwnKeys(SceneKeys{{Key: control.KeyR, Label: "Build a road"}}).shortcuts
	groups := s.groups()
	names := map[string][]string{}
	for _, g := range groups {
		names[g.name] = g.lines
	}
	if len(groups) != 4 || groups[0].name != "Game" || groups[1].name != "World" ||
		groups[2].name != "Cameras" || groups[3].name != "General" {
		t.Fatalf("groups %v, want Game, World, Cameras and General in that order", names)
	}
	if lines := strings.Join(names["Cameras"], "\n"); !strings.Contains(lines, "W (held)") || !strings.Contains(lines, "Scroll up") || !strings.Contains(lines, "wheel") {
		t.Errorf("the camera's lines %q, want WASD and the wheel among them", lines)
	}
	if lines := strings.Join(names["World"], "\n"); !strings.Contains(lines, "Shift+P") || !strings.Contains(lines, "Pause the game") {
		t.Errorf("the world's lines %q, want the clock's Shift+P", lines)
	}
	if lines := strings.Join(names["Game"], "\n"); lines != "R                      Build a road" {
		t.Errorf("the game's lines %q, want its own R alone", lines)
	}
	lines := strings.Join(names["General"], "\n")
	for _, want := range []string{"Shift+Esc", "Quit", "K ", "F11"} {
		if !strings.Contains(lines, want) {
			t.Errorf("the general lines %q, want the players' K, Shift+Esc and the engine's F11",
				lines)
			break
		}
	}
}

type partNow section.Part

func (p *partNow) Section() section.Part { return section.Part(*p) }

func TestShortcuts_ListTheGamesOwnBindingsUnderGame(t *testing.T) {
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 100, Height: 100},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10},
	})
	p := NewPlugin(w, cameras.NewPlugin(w))
	now := partNow(section.Players)
	p.sections = &now
	one := p.Local("one")
	if err := one.Bind(w.DefaultBindings()...); err != nil {
		t.Fatal(err)
	}
	now = partNow(section.Controls)
	spawn := control.Build(control.KeyPress{Key: control.KeyN}, "Spawn a walker",
		func(control.Context) (world.Spawn, bool) { return world.Spawn{}, false })
	if err := one.Bind(spawn); err != nil {
		t.Fatal(err)
	}
	groups := p.shortcuts.groups()
	if len(groups) < 2 || groups[0].name != "Game" || groups[1].name != "World" {
		t.Fatalf("groups %+v, want the game's first, under Game, then World", groups)
	}
	if own := strings.Join(groups[0].lines, "\n"); !strings.Contains(own, "Spawn a walker") ||
		strings.Contains(own, "Pause the game") {
		t.Errorf("the game's lines %q, want its N alone", own)
	}
	if world := strings.Join(groups[1].lines, "\n"); strings.Contains(world, "Spawn a walker") ||
		!strings.Contains(world, "Pause the game") {
		t.Errorf("the world's lines %q, want its defaults without the game's N", world)
	}
}

func TestShortcuts_ListWhatHoldsInTheCamerasMode(t *testing.T) {
	w := world.NewPlugin(world.Config{Space: world.SpaceCfg{Width: 100, Height: 100}, Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10}})
	cams := cameras.NewPlugin(w)
	p := NewPlugin(w, cams)
	cam := &ridingCam{Camera: cams.New(cameras.TopDown(), camera.Config{})}
	pl := p.Local("one")
	p.Through(pl).Over(geom.AABB{}, render.NewFeed(cam, nil))
	if err := pl.Bind(p.Defaults()...); err != nil {
		t.Fatal(err)
	}
	s := p.shortcuts
	camera := func() string {
		for _, g := range s.groups() {
			if g.name == "Cameras" {
				return strings.Join(g.lines, "\n")
			}
		}
		return ""
	}
	if lines := camera(); !strings.Contains(lines, "Scroll up") || !strings.Contains(lines, "cursor at an edge") || s.title() != "Shortcuts" {
		t.Errorf("free, the camera's lines %q under %q, want the scrolling keys under Shortcuts", lines, s.title())
	}
	cam.on = true
	if lines := camera(); strings.Contains(lines, "Scroll up") || strings.Contains(lines, "cursor at an edge") || !strings.Contains(lines, "wheel") {
		t.Errorf("riding, the camera's lines %q, want the wheel alone", lines)
	}
	if !strings.Contains(s.title(), "first person") {
		t.Errorf("riding, the title is %q, want it to say first person", s.title())
	}
}

type ridingCam struct {
	camera.Camera
	on bool
}

func (r *ridingCam) Fasten(camera.Fastening) {}

func (r *ridingCam) Fastening() camera.Fastening {
	if r.on {
		return camera.Fastening{Entity: 1, How: camera.Inside}
	}
	return camera.Fastening{}
}

func TestSceneKeys_HandleRunsTheKeyPressedWithItsModifiers(t *testing.T) {
	ran := ""
	keys := SceneKeys{
		{Key: control.KeyEscape, Shift: true, Label: "Quit", Do: func(game.Runtime, game.Composition) { ran += "quit " }},
		{Key: control.KeyB, Label: "Grid", Do: func(game.Runtime, game.Composition) { ran += "grid " }},
	}
	events := &control.InputEvents{}
	events.AddKeyEvent(control.KeyEscape, control.ActionPress) // no Shift: nothing
	events.AddKeyEvent(control.KeyB, control.ActionPress)
	events.AddKeyEvent(control.KeyB, control.ActionRelease)
	keys.Handle(events, nil, nil)
	if ran != "grid " {
		t.Errorf("ran %q, want the grid alone: Esc without Shift is nothing, a release nothing", ran)
	}
	events = &control.InputEvents{}
	events.Modifiers.Shift = true
	events.AddKeyEvent(control.KeyEscape, control.ActionPress)
	keys.Handle(events, nil, nil)
	if ran != "grid quit " {
		t.Errorf("ran %q, want the quit after Shift+Esc", ran)
	}
}

type engineStub struct {
	game.Runtime
	quit, paused bool
	props        game.Props
}

func (e *engineStub) Props() game.Props { return e.props }

func (e *engineStub) Quit()        { e.quit = true }
func (e *engineStub) Paused() bool { return e.paused }
func (e *engineStub) Pause()       { e.paused = true }
func (e *engineStub) Resume()      { e.paused = false }

type shown struct {
	game.Composition
	names []string
}

func (c *shown) Show(name string) { c.names = append(c.names, name) }

type engine struct {
	saves command.Queue[game.Save]
	loads command.Queue[game.Load]
	quits command.Queue[game.Quit]
}

func (e *engine) Queues() []command.CommandQueue {
	return []command.CommandQueue{&e.saves, &e.loads, &e.quits}
}

func (*engine) DefaultBindings() []control.Binding { return nil }

func TestHandle_TurnsTheGamesKeysIntoCommands(t *testing.T) {
	w := world.NewPlugin(world.Config{Space: world.SpaceCfg{Width: 100, Height: 100}, Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10}})
	eng := &engine{}
	if err := w.Carry(eng); err != nil {
		t.Fatal(err)
	}
	cams := cameras.NewPlugin(w)
	p := NewPlugin(w, cams)
	if err := p.Local("one").Bind(p.Defaults()...); err != nil {
		t.Fatal(err)
	}
	built := 0
	p.OwnKeys(SceneKeys{{Key: control.KeyR, Label: "Build a road", Do: func(game.Runtime, game.Composition) { built++ }}})
	rt, comp := &engineStub{props: game.Props{Description: "Walk up to a host and answer it."}}, &shown{}
	press := func(key control.Key, shift bool) {
		events := &control.InputEvents{}
		events.Modifiers.Shift = shift
		events.AddKeyEvent(key, control.ActionPress)
		p.Handle(events, rt, comp)
	}
	press(control.KeyEscape, false)
	if !eng.quits.Empty() {
		t.Fatal("Esc alone quit the game, want Shift+Esc")
	}
	press(control.KeyK, false)
	if len(comp.names) != 1 || comp.names[0] != ShortcutsName || !rt.paused {
		t.Errorf("K showed %v, paused %v; want the shortcuts shown and the game held", comp.names, rt.paused)
	}
	if p.shortcuts.description != "Walk up to a host and answer it." {
		t.Errorf("the shortcuts open over %q, want the game's description", p.shortcuts.description)
	}
	press(control.KeyR, false)
	if built != 1 {
		t.Errorf("the game's own R ran %d times, want once", built)
	}
	press(control.KeyF5, false)
	press(control.KeyF9, false)
	press(control.KeyEscape, true)
	if eng.saves.Empty() || eng.loads.Empty() || eng.quits.Empty() {
		t.Errorf("F5, F9 and Shift+Esc gave the engine save %v, load %v, quit %v; want each",
			!eng.saves.Empty(), !eng.loads.Empty(), !eng.quits.Empty())
	}
}

func TestWrapped_BreaksBetweenWordsWithinTheWidth(t *testing.T) {
	got := wrapped("Walk up to a host and answer it.", 12)
	want := []string{"Walk up to a", "host and", "answer it."}
	if !slices.Equal(got, want) {
		t.Errorf("wrapped %q, want %q", got, want)
	}
}

func TestShortcuts_TheListingLaysTheDescriptionOverTheTitleAndTheColumnsApart(t *testing.T) {
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 100, Height: 100},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10},
	})
	cams := cameras.NewPlugin(w)
	p := NewPlugin(w, cams)
	if err := p.Local("one").Bind(p.Defaults()...); err != nil {
		t.Fatal(err)
	}
	var keys SceneKeys
	for i := range 30 { // enough of the game's own keys to need a second column
		keys = append(keys, SceneKey{Key: control.KeyA, Label: fmt.Sprintf("Do thing %d", i)})
	}
	s := p.OwnKeys(keys).shortcuts
	s.description = "A game of many keys, described in a line that wraps."
	screen := geom.NewAABB(geom.NewVec(0, 0), geom.NewVec(800, 400))
	s.screen.Replace(s.listing(game.Props{ScreenWidth: 800, ScreenHeight: 400}))
	s.screen.Lay(screen)
	about := s.screen.Element("shortcuts.about").Box()
	title := s.screen.Element("shortcuts.title").Box()
	if about.BottomRight.Y > title.TopLeft.Y {
		t.Errorf("the description %v lies below the title %v", about, title)
	}
	first, second := s.screen.Element("shortcuts.column.0"), s.screen.Element("shortcuts.column.1")
	if first == nil || second == nil {
		t.Fatal("thirty keys over a screen 400 tall need two columns")
	}
	a, b := first.Box(), second.Box()
	if a.BottomRight.X > b.TopLeft.X {
		t.Errorf("the columns overlap: %v and %v", a, b)
	}
	if a.TopLeft.Y < title.BottomRight.Y || b.TopLeft.Y < title.BottomRight.Y {
		t.Errorf("a column %v or %v begins above the title's bottom %v", a, b, title.BottomRight.Y)
	}
}

type composed struct {
	game.Composition
	names []string
}

func (c *composed) Show(name string) { c.names = append(c.names, name) }
func (c *composed) Hide(name string) {
	c.names = slices.DeleteFunc(c.names, func(n string) bool { return n == name })
}
func (c *composed) Order() []string { return c.names }

func TestHandle_TheTildeTogglesTheTelemetryPanelUnderGeneral(t *testing.T) {
	w := world.NewPlugin(world.Config{Space: world.SpaceCfg{Width: 100, Height: 100}, Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10}})
	tp := telemetry.NewPlugin(w)
	p := NewPlugin(w, cameras.NewPlugin(w))
	if err := p.Local("one").Bind(p.Defaults()...); err != nil {
		t.Fatal(err)
	}
	var general []string
	for _, g := range p.shortcuts.groups() {
		if g.name == "General" {
			general = g.lines
		}
	}
	if lines := strings.Join(general, "\n"); !strings.Contains(lines, "~") ||
		!strings.Contains(lines, "telemetry panel") {
		t.Errorf("the general lines %q, want ~ for the telemetry panel", lines)
	}
	rt, comp := &engineStub{}, &composed{}
	press := func() {
		events := &control.InputEvents{}
		events.AddKeyEvent(control.KeyBackquote, control.ActionPress)
		p.Handle(events, rt, comp)
	}
	press()
	if !tp.Shown(comp) {
		t.Fatalf("~ showed %v, want the telemetry panel", comp.names)
	}
	press()
	if tp.Shown(comp) {
		t.Errorf("~ again left %v shown, want the panel hidden", comp.names)
	}
}

type toolHandler struct{ takes command.Queue[game.Save] }

func (h *toolHandler) Name() string                   { return "gram.bridges" }
func (h *toolHandler) Queues() []command.CommandQueue { return nil }
func (h *toolHandler) DefaultBindings() []control.Binding {
	return []control.Binding{control.Give(control.KeyPress{Key: control.KeyB,
		Mods: control.Mods{Shift: true}}, "Build a bridge", game.Save{})}
}

func TestShortcuts_ListAKeyUnderThePluginWhoseDefaultsGaveIt(t *testing.T) {
	w := world.NewPlugin(world.Config{Space: world.SpaceCfg{Width: 100, Height: 100}, Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10}})
	if err := w.Carry(&engine{}); err != nil {
		t.Fatal(err)
	}
	p := NewPlugin(w, cameras.NewPlugin(w), &toolHandler{})
	if err := p.Local("one").Bind(p.Defaults()...); err != nil {
		t.Fatal(err)
	}
	for _, g := range p.shortcuts.groups() {
		lines := strings.Join(g.lines, "\n")
		if g.name == "Bridges" && !strings.Contains(lines, "Build a bridge") {
			t.Errorf("the bridges' lines %q, want Build a bridge among them", lines)
		}
		if g.name == "General" && strings.Contains(lines, "Build a bridge") {
			t.Errorf("the general lines %q list Build a bridge: it is the bridges' key", lines)
		}
	}
}
