package world

import (
	"fmt"
	"log"
	"reflect"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/entity/kind/comp"
	ikinds "github.com/kjkrol/gram/plugins/world/internal/kinds"
)

var _ goke.System = (*spawnSystem)(nil)

// spawnSystem carries out the Spawns given since the last step: one Factory a kind, built once
// the kinds are all defined, the rows checked before any entity is made.
type spawnSystem struct {
	w     *module
	kinds []*spawner // by kind.ID
	// groups are this step's accepted rows by kind, in the order the kinds first came
	groups   []spawnGroup
	accepted int
}

// spawner is what spawns one kind: its factory and the writers of its columns.
type spawner struct {
	kind    ikinds.Kind
	factory *goke.Factory
	base    goke.Comp[Base]
	label   goke.Comp[entity.Label]
	writers []comp.Spawner
}

type spawnGroup struct {
	by      *spawner
	rows    []any
	entries []kind.Entry
}

func newSpawnSystem(w *module) *spawnSystem { return &spawnSystem{w: w} }

func (s *spawnSystem) Init(si *goke.SysInit) {
	s.w.kinds.r.Each(func(k ikinds.Kind) {
		sp := &spawner{kind: k, writers: writersOf(k)}
		comps := []goke.Addable{&sp.base, &sp.label}
		for _, wr := range sp.writers {
			comps = append(comps, wr.Columns()...)
		}
		sp.factory = si.NewFactory(comps...)
		s.kinds = append(s.kinds, sp)
	})
}

func (s *spawnSystem) Update(_ *goke.CmdBuf, _ time.Duration) {
	if s.w.spawns.Empty() {
		return
	}
	for i := range s.groups {
		s.groups[i].rows, s.groups[i].entries = s.groups[i].rows[:0], s.groups[i].entries[:0]
	}
	s.groups, s.accepted = s.groups[:0], 0
	s.w.spawns.Drain(func(i control.Issued[Spawn]) {
		if err := s.take(i.Command.Entry); err != nil {
			log.Printf("[world] spawn refused: %v", err)
		}
	})
	for _, g := range s.groups {
		s.w.spawnRows(g.by.factory, &g.by.base, &g.by.label, g.by.writers, g.by.kind, g.rows, g.entries)
		s.w.spawnedCount += len(g.rows)
		s.w.telemetry.Count += len(g.rows)
	}
}

// take checks one entry and puts its row with its kind's; an error says why it is refused.
func (s *spawnSystem) take(e kind.Entry) error {
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
	pos := k.Position.Resolve(row)
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
		if s.groups[i].by == by {
			s.groups[i].rows, s.groups[i].entries = append(s.groups[i].rows, row), append(s.groups[i].entries, e)
			return nil
		}
	}
	s.groups = append(s.groups, spawnGroup{by: by, rows: []any{row}, entries: []kind.Entry{e}})
	return nil
}

// writersOf are the writers of a kind's columns: its Appearance first, then each of its
// components (a comp.Without writes nothing).
func writersOf(k ikinds.Kind) []comp.Spawner {
	writers := []comp.Spawner{comp.Const(Appearance{SpriteID: k.SpriteID}).Spawner()}
	for _, c := range k.Comps {
		if wr := c.Spawner(); wr != nil {
			writers = append(writers, wr)
		}
	}
	return writers
}

// spawnRows makes one entity of k a row through factory — its Base from the row, its Label and
// the commands its entry tells it, placed in the
// space, the writers' columns from the row — the rows checked beforehand.
func (w *module) spawnRows(factory *goke.Factory, base *goke.Comp[Base], label *goke.Comp[entity.Label], writers []comp.Spawner, k ikinds.Kind, rows []any, entries []kind.Entry) {
	factory.Create(len(rows))
	index := 0
	for factory.Next() {
		bases, named := base.Slice(&factory.Cursor), label.Slice(&factory.Cursor)
		for i, id := range factory.IDs {
			row := rows[index]
			if entries != nil {
				e := entries[index]
				named[i] = entity.LabelOf(e.Name(), e.Group())
				for _, cmd := range e.Commands() {
					w.commands.PutFrom(id, cmd)
				}
			}
			bases[i] = Base{Pos: k.Position.Resolve(row), Vel: k.Velocity.Resolve(row), TypeID: k.TypeID}
			w.space.Place(&bases[i].Pos.AABB)
			for _, wr := range writers {
				wr.Write(&factory.Cursor, i, row, id)
			}
			index++
		}
	}
}

// sizeOf is an error for a size outside the bounds the world declared.
func (w *module) sizeOf(pos Position) error {
	min, max := float64(w.config.Entities.MinSize), float64(w.config.Entities.MaxSize)
	if pos.Size.X < min || pos.Size.X > max || pos.Size.Y < min || pos.Size.Y > max {
		return fmt.Errorf("size %vx%v outside declared bounds [%d, %d]", pos.Size.X, pos.Size.Y, w.config.Entities.MinSize, w.config.Entities.MaxSize)
	}
	return nil
}

// handled is an error for a command e tells its entity that no plugin in use carries out.
func (w *module) handled(e kind.Entry) error {
	for _, cmd := range e.Commands() {
		if t := reflect.TypeOf(control.Unwrap(cmd)); !w.commands.Takes(t) {
			return fmt.Errorf("the entry tells its entity a %v, which no plugin in use carries out", t)
		}
	}
	return nil
}
