package players

import (
	"fmt"
	"image/color"
	"reflect"
	"strings"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/render"
)

// SceneKey is a key of the game's own scene — save, quit, a debug toggle — that is no command to a
// plugin: what it does, labelled for the shortcuts list. Shift asks for Shift held.
type SceneKey struct {
	Key   control.Key
	Shift bool
	Label string
	Do    func(runtime game.Runtime, composition game.Composition)
}

// SceneKeys are a scene's keys: Handle runs the ones this tick's presses hit, and the shortcuts
// scene lists them under "Game".
type SceneKeys []SceneKey

// Handle runs every key of ks pressed this tick with the modifiers it asks for.
func (ks SceneKeys) Handle(events *control.InputEvents, runtime game.Runtime, composition game.Composition) {
	for _, k := range events.KeyEvents {
		if k.Action != control.ActionPress {
			continue
		}
		for _, sk := range ks {
			if sk.Key == k.Key && sk.Shift == events.Modifiers.Shift && sk.Do != nil {
				sk.Do(runtime, composition)
			}
		}
	}
}

// ShortcutsName is the name of the shortcuts scene in a game's stack.
const ShortcutsName = "gram.shortcuts"

// Shortcuts is the scene listing every key of the game: the local players' bindings, grouped by
// the plugin whose command each issues, and the game's own SceneKeys under "Game" with the
// engine's F11. Shown, it holds the game in the engine's pause, like a menu; Esc or K closes it.
// It is the players' own scene, in every Stage that uses them (Plugin.Scenes); K opens it (the
// command ShowShortcuts, a default key), given the scene hands its input to Plugin.Handle.
type Shortcuts struct {
	p     *Plugin
	keys  SceneKeys
	layer *shortcutsLayer
	held  bool // the game was paused already when the scene opened
}

var _ game.Scene = (*Shortcuts)(nil)

// newShortcuts is the players' own scene listing the keys.
func newShortcuts(p *Plugin) *Shortcuts {
	s := &Shortcuts{p: p}
	s.layer = &shortcutsLayer{s: s}
	return s
}

// OwnKeys gives the game's own keys — a debug toggle, something no plugin's command does — run by
// Handle and listed under "Game" with the players'. Call it before the Stage is set up.
func (p *Plugin) OwnKeys(keys SceneKeys) *Plugin {
	p.shortcuts.keys = keys
	return p
}

// Scenes is the players' one scene of their own, the list of shortcuts: the Stage has it in its
// stack — see game.Scenic.
func (p *Plugin) Scenes() []game.Scene { return []game.Scene{p.shortcuts} }

// Open shows the scene on top and holds the game paused while it is up.
func (s *Shortcuts) Open(runtime game.Runtime, composition game.Composition) {
	s.held = runtime.Paused()
	runtime.Pause()
	composition.Show(ShortcutsName)
}

// Close hides the scene and lets the game go on, unless it was paused before.
func (s *Shortcuts) Close(runtime game.Runtime, composition game.Composition) {
	composition.Hide(ShortcutsName)
	if !s.held {
		runtime.Resume()
	}
}

func (*Shortcuts) Name() string { return ShortcutsName }

func (s *Shortcuts) Layers() []render.Layer { return []render.Layer{s.layer} }

// HandleEvents closes the scene on Esc or K.
func (s *Shortcuts) HandleEvents(events *control.InputEvents, runtime game.Runtime, composition game.Composition) {
	for _, k := range events.KeyEvents {
		if k.Action == control.ActionPress && (k.Key == control.KeyEscape || k.Key == control.KeyK) && !events.Modifiers.Shift {
			s.Close(runtime, composition)
			return
		}
	}
}

func (*Shortcuts) Focusable() bool { return true }

// group is one heading of the list and its lines.
type group struct {
	name  string
	lines []string
}

// groups lists the bindings of the local players holding in their cameras' modes by their
// handlers, in the handlers' order, then the game's keys.
func (s *Shortcuts) groups() []group {
	byHandler := map[plugin.CommandHandler][]string{}
	seen := map[string]bool{}
	var game []string
	for _, pl := range s.p.Locals() {
		how := camera.HowOf(pl.pic.camera)
		for _, b := range pl.Bindings() {
			if !b.Holds(how) {
				continue
			}
			line := fmt.Sprintf("%-22s %s", Written(b.Trigger), b.Label)
			if seen[line] {
				continue
			}
			seen[line] = true
			h := s.p.handlerOf(b.Command())
			if h == plugin.CommandHandler(s.p) {
				game = append(game, line) // the players' own, but the game's as a player sees it
				continue
			}
			byHandler[h] = append(byHandler[h], line)
		}
	}
	var out []group
	for _, h := range s.p.handlers {
		if lines := byHandler[h]; len(lines) > 0 {
			out = append(out, group{name: handlerName(h), lines: lines})
		}
	}
	if lines := byHandler[nil]; len(lines) > 0 {
		out = append(out, group{name: "Other", lines: lines})
	}
	g := group{name: "Game", lines: game}
	for _, k := range s.keys {
		g.lines = append(g.lines, fmt.Sprintf("%-22s %s", Written(control.KeyPress{Key: k.Key, Mods: control.Mods{Shift: k.Shift}}), k.Label))
	}
	g.lines = append(g.lines, fmt.Sprintf("%-22s %s", "F11", "Full screen (the engine's)"))
	return append(out, g)
}

// title is the list's heading, naming the first person while a local player's camera rides
// in an entity.
func (s *Shortcuts) title() string {
	for _, pl := range s.p.Locals() {
		if camera.HowOf(pl.pic.camera) == camera.Inside {
			return "Shortcuts: first person, riding in the unit"
		}
	}
	return "Shortcuts"
}

// handlerName is a handler's heading: its plugin name without the "gram." prefix, capitalised.
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
	case control.KeyHeld:
		return keyName(t.Key) + " (held)"
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

// keyName is a key as written on it, where ebiten's name is not.
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

// shortcutsLayer draws the list over a dimmed screen.
type shortcutsLayer struct{ s *Shortcuts }

func (*shortcutsLayer) Init(*goke.SysInit) {}

// Column geometry of the list, in pixels.
const (
	shortcutsMargin = 24
	shortcutsLine   = 14
	shortcutsColumn = 340
)

func (l *shortcutsLayer) Draw(screen *render.Image) {
	b := screen.Bounds()
	render.FillRect(screen, 0, 0, float32(b.Dx()), float32(b.Dy()), color.RGBA{A: 170})
	x, y := shortcutsMargin, shortcutsMargin
	render.DebugPrintAt(screen, l.s.title(), x, y)
	y += 2 * shortcutsLine
	for _, g := range l.s.groups() {
		if y+(len(g.lines)+2)*shortcutsLine > b.Dy()-shortcutsMargin && y > 3*shortcutsLine {
			x, y = x+shortcutsColumn, shortcutsMargin+2*shortcutsLine // the next column
		}
		render.DebugPrintAt(screen, g.name, x, y)
		y += shortcutsLine
		for _, line := range g.lines {
			render.DebugPrintAt(screen, "  "+line, x, y)
			y += shortcutsLine
		}
		y += shortcutsLine
	}
}
