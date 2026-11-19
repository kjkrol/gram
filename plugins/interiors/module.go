package interiors

import (
	"time"

	"github.com/kjkrol/aabbworld/geom"
	cgoke "github.com/kjkrol/ceremony/goke"
	"github.com/kjkrol/ceremony/host"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/clock"
	"github.com/kjkrol/gram/lod"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/light"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/uid"
)

var _ goke.Module = (*module)(nil)

type module struct {
	p      *Plugin
	inside goke.Runnable
	step   clock.Block
	built  bool
}

func (m *module) RegSystems(ecs *goke.ECS) {
	if m.built {
		return
	}
	m.inside = ecs.RegSys(newInsideSystem(m.p))
	m.step = m.simulate
	m.built = true
}

func (m *module) RunPlan(ctx goke.RunCtx, d time.Duration) {
	clock.Simulate(m.p.worldPlugin.Write().Clock(), ctx, d, m.step)
}

func (m *module) simulate(ctx goke.RunCtx, step time.Duration) { ctx.Run(m.inside, step) }

func (m *module) SetupSystems() []goke.System { return nil }

func (m *module) LoadComps() []goke.CompToken {
	return []goke.CompToken{goke.LoadComp[Room](), goke.LoadComp[Inside]()}
}

var _ goke.System = (*insideSystem)(nil)

// insideSystem finds the room each entity on the board stands in: the room of the floor its feet
// are on (board.BoardRead.LevelFor), as its whole's pieces say
type insideSystem struct {
	p      *Plugin
	tickOf host.TickSource
	cols   *cgoke.Columns

	units     *goke.Query
	base      goke.Comp[world.Base]
	z         goke.Comp[world.Z]
	at        goke.Comp[unit.At]
	inside    goke.OptComp[Inside]
	look      goke.OptComp[render.Appearance]
	sheltered goke.OptComp[light.Sheltered]

	wholes  *goke.Query
	shelter goke.OptComp[light.Shelter]
	pieces  *goke.Query
	piece   goke.Comp[world.PieceOf]
	on      goke.Comp[cell.On]
	floor   goke.Comp[cell.Floor]

	insideID, shelterID goke.CompID
	// a chunk's: where each was and is now, and whether in a room at all, an entity's identity 0
	// being one as any
	was, now     []uid.UID64
	wasIn, nowIn []bool
	ids          []uid.UID64
	enteredAt    func(i int) bool
	entered      func(i int) Entered
	leftAt       func(i int) bool
	left         func(i int) Left
}

func newInsideSystem(p *Plugin) *insideSystem {
	s := &insideSystem{p: p, tickOf: p.worldPlugin.Read(0).Tick}
	s.enteredAt = func(i int) bool { return s.nowIn[i] && (!s.wasIn[i] || s.now[i] != s.was[i]) }
	s.entered = func(i int) Entered { return Entered{ID: s.ids[i], Room: s.now[i]} }
	s.leftAt = func(i int) bool { return s.wasIn[i] && (!s.nowIn[i] || s.was[i] != s.now[i]) }
	s.left = func(i int) Left { return Left{ID: s.ids[i], Room: s.was[i]} }
	return s
}

func (s *insideSystem) Init(si *goke.SysInit) {
	qb := si.NewQueryBuilder(&s.base, &s.z, &s.at).Optional(&s.inside).Optional(&s.look).
		Optional(&s.sheltered)
	s.cols = cgoke.NewColumns(qb)
	s.p.entered.Bind(s.cols)
	s.p.left.Bind(s.cols)
	s.units = qb.With(s.p.worldPlugin.Pace(lod.Contact)).Build()
	s.wholes = si.NewQueryBuilder().With(goke.Include[Room]()).Optional(&s.shelter).Build()
	s.pieces = si.NewQueryBuilder(&s.piece, &s.on, &s.floor).Build()
	s.insideID = si.RegComp[Inside]()
	s.shelterID = si.RegComp[light.Sheltered]()
}

