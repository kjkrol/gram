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
	w, h   int  // the screen the listing was built for
}

var _ game.Scene = (*SettingsScene)(nil)

func newSettings(p *Plugin) *SettingsScene {
	return &SettingsScene{p: p, screen: ui.NewScene(SettingsName, nil)}
}

// Open shows the screen on top, built afresh of the plugins' settings, and holds the game.
func (s *SettingsScene) Open(runtime game.Runtime, composition game.Composition) {
	var issuer *Player
	if locals := s.p.read.Locals(); len(locals) > 0 {
		issuer = locals[0]
	}
	s.screen.Issue(s.p.IssueAs(issuer))
	s.build(screenSize(s.screen, runtime.Props()))
	composition.Show(SettingsName)
	s.shown = true
	s.paused = s.p.worldPlugin.Read(0).Clock().State().Paused
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
	clk := s.p.worldPlugin.Write().Clock()
	if want := on || s.paused; clk.Read().State().Paused != want {
		clk.TogglePause()
	}
}

func (*SettingsScene) Name() string { return SettingsName }

func (s *SettingsScene) Drawers() []render.Renderer { return s.screen.Drawers() }

// HandleEvents closes the screen on Esc or F10 and hands the rest to the controls; the cursor
// is let go while the screen is up, a player riding or not; a screen of another size, the
// window resized or full, has the listing built anew for it.
func (s *SettingsScene) HandleEvents(events *control.InputEvents, runtime game.Runtime,
	composition game.Composition) {
	s.p.capture()
	if w, h := screenSize(s.screen, runtime.Props()); w != s.w || h != s.h {
		s.build(w, h)
	}
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
	twinColumn    = 230 // a twin section's column of controls
)

// build replaces the screen with the listing laid out for a screen w by h
func (s *SettingsScene) build(w, h int) {
	s.w, s.h = w, h
	s.screen.Replace(s.listing(w, h))
}

// listing is the screen: the title and the sections of settings, each framed, a heading and a
// line a setting, its label and its control, in the menus' columns.
func (s *SettingsScene) listing(w, h int) *ui.Element {
	return menu("settings", "Settings", s.tree().blocks("", 0, nil), w, h)
}

// a section of the screen: its name, the settings right under it and its subsections
type node struct {
	name     string
	settings []control.Setting
	subs     []*node
}

