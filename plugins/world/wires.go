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
	built   bool                 // set up: a wire defined now would never get its entity
	byName  map[uint64]uid.UID64 // each wire's entity by its Wiring
	signals control.Queue[rule.Signal]
	flips   []flip          // a step's Signals, net, before they are carried out
	effects *effect.Effects // the world's, which a Signal casts and dispels

	own    goke.Comp[rule.Wiring]
	spawn  goke.Comp[rule.Wiring]
	marks  goke.Comp[tag.Tags[effect.States]] // a wire's markers, so its first effect counts at once
	wired  *goke.Query                        // every entity wired to a wire, sought by Of
	wiredC goke.Comp[rule.Wired]
	plays  *goke.Query // every entity playing roles, sought by RolesOf
	playsC goke.Comp[tag.Tags[rule.Roles]]
}

// flip is where a step's Signals leave one effect on one wire: on or off, and whether a pulse
// cast it afresh.
type flip struct {
	id            uid.UID64
	e             effect.Effect
	was, on, cast bool
}

// define adds the wire named name; a name defined twice, or after Setup, panics.
func (w *wires) define(name string) *rule.Wire {
	if w.built {
		panic(fmt.Sprintf("world: wire %q defined after the game was set up", name))
	}
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
	wire, ok := w.byName[w.wiredC.At(w.wired.Cursor()).To]
	return wire, ok
}

// RolesOf is the roles id plays, a bit each: what a rule's Playing asks.
func (w *wires) RolesOf(id uid.UID64) uint64 {
	if w.plays == nil || !w.plays.Seek(id) {
		return 0
	}
	return uint64(*w.playsC.At(w.plays.Cursor()))
}

// system finds the wires a loaded game brought, makes the rest and carries out the Signals every
// step.
func (w *wires) system() goke.System {
	return goke.SystemFn{
		OnInit: func(si *goke.SysInit) {
			w.built, w.byName = true, map[uint64]uid.UID64{}
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
					w.byName[d.Wiring().Name] = id
				} else {
					missing = append(missing, d)
				}
			}
			if len(missing) == 0 {
				return
			}
			f := si.NewFactory(&w.spawn, &w.marks)
			f.Create(len(missing))
			k := 0
			for f.Next() {
				ownings := w.spawn.Slice(&f.Cursor)
				for i, id := range f.IDs {
					ownings[i] = missing[k].Wiring()
					missing[k].Made(id)
					w.byName[ownings[i].Name] = id
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
				if id, ok := s.Wire.Entity(); ok {
					w.flip(id, s.Effect, s.Toggle)
				}
			})
			for _, f := range w.flips {
				switch {
				case f.on && (f.cast || !f.was):
					w.effects.Cast(cb, f.id, f.e)
				case !f.on && f.was:
					w.effects.Dispel(f.id, f.e)
				}
			}
			w.flips = w.flips[:0]
		},
	}
}

// flip notes a Signal of e on the wire id: two flips in one step leave it as it was, whatever the
// effects carry out only at their pass.
func (w *wires) flip(id uid.UID64, e effect.Effect, toggle bool) {
	at := -1
	for i, f := range w.flips {
		if f.id == id && f.e == e {
			at = i
			break
		}
	}
	if at < 0 {
		on := w.effects.Has(id, e)
		w.flips = append(w.flips, flip{id: id, e: e, was: on, on: on})
		at = len(w.flips) - 1
	}
	f := &w.flips[at]
	if toggle && f.on {
		f.on, f.cast = false, false
		return
	}
	f.on, f.cast = true, true
}
