package rule_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/internal/steps"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/gram/rule/effect"
	"github.com/kjkrol/gram/rule/plan"
	"github.com/kjkrol/uid"
)

// newEffects is effects of their own, outside any world: each marker the next bit, Changed first.
func newEffects() *effect.Effects {
	var next tag.Tag[effect.States]
	return effect.New(func(string) tag.Tag[effect.States] { t := next; next++; return t })
}

// press is a made-up host's moment: an entity, pressed.
type press struct{ self uid.UID64 }

func (p press) Who() uid.UID64 { return p.self }

const wireTick = 100 * time.Millisecond

// wireRig is a wire's own entity, one wired to it and one wired to none, over real effects: each
// tick the test's edits land, the plans run, then the host's rules of a press on the two, then
// the effects' pass. Both the plans and the host are told the wire of an entity as the world
// tells it, by its Wired.
type wireRig struct {
	t                   *testing.T
	ecs                 *goke.ECS
	fx                  *effect.Effects
	wire                *rule.Wire
	wired, loose        uid.UID64
	now                 time.Duration
	edits               []func(cb *goke.CmdBuf)
	withoutWires        bool // the host's Tick leaves Wires nil
	host                *plugin.Rules[press]
	planned             comp.Comp // the plan of the two, if any
	wiredCol            goke.Comp[rule.Wired]
	wiredQuery          *goke.Query
	hostTokens, mkToken goke.Comp[token]
}

// newWireRig builds the rig; define makes the effects and hooks the rules or names the plan,
// before the game is set up.
func newWireRig(t *testing.T, define func(r *wireRig)) *wireRig {
	t.Helper()
	r := &wireRig{t: t, ecs: goke.New(), fx: newEffects(), wire: rule.NewWire("west"), host: &plugin.Rules[press]{}}
	define(r)
	r.ecs.Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		var wiring goke.Comp[rule.Wiring]
		f := si.NewFactory(&wiring)
		f.Create(1)
		f.Next()
		r.wire.Made(f.Cursor.IDs[0])
		wiring.Slice(&f.Cursor)[0] = r.wire.Wiring()

		var minds goke.Comp[steps.Mind]
		cols := []goke.Addable{&r.mkToken}
		if r.planned != nil {
			cols = append(cols, &minds)
		}
		mind := func(cur *goke.Cursor) {
			if r.planned != nil {
				minds.Slice(cur)[0] = r.planned.(comp.Template[steps.Mind]).Resolve(nil)
			}
		}
		g := si.NewFactory(append(cols, &r.wiredCol)...)
		g.Create(1)
		g.Next()
		r.wired = g.Cursor.IDs[0]
		id, _ := r.wire.Entity()
		r.wiredCol.Slice(&g.Cursor)[0] = rule.Wired{To: id}
		mind(&g.Cursor)
		h := si.NewFactory(cols...)
		h.Create(1)
		h.Next()
		r.loose = h.Cursor.IDs[0]
		mind(&h.Cursor)
	}})
	edits := r.ecs.RegSys(goke.SystemFn{
		OnInit: func(si *goke.SysInit) { r.wiredQuery = si.NewQueryBuilder(&r.wiredCol).Build() },
		OnUpdate: func(cb *goke.CmdBuf, _ time.Duration) {
			for _, e := range r.edits {
				e(cb)
			}
			r.edits = r.edits[:0]
		},
	})
	plans := steps.NewPlans(func() time.Duration { return r.now }, nil, 0, r.fx, &control.Carrier{})
	plans.Wires(r.wireOf)
	planned := r.ecs.RegSys(plans.System())
	var hostQuery *goke.Query
	host := r.ecs.RegSys(goke.SystemFn{
		OnInit: func(si *goke.SysInit) {
			qb := si.NewQueryBuilder(&r.hostTokens)
			r.host.Bind(qb)
			hostQuery = qb.Build()
		},
		OnUpdate: func(cb *goke.CmdBuf, d time.Duration) {
			tick := plugin.Tick{CmdBuf: cb, Dt: d, Effects: r.fx, Time: r.now + d, Wires: r.wireOf}
			if r.withoutWires {
				tick.Wires = nil
			}
			for hostQuery.All(); hostQuery.Next(); {
				cur := hostQuery.Cursor()
				r.host.Run(tick, cur, func(i int) press { return press{self: cur.IDs[i]} })
			}
		},
	})
	r.fx.Module().RegSystems(r.ecs)
	r.ecs.SetPlan(func(rc goke.RunCtx, d time.Duration) {
		rc.Run(edits, d)
		rc.Sync()
		rc.Run(planned, d)
		rc.Sync()
		rc.Run(host, d)
		rc.Sync()
		r.fx.Module().RunPlan(rc, d)
		r.now += d
	})
	return r
}

