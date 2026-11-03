package topography

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugin/section"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/ground"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/topography/painter"
	"github.com/kjkrol/gram/render"
)

// Read reads the topography's state on one goroutine at a time: Plugin.Read(w), worker w's,
// through the board's read of the same worker.
type Read struct {
	p *Plugin
	w int
}

func (r *Read) board() *board.BoardRead { return r.p.boardPlugin.Read(r.w).Board() }

var _ cameras.Cage = (*Read)(nil)
var _ cameras.Grained = (*Read)(nil)

// Read reads the topography's state on worker w of the game's pool, 0 the game's own goroutine;
// the other workers' reads come with Install.
func (p *Plugin) Read(w int) *Read { return p.reads[w] }

// Write changes the topography's state, on the game's goroutine.
type Write struct{ p *Plugin }

var _ ground.Shaper = (*Write)(nil)

// Write changes the topography's state, on the game's goroutine.
func (p *Plugin) Write() *Write { return &p.write }

// StyleOf is how the kind named name looks, as Style set it.
func (r *Read) StyleOf(name string) painter.Style { return r.p.painter.StyleOf(name) }

// Hazes reports whether the air hazes the ground.
func (r *Read) Hazes() bool { return r.p.Res.Settings.Haze }

// Reporter is the topography's part of the telemetry panel: in a world cut into sectors, how many
// the last frame drawn drew the ground of at each level, a line each.
func (r *Read) Reporter() render.Reporter { return reporter{r.p} }

// Relief is the ground's heights, to read and set from a game's code.
func (w *Write) Relief() Relief { return w.p.relief }

// Flatten lays the cell c flat at h, the ground round it following within the Shaping's
// MaxStep, and reports whether it did. Nothing moves where a road would be left lying as
// Transport Tycoon would not lay it.
func (w *Write) Flatten(c cell.ID, h float32) bool {
	p := w.p
	return p.relief.KeepRoads(func() { p.relief.Level(c, h, p.cfg.Shaping.MaxStep) })
}

// Grade puts the cell c's corners at corners, in CellOutline's order, the ground round them
// following within the Shaping's MaxStep, and reports whether they stand so. Nothing moves
// where a road would be left lying as Transport Tycoon would not lay it.
func (w *Write) Grade(c cell.ID, corners []float32) bool {
	p := w.p
	return p.relief.KeepRoads(func() { p.relief.Grade(c, corners, p.cfg.Shaping.MaxStep) })
}

// Seed sets the ground's heights raised when the Stage starts fresh.
func (w *Write) Seed(heights func(p geom.Vec) float32) {
	if err := w.p.worldPlugin.InSection("the ground's heights seeded", section.Spawn); err != nil {
		panic("topography: " + err.Error())
	}
	w.p.seeded = heights
}

// SetHaze has the air haze the ground and what stands on it as thick as the weather makes it,
// through any camera: far off from an eye, over the ground from above; off by default, all clear
// (Settings.Haze).
func (w *Write) SetHaze(on bool) {
	p := w.p
	p.Res.Settings.Haze = on
	if p.ground != nil {
		p.ground.Haze(on)
	}
}
