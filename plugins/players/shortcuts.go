package players

import (
	"fmt"
	"image/color"
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
// does, labelled for the shortcuts list. Shift and Ctrl ask for those held, exactly.
type SceneKey struct {
	Key         control.Key
	Shift, Ctrl bool
	Label       string
	Do          func(runtime game.Runtime, composition game.Composition)
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
			if sk.Key == k.Key && sk.Shift == events.Modifiers.Shift &&
				sk.Ctrl == events.Modifiers.Ctrl && sk.Do != nil {
				sk.Do(runtime, composition)
			}
		}
	}
}

// ShortcutsName is the name of the shortcuts scene in a game's stack.
const ShortcutsName = "gram.shortcuts"

// Shortcuts is the scene listing every key of the game: the game's own under "Game", the
// plugins' by plugin, the players' and the engine's under "General". Shown, it holds the game
// in the engine's pause; Esc or K closes it.
type Shortcuts struct {
	p           *Plugin
	keys        SceneKeys
	screen      *ui.Scene
	held        bool
	description string
}

var _ game.Scene = (*Shortcuts)(nil)

func newShortcuts(p *Plugin) *Shortcuts {
	return &Shortcuts{p: p, screen: ui.NewScene(ShortcutsName, nil)}
}

// OwnKeys gives the game's own keys, run by Handle and listed under "Game". Call it before the
// Stage is set up.
func (p *Plugin) OwnKeys(keys SceneKeys) *Plugin {
	p.shortcuts.keys = keys
	return p
}

// Scenes are the players' own: the list of shortcuts and the settings screen (game.Scenic).
func (p *Plugin) Scenes() []game.Scene { return []game.Scene{p.shortcuts, p.settings} }

// Open shows the scene on top, the game's description over the keys, and holds the game paused
// while it is up.
func (s *Shortcuts) Open(runtime game.Runtime, composition game.Composition) {
	s.description = runtime.Props().Description
	s.held = runtime.Paused()
	runtime.Pause()
	s.screen.Replace(s.listing(runtime.Props()))
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

func (s *Shortcuts) Drawers() []render.Drawer { return s.screen.Drawers() }

// HandleEvents closes the scene on Esc or K.
func (s *Shortcuts) HandleEvents(events *control.InputEvents, runtime game.Runtime, composition game.Composition) {
	s.p.capture()
	for _, k := range events.KeyEvents {
		if k.Action == control.ActionPress && (k.Key == control.KeyEscape || k.Key == control.KeyK) && !events.Modifiers.Shift {
			s.Close(runtime, composition)
			return
		}
	}
}

func (*Shortcuts) Focusable() bool { return true }

// a group's lines, a key and its label each, and the two written apart for a table
type group struct {
	name         string
	lines        []string
	keys, labels []string
}

func (g *group) add(key, label string) {
	g.lines = append(g.lines, fmt.Sprintf("%-22s %s", key, label))
	g.keys, g.labels = append(g.keys, key), append(g.labels, label)
}

func (s *Shortcuts) groups() []group {
	byHandler := map[plugin.CommandHandler]*group{}
	seen := map[string]bool{}
	own, general := &group{name: "Game"}, &group{name: "General"}
	for _, pl := range s.p.Locals() {
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
	if len(own.lines) > 0 {
		out = append(out, *own)
	}
	for _, h := range s.p.handlers {
		if g := byHandler[h]; g != nil {
			out = append(out, *g)
		}
	}
	general.add("F11", "Full screen (the engine's)")
	return append(out, *general)
}

func (s *Shortcuts) title() string {
	for _, pl := range s.p.Locals() {
		if camera.HowOf(pl.pic.camera) == camera.Inside {
			return "Shortcuts: first person, riding in the unit"
		}
	}
	return "Shortcuts"
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
	shortcutsMargin = 24
	shortcutsGap    = 16
)

var shortcutsDim = color.RGBA{A: 170}

// listing is the screen: the game's description, a gap, the title, and the groups of keys in
// columns as wide as their lines, each column as tall as the screen allows.
func (s *Shortcuts) listing(props game.Props) *ui.Element {
	font := ui.DefaultTheme().Font
	glyph, line := font.Measure("abcdefghij")
	glyphWidth, lineHeight := float64(glyph)/10, float64(line)
	width := float64(props.ScreenWidth) - 2*shortcutsMargin
	height := float64(props.ScreenHeight) - 2*shortcutsMargin

	var rows []ui.Part
	if d := s.description; d != "" {
		lines := wrapped(d, int(width/glyphWidth))
		rows = append(rows, ui.Fit(ui.Label(strings.Join(lines, "\n")).Named("shortcuts.about")),
			ui.Fixed(shortcutsGap, ui.Blank()))
		height -= float64(len(lines))*lineHeight + shortcutsGap
	}
	rows = append(rows, ui.Fit(ui.Title(s.title()).Named("shortcuts.title")),
		ui.Fixed(shortcutsGap, ui.Blank()))
	_, titleHeight := ui.DefaultTheme().Large.Measure(s.title())
	height -= float64(titleHeight) + shortcutsGap

	var columns []ui.Part
	var column []ui.Part
	used := 0.0
	for _, g := range s.groups() {
		tall := float64(len(g.lines)+1) * lineHeight
		if used > 0 && used+tall > height {
			columns = append(columns, ui.Fit(ui.Rows(column...).Named(columnName(len(columns)))),
				ui.Fixed(shortcutsGap*2, ui.Blank()))
			column, used = nil, 0
		}
		table := ui.Columns(
			ui.Fixed(shortcutsGap/2, ui.Blank()),
			ui.Fit(ui.Label(strings.Join(g.keys, "\n"))),
			ui.Fixed(shortcutsGap*2, ui.Blank()),
			ui.Fit(ui.Label(strings.Join(g.labels, "\n"))),
		)
		column = append(column, ui.Fit(ui.Heading(g.name)), ui.Fit(table),
			ui.Fixed(shortcutsGap, ui.Blank()))
		used += tall + shortcutsGap
	}
	if len(column) > 0 {
		columns = append(columns, ui.Fit(ui.Rows(column...).Named(columnName(len(columns)))))
	}
	rows = append(rows, ui.Share(1, ui.Columns(columns...)))
	return ui.Layers(
		ui.Blank().Fill(shortcutsDim),
		ui.Rows(rows...).Padding(shortcutsMargin),
	)
}

func columnName(i int) string { return fmt.Sprintf("shortcuts.column.%d", i/2) }

func wrapped(text string, width int) []string {
	var lines []string
	line := ""
	for _, word := range strings.Fields(text) {
		if line != "" && len(line)+1+len(word) > width {
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
