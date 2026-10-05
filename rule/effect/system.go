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

	// narrow walks the entities under an Active, wide those under a Wide: a plugin's own
	narrow, wide walk
	active       goke.Comp[Active]
	wideActive   goke.Comp[Wide]
	touched      map[reflect.Type]bool // the altered components of the entity in hand, reused
	thens        []effectID            // the Thens of the entity in hand, reused

	// lookup finds one entity's Active, lookupWide its Wide, for Cast and Dispel outside the walk.
	lookup       *goke.Query
	lookupActive goke.Comp[Active]
	lookupWide   *goke.Query
	lookupSlots  goke.Comp[Wide]
	activeID     goke.CompID
	built        bool
	// fresh are the Actives on their way to entities under no effect yet, by entity, until the
	// next pass: every cast on one before its Active lands joins the one being added.
	fresh map[uid.UID64]Active
	// attaching are the families on their way to entities lacking them this pass, by entity and
	// family, with the tags every effect attaching one gave it: the add assigned last carries all.
	attaching map[attachment]uint64
}

// walk is one query of the system with the columns bound to it: the markers and whatever the
// effects grant and alter.
type walk struct {
	query   *goke.Query
	states  *tagColumn[States] // the markers: Changed and each effect's own
	columns map[reflect.Type]column
}

// bind builds the walk over the entities carrying slots, with every column the effects name.
func (w *walk) bind(si *goke.SysInit, defs []def, slots goke.Trackable) {
	qb := si.NewQueryBuilder(slots)
	w.states, w.columns = &tagColumn[States]{}, map[reflect.Type]column{}
	w.states.bind(si, qb)
	w.columns[reflect.TypeFor[States]()] = w.states
	for _, d := range defs {
		for _, g := range d.grants {
			if _, ok := w.columns[g.family]; !ok {
				w.columns[g.family] = g.column()
				w.columns[g.family].bind(si, qb)
			}
		}
		for _, a := range d.alters {
			if _, ok := w.columns[a.comp]; !ok {
				w.columns[a.comp] = a.column()
				w.columns[a.comp].bind(si, qb)
			}
		}
	}
	w.query = qb.Build()
}

// attachment is one family on its way to one entity.
type attachment struct {
	id     uid.UID64
	family reflect.Type
}

func newEffectSystem(defs *[]def, originals *originals) *effectSystem {
	return &effectSystem{defs: defs, originals: originals, touched: map[reflect.Type]bool{},
		fresh: map[uid.UID64]Active{}, attaching: map[attachment]uint64{}}
}

func (s *effectSystem) Init(si *goke.SysInit) {
	s.narrow.bind(si, *s.defs, &s.active)
	s.wide.bind(si, *s.defs, &s.wideActive)
	s.lookup = si.NewQueryBuilder(&s.lookupActive).Build()
	s.lookupWide = si.NewQueryBuilder(&s.lookupSlots).Build()
	s.activeID = si.RegComp[Active]()
	s.built = true
}

func (s *effectSystem) Update(cb *goke.CmdBuf, d time.Duration) {
	clear(s.fresh)
	clear(s.attaching)
	for s.narrow.query.All(); s.narrow.query.Next(); {
		cursor := s.narrow.query.Cursor()
		actives := s.active.Slice(cursor)
		for i, id := range cursor.IDs {
			s.advance(&s.narrow, cb, cursor, i, id, actives[i].Slots[:], d)
		}
	}
	for s.wide.query.All(); s.wide.query.Next(); {
		cursor := s.wide.query.Cursor()
		wides := s.wideActive.Slice(cursor)
		for i, id := range cursor.IDs {
			s.advance(&s.wide, cb, cursor, i, id, wides[i].Slots[:], d)
		}
	}
}

// advance steps one entity's slots and sets its Changed.
func (s *effectSystem) advance(w *walk, cb *goke.CmdBuf, cursor *goke.Cursor, i int, id uid.UID64, slots []effectSlot, d time.Duration) {
	changed := false
	if !empty(slots) {
		changed = s.step(w, cb, cursor, i, id, slots, d)
	}
	s.mark(w, cb, cursor, i, id, changed)
}

// mark has the entity's Changed on when changed, off otherwise — its family given it where it has
// none and Changed is to go on.
func (s *effectSystem) mark(w *walk, cb *goke.CmdBuf, cursor *goke.Cursor, i int, id uid.UID64, changed bool) {
	if !w.states.present(cursor) {
		if changed {
			s.attach(w, cb, id, reflect.TypeFor[States](), uint64(tag.Tags[States](0).With(Changed)))
		}
		return
	}
	if changed {
		w.states.or(cursor, i, uint64(tag.Tags[States](0).With(Changed)))
	} else {
		w.states.clear(cursor, i, uint64(tag.Tags[States](0).With(Changed)))
	}
}

