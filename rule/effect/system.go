package effect

import (
	"reflect"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/uid"
)

var _ goke.System = (*effectSystem)(nil)

// effectSystem begins, counts down and ends every slot of every Active, applying and undoing
// what the effects grant and alter, casting a spent effect's Then; an entity whose components an
// Alter changed has its Changed on for the step after. An entity whose last effect ended keeps its
// Active, empty.
type effectSystem struct {
	defs      *[]def
	originals *originals

	query   *goke.Query
	active  goke.Comp[Active]
	states  *tagColumn[States] // the markers: Changed and each effect's own
	columns map[reflect.Type]column
	touched map[reflect.Type]bool // the altered components of the entity in hand, reused
	thens   []effectID            // the Thens of the entity in hand, reused

	// lookup finds one entity's Active for Cast and Dispel outside the walk.
	lookup       *goke.Query
	lookupActive goke.Comp[Active]
	activeID     goke.CompID
	built        bool
	// fresh are the Actives on their way to entities under no effect yet, by entity, until the
	// next pass: every cast on one before its Active lands joins the one being added.
	fresh map[uid.UID64]Active
	// attaching are the families on their way to entities lacking them this pass, by entity and
	// family, with the tags every effect attaching one gave it: the add assigned last carries all.
	attaching map[attachment]uint64
}

// attachment is one family on its way to one entity.
type attachment struct {
	id     uid.UID64
	family reflect.Type
}

func newEffectSystem(defs *[]def, originals *originals) *effectSystem {
	return &effectSystem{defs: defs, originals: originals, columns: map[reflect.Type]column{}, touched: map[reflect.Type]bool{},
		fresh: map[uid.UID64]Active{}, attaching: map[attachment]uint64{}}
}

func (s *effectSystem) Init(si *goke.SysInit) {
	qb := si.NewQueryBuilder(&s.active)
	s.states = &tagColumn[States]{}
	s.states.bind(si, qb)
	s.columns[reflect.TypeFor[States]()] = s.states
	for _, d := range *s.defs {
		for _, g := range d.grants {
			if _, ok := s.columns[g.family]; !ok {
				s.columns[g.family] = g.column()
				s.columns[g.family].bind(si, qb)
			}
		}
		for _, a := range d.alters {
			if _, ok := s.columns[a.comp]; !ok {
				s.columns[a.comp] = a.column()
				s.columns[a.comp].bind(si, qb)
			}
		}
	}
	s.query = qb.Build()
	s.lookup = si.NewQueryBuilder(&s.lookupActive).Build()
	s.activeID = si.RegComp[Active]()
	s.built = true
}

func (s *effectSystem) Update(cb *goke.CmdBuf, d time.Duration) {
	clear(s.fresh)
	clear(s.attaching)
	s.query.All()
	for s.query.Next() {
		cursor := s.query.Cursor()
		actives := s.active.Slice(cursor)
		for i, id := range cursor.IDs {
			a := &actives[i]
			changed := false
			if !a.empty() {
				changed = s.step(cb, cursor, i, id, a, d)
			}
			s.mark(cb, cursor, i, id, changed)
		}
	}
}

// mark has the entity's Changed on when changed, off otherwise — its family given it where it has
// none and Changed is to go on.
func (s *effectSystem) mark(cb *goke.CmdBuf, cursor *goke.Cursor, i int, id uid.UID64, changed bool) {
	if !s.states.present(cursor) {
		if changed {
			s.attach(cb, id, reflect.TypeFor[States](), uint64(tag.Tags[States](0).With(Changed)))
		}
		return
	}
	if changed {
		s.states.or(cursor, i, uint64(tag.Tags[States](0).With(Changed)))
	} else {
		s.states.clear(cursor, i, uint64(tag.Tags[States](0).With(Changed)))
	}
}

// step advances one entity: begins pending slots, counts running ones down, ends the spent, and
// casts the Thens of those that ran out; true when an Alter changed its components.
func (s *effectSystem) step(cb *goke.CmdBuf, cursor *goke.Cursor, i int, id uid.UID64, a *Active, d time.Duration) bool {
	touched := s.touched
	clear(touched)
	s.thens = s.thens[:0]
	for k := range a.Slots {
		slot := &a.Slots[k]
		switch slot.State {
		case slotPending:
			if s.begin(cb, cursor, i, id, slot, touched) {
				slot.State = slotRunning
			}
		case slotRunning:
			if slot.Left != Forever {
				slot.Left -= d
				if slot.Left <= 0 {
					s.end(cursor, i, id, a, k, touched)
				}
			}
		}
	}
	for comp := range touched {
		s.recompute(cursor, i, id, a, comp)
	}
	for _, next := range s.thens {
		s.queue(a, next, lastsOf(&(*s.defs)[next]))
	}
	if a.empty() {
		s.originals.forget(id)
	}
	return len(touched) > 0
}

// begin applies a slot's grants and marks its alters for recompute; false while a granted
// family is not on the entity yet — the missing ones are attached with the grant's tags on, and
// the slot waits a tick.
func (s *effectSystem) begin(cb *goke.CmdBuf, cursor *goke.Cursor, i int, id uid.UID64, slot *effectSlot, touched map[reflect.Type]bool) bool {
	d := &(*s.defs)[slot.Kind]
	ready := true
	for k, g := range d.grants {
		if s.columns[g.family].present(cursor) || attachedBefore(d.grants[:k], g.family) {
			continue
		}
		s.attach(cb, id, g.family, g.bits)
		ready = false
	}
	if !ready {
		return false
	}
	for _, g := range d.grants {
		s.columns[g.family].(tagWriter).or(cursor, i, g.bits)
	}
	for _, a := range d.alters {
		if s.columns[a.comp].present(cursor) {
			touched[a.comp] = true
		}
	}
	return true
}

