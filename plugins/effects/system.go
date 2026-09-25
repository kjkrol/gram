package effects

import (
	"github.com/kjkrol/gram/plugin/host"
	"reflect"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

var _ goke.System = (*effectSystem)(nil)

// effectSystem begins, counts down and ends every slot of every Active, applying and undoing
// what the effects grant and alter; an entity whose last effect ended is Idle for a tick.
type effectSystem struct {
	worldPlugin *world.Plugin
	defs        *[]def
	originals   *originals
	idlers      *host.EachHost[Idling]

	query   *goke.Query
	active  goke.Comp[Active]
	columns map[reflect.Type]column

	// idle walks the entities marked Idle last tick for the hosted behaviors, and bare the ones
	// without a Base; the mark goes from both.
	idle      *goke.Query
	bare      *goke.Query
	idleBase  goke.Comp[world.Base]
	idleID    goke.CompID
	idleIDs   []uid.UID64
	idleBases []world.Base

	// lookup finds one entity's Active for Cast and Dispel outside the walk.
	lookup       *goke.Query
	lookupActive goke.Comp[Active]
	activeID     goke.CompID
	built        bool
}

func newEffectSystem(worldPlugin *world.Plugin, defs *[]def, originals *originals, idlers *host.EachHost[Idling]) *effectSystem {
	return &effectSystem{worldPlugin: worldPlugin, defs: defs, originals: originals, idlers: idlers, columns: map[reflect.Type]column{}}
}

func (s *effectSystem) Init(si *goke.SysInit) {
	qb := si.NewQueryBuilder(&s.active)
	for _, d := range *s.defs {
		for _, g := range d.grants {
			if _, ok := s.columns[g.family]; !ok {
				s.columns[g.family] = g.column()
				s.columns[g.family].bind(qb)
			}
		}
		for _, a := range d.alters {
			if _, ok := s.columns[a.comp]; !ok {
				s.columns[a.comp] = a.column()
				s.columns[a.comp].bind(qb)
			}
		}
	}
	s.query = qb.Build()
	s.lookup = si.NewQueryBuilder(&s.lookupActive).Build()
	s.activeID = si.RegComp[Active]()
	s.idleID = si.RegComp[Idle]()
	iq := si.NewQueryBuilder(&s.idleBase).Include(goke.Include[Idle]())
	s.idlers.Bind(iq)
	s.idle = iq.Build()
	s.bare = si.NewQueryBuilder().Include(goke.Include[Idle]()).Exclude(goke.Exclude[world.Base]()).Build()
	s.built = true
}

func (s *effectSystem) Update(cb *goke.CmdBuf, d time.Duration) {
	t := plugin.Tick{CmdBuf: cb, Now: time.Now(), Dt: d}
	s.idle.All()
	for s.idle.Next() {
		cursor := s.idle.Cursor()
		s.idleIDs, s.idleBases = cursor.IDs, s.idleBase.Slice(cursor)
		if !s.idlers.Empty() {
			s.idlers.Run(t, cursor, s.idling)
		}
		for _, id := range s.idleIDs {
			cb.RemoveCompOne(id, s.idleID)
		}
	}
	for s.bare.All(); s.bare.Next(); {
		for _, id := range s.bare.Cursor().IDs {
			cb.RemoveCompOne(id, s.idleID)
		}
	}
	s.query.All()
	for s.query.Next() {
		cursor := s.query.Cursor()
		actives := s.active.Slice(cursor)
		for i, id := range cursor.IDs {
			s.step(t, cursor, i, id, &actives[i], d)
		}
	}
}

// step advances one entity: begins pending slots, counts running ones down, ends the spent.
func (s *effectSystem) step(t plugin.Tick, cursor *goke.Cursor, i int, id uid.UID64, a *Active, d time.Duration) {
	touched := map[reflect.Type]bool{}
	for k := range a.Slots {
		slot := &a.Slots[k]
		switch slot.State {
		case Pending:
			if s.begin(t, cursor, i, id, slot, touched) {
				slot.State = Running
			}
		case Running:
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
	a.Altered = len(touched) > 0
	if a.empty() {
		s.originals.forget(id)
		s.worldPlugin.Detach[Active](t.CmdBuf, id)
		t.CmdBuf.AddOne(id, s.idleID, Idle{})
	}
}

// idling describes the i-th entity of the Idle chunk being walked.
func (s *effectSystem) idling(i int) Idling { return Idling{ID: s.idleIDs[i], Base: &s.idleBases[i]} }

// begin applies a slot's grants and marks its alters for recompute; false while a granted
// family is not on the entity yet — it is attached and the slot waits a tick.
func (s *effectSystem) begin(t plugin.Tick, cursor *goke.Cursor, i int, id uid.UID64, slot *Slot, touched map[reflect.Type]bool) bool {
	d := &(*s.defs)[slot.Kind]
	for _, g := range d.grants {
		col := s.columns[g.family]
		if !col.present(cursor) {
			g.attach(t.CmdBuf, s.worldPlugin, id)
			return false
		}
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

// end frees slot k: bits nobody else grants are cleared, its alters marked for recompute.
func (s *effectSystem) end(cursor *goke.Cursor, i int, id uid.UID64, a *Active, k int, touched map[reflect.Type]bool) {
	d := &(*s.defs)[a.Slots[k].Kind]
	a.Slots[k] = Slot{}
	for _, g := range d.grants {
		bits := g.bits
		for _, other := range a.Slots {
			if other.State != Running {
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
		if slot.State != Running {
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

// cast puts effect on id for left, refreshing a slot it already holds unless it stacks; an
// entity without Active gets one attached, its slot pending.
func (s *effectSystem) cast(cb *goke.CmdBuf, id uid.UID64, effect ID, left time.Duration) {
	d := &(*s.defs)[effect]
	if s.lookup.Seek(id) {
		a := s.lookupActive.At(s.lookup.Cursor())
		if k := a.slot(effect); k >= 0 && !d.stacking {
			a.Slots[k].Left = left
			return
		}
		if k := a.free(); k >= 0 {
			a.Slots[k] = Slot{Kind: effect, Left: left, State: Pending}
		}
		return
	}
	var a Active
	a.Slots[0] = Slot{Kind: effect, Left: left, State: Pending}
	cb.AddOne(id, s.activeID, a)
}

// dispel ends effect on id at the next tick.
func (s *effectSystem) dispel(id uid.UID64, effect ID) {
	if !s.lookup.Seek(id) {
		return
	}
	a := s.lookupActive.At(s.lookup.Cursor())
	for k := range a.Slots {
		if a.Slots[k].State != Empty && a.Slots[k].Kind == effect {
			a.Slots[k].Left = 0
			if a.Slots[k].State == Pending {
				a.Slots[k] = Slot{}
			}
		}
	}
}

// has reports whether id is under effect.
func (s *effectSystem) has(id uid.UID64, effect ID) bool {
	return s.lookup.Seek(id) && s.lookupActive.At(s.lookup.Cursor()).Has(effect)
}

// tagWriter is what a grant's column can do.
type tagWriter interface {
	or(cursor *goke.Cursor, i int, bits uint64)
	clear(cursor *goke.Cursor, i int, bits uint64)
}

// valueAccess is what an alter's column can do.
type valueAccess interface {
	read(cursor *goke.Cursor, i int) any
	write(cursor *goke.Cursor, i int, v any)
	at(cursor *goke.Cursor, i int) any
}

// attach gives id the family with the granted bits, so the next tick can begin the slot.
func (g grant) attach(cb *goke.CmdBuf, w *world.Plugin, id uid.UID64) { g.attachFn(cb, w, id) }