// step advances one entity: begins pending slots, counts running ones down, ends the spent, and
// casts the Thens of those that ran out; true when an Alter changed its components or an effect
// that Shows began or ended.
func (s *effectSystem) step(w *walk, cb *goke.CmdBuf, cursor *goke.Cursor, i int, id uid.UID64, slots []effectSlot, d time.Duration) bool {
	touched := s.touched
	clear(touched)
	s.thens = s.thens[:0]
	shown := false
	for k := range slots {
		slot := &slots[k]
		switch slot.State {
		case slotPending:
			if s.begin(w, cb, cursor, i, id, slot, touched) {
				slot.State = slotRunning
				shown = shown || (*s.defs)[slot.Kind].shows
			}
		case slotRunning:
			if slot.Left != Forever {
				slot.Left -= d
				if slot.Left <= 0 {
					shown = shown || (*s.defs)[slot.Kind].shows
					s.end(w, cursor, i, id, slots, k, touched)
				}
			}
		}
	}
	for comp := range touched {
		s.recompute(w, cursor, i, id, slots, comp)
	}
	for _, next := range s.thens {
		s.queue(slots, next, lastsOf(&(*s.defs)[next]))
	}
	if empty(slots) {
		s.originals.forget(id)
	}
	return len(touched) > 0 || shown
}

// begin applies a slot's grants and marks its alters for recompute; false while a granted
// family is not on the entity yet — the missing ones are attached with the grant's tags on, and
// the slot waits a tick.
func (s *effectSystem) begin(w *walk, cb *goke.CmdBuf, cursor *goke.Cursor, i int, id uid.UID64, slot *effectSlot, touched map[reflect.Type]bool) bool {
	d := &(*s.defs)[slot.Kind]
	ready := true
	for k, g := range d.grants {
		if w.columns[g.family].present(cursor) || attachedBefore(d.grants[:k], g.family) {
			continue
		}
		s.attach(w, cb, id, g.family, g.bits)
		ready = false
	}
	if !ready {
		return false
	}
	for _, g := range d.grants {
		w.columns[g.family].(tagWriter).or(cursor, i, g.bits)
	}
	for _, a := range d.alters {
		if w.columns[a.comp].present(cursor) {
			touched[a.comp] = true
		}
	}
	return true
}

// attach gives id the family with bits on, joined to whatever else attached it this pass.
func (s *effectSystem) attach(w *walk, cb *goke.CmdBuf, id uid.UID64, family reflect.Type, bits uint64) {
	key := attachment{id: id, family: family}
	bits |= s.attaching[key]
	s.attaching[key] = bits
	w.columns[family].(tagWriter).attach(cb, id, bits)
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
func (s *effectSystem) end(w *walk, cursor *goke.Cursor, i int, id uid.UID64, slots []effectSlot, k int, touched map[reflect.Type]bool) {
	d := &(*s.defs)[slots[k].Kind]
	if d.follows && !slots[k].Dispelled {
		s.thens = append(s.thens, d.then)
	}
	slots[k] = effectSlot{}
	for _, g := range d.grants {
		bits := g.bits
		for _, other := range slots {
			if other.State != slotRunning {
				continue
			}
			for _, og := range (*s.defs)[other.Kind].grants {
				if og.family == g.family {
					bits &^= og.bits
				}
			}
		}
		if col := w.columns[g.family]; col.present(cursor) {
			col.(tagWriter).clear(cursor, i, bits)
		}
	}
	for _, alt := range d.alters {
		if w.columns[alt.comp].present(cursor) {
			touched[alt.comp] = true
		}
	}
}

// recompute puts comp back to its original and applies every running Alter of it in slot order;
// with none left the original is forgotten.
func (s *effectSystem) recompute(w *walk, cursor *goke.Cursor, i int, id uid.UID64, slots []effectSlot, comp reflect.Type) {
	col := w.columns[comp].(valueAccess)
	if original, ok := s.originals.get(id, comp); ok {
		col.write(cursor, i, original)
	} else {
		s.originals.put(id, comp, col.read(cursor, i))
	}
	applied := false
	for _, slot := range slots {
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
	if slots := s.slotsOf(id); slots != nil {
		s.queue(slots, effect, left)
		return
	}
	a := s.fresh[id]
	s.queue(a.Slots[:], effect, left)
	s.fresh[id] = a
	cb.AddOne(id, s.activeID, a)
}

// queue puts effect on a for left: a slot it holds refreshed unless it stacks, else a free one
// pending; with none free nothing happens.
func (s *effectSystem) queue(slots []effectSlot, effect effectID, left time.Duration) {
	if k := slotOf(slots, effect); k >= 0 && !(*s.defs)[effect].stacking {
		slots[k].Left, slots[k].Dispelled = left, false
		return
	}
	if k := free(slots); k >= 0 {
		slots[k] = effectSlot{Kind: effect, Left: left, State: slotPending}
	}
}

// dispel ends effect on id at the next tick, its Then not cast.
func (s *effectSystem) dispel(id uid.UID64, effect effectID) {
	slots := s.slotsOf(id)
	for k := range slots {
		if slots[k].State != slotEmpty && slots[k].Kind == effect {
			slots[k].Left, slots[k].Dispelled = 0, true
			if slots[k].State == slotPending {
				slots[k] = effectSlot{}
			}
		}
	}
}

// has reports whether id is under effect.
func (s *effectSystem) has(id uid.UID64, effect effectID) bool {
	return has(s.slotsOf(id), effect)
}

// slotsOf are the slots id carries, its Active's or its Wide's; nil for one under nothing yet.
func (s *effectSystem) slotsOf(id uid.UID64) []effectSlot {
	if s.lookup.Seek(id) {
		return s.lookupActive.At(s.lookup.Cursor()).Slots[:]
	}
	if s.lookupWide.Seek(id) {
		return s.lookupSlots.At(s.lookupWide.Cursor()).Slots[:]
	}
	return nil
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
