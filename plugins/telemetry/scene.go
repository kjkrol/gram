package telemetry

import (
	"image/color"
	"strings"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/ui"
	"github.com/kjkrol/uid"
)

// pixels: the panel's distance from the corner, its padding, a value's indent under its label
// and the widest a line's words run before they wrap
const (
	panelMargin  = 8
	panelPadding = 8
	panelIndent  = 16
	panelWrap    = 260
)

// the theme's panel colour, more see-through
var (
	panelFill = color.RGBA{R: 20, G: 22, B: 28, A: 150}
	panelText = color.RGBA{R: 240, G: 240, B: 240, A: 255}
	textShade = color.RGBA{A: 200}
)

// a line of the panel: the game's own, label and value, or a reporter's; a basic one shown in
// both modes
type line struct {
	label    string
	value    func() string
	reporter render.Reporter
	basic    bool
}

// entry is a label and what stands by it: a one-line value beside it, a value of several
// lines under it, each indented; a line too wide wraps, its rest indented too
type entry struct {
	label  string
	beside string
	under  []string
}

// entries appends the line's entries to dst
func (l line) entries(dst []entry, f *render.Font) []entry {
	add := func(label, value string) {
		e := entry{label: label}
		parts := strings.Split(value, "\n")
		if len(parts) == 1 {
			lines := wrap(f, value, panelWrap)
			e.beside, e.under = lines[0], lines[1:]
		} else {
			for _, part := range parts {
				e.under = append(e.under, wrap(f, part, panelWrap)...)
			}
		}
		dst = append(dst, e)
	}
	if l.reporter != nil {
		l.reporter.Report(add)
		return dst
	}
	add(l.label, l.value())
	return dst
}

// wrap is s on as many lines as keep it within width pixels of f, broken between words; a word
// wider than that stands alone
func wrap(f *render.Font, s string, width float32) []string {
	words := strings.Fields(s)
	if len(words) == 0 {
		return []string{s}
	}
	var lines []string
	cur := words[0]
	for _, w := range words[1:] {
		if next := cur + " " + w; measure(f, next) <= width {
			cur = next
			continue
		}
		lines = append(lines, cur)
		cur = w
	}
	return append(lines, cur)
}

func measure(f *render.Font, s string) float32 {
	w, _ := f.Measure(s)
	return w
}

// text is the entries as plain lines, a value of several under its label indented, for a test
func text(entries []entry) string {
	var sb strings.Builder
	for _, e := range entries {
		if sb.Len() > 0 {
			sb.WriteByte('\n')
		}
		sb.WriteString(e.label)
		sb.WriteByte(':')
		if e.beside != "" {
			sb.WriteByte(' ')
			sb.WriteString(e.beside)
		}
		for _, u := range e.under {
			sb.WriteString("\n  ")
			sb.WriteString(u)
		}
	}
	return sb.String()
}

// how long the panel's lines stand before they are built anew
const refreshEvery = 250 * time.Millisecond

// panel is the scene: never active, so the game under it keeps the input; it draws itself in
// the top-left corner, the labels bold, as wide as its lines have been since the mode changed
type panel struct {
	p       *Plugin
	screen  *ui.Scene
	entries []entry
	at      time.Time // when the entries were built
	every   time.Duration
	width   float32 // the widest the lines have run in this mode
	font    *render.Font
	bold    *render.Font
}

var _ game.Scene = (*panel)(nil)
var _ render.Renderer = (*panel)(nil)

func newPanel(p *Plugin) *panel {
	s := &panel{p: p, every: refreshEvery}
	s.screen = ui.NewScene(SceneName, ui.Drawer(s))
	return s
}

// Text is the panel's lines of its mode as plain text (ui.Text), built anew four times a second.
func (s *panel) Text(uid.UID64, bool) (string, bool) {
	s.build()
	return text(s.entries), len(s.entries) > 0
}

// build has the entries built anew once every refresh
func (s *panel) build() {
	if now := time.Now(); now.Sub(s.at) >= s.every {
		s.fonts()
		extended := s.p.Res.Settings.Extended
		s.entries = s.entries[:0]
		for _, l := range s.p.lines {
			if l.basic || extended {
				s.entries = l.entries(s.entries, s.font)
			}
		}
		s.at = now
	}
}

func (s *panel) fonts() {
	if s.font == nil {
		th := ui.DefaultTheme()
		s.font, s.bold = th.Font, th.Bold
	}
}

// stale has the entries built anew at the next read, the panel as narrow as they are
func (s *panel) stale() { s.at, s.width = time.Time{}, 0 }

func (s *panel) Name() string { return SceneName }

// Drawers is the screen's drawer, which draws the panel and readies the reporters; after it
// nothing is added.
func (s *panel) Drawers() []render.Drawer {
	s.p.made = true
	return s.screen.Drawers()
}

func (*panel) HandleEvents(*control.InputEvents, game.Runtime, game.Composition) {}

// Focusable is false: the panel never takes the input from the scene under it.
func (*panel) Focusable() bool { return false }

// Init readies the reporters, whose lines the panel draws.
func (s *panel) Init(si *goke.SysInit) {
	for _, r := range s.p.reporters {
		r.Init(si)
	}
}

// Draw draws the panel in the screen's top-left corner: a see-through plate, the labels bold,
// each value beside its label or under it indented.
func (s *panel) Draw(screen *render.Image) {
	s.build()
	if len(s.entries) == 0 {
		return
	}
	lh := s.font.LineHeight()
	var width float32
	rows := 0
	for _, e := range s.entries {
		w := measure(s.bold, e.label+":")
		if e.beside != "" {
			w += measure(s.font, " "+e.beside)
		}
		width = max(width, w)
		for _, u := range e.under {
			width = max(width, panelIndent+measure(s.font, u))
		}
		rows += 1 + len(e.under)
	}
	s.width = max(s.width, width)
	b := screen.Bounds()
	x0, y0 := float32(b.Min.X+panelMargin), float32(b.Min.Y+panelMargin)
	screen.FillRect(x0, y0, s.width+2*panelPadding, float32(rows)*lh+2*panelPadding,
		panelFill)
	x, y := x0+panelPadding, y0+panelPadding
	for _, e := range s.entries {
		label := e.label + ":"
		s.write(screen, s.bold, label, x, y)
		if e.beside != "" {
			s.write(screen, s.font, " "+e.beside, x+measure(s.bold, label), y)
		}
		y += lh
		for _, u := range e.under {
			s.write(screen, s.font, u, x+panelIndent, y)
			y += lh
		}
	}
}

func (s *panel) write(dst *render.Image, f *render.Font, text string, x, y float32) {
	render.DrawText(dst, f, text, x+1, y+1, textShade)
	render.DrawText(dst, f, text, x, y, panelText)
}
