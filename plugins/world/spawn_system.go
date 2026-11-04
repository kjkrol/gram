package world

import (
	"fmt"
	"log"
	"reflect"
	"slices"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/ceremony/comp"
	cgoke "github.com/kjkrol/ceremony/goke"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/kind"
	ikinds "github.com/kjkrol/gram/plugins/world/internal/kinds"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/uid"
)

var _ goke.System = (*spawnSystem)(nil)

type spawnSystem struct {
	w        *module
	kinds    []*spawner // by kind.ID
	groups   []spawnGroup
	accepted int
	// the aggregate giving a SpawnPart, and the stretch of it the part lies in
	givers    *goke.Query
	giver     goke.Comp[Base]
	whole     goke.OptComp[Aggregate]
	field     goke.OptComp[Field]
	partOfID  goke.CompID
	stretches *goke.Query
	stretch   goke.Comp[stretch]
}

type spawner struct {
	kind    ikinds.Kind
	factory *goke.Factory
	base    goke.Comp[Base]
	label   goke.Comp[ceremony.Label]
	spawner *cgoke.Spawner
}

type spawnGroup struct {
	by      *spawner
	rows    []any
	entries []kind.Entry
	places  []placing
}

// where an entity spawned goes: a part's box laid from At, the centre of the aggregate Whole
type placing struct {
	whole uid.UID64
	at    geom.Vec
	part  bool
}

func (p placing) laid(pos Position) Position {
	if p.part {
		pos.AABB = plane.NewAABB(geom.NewVec(pos.TopLeft.X+p.at.X, pos.TopLeft.Y+p.at.Y),
			pos.Size.X, pos.Size.Y)
	}
	return pos
}

func newSpawnSystem(w *module) *spawnSystem { return &spawnSystem{w: w} }

func (s *spawnSystem) Init(si *goke.SysInit) {
	s.w.kinds.r.Each(func(k ikinds.Kind) {
		sp := &spawner{kind: k, spawner: spawnerOf(k)}
		comps := append([]goke.Addable{&sp.base, &sp.label}, sp.spawner.Columns()...)
		sp.factory = si.NewFactory(comps...)
		s.kinds = append(s.kinds, sp)
	})
	s.givers = si.NewQueryBuilder(&s.giver).Optional(&s.whole).Optional(&s.field).Build()
	s.partOfID = si.RegComp[PartOf]()
	s.stretches = si.NewQueryBuilder(&s.stretch).Build()
}

func (s *spawnSystem) Update(cb *goke.CmdBuf, _ time.Duration) {
	if s.w.spawns.Empty() && s.w.spawnParts.Empty() {
		return
	}
	s.groups, s.accepted = s.groups[:0], 0
	s.w.spawns.Drain(func(i command.Issued[Spawn]) {
		if err := s.take(i.Command.Entry, placing{}); err != nil {
			log.Printf("[world] spawn refused: %v", err)
		}
	})
	s.w.spawnParts.Drain(func(i command.Issued[SpawnPart]) {
		if err := s.part(i); err != nil {
			log.Printf("[world] part refused: %v", err)
		}
	})
	for _, g := range s.groups {
		s.w.spawnRows(g.by.factory, &g.by.base, &g.by.label, g.by.spawner, g.by.kind, g.rows,
			g.entries, g.places, cb, s.partOfID)
		s.w.spawnedCount += len(g.rows)
		s.w.telemetry.Count += len(g.rows)
	}
}

func (s *spawnSystem) part(i command.Issued[SpawnPart]) error {
	cmd := i.Command
	if !i.ByEntity || !s.givers.Seek(i.Entity) || !s.whole.Present(s.givers.Cursor()) {
		return fmt.Errorf("kind %q: a part given by no aggregate", cmd.Entry.Kind())
	}
	cur := s.givers.Cursor()
	at := s.giver.At(cur).Pos.Center()
	if cmd.aimed && s.field.Present(cur) && s.stretches.Seek(cmd.in) {
		if st := s.stretch.At(s.stretches.Cursor()); st.Whole == i.Entity {
			at = middle(st.Box)
		}
	}
	return s.take(cmd.Entry, placing{whole: i.Entity, at: at.Add(cmd.At), part: true})
}