// attach gives id the family with bits on, joined to whatever else attached it this pass.
func (s *effectSystem) attach(cb *goke.CmdBuf, id uid.UID64, family reflect.Type, bits uint64) {
	key := attachment{id: id, family: family}
	bits |= s.attaching[key]
	s.attaching[key] = bits
	s.columns[family].(tagWriter).attach(cb, id, bits)
}

// attachedBefore reports whether one of grants is of family: attached once, as one component.
func attachedBefore(grants []grant, family reflect.Type) bool {
	for _, g := range grants {
		if g.family == family {
			return true
		}
	}
	return false
}

// end frees slot k: bits nobody else grants are cleared, its alters marked for recompute, and its
// Then noted when it ran out rather than was dispelled.
func (s *effectSystem) end(cursor *goke.Cursor, i int, id uid.UID64, a *Active, k int, touched map[reflect.Type]bool) {
	d := &(*s.defs)[a.Slots[k].Kind]
	if d.follows && !a.Slots[k].Dispelled {
		s.thens = append(s.thens, d.then)
	}
	a.Slots[k] = effectSlot{}
	for _, g := range d.grants {
		bits := g.bits
		for _, other := range a.Slots {
			if other.State != slotRunning {
				continue
			}
			for _, og := range (*s.defs)[other.Kind].grants {
				if og.family == g.family {
					bits &^= og.bits
				}
			}
		}
		if col := s.columns[g.family]; col.present(cursor) {
			col.(tagWriter).clear(cursor, i, bits)
		}
	}
	for _, alt := range d.alters {
		if s.columns[alt.comp].present(cursor) {
			touched[alt.comp] = true
		}
	}
}

// recompute puts comp back to its original and applies every running Alter of it in slot order;
// with none left the original is forgotten.
func (s *effectSystem) recompute(cursor *goke.Cursor, i int, id uid.UID64, a *Active, comp reflect.Type) {
	col := s.columns[comp].(valueAccess)
	if original, ok := s.originals.get(id, comp); ok {
		col.write(cursor, i, original)
	} else {
		s.originals.put(id, comp, col.read(cursor, i))
	}
	applied := false
	for _, slot := range a.Slots {
		if slot.State != slotRunning {
			continue
		}
		for _, alt := range (*s.defs)[slot.Kind].alters {
			if alt.comp == comp {
				alt.fn(col.at(cursor, i))
				applied = true
			}
		}
	}
	if !applied {
		s.originals.drop(id, comp)
	}
}

// cast puts effect on id for left, refreshing a slot it already holds unless it stacks — a slot
// dispelled this step is taken back; an entity without Active gets one attached, its slots
// pending: every cast before it lands joins it, the add assigned last carrying them all.
func (s *effectSystem) cast(cb *goke.CmdBuf, id uid.UID64, effect effectID, left time.Duration) {
	if s.lookup.Seek(id) {
		s.queue(s.lookupActive.At(s.lookup.Cursor()), effect, left)
		return
	}
	a := s.fresh[id]
	s.queue(&a, effect, left)
	s.fresh[id] = a
	cb.AddOne(id, s.activeID, a)
}

// queue puts effect on a for left: a slot it holds refreshed unless it stacks, else a free one
// pending; with none free nothing happens.
func (s *effectSystem) queue(a *Active, effect effectID, left time.Duration) {
	if k := a.slot(effect); k >= 0 && !(*s.defs)[effect].stacking {
		a.Slots[k].Left, a.Slots[k].Dispelled = left, false
		return
	}
	if k := a.free(); k >= 0 {
		a.Slots[k] = effectSlot{Kind: effect, Left: left, State: slotPending}
	}
}

// dispel ends effect on id at the next tick, its Then not cast.
func (s *effectSystem) dispel(id uid.UID64, effect effectID) {
	if !s.lookup.Seek(id) {
		return
	}
	a := s.lookupActive.At(s.lookup.Cursor())
	for k := range a.Slots {
		if a.Slots[k].State != slotEmpty && a.Slots[k].Kind == effect {
			a.Slots[k].Left, a.Slots[k].Dispelled = 0, true
			if a.Slots[k].State == slotPending {
				a.Slots[k] = effectSlot{}
			}
		}
	}
}

// has reports whether id is under effect.
func (s *effectSystem) has(id uid.UID64, effect effectID) bool {
	return s.lookup.Seek(id) && s.lookupActive.At(s.lookup.Cursor()).has(effect)
}

// tagWriter is what a grant's column can do.
type tagWriter interface {
	or(cursor *goke.Cursor, i int, bits uint64)
	clear(cursor *goke.Cursor, i int, bits uint64)
	attach(cb *goke.CmdBuf, id uid.UID64, bits uint64)
}

// valueAccess is what an alter's column can do.
type valueAccess interface {
	read(cursor *goke.Cursor, i int) any
	write(cursor *goke.Cursor, i int, v any)
	at(cursor *goke.Cursor, i int) any
}