// the sections the plugins share, in the order listed
var sharedSections = []string{
	control.Picture,
	control.PictureTactical,
	control.PictureTacticalLayers,
	control.PictureRiding,
	control.PictureRidingLayers,
	control.PictureCare,
	control.PictureShadows,
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

// blocks appends n's blocks to dst: its heading, its settings' lines, then its subsections',
// twins side by side; the root (level 0) has no heading and gives a block a top section, its
// blocks framed as one.
func (n *node) blocks(path string, level int, dst []block) []block {
	if level == 0 {
		for _, group := range n.grouped() {
			paths := under(path, group)
			var inner []block
			if len(group) > 1 {
				inner = twinBlocks(paths, 1, group, nil, true)
			} else {
				inner = group[0].blocks(paths[0], 1, nil)
			}
			dst = append(dst, framed(inner, paths[0]))
		}
		return dst
	}
	if i := len(dst) - 1; i < 0 || !dst[i].bare {
		dst = append(dst, block{bare: true})
	}
	dst[len(dst)-1].add(n.heading(path, level), settingsRow)
	for _, st := range n.settings {
		last := &dst[len(dst)-1]
		last.add(ui.Fixed(settingsRow, line(st, path, float32(level)*menuGap/2)), settingsRow)
		last.bare = false
	}
	for _, group := range n.grouped() {
		if len(group) > 1 {
			dst = twinBlocks(under(path, group), level+1, group, dst, true)
			continue
		}
		dst = group[0].blocks(under(path, group)[0], level+1, dst)
	}
	return dst
}

// under is the path of each of subs under path
func under(path string, subs []*node) []string {
	paths := make([]string, len(subs))
	for i, c := range subs {
		paths[i] = c.name
		if path != "" {
			paths[i] = path + "/" + c.name
		}
	}
	return paths
}

// shape is what a section looks like: its settings' labels and kinds and its subsections'
// names and shapes; sections of one shape are twins, laid side by side
func (n *node) shape() string {
	var b strings.Builder
	for _, st := range n.settings {
		fmt.Fprintf(&b, "%T %s;", st, labelOf(st))
	}
	for _, c := range n.subs {
		fmt.Fprintf(&b, "%s{%s}", c.name, c.shape())
	}
	return b.String()
}

// grouped is n's subs in order, twins of one shape together in the first's place
func (n *node) grouped() [][]*node {
	var groups [][]*node
	at := map[string]int{}
	for _, c := range n.subs {
		shape := c.shape()
		if i, ok := at[shape]; ok && shape != "" {
			groups[i] = append(groups[i], c)
			continue
		}
		at[shape] = len(groups)
		groups = append(groups, []*node{c})
	}
	return groups
}

// twinBlocks appends the twins' blocks to dst laid side by side: a row of their names over a
// column each where named, a line a setting with its label once and a control a twin, their
// subsections the same under one subheading
func twinBlocks(paths []string, level int, twins []*node, dst []block, named bool) []block {
	if i := len(dst) - 1; i < 0 || !dst[i].bare {
		dst = append(dst, block{bare: true})
	}
	indent := float32(level-1) * menuGap / 2
	if named {
		names := []ui.Part{ui.Fixed(indent, ui.Blank()), ui.Fixed(settingsLabel, ui.Blank())}
		for i, c := range twins {
			names = append(names, ui.Fixed(twinColumn,
				ui.MiddleLeft(ui.Subheading(c.name).Named("section:"+paths[i]))))
		}
		dst[len(dst)-1].add(ui.Fixed(settingsRow, ui.Columns(names...)), settingsRow)
	}
	for k, st := range twins[0].settings {
		row := []ui.Part{ui.Fixed(indent, ui.Blank()),
			ui.Fixed(settingsLabel, ui.MiddleLeft(ui.Label(labelOf(st))))}
		for i, c := range twins {
			row = append(row, ui.Fixed(twinColumn, ui.MiddleLeft(controlOf(c.settings[k], paths[i]))))
		}
		last := &dst[len(dst)-1]
		last.add(ui.Fixed(settingsRow, ui.Columns(row...)), settingsRow)
		last.bare = false
	}
	for k := range twins[0].subs {
		subs := make([]*node, len(twins))
		for i, c := range twins {
			subs[i] = c.subs[k]
		}
		dst = append(dst, block{bare: true})
		sub := ui.MiddleLeft(ui.Subheading(subs[0].name).
			Named("section:" + paths[0] + "/" + subs[0].name))
		dst[len(dst)-1].add(ui.Fixed(settingsRow, indented(indent+menuGap/2, sub)), settingsRow)
		subPaths := make([]string, len(subs))
		for i, c := range subs {
			subPaths[i] = paths[i] + "/" + c.name
		}
		dst = twinBlocks(subPaths, level+1, subs, dst, false)
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
	return ui.Fixed(settingsRow, indented(float32(level-1)*menuGap/2, sub))
}

func indented(px float32, e *ui.Element) *ui.Element {
	return ui.Columns(ui.Fixed(px, ui.Blank()), ui.Fit(e))
}

// line is a setting's label and its control, named "setting:", the path of its section and the
// label ("setting:Picture/Graphics/Shadows"), indent pixels in: a Switch a checkbox, a Choice a
// dropdown, a Range a slider with the number beside it.
func line(st control.Setting, path string, indent float32) *ui.Element {
	if st, ok := st.(control.Switch); ok {
		return indented(indent,
			ui.MiddleLeft(ui.Checkbox(st.Label, st.Get, st.Set).Named(settingName(path, st.Label))))
	}
	return ui.Columns(
		ui.Fixed(indent, ui.Blank()),
		ui.Fixed(settingsLabel, ui.MiddleLeft(ui.Label(labelOf(st)))),
		ui.Fit(ui.MiddleLeft(controlOf(st, path))),
	)
}

// controlOf is a setting's control alone, without its label, named as line names it
func controlOf(st control.Setting, path string) *ui.Element {
	switch st := st.(type) {
	case control.Switch:
		return ui.Checkbox("", st.Get, st.Set).Named(settingName(path, st.Label))
	case control.Choice:
		return ui.Dropdown(st.Options, st.Get, st.Set).Named(settingName(path, st.Label))
	case control.Range:
		shows := st.Shows
		if shows == nil {
			shows = func(v float64) string { return fmt.Sprintf("%g", v) }
		}
		return ui.Columns(
			ui.Fit(ui.MiddleLeft(ui.Slider(st.Min, st.Max, st.Step, st.Get, st.Set).
				Named(settingName(path, st.Label)))),
			ui.Fixed(menuGap, ui.Blank()),
			ui.Fit(ui.MiddleLeft(ui.LabelOf(written{get: st.Get, shows: shows}))),
		)
	}
	return ui.Blank()
}

func labelOf(st control.Setting) string {
	switch st := st.(type) {
	case control.Switch:
		return st.Label
	case control.Choice:
		return st.Label
	case control.Range:
		return st.Label
	}
	return ""
}

func settingName(path, label string) string { return "setting:" + path + "/" + label }

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
// telemetry panel, shown or not, the basics or everything.
func (p *Plugin) Settings() []control.Setting {
	out := []control.Setting{control.Switch{
		Label: "The game runs on behind this screen",
		Get:   func() bool { return p.Res.Settings.Running },
		Set:   func(on bool) command.Command { return KeepRunning{On: on} },
	}}
	if tp := telemetry.Of(p.worldPlugin); tp != nil {
		out = append(out,
			control.Switch{
				Label: "Telemetry panel",
				Get:   p.panelShown,
				Set: func(on bool) command.Command {
					return telemetry.Panel{Shown: on, Extended: p.panelExtended()}
				},
			},
			control.Choice{
				Label:   "Telemetry panel shows",
				Options: []string{"the basics", "everything"},
				Get:     func() int { return int(tp.Read(0).Mode()) },
				Set: func(i int) command.Command {
					return telemetry.Panel{Shown: p.panelShown(), Extended: i == 1}
				},
			})
	}
	return out
}
