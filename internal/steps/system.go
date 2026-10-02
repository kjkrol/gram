package steps

import (
	"fmt"
	"math/bits"
	"reflect"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/rule/effect"
	"github.com/kjkrol/uid"
)

// Plans runs the plans of every entity with a Mind, once every step of the world's simulation,
// on the world's clock. The world makes and runs it; a game gives its kinds a Plan.
type Plans struct {
	system *system
}

// NewPlans is the plans over now, the world's clock's time, and world, the world's own entity, drawing
// their Chance from seed, casting the world's effects fx and giving the commands its plans order
// to commands, the world's carrier.
func NewPlans(now func() time.Duration, world func() uid.UID64, seed uint64, fx *effect.Effects, commands *control.Carrier) *Plans {
	return &Plans{system: &system{now: now, world: world, seed: seed, effects: fx, commands: commands}}
}

// Wires has the plans find the wire an entity is wired to with lookup: the world's.
func (c *Plans) Wires(lookup func(uid.UID64) (uid.UID64, bool)) { c.system.wires = lookup }

// Roles has the plans find the roles an entity plays with lookup: the world's.
func (c *Plans) Roles(lookup func(uid.UID64) uint64) { c.system.roles = lookup }

// System is the plans' system, run in every step of the simulation.
func (c *Plans) System() goke.System { return c.system }

// LoadComps lists what the plans save: the minds, and the asks of every plan written.
func (c *Plans) LoadComps() []goke.CompToken {
	out := []goke.CompToken{goke.LoadComp[Mind]()}
	seen := map[string]bool{}
	for _, d := range definitions() {
		for _, t := range d.load {
			if !seen[t.Name] {
				seen[t.Name] = true
				out = append(out, t)
			}
		}
	}
	return out
}

// system runs the plans.
type system struct {
	now      func() time.Duration
	world    func() uid.UID64 // the world's own entity, which During asks
	seed     uint64           // the world's, which Chance draws from
	effects  *effect.Effects  // the world's, which Apply, Keep and the rest cast
	commands *control.Carrier // the world's, which Order gives to
	wires    func(uid.UID64) (uid.UID64, bool)
	roles    func(uid.UID64) uint64
	trees    map[uint64]*tree
	facts    map[reflect.Type]any // *fact[F] by F
	si       *goke.SysInit
	query    *goke.Query
	mind     goke.Comp[Mind]
	minds    *goke.Query // every entity with a mind: whom an ask may reach
	anyMind  goke.Comp[Mind]
}

var _ goke.System = (*system)(nil)

// binder is a step that reads or puts a fact: it binds it when the system is set up.
type binder interface{ bind(s *system) }

func (s *system) Init(si *goke.SysInit) {
	s.si, s.facts, s.trees = si, map[reflect.Type]any{}, map[uint64]*tree{}
	for id, d := range definitions() {
		t := layOut(d.name, d.root)
		for _, n := range t.nodes {
			if b, ok := n.exec.(binder); ok {
				b.bind(s)
			}
		}
		s.trees[id] = t
	}
	qb := si.NewQueryBuilder(&s.mind)
	for _, f := range s.facts {
		qb.Optional(f.(interface{ column() goke.OptTrackable }).column())
	}
	s.query = qb.Build()
	s.minds = si.NewQueryBuilder(&s.anyMind).Build()
}

// minded reports whether id is an entity with a mind: one that can take an ask up.
func (s *system) minded(id uid.UID64) bool { return s.minds.Seek(id) }

func (s *system) Update(cb *goke.CmdBuf, dt time.Duration) {
	c := ctx{sys: s, cb: cb, now: s.now(), pass: Pass{Dt: dt}}
	defer func() { c.about = false }()
	for s.query.All(); s.query.Next(); {
		cur := s.query.Cursor()
		minds := s.mind.Slice(cur)
		for i, id := range cur.IDs {
			m := &minds[i]
			t := s.trees[m.Plan]
			if t == nil {
				panic(fmt.Sprintf("rule: entity %v runs a plan no kind gave (%x)", id, m.Plan))
			}
			c.cur, c.i, c.id, c.entity, c.mind, c.tree = cur, i, id, true, m, t
			c.prev, c.next = m.Running, StepSet{}
			c.run(0)
			for w := range c.prev {
				for halted := c.prev[w] &^ c.next[w]; halted != 0; halted &= halted - 1 {
					at := w*64 + bits.TrailingZeros64(halted)
					t.nodes[at].exec.halt(&c, at)
				}
			}
			m.Running = c.next
			for _, f := range s.facts {
				f.(interface{ sweep(*ctx) }).sweep(&c)
			}
		}
	}
}

