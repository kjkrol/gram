package players

import (
	"fmt"
	"math"
	"strings"
	"sync"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/telemetry"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/ui"
	"github.com/kjkrol/uid"
	"golang.org/x/image/font/gofont/gobold"
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
	return &SettingsScene{p: p, screen: ui.NewScene(SettingsName, nil).Theme(settingsTheme())}
}

// settingsTheme is the default with the title in Go Bold at titleSize
func settingsTheme() ui.Theme {
	th := ui.DefaultTheme()
	th.Large = titleFont()
	return th
}

var titleFont = sync.OnceValue(func() *render.Font {
	f, err := render.NewFont(gobold.TTF, titleSize)
	if err != nil {
		panic(err)
	}
	return f
})

// Open shows the screen on top, built afresh of the plugins' settings, and holds the game.
func (s *SettingsScene) Open(runtime game.Runtime, composition game.Composition) {
	var issuer *Player
	if locals := s.p.Locals(); len(locals) > 0 {
		issuer = locals[0]
	}
	s.screen.Issue(s.p.IssueAs(issuer))
	s.build(screenSize(s.screen, runtime.Props()))
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
	settingsRow    = 32
	settingsLabel  = 220
	twinColumn     = 230 // a twin section's column of controls
	sectionPadding = 8   // inside a section's frame
	titleSize      = 28  // the title's font
)

// build replaces the screen with the listing laid out for a screen w by h
func (s *SettingsScene) build(w, h int) {
	s.w, s.h = w, h
	s.screen.Replace(s.listing(w, h))
}

// screenSize is the size a menu lays itself out for: the screen the scene was last laid over,
// the window as it is, else the Props' as the game starts
func screenSize(scene *ui.Scene, props game.Props) (w, h int) {
	if box := scene.Laid(); box.BottomRight.X > box.TopLeft.X {
		return int(box.BottomRight.X - box.TopLeft.X), int(box.BottomRight.Y - box.TopLeft.Y)
	}
	return props.ScreenWidth, props.ScreenHeight
}

// listing is the screen: the title and, centred in as many columns as the screen's width takes,
// the sections of settings, each framed, a heading and a line a setting, its label and its
// control; the columns broken by the screen's height where they fit its width, fewer and taller
// where they do not, scrolled then (ui.Scroll).
func (s *SettingsScene) listing(w, h int) *ui.Element {
	width := float64(w) - 2*shortcutsMargin
	height := float64(h) - 2*shortcutsMargin
	_, titleHeight := titleFont().Measure("Settings")
	height -= float64(titleHeight) + shortcutsGap

	blocks := s.tree().blocks("", 0, nil)
	columns := columned(blocks, height)
	for n := len(columns) - 1; n >= 1 && widthOf(columns, width, height) > width; n-- {
		columns = columned(blocks, tallEnough(blocks, n))
	}
	return ui.Layers(
		ui.Blank().Fill(shortcutsDim),
		ui.TopMiddle(ui.Rows(
			ui.Fit(ui.TopMiddle(ui.Title("Settings").Named("settings.title"))),
			ui.Fixed(shortcutsGap, ui.Blank()),
			ui.Fixed(height, ui.Scroll(ui.Columns(spaced(columns)...)).Named("settings.scroll")),
		)).Padding(shortcutsMargin),
	)
}

// columned is the blocks in columns at most height tall, a block never split
func columned(blocks []block, height float64) []*ui.Element {
	var columns []*ui.Element
	var column []ui.Part
	used := 0.0
	for _, b := range blocks {
		if used > 0 && used+b.tall > height {
			columns = append(columns, ui.Rows(column...))
			column, used = nil, 0
		}
		column = append(column, b.parts...)
		used += b.tall
	}
	if len(column) > 0 {
		columns = append(columns, ui.Rows(column...))
	}
	return columns
}

// tallEnough is the least height that holds the blocks in n columns
func tallEnough(blocks []block, n int) float64 {
	total, tallest := 0.0, 0.0
	for _, b := range blocks {
		total += b.tall
		tallest = max(tallest, b.tall)
	}
	height := max(total/float64(n), tallest)
	for len(columned(blocks, height)) > n {
		height += settingsRow
	}
	return height
}

// widthOf is the columns' width with the gaps between them, measured laid out once
func widthOf(columns []*ui.Element, width, height float64) float64 {
	probe := ui.NewScene("", ui.Columns(spaced(columns)...))
	probe.Drawers()
	probe.Lay(geom.NewAABB(geom.NewVec(0, 0), geom.NewVec(width, height)))
	w := 0.0
	for i := range columns {
		box := probe.Element(settingsColumn(i)).Box()
		w += box.BottomRight.X - box.TopLeft.X
	}
	return w + float64(len(columns)-1)*shortcutsGap*2
}

// spaced is the columns as parts, named by their place, a gap between them
func spaced(columns []*ui.Element) []ui.Part {
	parts := make([]ui.Part, 0, 2*len(columns))
	for i, c := range columns {
		if i > 0 {
			parts = append(parts, ui.Fixed(shortcutsGap*2, ui.Blank()))
		}
		parts = append(parts, ui.Fit(c.Named(settingsColumn(i))))
	}
	return parts
}

func settingsColumn(i int) string { return fmt.Sprintf("settings.column.%d", i) }

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

// a block never splits across columns: a top section framed whole, or within one a heading (a
// parent's without entries of its own joined to its first child's) and the lines right under it
type block struct {
	parts []ui.Part
	tall  float64
	bare  bool // headings alone so far
}

func (b *block) add(part ui.Part, tall float64) {
	b.parts = append(b.parts, part)
	b.tall += tall
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
		last.add(ui.Fixed(settingsRow, line(st, path, float64(level)*shortcutsGap/2)), settingsRow)
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

// framed is blocks as one: their parts in a panel bordered a pixel, named "frame:" and the
// section's path, a gap under it
func framed(blocks []block, path string) block {
	var parts []ui.Part
	tall := 0.0
	for _, b := range blocks {
		parts = append(parts, b.parts...)
		tall += b.tall
	}
	tall += 2 * sectionPadding
	frame := ui.Panel(ui.Rows(parts...)).Padding(sectionPadding).Named("frame:" + path)
	frame.Border(settingsTheme().Border, 1)
	return block{parts: []ui.Part{ui.Fixed(tall, frame), ui.Fixed(shortcutsGap, ui.Blank())},
		tall: tall + shortcutsGap}
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
	indent := float64(level-1) * shortcutsGap / 2
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
		dst[len(dst)-1].add(ui.Fixed(settingsRow, indented(indent+shortcutsGap/2, sub)), settingsRow)
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
	return ui.Fixed(settingsRow, indented(float64(level-1)*shortcutsGap/2, sub))
}

func indented(px float64, e *ui.Element) *ui.Element {
	return ui.Columns(ui.Fixed(px, ui.Blank()), ui.Fit(e))
}

// line is a setting's label and its control, named "setting:", the path of its section and the
// label ("setting:Picture/Graphics/Shadows"), indent pixels in: a Switch a checkbox, a Choice a
// dropdown, a Range a slider with the number beside it.
func line(st control.Setting, path string, indent float64) *ui.Element {
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
			ui.Fixed(shortcutsGap, ui.Blank()),
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
				Get:     func() int { return int(tp.Mode()) },
				Set: func(i int) command.Command {
					return telemetry.Panel{Shown: p.panelShown(), Extended: i == 1}
				},
			})
	}
	return out
}
