package topography

import (
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	irelief "github.com/kjkrol/gram/plugins/topography/internal/relief"
)

// Raise lifts the ground at At by one Shaping.Step: the nearest corner on a square grid, the cell
// on a hex one.
type Raise struct{ At geom.Vec }

// Lower sinks the ground at At by one Shaping.Step, as Raise lifts it.
type Lower struct{ At geom.Vec }

// Level brings the ground between From and To to the height at From.
type Level struct{ From, To geom.Vec }

// Shaping is how Raise, Lower and Level move the ground: Step per Raise or Lower, and at most
// MaxStep between two corners along a cell's edge (two neighbouring cells on a hex grid), the
// ground round about following as in Transport Tycoon. MaxStep zero lets any slope stand.
type Shaping struct{ Step, MaxStep float64 }

// shaper carries out the shaping commands — Raise, Lower, Level — on the relief as its system runs,
// moving the ground as its Shaping says; its queues take them.
type shaper struct {
	relief *irelief.Relief
	cfg    Shaping
	raise  control.Queue[Raise]
	lower  control.Queue[Lower]
	level  control.Queue[Level]
}

// queues are the shaping commands' queues: Raise, Lower, Level.
func (s *shaper) queues() []control.CommandQueue {
	return []control.CommandQueue{&s.raise, &s.lower, &s.level}
}

// system is the shaper as a goke.System, run once a tick in the interface part of a plan.
func (s *shaper) system() goke.System { return shapingSystem{s} }

var _ goke.System = shapingSystem{}

// shapingSystem carries out the shaping commands as they come.
type shapingSystem struct{ s *shaper }

func (shapingSystem) Init(*goke.SysInit) {}

func (y shapingSystem) Update(*goke.CmdBuf, time.Duration) {
	s, r := y.s, y.s.relief
	s.raise.Drain(func(i control.Issued[Raise]) { r.Lift(i.Command.At, s.cfg.Step, s.cfg.MaxStep) })
	s.lower.Drain(func(i control.Issued[Lower]) { r.Lift(i.Command.At, -s.cfg.Step, s.cfg.MaxStep) })
	s.level.Drain(func(i control.Issued[Level]) { r.Flatten(i.Command.From, i.Command.To, s.cfg.MaxStep) })
}
