package board

import (
	"github.com/kjkrol/gram/plugins/board/cell"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugin/host"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/entity/tag"
	"github.com/kjkrol/uid"
)

// Standing is where an entity on the board stands this tick: the cell under its centre, that
// cell's kind and the game's tags of its place (a plate under it), its box (Grid.CellsUnder lists
// every cell it touches) and the domains it moves in (its Mover's; Land without one). Board hosts
// rules of it.
type Standing struct {
	ID     uid.UID64
	Cell   cell.ID
	Kind   cell.Kind
	Places tag.Tags[cell.Family] // the game's tags of the cell's place: a plate, a zone
	Box    geom.AABB
	Domain cell.Domain
}

// Who is the entity standing: whose moment it is, for a rule.
func (s Standing) Who() uid.UID64 { return s.ID }

// Placed: a rule's Here acts on the cells under the entity's box, its Around on the rings round
// them too.
func (Standing) Placed() {}

// Fallen reports whether the entity stands where its domain may not be: in a hole, in water on
// foot.
func (s Standing) Fallen() bool { return !s.Kind.Admits(s.Domain) }

var _ goke.System = (*standingSystem)(nil)

// standingSystem tells every rule where each entity carrying At stands, after movement and
// collisions have had their say.
type standingSystem struct {
	brd       *Board
	host      *host.EachHost[Standing]
	tick      plugin.TickSource // the world's
	occupancy Occupancy         // the board's, whose holds of the gone are let go every step
	alive     *goke.Query       // who is still in the world
	aliveBase goke.Comp[world.Base]
	gone      func(uid.UID64) bool // isGone, bound once
	around    func(moment any, rings int, each func(uid.UID64))

	query *goke.Query
	base  goke.Comp[world.Base]
	cell  goke.Comp[At]
	mover goke.OptComp[Mover]

	ids    []uid.UID64
	bases  []world.Base
	cells  []At
	movers []Mover
}

func newStandingSystem(brd *Board, each *host.EachHost[Standing], occupancy Occupancy) *standingSystem {
	s := &standingSystem{brd: brd, host: each, occupancy: occupancy, around: brd.placesAround}
	s.gone = s.isGone
	host.Own(each, &s.mover)
	return s
}

func (s *standingSystem) Init(si *goke.SysInit) {
	qb := si.NewQueryBuilder(&s.base, &s.cell)
	s.host.Bind(qb)
	s.query = qb.Build()
	s.alive = si.NewQueryBuilder(&s.aliveBase).Build()
}

// isGone reports whether id is no longer in the world.
func (s *standingSystem) isGone(id uid.UID64) bool { return !s.alive.Seek(id) }

func (s *standingSystem) Update(cb *goke.CmdBuf, d time.Duration) {
	if s.occupancy != nil {
		s.occupancy.Release(s.gone)
	}
	if s.host.Empty() {
		return
	}
	tick := s.tick.Of(cb, d)
	tick.Around = s.around
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
		c = s.cells[i].Cell
	}
	return Standing{ID: s.ids[i], Cell: c, Kind: s.brd.Kind(c), Places: s.brd.placesOf(c), Box: s.bases[i].Pos.AABB.AABB, Domain: DomainAt(s.movers, i)}
}