// wireOf is the wire id is wired to, read off its Wired as the world reads it.
func (r *wireRig) wireOf(id uid.UID64) (uid.UID64, bool) {
	if !r.wiredQuery.Seek(id) {
		return 0, false
	}
	return r.wiredCol.At(r.wiredQuery.Cursor()).To, true
}

func (r *wireRig) tick(n int) {
	for range n {
		r.ecs.Tick(wireTick)
	}
}

// hook adds the rules to the host.
func (r *wireRig) hook(rules ...rule.Rule) {
	for _, h := range rules {
		if err := r.host.Add(h); err != nil {
			r.t.Fatal(err)
		}
	}
}

// pull puts e on the wire's entity with the next tick.
func (r *wireRig) pull(e effect.Effect) {
	id, _ := r.wire.Entity()
	r.edits = append(r.edits, func(cb *goke.CmdBuf) { r.fx.Cast(cb, id, e) })
}

// release takes e off the wire's entity with the next effects' pass.
func (r *wireRig) release(e effect.Effect) {
	id, _ := r.wire.Entity()
	r.fx.Dispel(id, e)
}

// on reports whether the wire's entity, the wired one and the loose one are under e.
func (r *wireRig) on(e effect.Effect) (wire, wired, loose bool) {
	id, _ := r.wire.Entity()
	return r.fx.Has(id, e), r.fx.Has(r.wired, e), r.fx.Has(r.loose, e)
}

// A rule's WhileWire runs its step while the entity's wire is under the effect, and fails while
// it is not and for one wired to none: open is kept on the wired one only while its wire is on,
// stray on whoever's WhileWire failed.
func TestRule_WhileWireRunsWhileTheWireIsUnderTheEffect(t *testing.T) {
	var on, open, stray effect.Effect
	r := newWireRig(t, func(r *wireRig) {
		on = r.fx.Define("on", effect.Spec{})
		open = r.fx.Define("open", effect.Spec{})
		stray = r.fx.Define("stray", effect.Spec{})
		r.hook(rule.On("open while on", rule.All, func(m *rule.Moment[press]) rule.Step {
			return m.OneOf(m.WhileWire(on, m.Keep(open)), m.Keep(stray))
		}))
	})
	r.tick(3)
	if _, wired, loose := r.on(open); wired || loose {
		t.Fatalf("open before the wire was on: wired %v, loose %v", wired, loose)
	}
	if _, wired, loose := r.on(stray); !wired || !loose {
		t.Fatalf("the wire off, WhileWire failed: stray on wired %v, loose %v; want both", wired, loose)
	}

	r.pull(on)
	r.tick(3)
	if wire, wired, loose := r.on(open); wire || !wired || loose {
		t.Errorf("the wire on: open on wire %v, wired %v, loose %v; want the wired one alone", wire, wired, loose)
	}
	if _, wired, loose := r.on(stray); wired || !loose {
		t.Errorf("the wire on: stray on wired %v, loose %v; want the loose one alone (wired to none)", wired, loose)
	}

	r.release(on)
	r.tick(5)
	if _, wired, _ := r.on(open); wired {
		t.Error("the wire off again, the wired one is still open")
	}
}

