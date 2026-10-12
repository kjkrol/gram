package effect_test

import (
	"testing"

	"github.com/kjkrol/gram/entity"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/gram/rule/effect"
)

func hookGlow(r *rig, glow, shield effect.Effect, douse *bool) {
	keep, put := rule.Keep(glow), rule.Dispel(glow)
	if shield != (effect.Effect{}) {
		keep = rule.Branch[any]{When: rule.Free(shield), Do: rule.Keep(glow)}
		put = rule.Steps(rule.Apply(shield), rule.Dispel(glow))
	}
	dousing := func(world.Moving) bool { return *douse }
	r.rules = append(r.rules,
		rule.Branch[world.Moving]{Name: "glow", Do: keep},
		rule.Branch[world.Moving]{Name: "douse", When: dousing, Do: put})
}

func TestRule_DispelTakesOffWhatAnotherRuleKeepsTillItsNextStep(t *testing.T) {
	var glow effect.Effect
	douse := false
	r := newRig(t, true, func(r *rig) {
		r.fx.Define("glow", effect.Spec{})
		glow = r.fx.Named("glow")
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

func TestRule_AShieldLetsTheDispellerWin(t *testing.T) {
	var glow, shield effect.Effect
	douse := false
	r := newRig(t, true, func(r *rig) {
		r.fx.Define("glow", effect.Spec{})
		glow = r.fx.Named("glow")
		r.fx.Define("shield", effect.Spec{effect.Lasts(3 * tick)})
		shield = r.fx.Named("shield")
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

func TestPlan_DispelTakesAnEffectOff(t *testing.T) {
	var glow effect.Effect
	r := newRig(t, true, func(r *rig) {
		r.fx.Define("glow", effect.Spec{})
		glow = r.fx.Named("glow")
		r.comps = append(r.comps, asPlan(r.w, "douse in a while", rule.Steps(
			rule.Wait(3*tick),
			rule.Dispel(glow),
			rule.Idle(),
		)))
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

func TestPlan_KeepGivesWayWhenSomeoneElseDispels(t *testing.T) {
	var glow, gaveWay effect.Effect
	r := newRig(t, true, func(r *rig) {
		r.fx.Define("glow", effect.Spec{})
		glow = r.fx.Named("glow")
		r.fx.Define("gave way", effect.Spec{})
		gaveWay = r.fx.Named("gave way")
		r.comps = append(r.comps, asPlan(r.w, "glow till doused", rule.Steps(
			rule.Invert(rule.Keep(glow)),
			rule.Apply(gaveWay),
			rule.Idle(),
		)))
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

func TestDuring_RunsWhileTheWorldIsUnderTheEffect(t *testing.T) {
	for _, planned := range []bool{false, true} {
		var lever, open effect.Effect
		r := newRig(t, true, func(r *rig) {
			r.fx.Define("lever", effect.Spec{effect.Lasts(2 * tick)})
			lever = r.fx.Named("lever")
			r.fx.Define("open", effect.Spec{})
			open = r.fx.Named("open")
			if planned {
				r.comps = append(r.comps, asPlan(r.w, "open while pulled", rule.OneOf(
					rule.Branch[any]{When: rule.During(lever), Do: rule.Keep(open)},
					rule.Idle(),
				)))
				return
			}
			r.rules = append(r.rules, rule.Branch[world.Moving]{
				Name: "open while pulled",
				When: rule.During(lever),
				Do:   rule.Keep(open),
			})
		})
		if err := r.w.Carry(r.w); err != nil {
			t.Fatal(err)
		}
		r.tick()
		r.tick()
		if r.fx.Has(r.id, open) {
			t.Fatalf("planned %v: open before the lever was pulled", planned)
		}
		r.w.Carrier().Put(1, rule.Cast(lever).On(entity.World))
		r.tick()
		r.tick()
		if !r.fx.Has(r.id, open) || !r.fx.Has(r.w.Clock().Entity(), lever) {
			t.Fatalf("planned %v: lever on the world %v, open %v; want both", planned, r.fx.Has(r.w.Clock().Entity(), lever), r.fx.Has(r.id, open))
		}
		for range 6 {
			r.tick()
		}
		if r.fx.Has(r.id, open) {
			t.Errorf("planned %v: still open after the lever went back", planned)
		}
	}
}

func asPlan(_ *world.Plugin, name string, root rule.Step) comp.Comp { return rule.Plan(name, root) }
