package players

import (
	"fmt"
	"math"

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

// SettingsScene is the screen of every plugin's settings (plugin.Tunable), a control each,
// grouped by plugin as the shortcuts are. F10 opens it, Esc or F10 closes it; while it is up the
// game stands in the tactical pause unless Settings.Running.
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

// listing is the screen: the title and, in columns as tall as the screen allows, a heading a
// plugin and a line a setting, its label and its control.
func (s *SettingsScene) listing(props game.Props) *ui.Element {
	height := float64(props.ScreenHeight) - 2*shortcutsMargin
	_, titleHeight := ui.DefaultTheme().Large.Measure("Settings")
	height -= float64(titleHeight) + shortcutsGap

	var columns []ui.Part
	var column []ui.Part
	used := 0.0
	for _, h := range s.p.handlers {
		t, ok := h.(plugin.Tunable)
		if !ok {
			continue
		}
		settings := t.Settings()
		if len(settings) == 0 {
			continue
		}
		tall := float64(len(settings)+1) * settingsRow
		if used > 0 && used+tall > height {
			columns = append(columns, ui.Fit(ui.Rows(column...).Named(settingsColumn(len(columns)))),
				ui.Fixed(shortcutsGap*2, ui.Blank()))
			column, used = nil, 0
		}
		column = append(column, ui.Fit(ui.Heading(handlerName(h))))
		for _, st := range settings {
			column = append(column, ui.Fixed(settingsRow, line(st)))
		}
		column = append(column, ui.Fixed(shortcutsGap, ui.Blank()))
		used += tall + shortcutsGap
	}
	if len(column) > 0 {
		columns = append(columns, ui.Fit(ui.Rows(column...).Named(settingsColumn(len(columns)))))
	}
	return ui.Layers(
		ui.Blank().Fill(shortcutsDim),
		ui.Rows(
			ui.Fit(ui.Title("Settings").Named("settings.title")),
			ui.Fixed(shortcutsGap, ui.Blank()),
			ui.Share(1, ui.Columns(columns...)),
		).Padding(shortcutsMargin),
	)
}

func settingsColumn(i int) string { return fmt.Sprintf("settings.column.%d", i/2) }

// line is a setting's label and its control, named "setting:" and the label: a Switch a
// checkbox, a Choice a dropdown, a Range a slider with the number beside it.
func line(st control.Setting) *ui.Element {
	var label string
	var ctl *ui.Element
	switch st := st.(type) {
	case control.Switch:
		return ui.Columns(ui.Fixed(shortcutsGap/2, ui.Blank()),
			ui.Fit(ui.MiddleLeft(ui.Checkbox(st.Label, st.Get, st.Set).Named("setting:"+st.Label))))
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
		ui.Fixed(shortcutsGap/2, ui.Blank()),
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
