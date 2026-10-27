package players

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/ui"
)

// SceneKey is a key of the game's own, a debug toggle, that is no command to a plugin: what it
// does, labelled for the controls screen. Shift and Ctrl ask for those held, exactly.
type SceneKey struct {
	Key         control.Key
	Shift, Ctrl bool
	Label       string
	Do          func(runtime game.Runtime, composition game.Composition)
}

// SceneKeys are a scene's keys: Handle runs the ones this tick's presses hit, and the controls
// screen lists them under "Game".
type SceneKeys []SceneKey

// Handle runs every key of ks pressed this tick with the modifiers it asks for.
func (ks SceneKeys) Handle(events *control.InputEvents, runtime game.Runtime,
	composition game.Composition) {
	for _, k := range events.KeyEvents {
		if k.Action != control.ActionPress {
			continue
		}
		for _, sk := range ks {
			if sk.Key == k.Key && sk.Shift == events.Modifiers.Shift &&
				sk.Ctrl == events.Modifiers.Ctrl && sk.Do != nil {
				sk.Do(runtime, composition)
			}
		}
	}
}

// ControlsName is the name of the controls scene in a game's stack.
const ControlsName = "gram.controls"

// ControlsScene is the screen of every key and mouse action of the game, laid as the settings
// screen is, a framed section a group: the game's own under "Game", the plugins' by plugin, the
// players' and the engine's under "General". K opens it, Esc or K closes it; shown, it holds the
// game in the engine's pause.
type ControlsScene struct {
	p      *Plugin
	keys   SceneKeys
	screen *ui.Scene
	held   bool
	w, h   int // the screen the listing was built for
}

var _ game.Scene = (*ControlsScene)(nil)

func newControls(p *Plugin) *ControlsScene {
	return &ControlsScene{p: p, screen: ui.NewScene(ControlsName, nil)}
}

// OwnKeys gives the game's own keys, run by Handle and listed under "Game". Call it before the
// Stage is set up.
func (p *Plugin) OwnKeys(keys SceneKeys) *Plugin {
	p.controls.keys = keys
	return p
}

// Scenes are the players' own: the controls screen and the settings screen (game.Scenic).
func (p *Plugin) Scenes() []game.Scene { return []game.Scene{p.controls, p.settings} }

// Open shows the screen on top and holds the game paused while it is up.
func (s *ControlsScene) Open(runtime game.Runtime, composition game.Composition) {
	s.held = runtime.Paused()
	runtime.Pause()
	s.build(screenSize(s.screen, runtime.Props()))
	composition.Show(ControlsName)
}

// Close hides the screen and lets the game go on, unless it was paused before.
func (s *ControlsScene) Close(runtime game.Runtime, composition game.Composition) {
	composition.Hide(ControlsName)
	if !s.held {
		runtime.Resume()
	}
}

func (*ControlsScene) Name() string { return ControlsName }

func (s *ControlsScene) Drawers() []render.Renderer { return s.screen.Drawers() }

// HandleEvents closes the screen on Esc or K and hands the rest to the screen, its scroll; a
// screen of another size, the window resized or full, has the listing built anew for it.
func (s *ControlsScene) HandleEvents(events *control.InputEvents, runtime game.Runtime,
	composition game.Composition) {
	s.p.capture()
	if w, h := screenSize(s.screen, runtime.Props()); w != s.w || h != s.h {
		s.build(w, h)
	}
	for _, k := range events.KeyEvents {
		if k.Action == control.ActionPress && !events.Modifiers.Shift &&
			(k.Key == control.KeyEscape || k.Key == control.KeyK) {
			s.Close(runtime, composition)
			return
		}
	}
	s.screen.HandleEvents(events, runtime, composition)
}

func (*ControlsScene) Focusable() bool { return true }

// a group's lines: a trigger as written and what it does, the two apart for the columns
type group struct {
	name         string
	keys, labels []string
}

func (g *group) add(key, label string) {
	g.keys, g.labels = append(g.keys, key), append(g.labels, label)
}

func (s *ControlsScene) groups() []group {
	byHandler := map[plugin.CommandHandler]*group{}
	seen := map[string]bool{}
	own, general := &group{name: "Game"}, &group{name: "General"}
	for _, pl := range s.p.read.Locals() {
		how := camera.HowOf(pl.pic.camera)
		for i, b := range pl.Bindings() {
			if !pl.fires(b, how) {
				continue
			}
			key := Written(b.Trigger)
			if seen[key+" "+b.Label] {
				continue
			}
			seen[key+" "+b.Label] = true
			switch h := s.p.originOf(b); {
			case pl.own[i]:
				own.add(key, b.Label)
			case h == plugin.CommandHandler(s.p) || h == nil:
				general.add(key, b.Label)
			default:
				if byHandler[h] == nil {
					byHandler[h] = &group{name: handlerName(h)}
				}
				byHandler[h].add(key, b.Label)
			}
		}
	}
	for _, k := range s.keys {
		trigger := control.KeyPress{Key: k.Key, Mods: control.Mods{Shift: k.Shift}}
		own.add(Written(trigger), k.Label)
	}
	var out []group
	if len(own.keys) > 0 {
		out = append(out, *own)
	}
	for _, h := range s.p.handlers {
		if g := byHandler[h]; g != nil {
			out = append(out, *g)
		}
	}
	general.add(control.FullscreenKey())
	return append(out, *general)
}