// A Tick telling no wires leaves every entity wired to none: WhileWire fails for the wired one
// too, whatever its wire is under.
func TestRule_WhileWireFailsWithoutTheWires(t *testing.T) {
	var on, open effect.Effect
	r := newWireRig(t, func(r *wireRig) {
		on = r.fx.Define("on", effect.Spec{})
		open = r.fx.Define("open", effect.Spec{})
		r.hook(rule.On("open while on", rule.All, func(m *rule.Moment[press]) rule.Step {
			return m.WhileWire(on, m.Keep(open))
		}))
	})
	r.withoutWires = true
	r.pull(on)
	r.tick(3)
	if wire, _, _ := r.on(on); !wire {
		t.Fatal("the wire was never put on")
	}
	if _, wired, loose := r.on(open); wired || loose {
		t.Errorf("open with no wires told: wired %v, loose %v; want neither", wired, loose)
	}
}

// A rule's OnWire runs its step on the wire's entity in place of the entity's own, and the steps
// after it on the entity again; for one wired to none it fails.
func TestRule_OnWireAppliesOnTheWiresEntity(t *testing.T) {
	var on, pressed, stray effect.Effect
	r := newWireRig(t, func(r *wireRig) {
		on = r.fx.Define("on", effect.Spec{effect.Lasts(2 * wireTick)})
		pressed = r.fx.Define("pressed", effect.Spec{})
		stray = r.fx.Define("stray", effect.Spec{})
		r.hook(rule.On("press", rule.All, func(m *rule.Moment[press]) rule.Step {
			return m.OneOf(m.Steps(m.OnWire(m.Apply(on)), m.Apply(pressed)), m.Apply(stray))
		}))
	})
	r.tick(2)
	if wire, wired, loose := r.on(on); !wire || wired || loose {
		t.Errorf("on: wire %v, wired %v, loose %v; want the wire's entity alone", wire, wired, loose)
	}
	if wire, wired, loose := r.on(pressed); wire || !wired || loose {
		t.Errorf("pressed after OnWire: wire %v, wired %v, loose %v; want the wired one alone", wire, wired, loose)
	}
	if wire, wired, loose := r.on(stray); wire || wired || !loose {
		t.Errorf("stray: wire %v, wired %v, loose %v; want the loose one alone (wired to none)", wire, wired, loose)
	}
}

// A plan's WhileWire runs its step while the actor's wire is under the effect: the plan's Keep
// holds open on the wired actor while its wire is on and lets it go when the wire goes off; the
// actor wired to none never opens.
func TestPlan_WhileWireRunsWhileTheWireIsUnderTheEffect(t *testing.T) {
	var on, open effect.Effect
	r := newWireRig(t, func(r *wireRig) {
		on = r.fx.Define("on", effect.Spec{})
		open = r.fx.Define("open", effect.Spec{})
		r.planned = plan.New("wire test: open while on", func(a *plan.Actor) rule.Step {
			return a.OneOf(a.WhileWire(on, a.Keep(open)), a.Idle())
		})
	})
	r.tick(3)
	if _, wired, loose := r.on(open); wired || loose {
		t.Fatalf("open before the wire was on: wired %v, loose %v", wired, loose)
	}
	r.pull(on)
	r.tick(3)
	if wire, wired, loose := r.on(open); wire || !wired || loose {
		t.Fatalf("the wire on: open on wire %v, wired %v, loose %v; want the wired one alone", wire, wired, loose)
	}
	r.release(on)
	r.tick(3)
	if _, wired, _ := r.on(open); wired {
		t.Error("the wire off again, the plan's Keep left the wired one open")
	}
}

// A plan's OnWire runs its step on the wire's entity in place of the actor, the steps after it on
// the actor again; for an actor wired to none it fails.
func TestPlan_OnWireAppliesOnTheWiresEntity(t *testing.T) {
	var on, pressed, stray effect.Effect
	r := newWireRig(t, func(r *wireRig) {
		on = r.fx.Define("on", effect.Spec{})
		pressed = r.fx.Define("pressed", effect.Spec{})
		stray = r.fx.Define("stray", effect.Spec{})
		r.planned = plan.New("wire test: press", func(a *plan.Actor) rule.Step {
			return a.OneOf(
				a.Steps(a.OnWire(a.Apply(on)), a.Apply(pressed), a.Idle()),
				a.Steps(a.Apply(stray), a.Idle()),
			)
		})
	})
	r.tick(2)
	if wire, wired, loose := r.on(on); !wire || wired || loose {
		t.Errorf("on: wire %v, wired %v, loose %v; want the wire's entity alone", wire, wired, loose)
	}
	if wire, wired, loose := r.on(pressed); wire || !wired || loose {
		t.Errorf("pressed after OnWire: wire %v, wired %v, loose %v; want the wired one alone", wire, wired, loose)
	}
	if wire, wired, loose := r.on(stray); wire || wired || !loose {
		t.Errorf("stray: wire %v, wired %v, loose %v; want the loose one alone (wired to none)", wire, wired, loose)
	}
}