// ctx is one entity's tick of its tree.
type ctx struct {
	sys        *system // nil for a rule
	instant    bool    // a rule's pass: its moment, no mind kept
	pass       Pass
	payload    any       // a rule's moment, a pointer to it
	subject    uid.UID64 // whom the fact the running node stands under is about, when about
	about      bool
	names      bool // a rule's moment names a subject: an Aimed command fails while it names nobody
	cb         *goke.CmdBuf
	now        time.Duration
	cur        *goke.Cursor
	i          int
	id         uid.UID64
	entity     bool // id holds whom the node acts for; a rule of a clock.Moment has none
	wired      bool // under a plan's OnWire: a Keep is renewed every step, lapsing once it is not
	mind       *Mind
	tree       *tree
	prev, next StepSet // the nodes running before this tick, and those running after it
}

// run runs node at for this tick: readied first when it did not run last tick.
func (c *ctx) run(at int) Status {
	n := &c.tree.nodes[at]
	if !c.wasRunning(at) {
		n.exec.enter(c, at)
	}
	s := n.exec.tick(c, at, n.kids)
	if s == Running {
		c.next.add(at)
	}
	return s
}

// wasRunning reports whether node at ran last tick and was not done.
func (c *ctx) wasRunning(at int) bool { return c.prev.has(at) }

// fact is a fact's column in the system's query, and its component, for putting it on another
// entity and taking it off.
type fact[F any] struct {
	col goke.OptComp[F]
	id  goke.CompID
}

// putOn gives the entity to the fact v from the next sync on; false, and nothing given, when to
// has no mind to take it up — gone, or never given one.
func (f *fact[F]) putOn(c *ctx, to uid.UID64, v F) bool {
	if !c.sys.minded(to) {
		return false
	}
	c.cb.AddOne(to, f.id, v)
	return true
}

// drop takes the fact off the entity from the next sync on, when it has it.
func (f *fact[F]) drop(c *ctx) {
	if f.of(c) != nil {
		c.cb.RemoveCompOne(c.id, f.id)
	}
}

// sweep drops the fact once it is older than AskLife, for a fact that ages.
func (f *fact[F]) sweep(c *ctx) {
	if v := f.of(c); v != nil {
		if a, ok := any(*v).(aged); ok && c.now-a.at() > AskLife {
			c.cb.RemoveCompOne(c.id, f.id)
		}
	}
}

func (f *fact[F]) column() goke.OptTrackable { return &f.col }

// of is the entity's F, nil without one.
func (f *fact[F]) of(c *ctx) *F {
	s := f.col.Slice(c.cur)
	if s == nil {
		return nil
	}
	return &s[c.i]
}

// factOf is s's column of F, made at its first.
func factOf[F any](s *system) *fact[F] {
	key := reflect.TypeFor[F]()
	if f, ok := s.facts[key]; ok {
		return f.(*fact[F])
	}
	f := &fact[F]{id: s.si.RegComp[F]()}
	s.facts[key] = f
	return f
}

// under runs node at with the fact f in scope: its subject, when it is about another entity, is
// whom an Ask, a Relay or an Aimed command under it speaks to; a fact about nobody leaves the
// subject from further out.
func (c *ctx) under(f any, at int) Status {
	s, ok := f.(Subject)
	if !ok {
		return c.run(at)
	}
	who, about := s.Subject()
	if !about {
		return c.run(at)
	}
	outer, was := c.subject, c.about
	c.subject, c.about = who, true
	st := c.run(at)
	c.subject, c.about = outer, was
	return st
}