func (s *ControlsScene) title() string {
	for _, pl := range s.p.read.Locals() {
		switch camera.HowOf(pl.pic.camera) {
		case camera.Inside:
			return "Controls: first person, riding in the unit"
		case camera.Behind:
			return "Controls: third person, behind the unit"
		}
	}
	return "Controls"
}

func handlerName(h plugin.CommandHandler) string {
	name := reflect.TypeOf(h).String()
	if n, ok := h.(interface{ Name() string }); ok {
		name = n.Name()
	}
	name = strings.TrimPrefix(name, "gram.")
	if i := strings.LastIndex(name, "."); i >= 0 {
		name = name[i+1:]
	}
	if name == "" {
		return "Other"
	}
	return strings.ToUpper(name[:1]) + name[1:]
}

// Written is a trigger as a help screen writes it: Shift+W, Q (held), right click, L + left drag,
// wheel, cursor at an edge.
func Written(t control.Trigger) string {
	switch t := t.(type) {
	case control.KeyPress:
		return withMods(t.Mods, keyName(t.Key))
	case control.KeyDown:
		return keyName(t.Key)
	case control.KeyHeld:
		return keyName(t.Key) + " (held)"
	case control.KeyReleased:
		return keyName(t.Key) + " (hold, let go)"
	case control.ButtonPress:
		return withMods(t.Mods, buttonName(t.Button)+" click")
	case control.Drag:
		return withMods(t.Mods, buttonName(t.Button)+" drag")
	case control.ButtonHeld:
		return buttonName(t.Button) + " drag"
	case control.Wheel:
		return "wheel"
	case control.CursorAtEdge:
		return "cursor at an edge"
	case control.CursorInCorner:
		return "cursor in a corner"
	case control.CursorMove:
		return "mouse"
	case control.CursorOver:
		return "cursor over the world"
	}
	return fmt.Sprintf("%T", t)
}

func withMods(m control.Mods, what string) string {
	var parts []string
	if m.Ctrl {
		parts = append(parts, "Ctrl")
	}
	if m.Alt {
		parts = append(parts, "Alt")
	}
	if m.Shift {
		parts = append(parts, "Shift")
	}
	if k, ok := m.Held(); ok {
		parts = append(parts, keyName(k))
	}
	parts = append(parts, what)
	return strings.Join(parts, "+")
}

func buttonName(b control.MouseButton) string {
	switch b {
	case control.MouseButtonLeft:
		return "left"
	case control.MouseButtonRight:
		return "right"
	case control.MouseButtonMiddle:
		return "middle"
	}
	return fmt.Sprintf("button %d", b)
}

func keyName(k control.Key) string {
	switch k {
	case control.KeyBracketLeft:
		return "["
	case control.KeyBracketRight:
		return "]"
	case control.KeyEqual:
		return "="
	case control.KeyMinus:
		return "-"
	case control.KeyBackquote:
		return "~"
	case control.KeyEscape:
		return "Esc"
	case control.KeyArrowUp:
		return "Up"
	case control.KeyArrowDown:
		return "Down"
	case control.KeyArrowLeft:
		return "Left"
	case control.KeyArrowRight:
		return "Right"
	}
	return k.String()
}

// pixels
const (
	controlsRow   = 24  // a line: a trigger and what it does, one line of it
	controlsLabel = 300 // the most a label takes before it wraps
)

// build replaces the screen with the listing laid out for a screen w by h
func (s *ControlsScene) build(w, h int) {
	s.w, s.h = w, h
	s.screen.Replace(s.listing(w, h))
}

// listing is the screen: the title and the groups, each a framed section (frame:<group>) with
// a heading (section:<group>) and a line a binding, the trigger in a column as wide as the widest
// of them all and what it does beside, wrapped past controlsLabel, in the menus' columns.
func (s *ControlsScene) listing(w, h int) *ui.Element {
	groups := s.groups()
	font := ui.DefaultTheme().Font
	keyWidth := float32(0)
	for _, g := range groups {
		for _, k := range g.keys {
			kw, _ := font.Measure(k)
			keyWidth = max(keyWidth, kw)
		}
	}
	keyWidth += 2 * menuGap
	lead := (controlsRow - font.LineHeight()) / 2
	var blocks []block
	for _, g := range groups {
		var b block
		b.add(ui.Fit(ui.Heading(g.name).Named("section:"+g.name)), settingsRow)
		for i, k := range g.keys {
			lines := wrapped(font, g.labels[i], controlsLabel)
			tall := controlsRow + float32(len(lines)-1)*font.LineHeight()
			b.add(ui.Fixed(tall, ui.Columns(
				ui.Fixed(keyWidth, ui.Rows(ui.Fixed(lead, ui.Blank()), ui.Fit(ui.Label(k)))),
				ui.Fit(ui.Rows(ui.Fixed(lead, ui.Blank()),
					ui.Fit(ui.Label(strings.Join(lines, "\n"))))),
			)), tall)
		}
		blocks = append(blocks, framed([]block{b}, g.name))
	}
	return menu("controls", s.title(), blocks, w, h)
}

// wrapped is text in lines at most width pixels wide in font, broken between words
func wrapped(font *render.Font, text string, width float32) []string {
	var lines []string
	line := ""
	for _, word := range strings.Fields(text) {
		if w, _ := font.Measure(line + " " + word); line != "" && w > width {
			lines, line = append(lines, line), ""
		}
		if line != "" {
			line += " "
		}
		line += word
	}
	if line != "" {
		lines = append(lines, line)
	}
	return lines
}