// A wire's Wiring is its name hashed with FNV-64a: the same for the same name in every run and
// version — a loaded game finds its wires by it — and another for another name.
func TestNewWire_HashesItsNameStably(t *testing.T) {
	if got, want := rule.NewWire("west").Wiring(), (rule.Wiring{Name: 0x3db014f61a15542e}); got != want {
		t.Errorf("Wiring of \"west\" = %#x, want %#x", got.Name, want.Name)
	}
	if got, want := rule.NewWire("").Wiring(), (rule.Wiring{Name: 0xcbf29ce484222325}); got != want {
		t.Errorf("Wiring of \"\" = %#x, want the FNV-64a offset basis %#x", got.Name, want.Name)
	}
	if rule.NewWire("west").Wiring() != rule.NewWire("west").Wiring() {
		t.Error("one name hashed two ways")
	}
	seen := map[rule.Wiring]string{}
	for _, name := range []string{"west", "east", "West", "wes", "west ", "west2", "lever", ""} {
		w := rule.NewWire(name)
		if w.String() != "the wire "+name {
			t.Errorf("String() = %q, want the wire %s", w.String(), name)
		}
		if other, ok := seen[w.Wiring()]; ok {
			t.Errorf("%q and %q hash alike", name, other)
		}
		seen[w.Wiring()] = name
	}
}

// A wire has no entity until the world tells it one; then its Wired names it.
func TestWire_EntityOnceMade(t *testing.T) {
	w := rule.NewWire("west")
	if _, made := w.Entity(); made {
		t.Fatal("a new wire has an entity")
	}
	w.Made(7)
	if id, made := w.Entity(); !made || id != 7 {
		t.Errorf("Entity() = %v, %v; want 7, true", id, made)
	}
}

// Key and Switch are bindings on their trigger, under their label, issuing a Signal of the wire
// and the effect — the Switch's a toggle.
func TestWire_KeyAndSwitchIssueSignals(t *testing.T) {
	fx := newEffects()
	on := fx.Define("on", effect.Spec{effect.Lasts(time.Second)})
	lit := fx.Define("lit", effect.Spec{})
	w := rule.NewWire("west")
	for _, c := range []struct {
		b       control.Binding
		trigger control.Trigger
		label   string
		want    rule.Signal
	}{
		{w.Key(on, control.KeyPress{Key: control.Key1}, "Pull the west lever"), control.KeyPress{Key: control.Key1},
			"Pull the west lever", rule.Signal{Wire: w, Effect: on}},
		{w.Switch(lit, control.KeyPress{Key: control.Key2, Mods: control.Mods{Shift: true}}, "Flip the west switch"),
			control.KeyPress{Key: control.Key2, Mods: control.Mods{Shift: true}}, "Flip the west switch",
			rule.Signal{Wire: w, Effect: lit, Toggle: true}},
	} {
		if c.b.Trigger != c.trigger || c.b.Label != c.label {
			t.Errorf("binding %q on %+v; want %q on %+v", c.b.Label, c.b.Trigger, c.label, c.trigger)
		}
		if got := c.b.Command(); got != reflect.TypeFor[rule.Signal]() {
			t.Errorf("%q issues a %v, want a rule.Signal", c.label, got)
		}
		cmd, ok := c.b.Build(control.Context{})
		if !ok {
			t.Fatalf("%q declined to build", c.label)
		}
		if got := cmd.(rule.Signal); got != c.want {
			t.Errorf("%q built %+v, want %+v", c.label, got, c.want)
		}
	}
}