func (s *spawnSystem) take(e kind.Entry, place placing) error {
	k, ok := s.w.kinds.r.Kind(e.Kind())
	if !ok {
		return fmt.Errorf("unknown kind %q", e.Kind())
	}
	if int(k.TypeID) >= len(s.kinds) {
		return fmt.Errorf("kind %q was defined after the world was set up", k.Name)
	}
	if got := reflect.TypeOf(e.Row()); got != k.Row {
		return fmt.Errorf("kind %q: the entry carries a %v, its rows are %v", k.Name, got, k.Row)
	}
	if err := s.w.handled(e); err != nil {
		return fmt.Errorf("kind %q: %w", k.Name, err)
	}
	row := e.Row()
	pos := place.laid(k.Position.Resolve(row))
	if err := s.w.sizeOf(pos); err != nil {
		return fmt.Errorf("kind %q: %w", k.Name, err)
	}
	if box := pos.AABB; !s.w.space.Place(&box) {
		return fmt.Errorf("kind %q: %v lies wholly past an open edge", k.Name, pos.AABB.AABB)
	}
	if s.w.spawnedCount+s.accepted >= s.w.config.Entities.MaxCount {
		return fmt.Errorf("kind %q: the world is full, Config.Entities.MaxCount %d", k.Name, s.w.config.Entities.MaxCount)
	}
	s.accepted++
	by := s.kinds[k.TypeID]
	for i := range s.groups {
		if g := &s.groups[i]; g.by == by {
			g.rows, g.entries = append(g.rows, row), append(g.entries, e)
			g.places = append(g.places, place)
			return nil
		}
	}
	// a slot of a past step keeps its arrays
	s.groups = slices.Grow(s.groups, 1)[:len(s.groups)+1]
	g := &s.groups[len(s.groups)-1]
	g.by, g.rows, g.entries = by, append(g.rows[:0], row), append(g.entries[:0], e)
	g.places = append(g.places[:0], place)
	return nil
}

func spawnerOf(k ikinds.Kind) *cgoke.Spawner {
	comps := []comp.Comp{comp.Const(render.Appearance{SpriteID: k.SpriteID})}
	for _, c := range k.Comps {
		if _, left := comp.Omitted(c); !left {
			comps = append(comps, c)
		}
	}
	return cgoke.NewSpawner(comps...)
}

// the rows are checked beforehand; entries and places, each row's own, may be nil, and a part
// placed is given PartOf through cb
func (w *module) spawnRows(
	factory *goke.Factory,
	base *goke.Comp[Base],
	label *goke.Comp[ceremony.Label],
	sp *cgoke.Spawner,
	k ikinds.Kind,
	rows []any,
	entries []kind.Entry,
	places []placing,
	cb *goke.CmdBuf,
	partOf goke.CompID,
) {
	factory.Create(len(rows))
	index := 0
	for factory.Next() {
		bases, named := base.Slice(&factory.Cursor), label.Slice(&factory.Cursor)
		for i, id := range factory.IDs {
			row := rows[index]
			if entries != nil {
				e := entries[index]
				named[i] = ceremony.LabelOf(e.Name(), e.Group())
				for _, cmd := range e.Commands() {
					w.commands.PutFrom(id, cmd)
				}
			}
			pos := k.Position.Resolve(row)
			if places != nil {
				place := places[index]
				pos = place.laid(pos)
				if place.part {
					cb.AddOne(id, partOf, PartOf{Whole: place.whole})
				}
			}
			bases[i] = Base{Pos: pos, Vel: k.Velocity.Resolve(row), TypeID: k.TypeID}
			w.space.Place(&bases[i].Pos.AABB)
			sp.Write(&factory.Cursor, i, row)
			index++
		}
	}
}

func (w *module) sizeOf(pos Position) error {
	min, max := float32(w.config.Entities.MinSize), float32(w.config.Entities.MaxSize)
	if pos.Size.X < min || pos.Size.X > max || pos.Size.Y < min || pos.Size.Y > max {
		return fmt.Errorf("size %vx%v outside declared bounds [%d, %d]", pos.Size.X, pos.Size.Y, w.config.Entities.MinSize, w.config.Entities.MaxSize)
	}
	return nil
}

func (w *module) handled(e kind.Entry) error {
	for _, cmd := range e.Commands() {
		if t := reflect.TypeOf(command.Unwrap(cmd)); !w.commands.Takes(t) {
			return fmt.Errorf("the entry tells its entity a %v, which no plugin in use carries out", t)
		}
	}
	return nil
}
