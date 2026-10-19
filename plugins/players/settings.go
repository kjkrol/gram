package players

import (
	"fmt"
	"math"
	"strings"

	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/telemetry"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/ui"
	"github.com/kjkrol/uid"
)

// SettingsName is the name of the settings scene in a game's stack.
const SettingsName = "gram.settings"

// Settings is what the players keep of their own, saved with the game: whether the game runs on
// behind the settings screen, or stands in the tactical pause while it is up.
type Settings struct {
	Running bool
}

// Resources is what the players publish: their Settings.
type Resources struct {
	Settings Settings
}

// SettingsScene is the screen of every plugin's settings (plugin.Tunable), a control each, in
// sections: those the plugins share first, then a plugin's own. F10 opens it, Esc or F10 closes
// it; while it is up the game stands in the tactical pause unless Settings.Running.
type SettingsScene struct {
	p      *Plugin
	screen *ui.Scene
	shown  bool
	paused bool // the clock was paused as the screen opened: left so as it closes
}

var _ game.Scene = (*SettingsScene)(nil)

func newSettings(p *Plugin) *SettingsScene {
	return &SettingsScene{p: p, screen: ui.NewScene(SettingsName, nil)}
}

// Open shows the screen on top, built afresh of the plugins' settings, and holds the game.
func (s *SettingsScene) Open(runtime game.Runtime, composition game.Composition) {
	var issuer *Player
	if locals := s.p.Locals(); len(locals) > 0 {
		issuer = locals[0]
	}
	s.screen.Issue(s.p.IssueAs(issuer))
	s.screen.Replace(s.listing(runtime.Props()))
	composition.Show(SettingsName)
	s.shown = true
	s.paused = s.p.worldPlugin.Clock().State().Paused
	s.hold(!s.p.Res.Settings.Running)
}

// Close hides the screen and lets the game go on, unless it was paused before.
func (s *SettingsScene) Close(runtime game.Runtime, composition game.Composition) {
	composition.Hide(SettingsName)
	s.shown = false
	s.hold(false)
}

// hold has the clock stand while on, as it stood before the screen opened otherwise.
func (s *SettingsScene) hold(on bool) {
	clk := s.p.worldPlugin.Clock()
	if want := on || s.paused; clk.State().Paused != want {
		clk.TogglePause()
	}
}

func (*SettingsScene) Name() string { return SettingsName }

func (s *SettingsScene) Drawers() []render.Drawer { return s.screen.Drawers() }

// HandleEvents closes the screen on Esc or F10 and hands the rest to the controls.
func (s *SettingsScene) HandleEvents(events *control.InputEvents, runtime game.Runtime,
	composition game.Composition) {
	for _, k := range events.KeyEvents {
		if k.Action != control.ActionPress || events.Modifiers.Shift || events.Modifiers.Ctrl {
			continue
		}
		if k.Key == control.KeyF10 || k.Key == control.KeyEscape && !s.screen.Unfolded() {
			s.Close(runtime, composition)
			return
		}
	}
	s.screen.HandleEvents(events, runtime, composition)
	s.p.carry(runtime, composition)
}

func (*SettingsScene) Focusable() bool { return true }

// pixels
const (
	settingsRow   = 32
	settingsLabel = 220
)

// listing is the screen: the title and, centred in columns as tall as the screen allows, the
// sections of settings, a heading each and a line a setting, its label and its control.
func (s *SettingsScene) listing(props game.Props) *ui.Element {
	height := float64(props.ScreenHeight) - 2*shortcutsMargin
	_, titleHeight := ui.DefaultTheme().Large.Measure("Settings")
	height -= float64(titleHeight) + shortcutsGap

	var columns []ui.Part
	var column []ui.Part
	used := 0.0
	for _, b := range s.tree().blocks("", 0, nil) {
		if used > 0 && used+b.tall > height {
			columns = append(columns,
				ui.Fit(ui.Rows(column...).Named(settingsColumn(len(columns)))),
				ui.Fixed(shortcutsGap*2, ui.Blank()))
			column, used = nil, 0
		}
		column = append(column, b.parts...)
		used += b.tall
	}
	if len(column) > 0 {
		columns = append(columns, ui.Fit(ui.Rows(column...).Named(settingsColumn(len(columns)))))
	}
	return ui.Layers(
		ui.Blank().Fill(shortcutsDim),
		ui.TopMiddle(ui.Rows(
			ui.Fit(ui.TopMiddle(ui.Title("Settings").Named("settings.title"))),
			ui.Fixed(shortcutsGap, ui.Blank()),
			ui.Fit(ui.Columns(columns...)),
		)).Padding(shortcutsMargin),
	)
}

func settingsColumn(i int) string { return fmt.Sprintf("settings.column.%d", i/2) }

// a section of the screen: its name, the settings right under it and its subsections
type node struct {
	name     string
	settings []control.Setting
	subs     []*node
}

// the sections the plugins share, in the order listed
var sharedSections = []string{
	control.Picture,
	control.PictureResolution,
	control.PictureAdaptive,
	control.PictureRings,
	control.PictureCare,
	control.PictureAreas,
	control.Graphics,
}

