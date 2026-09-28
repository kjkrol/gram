package players

import (
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/plugins/world"
)

func TestWritten_SpellsATriggerAsAHelpScreenDoes(t *testing.T) {
	for _, c := range []struct {
		trigger control.Trigger
		want    string
	}{
		{control.KeyPress{Key: ebiten.KeyW, Mods: control.Mods{Shift: true}}, "Shift+W"},
		{control.KeyPress{Key: ebiten.KeyBracketRight}, "]"},
		{control.KeyPress{Key: ebiten.KeyEscape, Mods: control.Mods{Shift: true}}, "Shift+Esc"},
		{control.KeyHeld{Key: ebiten.KeyQ}, "Q (held)"},
		{control.ButtonPress{Button: ebiten.MouseButtonRight, Mods: control.Mods{Shift: true}.Holding(ebiten.KeyS)}, "Shift+S+right click"},
		{control.Drag{Button: ebiten.MouseButtonLeft, Mods: control.Mods{}.Holding(ebiten.KeyL)}, "L+left drag"},
		{control.ButtonHeld{Button: ebiten.MouseButtonMiddle}, "middle drag"},
		{control.Wheel{}, "wheel"},
		{control.CursorAtEdge{}, "cursor at an edge"},
	} {
		if got := Written(c.trigger); got != c.want {
			t.Errorf("Written(%#v) = %q, want %q", c.trigger, got, c.want)
		}
	}
}

// The shortcuts list every binding of the local players under the plugin that owns its command,
// the camera's under Camera, and the scene's keys under Game.
func TestShortcuts_ListTheBindingsByPluginAndTheScenesKeys(t *testing.T) {
	w := world.NewPlugin(world.Config{Space: world.SpaceCfg{Width: 100, Height: 100}, Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10}})
	p := NewPlugin(w)
	if err := p.Local("one").Bind(p.Defaults()...); err != nil {
		t.Fatal(err)
	}
	s := p.Shortcuts(SceneKeys{{Key: ebiten.KeyEscape, Shift: true, Label: "Quit"}})
	groups := s.groups()
	names := map[string][]string{}
	for _, g := range groups {
		names[g.name] = g.lines
	}
	if len(groups) != 3 || groups[0].name != "Camera" || groups[1].name != "World" || groups[2].name != "Game" {
		t.Fatalf("groups %v, want Camera, World and Game in that order", names)
	}
	if lines := strings.Join(names["Camera"], "\n"); !strings.Contains(lines, "W (held)") || !strings.Contains(lines, "Scroll up") || !strings.Contains(lines, "wheel") {
		t.Errorf("the camera's lines %q, want WASD and the wheel among them", lines)
	}
	if lines := strings.Join(names["World"], "\n"); !strings.Contains(lines, "Space") || !strings.Contains(lines, "Pause the game") {
		t.Errorf("the world's lines %q, want the clock's Space", lines)
	}
	if lines := strings.Join(names["Game"], "\n"); !strings.Contains(lines, "Shift+Esc") || !strings.Contains(lines, "Quit") || !strings.Contains(lines, "F11") {
		t.Errorf("the game's lines %q, want Shift+Esc to quit and the engine's F11", lines)
	}
}

// The shortcuts list only what holds in the mode the local players' cameras are in: riding in an
// entity, the free camera's WASD and edge scroll are gone and the title says so.
func TestShortcuts_ListWhatHoldsInTheCamerasMode(t *testing.T) {
	w := world.NewPlugin(world.Config{Space: world.SpaceCfg{Width: 100, Height: 100}, Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10}})
	p := NewPlugin(w)
	pl := p.Local("one")
	cam := &ridingCam{Camera: pl.Camera}
	pl.Camera = cam
	if err := pl.Bind(p.Defaults()...); err != nil {
		t.Fatal(err)
	}
	s := p.Shortcuts(nil)
	camera := func() string {
		for _, g := range s.groups() {
			if g.name == "Camera" {
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

func (r *ridingCam) FirstPerson() bool { return r.on }

// The scene's keys run on their key with the modifiers they ask for, and not otherwise.
func TestSceneKeys_HandleRunsTheKeyPressedWithItsModifiers(t *testing.T) {
	ran := ""
	keys := SceneKeys{
		{Key: ebiten.KeyEscape, Shift: true, Label: "Quit", Do: func(game.Runtime, game.Composition) { ran += "quit " }},
		{Key: ebiten.KeyB, Label: "Grid", Do: func(game.Runtime, game.Composition) { ran += "grid " }},
	}
	events := &control.InputEvents{}
	events.AddKeyEvent(ebiten.KeyEscape, control.ActionPress) // no Shift: nothing
	events.AddKeyEvent(ebiten.KeyB, control.ActionPress)
	events.AddKeyEvent(ebiten.KeyB, control.ActionRelease)
	keys.Handle(events, nil, nil)
	if ran != "grid " {
		t.Errorf("ran %q, want the grid alone: Esc without Shift is nothing, a release nothing", ran)
	}
	events = &control.InputEvents{}
	events.Modifiers.Shift = true
	events.AddKeyEvent(ebiten.KeyEscape, control.ActionPress)
	keys.Handle(events, nil, nil)
	if ran != "grid quit " {
		t.Errorf("ran %q, want the quit after Shift+Esc", ran)
	}
}
