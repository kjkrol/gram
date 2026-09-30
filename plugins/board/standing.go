package board

import (
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugin/host"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

// Each is a behavior run every tick on every entity on the board carrying T, told where it stands;
// naturally Each[Mover]. Register it with Plugin.RegisterBehavior.
func Each[T any](react func(t plugin.Tick, state *T, s Standing)) plugin.Behavior {
	return host.Each(react)
}

// Every is Each without a state component: every entity on the board, every tick.
func Every(react func(t plugin.Tick, s Standing)) plugin.Behavior { return host.Every(react) }

// Standing is where an entity on the board stands this tick: the cell under its centre, that
// cell's kind, and its box (Grid.CellsUnder lists every cell it touches). Board hosts Each
// behaviors of it; one over Mover knows the entity's domain.
type Standing struct {
	ID   uid.UID64
	Cell CellID
	Kind CellKind
	Box  geom.AABB
}

// Fell reports whether an entity moving in d stands where it may not: in a hole, in water on foot.
func (s Standing) Fell(d Domain) bool { return !s.Kind.Admits(d) }

var _ goke.System = (*standingSystem)(nil)

// standingSystem tells every Each behavior where each entity carrying Cell stands, after
// movement and collisions have had their say.
type standingSystem struct {
	brd  *Board
	host *host.EachHost[Standing]

	query *goke.Query
	base  goke.Comp[world.Base]
	cell  goke.Comp[Cell]

	ids   []uid.UID64
	bases []world.Base
	cells []Cell
}

func newStandingSystem(brd *Board, host *host.EachHost[Standing]) *standingSystem {
	return &standingSystem{brd: brd, host: host}
}

func (s *standingSystem) Init(si *goke.SysInit) {
	qb := si.NewQueryBuilder(&s.base, &s.cell)
	s.host.Bind(qb)
	s.query = qb.Build()
}

func (s *standingSystem) Update(cb *goke.CmdBuf, d time.Duration) {
	if s.host.Empty() {
		return
	}
	tick := plugin.Tick{CmdBuf: cb, Now: time.Now(), Dt: d}
	s.query.All()
	for s.query.Next() {
		cursor := s.query.Cursor()
		s.ids, s.bases, s.cells = cursor.IDs, s.base.Slice(cursor), s.cell.Slice(cursor)
		s.host.Run(tick, cursor, s.at)
	}
}

// at describes the i-th entity of the chunk being walked.
func (s *standingSystem) at(i int) Standing {
	c, ok := s.brd.CellAt(Center(s.bases[i].Pos))
	if !ok {
		c = s.cells[i].ID
	}
	return Standing{ID: s.ids[i], Cell: c, Kind: s.brd.Kind(c), Box: s.bases[i].Pos.AABB.AABB}
}
