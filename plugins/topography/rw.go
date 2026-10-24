package topography

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugin/section"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/ground"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/topography/painter"
	"github.com/kjkrol/gram/plugins/topography/relief"
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
var _ cameras.Lever = (*Write)(nil)

// Write changes the topography's state, on the game's goroutine.
func (p *Plugin) Write() *Write { return &p.write }

// StyleOf is how the kind named name looks, as Style set it.
func (r *Read) StyleOf(name string) painter.Style { return r.p.painter.StyleOf(name) }

// Hazes reports whether the air hazes the ground.
func (r *Read) Hazes() bool { return r.p.Res.Settings.Haze }

// Care is how many sectors out from the middle of the screen the ground is shaded with full
// care, 0 every one.
func (r *Read) Care() int {
	if r.p.ground == nil {
		return 0
	}
	return r.p.Res.Settings.Care
}

// CareAdaptive reports whether the full care's reach follows the frame budget.
func (r *Read) CareAdaptive() bool { return r.p.Res.Settings.AdaptiveCare }

// CareShown reports whether the care's bands are drawn.
func (r *Read) CareShown() bool { return r.p.Res.Settings.ShowCare }

// Reporter is the topography's part of the telemetry panel: how far out the ground is shaded
// with full care and how many sectors took each care in the last frame, a line each.
func (r *Read) Reporter() render.Reporter { return reporter{r.p} }

// Climbing is what slopes do to whoever goes over them, as the Config set it.
func (r *Read) Climbing() relief.Climbing { return r.p.climbing }

// Relief is the ground's heights, to read and set from a game's code.
func (w *Write) Relief() Relief { return w.p.relief }

// Flatten lays the cell c flat at h, the ground round it following within the Shaping's
// MaxStep, and reports whether it did. Nothing moves where a road would be left lying as
// Transport Tycoon would not lay it.
func (w *Write) Flatten(c cell.ID, h float64) bool {
	p := w.p
	return p.relief.KeepRoads(func() { p.relief.Level(c, h, p.cfg.Shaping.MaxStep) })
}

// Seed sets the ground's heights raised when the Stage starts fresh.
func (w *Write) Seed(heights func(p geom.Vec) float64) {
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
	if p.hexes != nil {
		p.hexes.Haze(on)
	}
}

// SetCare has the ground within rings sectors of the one under the middle of the screen (and of
// the one under the eye) shaded with full care and one care less every rings sectors farther,
// down the five cares (the package's doc, "Drawing and commands"); 0, every sector with full
// care. Nothing on a hex board.
func (w *Write) SetCare(rings int) {
	p := w.p
	if p.ground != nil {
		p.ground.SetCare(rings)
		p.Res.Settings.Care = p.ground.Care()
	}
}

// AdaptCare has the full care's reach follow the cameras' frame budget, or not
// (Settings.AdaptiveCare): the topography's Write is a cameras.Lever, stepped down the care's
// ladder as the rate falls under the budget and back up as it stays over.
func (w *Write) AdaptCare(on bool) { w.p.Res.Settings.AdaptiveCare = on }

// ShowCare has the care's bands drawn over the ground, or not, as the ShowCare command switches
// it (Settings.ShowCare): the boundaries between sectors of different care. Nothing on a hex
// board, nor with every sector shaded with full care.
func (w *Write) ShowCare(shown bool) { w.p.Res.Settings.ShowCare = shown }

// Following is whether the care follows the frame budget (cameras.Lever).
func (w *Write) Following() bool { return w.p.Res.Settings.AdaptiveCare }

// Coarser steps the full care's reach a rung down the ladder, true where it could
// (cameras.Lever).
func (w *Write) Coarser() bool { return w.stepCare(-1) }

// Finer steps the full care's reach a rung up the ladder, every sector the last, true where it
// could (cameras.Lever).
func (w *Write) Finer() bool { return w.stepCare(1) }

func (w *Write) stepCare(step int) bool {
	p := w.p
	if p.ground == nil {
		return false
	}
	before := p.read.Care()
	w.SetCare(p.careStepped(step))
	return p.read.Care() != before
}
