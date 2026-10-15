package telemetry

import (
	"image/color"
	"strings"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/ui"
	"github.com/kjkrol/uid"
)

const panelMargin = 8

// the theme's panel colour, more see-through
var panelFill = color.RGBA{R: 20, G: 22, B: 28, A: 150}

// a line of the panel: the game's own, label and value, or a reporter's
type line struct {
	label    string
	value    func() string
	reporter render.Reporter
}

// a value of several lines goes under its label, each indented, as the shortcuts list its groups
func (l line) write(sb *strings.Builder) {
	add := func(label, value string) {
		if sb.Len() > 0 {
			sb.WriteByte('\n')
		}
		sb.WriteString(label)
		sb.WriteByte(':')
		if !strings.Contains(value, "\n") {
			sb.WriteByte(' ')
			sb.WriteString(value)
			return
		}
		for part := range strings.SplitSeq(value, "\n") {
			sb.WriteString("\n  ")
			sb.WriteString(part)
		}
	}
	if l.reporter != nil {
		l.reporter.Report(add)
		return
	}
	add(l.label, l.value())
}

// panel is the scene: never active, so the game under it keeps the input
type panel struct {
	p      *Plugin
	screen *ui.Scene
}

var _ game.Scene = (*panel)(nil)

func newPanel(p *Plugin) *panel {
	s := &panel{p: p}
	s.screen = ui.NewScene(SceneName, ui.TopLeft(
		ui.Panel(ui.LabelOf(s)).Fill(panelFill).Border(color.RGBA{}, 0).Steady(),
	).Margin(panelMargin))
	return s
}

// Text is the panel's lines, read every frame (ui.Text).
func (s *panel) Text(uid.UID64, bool) (string, bool) {
	var sb strings.Builder
	for _, l := range s.p.lines {
		l.write(&sb)
	}
	return sb.String(), sb.Len() > 0
}

func (s *panel) Name() string { return SceneName }

// Drawers is the screen's drawer and one readying the reporters; after it nothing is added.
func (s *panel) Drawers() []render.Drawer {
	s.p.made = true
	return append(s.screen.Drawers(), reporters(s.p.reporters))
}

func (*panel) HandleEvents(*control.InputEvents, game.Runtime, game.Composition) {}

// Focusable is false: the panel never takes the input from the scene under it.
func (*panel) Focusable() bool { return false }

// a drawer that only readies the reporters: the panel's text draws their lines
type reporters []render.Reporter

func (rs reporters) Init(si *goke.SysInit) {
	for _, r := range rs {
		r.Init(si)
	}
}

func (reporters) Draw(*render.Image) {}
