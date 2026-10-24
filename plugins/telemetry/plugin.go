package telemetry

import (
	"fmt"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

// SceneName is the name of the panel's scene in a Stage's stack.
const SceneName = "gram.telemetry"

// Plugin is the telemetry panel: its lines, and the scene that draws them top-left.
type Plugin struct {
	*world.Self

	Res Resources

	worldPlugin *world.Plugin
	tps         *game.TPS
	lines       []line
	reporters   []render.Reporter
	panel       *panel
	kept        *plugin.Kept[Settings]
	made        bool
	read        Read
	write       Write

	readings render.Buffered[reading]
	live     reading // read before any is published
	count    uint64
	readAt   time.Time
	readMode bool
	every    time.Duration
	fresh    bool // a reading published, not yet swapped
	swapped  bool
}

// reading is the panel's lines as read off the game: each label and its value, in the mode they
// were read in, the count of readings so far
type reading struct {
	said     []said
	extended bool
	n        uint64
}

var _ plugin.Plugin = (*Plugin)(nil)
var _ plugin.Restorer = (*Plugin)(nil)
var _ plugin.Publisher = (*Plugin)(nil)
var _ game.Scenic = (*Plugin)(nil)

// Mode is what the panel shows: the basics, the frame rate, the ticks a second, where a frame's
// time goes and the entities, or every line.
type Mode int

const (
	Basic Mode = iota
	Extended
)

// NewPlugin makes the panel over worldPlugin's world with the standard lines: the basics, the
// frame rate, the ticks a second, where a frame's time goes and the entities, and the game time.
func NewPlugin(worldPlugin *world.Plugin) *Plugin {
	p := &Plugin{worldPlugin: worldPlugin, every: refreshEvery}
	p.read.p, p.write.p = p, p
	p.Self = world.NewSelf(worldPlugin, p)
	p.panel = newPanel(p)
	p.kept = plugin.Keep(&p.Res.Settings)
	p.lines = []line{
		{label: "FPS", value: fps, basic: true},
		{label: "TPS", value: p.ticks, basic: true},
		{label: "Frame", value: frameTime, basic: true},
		{label: "Entities", value: p.entities, basic: true},
	}
	p.With(worldPlugin.Read(0).Clock().Reporter())
	return p
}

// Of is the telemetry plugin made over worldPlugin, nil where none was.
func Of(worldPlugin *world.Plugin) *Plugin {
	for _, made := range worldPlugin.Made().All() {
		if p, ok := made.(*Plugin); ok {
			return p
		}
	}
	return nil
}

// With adds the lines of reporters, in order, under the lines so far, shown in the Extended mode;
// before the Stage's scenes are made.
func (p *Plugin) With(reporters ...render.Reporter) *Plugin {
	p.adding()
	p.reporters = append(p.reporters, reporters...)
	for _, r := range reporters {
		p.lines = append(p.lines, line{reporter: r})
	}
	return p
}

// Line adds a line of the game's own, label: value, value read four times a second, shown in the
// Extended mode; a value of several lines stands under its label, each indented. Before the
// Stage's scenes are made.
func (p *Plugin) Line(label string, value func() string) *Plugin {
	p.adding()
	p.lines = append(p.lines, line{label: label, value: value})
	return p
}

// Clear drops every line, the standard ones too, for a game starting from nothing; before the
// Stage's scenes are made.
func (p *Plugin) Clear() *Plugin {
	p.adding()
	p.lines, p.reporters = nil, nil
	return p
}

func (p *Plugin) adding() {
	if p.made {
		panic("telemetry: a line added after the scene was made")
	}
}

// Scenes is the panel's one scene (game.Scenic), hidden until shown.
func (p *Plugin) Scenes() []game.Scene { return []game.Scene{p.panel} }

func (p *Plugin) ticks() string {
	if p.tps == nil {
		return "-"
	}
	return fmt.Sprint(p.tps.Ticks)
}

func (p *Plugin) entities() string { return fmt.Sprint(p.worldPlugin.Res.Telemetry.Count) }

func fps() string { return fmt.Sprintf("%0.1f", render.ActualFPS()) }

func frameTime() string {
	update, draw, present := render.Spent()
	ms := func(d time.Duration) string { return fmt.Sprintf("%.1f ms", float64(d)/1e6) }
	return "tick " + ms(update) + "\ndrawing " + ms(draw) + "\npresenting " + ms(present)
}

// ---- plugin.Plugin ----

func (p *Plugin) Name() string { return "gram.telemetry" }

// Install reads the engine's tick counter where the Installer has one.
func (p *Plugin) Install(ctx plugin.Installer) error {
	if t, ok := ctx.(interface{ TPS() *game.TPS }); ok {
		p.tps = t.TPS()
	}
	return nil
}

func (p *Plugin) RunPlan(goke.RunCtx, time.Duration) {}

// EventHandler is nil: the players' key shows and hides the panel.
func (p *Plugin) EventHandler() control.EventHandler { return nil }

// Serializable is the panel's Settings, kept whole; whether it is shown is the Composition's.
func (p *Plugin) Serializable() plugin.Serializable { return p.kept }

// Restore takes the Settings loaded.
func (p *Plugin) Restore() { p.kept.Restore() }

// Publish reads the panel's lines off the game, the reporters' too, four times a second and as
// the mode changes.
func (p *Plugin) Publish() {
	if p.due() {
		p.readInto(p.readings.Back())
		p.fresh = true
	}
}

// Swap has the lines read last drawn.
func (p *Plugin) Swap() {
	if p.fresh {
		p.readings.Swap()
		p.fresh, p.swapped = false, true
	}
}

// drawn is the lines the frame drawn shows: as last published, read now before any was
func (p *Plugin) drawn() *reading {
	if p.swapped {
		return p.readings.Front()
	}
	if p.due() {
		p.readInto(&p.live)
	}
	return &p.live
}

// due reports whether the lines are to be read again: never read, a refresh gone by, the mode
// changed
func (p *Plugin) due() bool {
	return p.count == 0 || time.Since(p.readAt) >= p.every ||
		p.readMode != p.Res.Settings.Extended
}

func (p *Plugin) readInto(r *reading) {
	p.count++
	p.readAt, p.readMode = time.Now(), p.Res.Settings.Extended
	r.n, r.extended, r.said = p.count, p.readMode, r.said[:0]
	for _, l := range p.lines {
		if l.basic || r.extended {
			r.said = l.read(r.said)
		}
	}
}
