package world

import (
	"fmt"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/gram/rule/effect"
	"github.com/kjkrol/uid"
)

// wires are the wires a game defined (Plugin.Wire): each its own entity, made at Setup or found in
// a loaded game by its Wiring, and what the Signals given to them put on and take off.
type wires struct {
	defined []*rule.Wire
	signals control.Queue[rule.Signal]
	effects *effect.Effects // the world's, which a Signal casts and dispels

	own    goke.Comp[rule.Wiring]
	spawn  goke.Comp[rule.Wiring]
	wired  *goke.Query // every entity wired to a wire, sought by Of
	wiredC goke.Comp[rule.Wired]
	plays  *goke.Query // every entity playing roles, sought by RolesOf
	playsC goke.Comp[tag.Tags[rule.Roles]]
}

// define adds the wire named name; a name defined twice panics.
func (w *wires) define(name string) *rule.Wire {
	wire := rule.NewWire(name)
	for _, d := range w.defined {
		if d.Wiring() == wire.Wiring() {
			panic(fmt.Sprintf("world: wire %q is defined twice", name))
		}
	}
	w.defined = append(w.defined, wire)
	return wire
}

// Of is the wire id is wired to: what a rule's OnWire and WhileWire follow.
func (w *wires) Of(id uid.UID64) (uid.UID64, bool) {
	if w.wired == nil || !w.wired.Seek(id) {
		return 0, false
	}
	return w.wiredC.At(w.wired.Cursor()).To, true
}

// RolesOf is the roles id plays, a bit each: what a rule's Playing asks.
func (w *wires) RolesOf(id uid.UID64) uint64 {
	if w.plays == nil || !w.plays.Seek(id) {
		return 0
	}
	return uint64(*w.playsC.At(w.plays.Cursor()))
}

// system finds the wires a loaded game brought, makes the rest — before anything wired to them is
// made — and carries out the Signals every step.
func (w *wires) system() goke.System {
	return goke.SystemFn{
		OnInit: func(si *goke.SysInit) {
			w.wired = si.NewQueryBuilder(&w.wiredC).Build()
			w.plays = si.NewQueryBuilder(&w.playsC).Build()
			found := map[uint64]uid.UID64{}
			q := si.NewQueryBuilder(&w.own).Build()
			for q.All(); q.Next(); {
				cur := q.Cursor()
				for i, o := range w.own.Slice(cur) {
					found[o.Name] = cur.IDs[i]
				}
			}
			var missing []*rule.Wire
			for _, d := range w.defined {
				if id, ok := found[d.Wiring().Name]; ok {
					d.Made(id)
				} else {
					missing = append(missing, d)
				}
			}
			if len(missing) == 0 {
				return
			}
			f := si.NewFactory(&w.spawn)
			f.Create(len(missing))
			k := 0
			for f.Next() {
				ownings := w.spawn.Slice(&f.Cursor)
				for i, id := range f.IDs {
					ownings[i] = missing[k].Wiring()
					missing[k].Made(id)
					k++
				}
			}
		},
		OnUpdate: func(cb *goke.CmdBuf, _ time.Duration) {
			w.signals.Drain(func(i control.Issued[rule.Signal]) {
				s := i.Command
				if s.Wire == nil || s.Effect == (effect.Effect{}) {
					return
				}
				id, ok := s.Wire.Entity()
				if !ok {
					return
				}
				if s.Toggle && w.effects.Has(id, s.Effect) {
					w.effects.Dispel(id, s.Effect)
					return
				}
				w.effects.Cast(cb, id, s.Effect)
			})
		},
	}
}
