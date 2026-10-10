package players

import (
	"slices"
	"strings"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/plugins/cameras"
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

// The shortcuts list every binding of the local players under the plugin that owns its command,
// the cameras' under Cameras, and the players' own and the scene's keys under Game.
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
	if len(groups) != 3 || groups[0].name != "World" || groups[1].name != "Cameras" || groups[2].name != "Game" {
		t.Fatalf("groups %v, want World, Cameras and Game in that order", names)
	}
	if lines := strings.Join(names["Cameras"], "\n"); !strings.Contains(lines, "W (held)") || !strings.Contains(lines, "Scroll up") || !strings.Contains(lines, "wheel") {
		t.Errorf("the camera's lines %q, want WASD and the wheel among them", lines)
	}
	if lines := strings.Join(names["World"], "\n"); !strings.Contains(lines, "Space") || !strings.Contains(lines, "Pause the game") {
		t.Errorf("the world's lines %q, want the clock's Space", lines)
	}
	if lines := strings.Join(names["Game"], "\n"); !strings.Contains(lines, "Shift+Esc") || !strings.Contains(lines, "Quit") || !strings.Contains(lines, "K ") || !strings.Contains(lines, "Build a road") || !strings.Contains(lines, "F11") {
		t.Errorf("the game's lines %q, want the players' K and Shift+Esc, the game's own R and the engine's F11", lines)
	}
}

// The shortcuts list only what holds in the mode the local players' cameras are in: riding in an
// entity, the free camera's WASD and edge scroll are gone and the title says so.
func TestShortcuts_ListWhatHoldsInTheCamerasMode(t *testing.T) {
	w := world.NewPlugin(world.Config{Space: world.SpaceCfg{Width: 100, Height: 100}, Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10}})
	cams := cameras.NewPlugin(w)
	p := NewPlugin(w, cams)
	cam := &ridingCam{Camera: cams.New(cameras.TopDown(), camera.Config{})}
	pl := p.Local("one")
	p.Through(pl).Over(geom.AABB{}, render.NewFeed(cam, nil)) // it acts through a picture drawn through cam
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

// The scene's keys run on their key with the modifiers they ask for, and not otherwise.
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

// engineStub is a game.Runtime that notes what it is asked.
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

// shown is a game.Composition that notes what is shown.
type shown struct {
	game.Composition
	names []string
}

func (c *shown) Show(name string) { c.names = append(c.names, name) }

// engine takes the engine's commands in a test: they are its, not the players'.
type engine struct {
	saves control.Queue[game.Save]
	loads control.Queue[game.Load]
	quits control.Queue[game.Quit]
}

func (e *engine) Queues() []control.CommandQueue {
	return []control.CommandQueue{&e.saves, &e.loads, &e.quits}
}

func (*engine) DefaultBindings() []control.Binding { return nil }

// Handle turns a scene's input into commands: K shows the list of shortcuts at once, which needs
// the Runtime; F5, F9 and Shift+Esc — Esc alone not — give the engine's Save, Load and Quit; a key
// of the game's own runs.
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
