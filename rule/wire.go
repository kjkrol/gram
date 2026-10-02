package rule

import (
	"hash/fnv"

	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/rule/effect"
	"github.com/kjkrol/uid"
)

// Wire is a connection by name — a lever and its trapdoors, a plate and its gate. Its own entity
// holds its state, an effect on it; whatever is wired to it carries Wired, and its rules read the
// wire's state (WhileWire) or drive it (OnWire). Any number of wires share one effect, so a game
// with a hundred levers spends one effect and the roles that read it. Define one by name with
// world.Plugin.Wire.
type Wire struct {
	name   string
	hash   uint64
	entity uid.UID64
	made   bool
}

// Wiring is the component of a wire's own entity: the wire's name, hashed, by which a loaded game
// finds it.
type Wiring struct{ Name uint64 }

// Wired is the component of an entity wired to a wire: To is the wire's name, hashed, as its
// Wiring says, so a kind gives it before the wire's entity is made and a save keeps it.
type Wired struct{ To uint64 }

// Signal is the command to put Effect on Wire's entity — Toggle, to take it off when it is on: a
// key pulling a lever, a switch flipped. The world carries it out.
type Signal struct {
	Wire   *Wire
	Effect effect.Effect
	Toggle bool
}

// NewWire is the wire named name: what world.Plugin.Wire makes.
func NewWire(name string) *Wire {
	h := fnv.New64a()
	h.Write([]byte(name))
	return &Wire{name: name, hash: h.Sum64()}
}

// Wiring is the component of the wire's own entity.
func (w *Wire) Wiring() Wiring { return Wiring{Name: w.hash} }

// Wired is the component of whatever is wired to the wire: comp.Const(w.Wired()) in a kind.
func (w *Wire) Wired() Wired { return Wired{To: w.hash} }

// String names the wire.
func (w *Wire) String() string { return "the wire " + w.name }

// Entity is the wire's own entity, once the world has made it or found it in a loaded game.
func (w *Wire) Entity() (uid.UID64, bool) { return w.entity, w.made }

// Made tells the wire its entity: for the world that makes it.
func (w *Wire) Made(id uid.UID64) { w.entity, w.made = id, true }

// Key is a binding putting e on the wire every time trigger fires, label saying so in the list of
// keys: a lever pulled, its effect lasting as its Spec says.
func (w *Wire) Key(e effect.Effect, trigger control.Trigger, label string) control.Binding {
	return control.Command(trigger, label, func(control.Context) (Signal, bool) { return Signal{Wire: w, Effect: e}, true })
}

// Switch is a binding putting e on the wire, or taking it off when it is on, every time trigger
// fires: a switch, on until flipped again (an e without Lasts holds until then, saved with the game).
func (w *Wire) Switch(e effect.Effect, trigger control.Trigger, label string) control.Binding {
	return control.Command(trigger, label, func(control.Context) (Signal, bool) {
		return Signal{Wire: w, Effect: e, Toggle: true}, true
	})
}
