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
	w      *module
	kinds  []*spawner // by kind.ID
	groups []spawnGroup
	// the handles of the wholes given this step, spawned before the groups, and the entity of each
	// by its whole's slot
	handles  []spawnGroup
	made     []uid.UID64
	pieces   []*spawner
	accepted int
	// the aggregate giving a SpawnPart, and the stretch of it the part lies in
	givers    *goke.Query
	giver     goke.Comp[Base]
	whole     goke.OptComp[Aggregate]
	field     goke.OptComp[Field]
	partOfID  goke.CompID
	pieceOfID goke.CompID
	stretches *goke.Query
	stretch   goke.Comp[stretch]
}

type spawner struct {
	kind    ikinds.Kind
	factory *goke.Factory
	base    goke.Comp[Base]
	label   goke.Comp[ceremony.Label]
	spawner *cgoke.Spawner
	// the kind's Lie, where it gives one
	lie  comp.Template[Lie]
	lies bool
	// whether the kind carries Whole
	handle bool
	// a seed's pieces' PieceOf, written as they spawn; nil, given through a command buffer
	pieceOf *goke.Comp[PieceOf]
}

type spawnGroup struct {
	by      *spawner
	rows    []any
	entries []kind.Entry
	places  []placing
}

// where an entity spawned goes and whose it is: a part laid at, off the centre of its aggregate
// whole; a Whole's handle, or a piece of it under the handle whole once that has spawned, both by
// their whole's slot among the step's; the entity spawned, once it is
type placing struct {
	whole  uid.UID64
	at     geom.Vec
	part   bool
	handle bool
	piece  bool
	slot   int
	id     uid.UID64
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
	s.w.kinds.r.Each(func(k ikinds.Kind) { s.kinds = append(s.kinds, newSpawner(si, k, false)) })
	s.givers = si.NewQueryBuilder(&s.giver).Optional(&s.whole).Optional(&s.field).Build()
	s.partOfID = si.RegComp[PartOf]()
	s.pieceOfID = si.RegComp[PieceOf]()
	s.stretches = si.NewQueryBuilder(&s.stretch).Build()
}

// newSpawner is the spawner of k's entities through a factory of its own, one of a seed's pieces
// writing their PieceOf as they spawn
func newSpawner(si *goke.SysInit, k ikinds.Kind, pieces bool) *spawner {
	sp := &spawner{kind: k, spawner: spawnerOf(k), handle: carries[Whole](k)}
	for _, c := range k.Comps {
		if lie, ok := c.(comp.Template[Lie]); ok {
			sp.lie, sp.lies = lie, true
		}
	}
	comps := append([]goke.Addable{&sp.base, &sp.label}, sp.spawner.Columns()...)
	if pieces {
		sp.pieceOf = new(goke.Comp[PieceOf])
		comps = append(comps, sp.pieceOf)
	}
	sp.factory = si.NewFactory(comps...)
	return sp
}

func (s *spawnSystem) Update(cb *goke.CmdBuf, _ time.Duration) {
	previews := len(s.w.wholes.previews) > 0
	if s.w.spawns.Empty() && s.w.spawnParts.Empty() && s.w.spawnWholes.Empty() && !previews {
		return
	}
	s.groups, s.handles, s.made, s.accepted = s.groups[:0], s.handles[:0], s.made[:0], 0
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
	s.w.spawnWholes.Drain(func(i command.Issued[SpawnWhole]) {
		if err := s.takeWhole(i.Command); err != nil {
			log.Printf("[world] whole refused: %v", err)
		}
	})
	s.w.spawnHandled(s.handles, s.groups, s.made, cb, s.partOfID, s.pieceOfID)
	s.w.spawnedCount += s.accepted
	s.w.telemetry.Count += s.accepted
	if previews {
		s.w.wholes.foldPreviews()
	}
}

