package world_test

import (
	"testing"

	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/act"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/ceremony/cond"
	"github.com/kjkrol/ceremony/effect"
	"github.com/kjkrol/ceremony/plan"
	"github.com/kjkrol/gram/plugins/world"
)

func hookGlow(r *rig, glow, shield effect.Effect, douse *bool) {
	var keep, put act.Step = act.Self.Hold(glow), act.Self.Take(glow)
	if shield != (effect.Effect{}) {
		keep = act.Branch[any]{When: cond.Self.If(cond.Not(shield)), Then: act.Self.Hold(glow)}
		put = act.Steps(act.Self.Put(shield), act.Self.Take(glow))
	}
	dousing := cond.NewWord("dousing", func(world.Moving) bool { return *douse })
	r.rules = append(r.rules,
		act.Branch[world.Moving]{Name: "glow", When: cond.Self.If(cond.Always), Then: keep},
		act.Branch[world.Moving]{Name: "douse", When: cond.Self.If(dousing), Then: put})
}

func TestRule_TakeTakesOffWhatAnotherRuleHoldsTillItsNextStep(t *testing.T) {
	var glow effect.Effect
	douse := false
	r := newRig(t, true, func(r *rig) {
		r.fx.Define("glow", effect.Spec{})
		glow = r.fx.Named("glow")
		hookGlow(r, glow, effect.Effect{}, &douse)
	})
	r.fxTick()
	r.fxTick()
	if !r.marked(glow.Mark()) {
		t.Fatal("the kept effect never began")
	}
	douse = true
	r.fxTick()
	douse = false
	if r.marked(glow.Mark()) {
		t.Error("the dispelled effect is still on")
	}
	r.fxTick()
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
		r.fx.Define("shield", effect.Spec{effect.Lasts(3 * fxTick)})
		shield = r.fx.Named("shield")
		hookGlow(r, glow, shield, &douse)
	})
	r.fxTick()
	r.fxTick()
	douse = true
	r.fxTick()
	douse = false
	for i := range 2 {
		if r.marked(glow.Mark()) {
			t.Fatalf("%d steps after the douse the glow is back under the shield", i)
		}
		r.fxTick()
	}
	for range 3 {
		r.fxTick()
	}
	if !r.marked(glow.Mark()) {
		t.Error("the shield over, the keeper did not have its effect back")
	}
}

func TestPlan_TakeTakesAnEffectOff(t *testing.T) {
	var glow effect.Effect
	r := newRig(t, true, func(r *rig) {
		r.fx.Define("glow", effect.Spec{})
		glow = r.fx.Named("glow")
		r.comps = append(r.comps, asPlan(r.w, "douse in a while", plan.Steps(
			plan.Self.Wait(3*fxTick),
			plan.Self.Take(glow),
			plan.Self.Idle(),
		)))
	})
	r.cast(glow)
	r.fxTick()
	if !r.marked(glow.Mark()) {
		t.Fatal("the cast effect never began")
	}
	for range 4 {
		r.fxTick()
	}
	if r.fx.Has(r.id, glow) {
		t.Error("the plan's Dispel left the effect on")
	}
}

func TestPlan_HoldGivesWayWhenSomeoneElseTakes(t *testing.T) {
	var glow, gaveWay effect.Effect
	r := newRig(t, true, func(r *rig) {
		r.fx.Define("glow", effect.Spec{})
		glow = r.fx.Named("glow")
		r.fx.Define("gave way", effect.Spec{})
		gaveWay = r.fx.Named("gave way")
		r.comps = append(r.comps, asPlan(r.w, "glow till doused", plan.OneOf(
			plan.Loop[any]{
				Name: "glow",
				When: cond.Self.If(cond.Not(gaveWay)),
				Then: plan.Self.Hold(glow),
			},
			plan.Loop[any]{
				Name: "gave way",
				When: cond.Self.If(cond.Always),
				Then: plan.Steps(plan.Self.Put(gaveWay), plan.Self.Idle()),
			},
		)))
	})
	for range 3 {
		r.fxTick()
	}
	if !r.fx.Has(r.id, glow) || r.fx.Has(r.id, gaveWay) {
		t.Fatalf("kept: glow %v, gave way %v; want true, false", r.fx.Has(r.id, glow), r.fx.Has(r.id, gaveWay))
	}
	r.fx.Dispel(r.id, glow)
	for range 3 {
		r.fxTick()
	}
	if r.fx.Has(r.id, glow) || !r.fx.Has(r.id, gaveWay) {
		t.Errorf("doused: glow %v, gave way %v; want false, true", r.fx.Has(r.id, glow), r.fx.Has(r.id, gaveWay))
	}
}

func TestDuring_RunsWhileTheWorldIsUnderTheEffect(t *testing.T) {
	for _, planned := range []bool{false, true} {
		var lever, open effect.Effect
		r := newRig(t, true, func(r *rig) {
			r.fx.Define("lever", effect.Spec{effect.Lasts(2 * fxTick)})
			lever = r.fx.Named("lever")
			r.fx.Define("open", effect.Spec{})
			open = r.fx.Named("open")
			if planned {
				r.comps = append(r.comps, asPlan(r.w, "open while pulled", plan.OneOf(
					plan.Loop[any]{
						Name: "open",
						When: cond.On(r.w).If(lever),
						Then: plan.Self.Hold(open),
					},
					plan.Loop[any]{Name: "idle", When: cond.Self.If(cond.Always), Then: plan.Self.Idle()},
				)))
				return
			}
			r.rules = append(r.rules, act.Branch[world.Moving]{
				Name: "open while pulled",
				When: cond.On(r.w).If(lever),
				Then: act.Self.Hold(open),
			})
		})
		if err := r.w.Carry(r.w); err != nil {
			t.Fatal(err)
		}
		r.fxTick()
		r.fxTick()
		if r.fx.Has(r.id, open) {
			t.Fatalf("planned %v: open before the lever was pulled", planned)
		}
		r.w.Carrier().Put(1, ceremony.Put(lever).On(ceremony.World))
		r.fxTick()
		r.fxTick()
		if !r.fx.Has(r.id, open) || !r.fx.Has(r.w.Clock().Entity(), lever) {
			t.Fatalf("planned %v: lever on the world %v, open %v; want both", planned, r.fx.Has(r.w.Clock().Entity(), lever), r.fx.Has(r.id, open))
		}
		for range 6 {
			r.fxTick()
		}
		if r.fx.Has(r.id, open) {
			t.Errorf("planned %v: still open after the lever went back", planned)
		}
	}
}

func asPlan(_ *world.Plugin, name string, root plan.Part) comp.Comp {
	return plan.New(name, root)
}
