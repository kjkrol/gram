package effect_test

import (
	"testing"

	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/gram/rule/effect"
)

// hookGlow hooks on the world a rule keeping glow on while it fires, then a rule dispelling it at
// the steps douse says — with shield, the dispeller casts it first and the keeper keeps off it.
func hookGlow(r *rig, glow, shield effect.Effect, douse *bool) {
	keep := func(m *rule.Moment[world.Moving]) rule.Step { return m.Keep(glow) }
	put := func(m *rule.Moment[world.Moving]) rule.Step { return m.Dispel(glow) }
	if shield != (effect.Effect{}) {
		keep = func(m *rule.Moment[world.Moving]) rule.Step { return m.Unless(shield, m.Keep(glow)) }
		put = func(m *rule.Moment[world.Moving]) rule.Step { return m.Steps(m.Apply(shield), m.Dispel(glow)) }
	}
	dousing := func(world.Moving) bool { return *douse }
	if err := r.w.Hook(
		rule.On("glow", rule.All, keep),
		rule.On("douse", rule.All, func(m *rule.Moment[world.Moving]) rule.Step { return m.If(dousing, put(m)) }),
	); err != nil {
		r.t.Fatal(err)
	}
}

// A rule's Dispel takes off an effect another rule keeps; the keeper, its cause going on, has it
// back the step after.
func TestRule_DispelTakesOffWhatAnotherRuleKeepsTillItsNextStep(t *testing.T) {
	var glow effect.Effect
	douse := false
	r := newRig(t, true, func(r *rig) {
		glow = r.fx.Define("glow", effect.Spec{})
		hookGlow(r, glow, effect.Effect{}, &douse)
	})
	r.tick()
	r.tick()
	if !r.marked(glow.Mark()) {
		t.Fatal("the kept effect never began")
	}
	douse = true
	r.tick()
	douse = false
	if r.marked(glow.Mark()) {
		t.Error("the dispelled effect is still on")
	}
	r.tick()
	if !r.marked(glow.Mark()) {
		t.Error("the keeper did not have its effect back the step after")
	}
}

// A shield the dispeller casts, which the keeper keeps off, lets the dispeller win while it lasts.
func TestRule_AShieldLetsTheDispellerWin(t *testing.T) {
	var glow, shield effect.Effect
	douse := false
	r := newRig(t, true, func(r *rig) {
		glow = r.fx.Define("glow", effect.Spec{})
		shield = r.fx.Define("shield", effect.Spec{effect.Lasts(3 * tick)})
		hookGlow(r, glow, shield, &douse)
	})
	r.tick()
	r.tick()
	douse = true
	r.tick()
	douse = false
	for i := range 2 {
		if r.marked(glow.Mark()) {
			t.Fatalf("%d steps after the douse the glow is back under the shield", i)
		}
		r.tick()
	}
	for range 3 {
		r.tick()
	}
	if !r.marked(glow.Mark()) {
		t.Error("the shield over, the keeper did not have its effect back")
	}
}

// A plan's Dispel takes an effect off the actor.
func TestPlan_DispelTakesAnEffectOff(t *testing.T) {
	var glow effect.Effect
	r := newRig(t, true, func(r *rig) {
		glow = r.fx.Define("glow", effect.Spec{})
		r.comps = append(r.comps, rule.Plan("douse in a while", func(a *rule.Actor) rule.Step {
			return a.Steps(a.Wait(3*tick), a.Dispel(glow), a.Idle())
		}))
	})
	r.cast(glow)
	r.tick()
	if !r.marked(glow.Mark()) {
		t.Fatal("the cast effect never began")
	}
	for range 4 {
		r.tick()
	}
	if r.fx.Has(r.id, glow) {
		t.Error("the plan's Dispel left the effect on")
	}
}

// A plan's Keep gives way when someone else takes its effect off: the branch fails and the plan
// goes on, the effect left off.
func TestPlan_KeepGivesWayWhenSomeoneElseDispels(t *testing.T) {
	var glow, gaveWay effect.Effect
	r := newRig(t, true, func(r *rig) {
		glow = r.fx.Define("glow", effect.Spec{})
		gaveWay = r.fx.Define("gave way", effect.Spec{})
		r.comps = append(r.comps, rule.Plan("glow till doused", func(a *rule.Actor) rule.Step {
			return a.Steps(a.Not(a.Keep(glow)), a.Apply(gaveWay), a.Idle())
		}))
	})
	for range 3 {
		r.tick()
	}
	if !r.fx.Has(r.id, glow) || r.fx.Has(r.id, gaveWay) {
		t.Fatalf("kept: glow %v, gave way %v; want true, false", r.fx.Has(r.id, glow), r.fx.Has(r.id, gaveWay))
	}
	r.fx.Dispel(r.id, glow)
	for range 3 {
		r.tick()
	}
	if r.fx.Has(r.id, glow) || !r.fx.Has(r.id, gaveWay) {
		t.Errorf("doused: glow %v, gave way %v; want false, true", r.fx.Has(r.id, glow), r.fx.Has(r.id, gaveWay))
	}
}

// A player's world.Apply puts an effect on the world; a rule's During runs its step while the
// world is under it, and a plan's alike.
func TestDuring_RunsWhileTheWorldIsUnderTheEffect(t *testing.T) {
	for _, plan := range []bool{false, true} {
		var lever, open effect.Effect
		r := newRig(t, true, func(r *rig) {
			lever = r.fx.Define("lever", effect.Spec{effect.Lasts(2 * tick)})
			open = r.fx.Define("open", effect.Spec{})
			if plan {
				r.comps = append(r.comps, rule.Plan("open while pulled", func(a *rule.Actor) rule.Step {
					return a.OneOf(a.During(lever, a.Keep(open)), a.Idle())
				}))
				return
			}
			if err := r.w.Hook(rule.On("open while pulled", rule.All, func(m *rule.Moment[world.Moving]) rule.Step {
				return m.During(lever, m.Keep(open))
			})); err != nil {
				r.t.Fatal(err)
			}
		})
		if err := r.w.Carry(r.w); err != nil {
			t.Fatal(err)
		}
		r.tick()
		r.tick()
		if r.fx.Has(r.id, open) {
			t.Fatalf("plan %v: open before the lever was pulled", plan)
		}
		r.w.Commands().Put(1, world.Apply{Effect: lever})
		r.tick() // the lever is put on the world
		r.tick()
		r.tick()
		if !r.fx.Has(r.id, open) || !r.fx.Has(r.w.Clock().Entity(), lever) {
			t.Fatalf("plan %v: lever on the world %v, open %v; want both", plan, r.fx.Has(r.w.Clock().Entity(), lever), r.fx.Has(r.id, open))
		}
		for range 6 {
			r.tick()
		}
		if r.fx.Has(r.id, open) {
			t.Errorf("plan %v: still open after the lever went back", plan)
		}
	}
}