func (s *spawnSystem) part(i command.Issued[SpawnPart]) error {
	cmd := i.Command
	if !i.ByEntity || !s.givers.Seek(i.Entity) || !s.whole.Present(s.givers.Cursor()) {
		return fmt.Errorf("kind %q: a part given by no aggregate", cmd.Entry.Kind())
	}
	if cmd.aimed && isPreview(cmd.in) {
		return s.shoot(i.Entity, cmd)
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
	by, _, err := s.checked(e, place)
	if err != nil {
		return err
	}
	if err := s.room(by.kind.Name, 1); err != nil {
		return err
	}
	s.groups = s.group(s.groups, by, e, place)
	return nil
}

// room is why the world has no room for n more of the kind name this step, nil where it has
func (s *spawnSystem) room(name string, n int) error {
	if s.w.spawnedCount+s.accepted+n > s.w.config.Entities.MaxCount {
		return fmt.Errorf("kind %q: the world is full, Config.Entities.MaxCount %d", name,
			s.w.config.Entities.MaxCount)
	}
	return nil
}

func (s *spawnSystem) group(groups []spawnGroup, by *spawner, e kind.Entry,
	place placing) []spawnGroup {
	s.accepted++
	return grouped(groups, by, e, place)
}

// grouped is groups with e, laid as place says, in by's group
func grouped(groups []spawnGroup, by *spawner, e kind.Entry, place placing) []spawnGroup {
	row := e.Row()
	for i := range groups {
		if g := &groups[i]; g.by == by {
			g.rows, g.entries = append(g.rows, row), append(g.entries, e)
			g.places = append(g.places, place)
			return groups
		}
	}
	// a slot of a past step keeps its arrays
	groups = slices.Grow(groups, 1)[:len(groups)+1]
	g := &groups[len(groups)-1]
	g.by, g.rows, g.entries = by, append(g.rows[:0], row), append(g.entries[:0], e)
	g.places = append(g.places[:0], place)
	return groups
}

// checked is the spawner of e's kind and the box e's entity lies over laid as place says, put
// into the world, or why the world refuses e but for being full
func (s *spawnSystem) checked(e kind.Entry, place placing) (*spawner, plane.AABB, error) {
	k, ok := s.w.kinds.r.Kind(e.Kind())
	if !ok {
		return nil, plane.AABB{}, fmt.Errorf("unknown kind %q", e.Kind())
	}
	if int(k.TypeID) >= len(s.kinds) {
		return nil, plane.AABB{}, fmt.Errorf("kind %q was defined after the world was set up",
			k.Name)
	}
	if got := reflect.TypeOf(e.Row()); got != k.Row {
		return nil, plane.AABB{}, fmt.Errorf("kind %q: the entry carries a %v, its rows are %v",
			k.Name, got, k.Row)
	}
	if err := s.w.handled(e); err != nil {
		return nil, plane.AABB{}, fmt.Errorf("kind %q: %w", k.Name, err)
	}
	pos := place.laid(k.Position.Resolve(e.Row()))
	err := s.w.sizeOf(pos)
	if place.handle {
		err = s.w.handleSizeOf(pos)
	}
	if err != nil {
		return nil, plane.AABB{}, fmt.Errorf("kind %q: %w", k.Name, err)
	}
	box := pos.AABB
	if !s.w.space.Place(&box) {
		return nil, plane.AABB{}, fmt.Errorf("kind %q: %v lies wholly past an open edge", k.Name,
			pos.AABB.AABB)
	}
	return s.kinds[k.TypeID], box, nil
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

// the rows are checked beforehand; entries and places, each row's own, may be nil; each place is
// told its entity, a part given PartOf and a piece PieceOf through cb, unless by writes a piece's
func (w *module) spawnRows(by *spawner, rows []any, entries []kind.Entry, places []placing,
	cb *goke.CmdBuf, partOf, pieceOf goke.CompID) {
	k := by.kind
	by.factory.Create(len(rows))
	index := 0
	for by.factory.Next() {
		cur := &by.factory.Cursor
		bases, named := by.base.Slice(cur), by.label.Slice(cur)
		var pieces []PieceOf
		if by.pieceOf != nil {
			pieces = by.pieceOf.Slice(cur)
		}
		for i, id := range by.factory.IDs {
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
				place := &places[index]
				place.id = id
				pos = place.laid(pos)
				switch {
				case place.part:
					cb.AddOne(id, partOf, PartOf{Whole: place.whole})
				case place.piece && pieces != nil:
					pieces[i] = PieceOf{Whole: place.whole}
				case place.piece:
					cb.AddOne(id, pieceOf, PieceOf{Whole: place.whole})
				}
			}
			bases[i] = Base{Pos: pos, Vel: k.Velocity.Resolve(row), TypeID: k.TypeID}
			w.space.Place(&bases[i].Pos.AABB)
			by.spawner.Write(cur, i, row)
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

// a handle's box joins its pieces', so the least size alone bounds it
func (w *module) handleSizeOf(pos Position) error {
	if least := float32(w.config.Entities.MinSize); pos.Size.X < least || pos.Size.Y < least {
		return fmt.Errorf("a handle's size %vx%v under the declared least %d", pos.Size.X,
			pos.Size.Y, w.config.Entities.MinSize)
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