// tree is the sections in order: those the plugins share first, then a plugin's own, each
// setting under its Section or in its plugin's; a section with nothing under it is left out.
func (s *SettingsScene) tree() *node {
	root := &node{}
	for _, path := range sharedSections {
		root.at(path)
	}
	for _, h := range s.p.handlers {
		t, ok := h.(plugin.Tunable)
		if !ok {
			continue
		}
		own := root.at(handlerName(h))
		for _, st := range t.Settings() {
			to := own
			if path := control.SectionOf(st); path != "" {
				to = root.at(path)
			}
			to.settings = append(to.settings, st)
		}
	}
	root.prune()
	return root
}

// at is the section at path ("a/b/c") under n, made where it is not there.
func (n *node) at(path string) *node {
	name, rest, _ := strings.Cut(path, "/")
	var sub *node
	for _, c := range n.subs {
		if c.name == name {
			sub = c
			break
		}
	}
	if sub == nil {
		sub = &node{name: name}
		n.subs = append(n.subs, sub)
	}
	if rest == "" {
		return sub
	}
	return sub.at(rest)
}

func (n *node) prune() {
	kept := n.subs[:0]
	for _, c := range n.subs {
		c.prune()
		if len(c.settings) > 0 || len(c.subs) > 0 {
			kept = append(kept, c)
		}
	}
	n.subs = kept
}

// a block never splits across columns: a heading (a parent's without entries of its own
// joined to its first child's) and the lines right under it
type block struct {
	parts []ui.Part
	tall  float64
	bare  bool // headings alone so far
}

func (b *block) add(part ui.Part, tall float64) {
	b.parts = append(b.parts, part)
	b.tall += tall
}

// blocks appends n's blocks to dst: its heading, its settings' lines, then its subsections';
// the root (level 0) has no heading and a top section's last block ends in a gap.
func (n *node) blocks(path string, level int, dst []block) []block {
	if level > 0 {
		if i := len(dst) - 1; i < 0 || !dst[i].bare {
			dst = append(dst, block{bare: true})
		}
		dst[len(dst)-1].add(n.heading(path, level), settingsRow)
	}
	for _, st := range n.settings {
		last := &dst[len(dst)-1]
		last.add(ui.Fixed(settingsRow, line(st, float64(level)*shortcutsGap/2)), settingsRow)
		last.bare = false
	}
	for _, c := range n.subs {
		under := c.name
		if path != "" {
			under = path + "/" + c.name
		}
		dst = c.blocks(under, level+1, dst)
	}
	if level == 1 {
		dst[len(dst)-1].add(ui.Fixed(shortcutsGap, ui.Blank()), shortcutsGap)
	}
	return dst
}

// heading is the section's name, named "section:" and its path: a Heading at the top level,
// a Subheading in a row of its own deeper, indented half a gap a level.
func (n *node) heading(path string, level int) ui.Part {
	if level == 1 {
		return ui.Fit(ui.Heading(n.name).Named("section:" + path))
	}
	sub := ui.MiddleLeft(ui.Subheading(n.name).Named("section:" + path))
	return ui.Fixed(settingsRow, indented(float64(level-1)*shortcutsGap/2, sub))
}

func indented(px float64, e *ui.Element) *ui.Element {
	return ui.Columns(ui.Fixed(px, ui.Blank()), ui.Fit(e))
}

// line is a setting's label and its control, named "setting:" and the label, indent pixels in: a
// Switch a checkbox, a Choice a dropdown, a Range a slider with the number beside it.
func line(st control.Setting, indent float64) *ui.Element {
	var label string
	var ctl *ui.Element
	switch st := st.(type) {
	case control.Switch:
		return indented(indent,
			ui.MiddleLeft(ui.Checkbox(st.Label, st.Get, st.Set).Named("setting:"+st.Label)))
	case control.Choice:
		label, ctl = st.Label, ui.Dropdown(st.Options, st.Get, st.Set).Named("setting:"+st.Label)
	case control.Range:
		label = st.Label
		shows := st.Shows
		if shows == nil {
			shows = func(v float64) string { return fmt.Sprintf("%g", v) }
		}
		ctl = ui.Columns(
			ui.Fit(ui.MiddleLeft(ui.Slider(st.Min, st.Max, st.Step, st.Get, st.Set).
				Named("setting:"+st.Label))),
			ui.Fixed(shortcutsGap, ui.Blank()),
			ui.Fit(ui.MiddleLeft(ui.LabelOf(written{get: st.Get, shows: shows}))),
		)
	default:
		return ui.Blank()
	}
	return ui.Columns(
		ui.Fixed(indent, ui.Blank()),
		ui.Fixed(settingsLabel, ui.MiddleLeft(ui.Label(label))),
		ui.Fit(ui.MiddleLeft(ctl)),
	)
}

// written is a Range's number as its Shows writes it, read every frame (ui.Text).
type written struct {
	get   func() float64
	shows func(float64) string
}

func (w written) Text(uid.UID64, bool) (string, bool) {
	v := w.get()
	if math.IsNaN(v) {
		return "", true
	}
	return w.shows(v), true
}

// Settings are the players' own: whether the game runs on behind the settings screen, and the
// telemetry panel.
func (p *Plugin) Settings() []control.Setting {
	out := []control.Setting{control.Switch{
		Label: "The game runs on behind this screen",
		Get:   func() bool { return p.Res.Settings.Running },
		Set:   func(on bool) command.Command { return KeepRunning{On: on} },
	}}
	if tp := telemetry.Of(p.worldPlugin); tp != nil {
		out = append(out, control.Switch{
			Label: "Telemetry panel",
			Get:   p.panelShown,
			Set:   func(on bool) command.Command { return telemetry.Panel{Shown: on} },
		})
	}
	return out
}