func (s *insideSystem) Update(cb *goke.CmdBuf, d time.Duration) {
	p := s.p
	brd := p.board.Read(0).Board()
	clear(p.bared)
	if !brd.Rooms() && len(p.rooms) == 0 {
		return
	}
	if v := brd.Passage() + 1; p.roomsAt != v {
		p.roomsAt = v
		s.find()
	}
	clear(p.lamps)
	for s.wholes.All(); s.wholes.Next(); {
		cur := s.wholes.Cursor()
		if shelters := s.shelter.Slice(cur); shelters != nil {
			for i, id := range cur.IDs {
				p.lamps[id] = shelters[i].Lamp
			}
		}
	}
	hosted := !p.entered.Empty() || !p.left.Empty()
	read := p.worldPlugin.Read(0)
	for s.units.Paced(d); s.units.Next(); {
		cur := s.units.Cursor()
		bases, zs := s.base.Slice(cur), s.z.Slice(cur)
		insides, looks := s.inside.Slice(cur), s.look.Slice(cur)
		s.ids, s.was, s.now = cur.IDs, s.was[:0], s.now[:0]
		s.wasIn, s.nowIn = s.wasIn[:0], s.nowIn[:0]
		wasIn := insides != nil
		for i, id := range cur.IDs {
			var was uid.UID64
			if wasIn {
				was = insides[i].Room
			}
			room, in := s.roomAt(bases[i].Pos.Center(), &zs[i])
			s.was, s.now = append(s.was, was), append(s.now, room)
			s.wasIn, s.nowIn = append(s.wasIn, wasIn), append(s.nowIn, in)
			switch {
			case in == wasIn && room == was:
			case wasIn && in:
				insides[i].Room = room
			case wasIn:
				cb.RemoveCompOne(id, s.insideID)
			case in:
				cb.AddOne(id, s.insideID, Inside{Room: room})
			}
			s.shelterOf(cb, cur, i, id, room, in)
			if in && looks != nil && read.Uncovered(looks[i].SpriteID) {
				p.bared[room] = true
			}
		}
		if hosted {
			tick := s.tickOf.Of(s.units.Dt())
			s.p.left.RunWhere(tick, s.cols.Row(cur), len(cur.IDs), s.leftAt, s.left)
			s.p.entered.RunWhere(tick, s.cols.Row(cur), len(cur.IDs), s.enteredAt, s.entered)
		}
	}
}

// find finds the rooms anew, by the cells' storeys their floors lie on
func (s *insideSystem) find() {
	p := s.p
	clear(p.rooms)
	rooms := map[uid.UID64]bool{}
	for s.wholes.All(); s.wholes.Next(); {
		for _, id := range s.wholes.Cursor().IDs {
			rooms[id] = true
		}
	}
	for s.pieces.All(); s.pieces.Next(); {
		cur := s.pieces.Cursor()
		pieces, ons, floors := s.piece.Slice(cur), s.on.Slice(cur), s.floor.Slice(cur)
		for i := range cur.IDs {
			if rooms[pieces[i].Whole] && floors[i].Room() {
				p.rooms[place{ons[i].Cell, floors[i].Storey}] = pieces[i].Whole
			}
		}
	}
}

// roomAt is the room whose floor whoever stands at at with its feet as z says is on; false in
// none
func (s *insideSystem) roomAt(at geom.Vec, z *world.Z) (uid.UID64, bool) {
	c, level, ok := s.p.board.Read(0).Board().LevelFor(at, z.Altitude, z.Reach())
	if !ok || level == cell.OnGround {
		return 0, false
	}
	room, ok := s.p.rooms[place{c, uint8(level - cell.OnDeck)}]
	return room, ok
}

// shelterOf has the entity id, row i of the chunk at cur, Sheltered in room, where it stands in
// one (in) whose whole is a light.Shelter, else in none
func (s *insideSystem) shelterOf(cb *goke.CmdBuf, cur *goke.Cursor, i int, id, room uid.UID64,
	in bool) {
	shelters := s.sheltered.Slice(cur)
	if _, roofed := s.p.lamps[room]; !roofed {
		in = false
	}
	switch {
	case !in:
		if shelters != nil {
			cb.RemoveCompOne(id, s.shelterID)
		}
	case shelters != nil:
		shelters[i].In = room
	default:
		cb.AddOne(id, s.shelterID, light.Sheltered{In: room})
	}
}
