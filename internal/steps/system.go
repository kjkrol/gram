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

// Plans runs the plans of every entity with a Mind, once every step of the world's simulation.
type Plans struct {
	system *system
}

// NewPlans is the plans on the clock now, over the world's own entity world, drawing Chance
// from seed, casting fx and giving their commands to commands.
func NewPlans(now func() time.Duration, world func() uid.UID64, seed uint64, fx *effect.Effects, commands *control.Carrier) *Plans {
	return &Plans{system: &system{now: now, world: world, seed: seed, effects: fx, commands: commands}}
}

// Roles has the plans find the roles an entity plays with lookup.
func (c *Plans) Roles(lookup func(uid.UID64) uint64) { c.system.roles = lookup }

// System is the plans' system, run in every step of the simulation.
func (c *Plans) System() goke.System { return c.system }

// LoadComps lists what the plans save.
func (c *Plans) LoadComps() []goke.CompToken {
	out := []goke.CompToken{goke.LoadComp[Mind]()}
	seen := map[string]bool{}
	for _, d := range c.system.defs {
		for _, t := range d.load {
			if !seen[t.Name] {
				seen[t.Name] = true
				out = append(out, t)
			}
		}
	}
	return out
}

type system struct {
	now      func() time.Duration
	world    func() uid.UID64
	seed     uint64
	effects  *effect.Effects
	commands *control.Carrier
	roles    func(uid.UID64) uint64
	defs     map[uint64]definition // by name hashed
	trees    map[uint64]*tree
	facts    map[reflect.Type]any // *fact[F] by F
	si       *goke.SysInit
	query    *goke.Query
	mind     goke.Comp[Mind]
	minds    *goke.Query
	anyMind  goke.Comp[Mind]
}

var _ goke.System = (*system)(nil)

type binder interface{ bind(s *system) }

func (s *system) Init(si *goke.SysInit) {
	s.si, s.facts, s.trees = si, map[reflect.Type]any{}, map[uint64]*tree{}
	for id, d := range s.defs {
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

type ctx struct {
	sys        *system // nil for a rule
	instant    bool
	pass       Pass
	payload    any // a pointer to a rule's moment
	subject    uid.UID64
	about      bool
	names      bool // the moment names a subject: an Aimed command fails while it names nobody
	cb         *goke.CmdBuf
	now        time.Duration
	cur        *goke.Cursor
	i          int
	id         uid.UID64
	entity     bool // false for a rule of a clock.Moment: nobody to act for
	mind       *Mind
	tree       *tree
	prev, next StepSet
}

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

func (c *ctx) wasRunning(at int) bool { return c.prev.has(at) }

type fact[F any] struct {
	col goke.OptComp[F]
	id  goke.CompID
}

// false, and nothing given, when to has no mind to take it up
func (f *fact[F]) putOn(c *ctx, to uid.UID64, v F) bool {
	if !c.sys.minded(to) {
		return false
	}
	c.cb.AddOne(to, f.id, v)
	return true
}

func (f *fact[F]) drop(c *ctx) {
	if f.of(c) != nil {
		c.cb.RemoveCompOne(c.id, f.id)
	}
}

func (f *fact[F]) sweep(c *ctx) {
	if v := f.of(c); v != nil {
		if a, ok := any(*v).(aged); ok && c.now-a.at() > AskLife {
			c.cb.RemoveCompOne(c.id, f.id)
		}
	}
}

func (f *fact[F]) column() goke.OptTrackable { return &f.col }

func (f *fact[F]) of(c *ctx) *F {
	s := f.col.Slice(c.cur)
	if s == nil {
		return nil
	}
	return &s[c.i]
}

func factOf[F any](s *system) *fact[F] {
	key := reflect.TypeFor[F]()
	if f, ok := s.facts[key]; ok {
		return f.(*fact[F])
	}
	f := &fact[F]{id: s.si.RegComp[F]()}
	s.facts[key] = f
	return f
}

// a fact about nobody leaves the subject from further out
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
