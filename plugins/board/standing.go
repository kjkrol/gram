package board

import (
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugin/host"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

// Standing is where an entity on the board stands this tick: the cell under its centre, that
// cell's kind, its box (Grid.CellsUnder lists every cell it touches) and the domains it moves in
// (its Mover's; Land without one). Board hosts triggers of it.
type Standing struct {
	ID     uid.UID64
	Cell   CellID
	Kind   CellKind
	Box    geom.AABB
	Domain Domain
}

// Who is the entity standing: whose moment it is, for a trigger.
func (s Standing) Who() uid.UID64 { return s.ID }

// Fallen reports whether the entity stands where its domain may not be: in a hole, in water on
// foot.
func (s Standing) Fallen() bool { return !s.Kind.Admits(s.Domain) }

var _ goke.System = (*standingSystem)(nil)

// standingSystem tells every trigger where each entity carrying Cell stands, after movement and
// collisions have had their say.
type standingSystem struct {
	brd      *Board
	host     *host.EachHost[Standing]
	commands *control.Carrier

	query *goke.Query
	base  goke.Comp[world.Base]
	cell  goke.Comp[Cell]
	mover goke.OptComp[Mover]

	ids    []uid.UID64
	bases  []world.Base
	cells  []Cell
	movers []Mover
}

func newStandingSystem(brd *Board, each *host.EachHost[Standing]) *standingSystem {
	s := &standingSystem{brd: brd, host: each}
	host.Own(each, &s.mover)
	return s
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
	tick := plugin.Tick{CmdBuf: cb, Now: time.Now(), Dt: d, Commands: s.commands}
	s.query.All()
	for s.query.Next() {
		cursor := s.query.Cursor()
		s.ids, s.bases, s.cells, s.movers = cursor.IDs, s.base.Slice(cursor), s.cell.Slice(cursor), s.mover.Slice(cursor)
		s.host.Run(tick, cursor, s.at)
	}
}

// at describes the i-th entity of the chunk being walked.
func (s *standingSystem) at(i int) Standing {
	c, ok := s.brd.CellAt(Center(s.bases[i].Pos))
	if !ok {
		c = s.cells[i].ID
	}
	return Standing{ID: s.ids[i], Cell: c, Kind: s.brd.Kind(c), Box: s.bases[i].Pos.AABB.AABB, Domain: DomainAt(s.movers, i)}
}
