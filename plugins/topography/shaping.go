package topography

import (
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/goke/v3"
	irelief "github.com/kjkrol/gram/plugins/topography/internal/relief"
)

// Raise lifts the ground at At by one Shaping.Step: the nearest corner on a square grid, the cell
// on a hex one.
type Raise struct{ At geom.Vec }

// Lower sinks the ground at At by one Shaping.Step, as Raise lifts it.
type Lower struct{ At geom.Vec }

// Level brings the ground between From and To to the height at From.
type Level struct{ From, To geom.Vec }

func (Raise) Command() {}
func (Lower) Command() {}
func (Level) Command() {}

var (
	_ command.Command = Raise{}
	_ command.Command = Lower{}
	_ command.Command = Level{}
)

// Shaping is how Raise, Lower and Level move the ground: Step per Raise or Lower, and at most
// MaxStep between two corners along a cell's edge (two neighbouring cells on a hex grid), the
// ground round about following as in Transport Tycoon. MaxStep zero lets any slope stand.
type Shaping struct{ Step, MaxStep float64 }

type shaper struct {
	relief *irelief.Relief
	cfg    Shaping
	raise  command.Queue[Raise]
	lower  command.Queue[Lower]
	level  command.Queue[Level]
}

func (s *shaper) queues() []command.CommandQueue {
	return []command.CommandQueue{&s.raise, &s.lower, &s.level}
}

func (s *shaper) system() goke.System { return shapingSystem{s} }

var _ goke.System = shapingSystem{}

type shapingSystem struct{ s *shaper }

func (shapingSystem) Init(*goke.SysInit) {}

func (y shapingSystem) Update(*goke.CmdBuf, time.Duration) {
	s, r := y.s, y.s.relief
	s.raise.Drain(func(i command.Issued[Raise]) {
		r.KeepRoads(func() { r.Lift(i.Command.At, s.cfg.Step, s.cfg.MaxStep) })
	})
	s.lower.Drain(func(i command.Issued[Lower]) {
		r.KeepRoads(func() { r.Lift(i.Command.At, -s.cfg.Step, s.cfg.MaxStep) })
	})
	s.level.Drain(func(i command.Issued[Level]) {
		r.KeepRoads(func() { r.Flatten(i.Command.From, i.Command.To, s.cfg.MaxStep) })
	})
}
