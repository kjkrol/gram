package world_test

import (
	"fmt"
	cgoke "github.com/kjkrol/ceremony/goke"
	"strings"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/act"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/ceremony/effect"
	"github.com/kjkrol/ceremony/host"
	"github.com/kjkrol/ceremony/tag"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/steering"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/uid"
)

type fxInstallCtx struct {
	hosts   []host.Host
	ecs     *goke.ECS
	pending []func() []goke.System
}

func (c *fxInstallCtx) UseModule(m goke.Module) {
	regSys := goke.SystemFn{OnInit: func(*goke.SysInit) { m.RegSystems(c.ecs) }}
	c.pending = append(c.pending, func() []goke.System { return append(m.SetupSystems(), regSys) })
}
func (c *fxInstallCtx) Setup(providers ...goke.SetupProvider) {
	for _, p := range providers {
		c.pending = append(c.pending, p.SetupSystems)
	}
}
func (c *fxInstallCtx) RegSys(factory func() goke.System) goke.Runnable {
	return c.ecs.RegSys(factory())
}
func (c *fxInstallCtx) ECS() *goke.ECS { return c.ecs }

type fxMoods struct{}

const fxTick = time.Second / 10

type rig struct {
	t       *testing.T
	w       *world.Plugin
	fx      *cgoke.Effects
	ecs     *goke.ECS
	id      uid.UID64
	angry   tag.Tag[fxMoods]
	query   *goke.Query
	base    goke.Comp[world.Base]
	steer   goke.Comp[steering.Steering]
	look    goke.Comp[render.Appearance]
	marks   goke.OptComp[tag.Tags[fxMoods]]
	states  goke.OptComp[tag.Tags[effect.States]]
	course  goke.OptComp[steering.Course]
	casting func(cb *goke.CmdBuf)
	comps   []comp.Comp
	rules   []act.Rule
}

func newRig(t *testing.T, withFamily bool, define func(r *rig)) *rig {
	t.Helper()
	r := &rig{t: t}
	r.w = world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 400, Height: 400},
		Entities: world.EntitiesCfg{MaxCount: 4, MinSize: 10, MaxSize: 10},
	})
	r.angry = r.w.Kinds().DefineTag[fxMoods]("angry")
	r.fx = r.w.Effects()
	define(r)

	ctx := &fxInstallCtx{ecs: goke.New()}
	if err := r.w.Install(ctx); err != nil {
		t.Fatal(err)
	}
	if err := ctx.Deliver(r.rules...); err != nil {
		t.Fatal(err)
	}
	spec := kind.Spec{
		comp.Const(world.Position{AABB: plane.NewAABB(geom.NewVec(100, 100), 10, 10)}),
		comp.Const(world.Velocity{}),
		comp.Const(steering.Steering{MaxSpeed: 10}),
		comp.Const(steering.Course{}),
	}
	if withFamily {
		spec = append(spec, comp.Tagged[fxMoods](), comp.Marks[effect.States]())
	}
	spec = append(spec, r.comps...)
	kind.Define[struct{}](r.w.Kinds(), "unit", spec)
	unit := kind.Named[struct{}](r.w.Kinds(), "unit")
	r.w.Seed(unit.Entry(struct{}{}))
	if err := r.w.Populate(); err != nil {
		t.Fatal(err)
	}

	var systems []goke.System
	for _, produce := range ctx.pending {
		systems = append(systems, produce()...)
	}
	systems = append(systems, goke.SystemFn{OnInit: func(si *goke.SysInit) {
		r.query = si.NewQueryBuilder(&r.base, &r.steer, &r.look).Optional(&r.marks, &r.states, &r.course).Build()
	}})
	ctx.ecs.Setup(systems...)
	caster := ctx.ecs.RegSys(goke.SystemFn{OnUpdate: func(cb *goke.CmdBuf, _ time.Duration) {
		if r.casting != nil {
			r.casting(cb)
			r.casting = nil
		}
	}})
	ctx.ecs.SetPlan(func(rc goke.RunCtx, d time.Duration) {
		rc.Run(caster, d)
		rc.Sync()
		r.w.RunPlan(rc, d)
		r.w.Clock().Replay(rc, d)
		rc.Sync()
	})
	r.ecs = ctx.ecs
	for r.query.All(); r.query.Next(); {
		r.id = r.query.Cursor().IDs[0]
	}
	return r
}

func (r *rig) fxTick() { r.ecs.Tick(fxTick) }

func (r *rig) cast(fx effect.Effect) {
	r.casting = func(cb *goke.CmdBuf) { r.fx.Cast(r.id, fx) }
}

func (r *rig) state() (speed float64, sprite uint8, angry bool, active bool) {
	for r.query.All(); r.query.Next(); {
		cur := r.query.Cursor()
		speed = r.steer.Slice(cur)[0].MaxSpeed
		sprite = uint8(r.look.Slice(cur)[0].SpriteID)
		if m := r.marks.Slice(cur); m != nil {
			angry = m[0].Has(r.angry)
		}
		if s := r.states.Slice(cur); s != nil {
			active = s[0].Without(effect.Changed) != 0
		}
	}
	return
}

func (r *rig) marked(t tag.Tag[effect.States]) bool {
	on := false
	for r.query.All(); r.query.Next(); {
		m := r.states.Slice(r.query.Cursor())
		on = m != nil && m[0].Has(t)
	}
	return on
}

func TestEffects_GrantAndAlterHoldForLastsThenRevert(t *testing.T) {
	var rage effect.Effect
	r := newRig(t, true, func(r *rig) {
		r.fx.Define("rage", effect.Spec{
			effect.Lasts(3 * fxTick),
			effect.Grant(r.angry),
			effect.Alter(func(s *steering.Steering) { s.MaxSpeed *= 2 }),
			effect.Alter(func(a *render.Appearance) { a.SpriteID = 7 }),
		})
		rage = r.fx.Named("rage")
	})
	r.cast(rage)
	r.fxTick() // the effects' pass runs after the cast
	r.fxTick()
	if speed, sprite, angry, active := r.state(); speed != 20 || sprite != 7 || !angry || !active {
		t.Fatalf("running: speed %v sprite %d angry %v active %v, want 20, 7, true, true", speed, sprite, angry, active)
	}
	if !r.marked(rage.Mark()) {
		t.Error("rage's own marker is off while it runs")
	}
	for range 3 {
		r.fxTick()
	}
	if speed, sprite, angry, active := r.state(); speed != 10 || sprite != 0 || angry || active {
		t.Errorf("after its time: speed %v sprite %d angry %v active %v, want 10, 0, false, false", speed, sprite, angry, active)
	}
	if r.marked(rage.Mark()) {
		t.Error("rage's own marker stayed after it ended")
	}
}

func TestEffects_TwoCastAtOnceOnAFreshEntityBothLand(t *testing.T) {
	var haste, mark effect.Effect
	r := newRig(t, false, func(r *rig) {
		r.fx.Define("haste", effect.Spec{effect.Lasts(3 * fxTick), effect.Alter(func(s *steering.Steering) { s.MaxSpeed *= 2 })})
		haste = r.fx.Named("haste")
		r.fx.Define("mark", effect.Spec{effect.Lasts(3 * fxTick), effect.Grant(r.angry)})
		mark = r.fx.Named("mark")
	})
	r.casting = func(cb *goke.CmdBuf) {
		r.fx.Cast(r.id, haste)
		r.fx.Cast(r.id, mark)
	}
	r.fxTick()
	if !r.marked(haste.Mark()) || !r.marked(mark.Mark()) {
		t.Errorf("haste %v mark %v after the first fxTick, want both markers on as their family is attached", r.marked(haste.Mark()), r.marked(mark.Mark()))
	}
	r.fxTick()
	if speed, _, angry, _ := r.state(); speed != 20 || !angry {
		t.Errorf("speed %v angry %v, want 20 and true: both effects cast at once on it", speed, angry)
	}
	if !r.marked(haste.Mark()) || !r.marked(mark.Mark()) {
		t.Errorf("haste %v mark %v, want both markers on", r.marked(haste.Mark()), r.marked(mark.Mark()))
	}
}

func TestEffects_ChangedMarksTheStepsThatRewroteAComponent(t *testing.T) {
	var haste, mark effect.Effect
	r := newRig(t, true, func(r *rig) {
		r.fx.Define("haste", effect.Spec{effect.Lasts(3 * fxTick), effect.Alter(func(s *steering.Steering) { s.MaxSpeed *= 2 })})
		haste = r.fx.Named("haste")
		r.fx.Define("mark", effect.Spec{effect.Lasts(4 * fxTick), effect.Grant(r.angry)})
		mark = r.fx.Named("mark")
	})
	r.cast(haste)
	r.fxTick()
	if !r.marked(effect.Changed) {
		t.Error("the step that began an Alter left Changed off")
	}
	r.fxTick()
	if r.marked(effect.Changed) {
		t.Error("a step that rewrote nothing left Changed on")
	}
	r.cast(mark)
	r.fxTick() // a Grant begins beside it
	if r.marked(effect.Changed) {
		t.Error("a Grant beginning turned Changed on")
	}
	r.fxTick() // haste's time is up
	if speed, _, _, _ := r.state(); speed != 10 || !r.marked(effect.Changed) {
		t.Errorf("haste ended: speed %v, Changed %v; want 10 and on", speed, r.marked(effect.Changed))
	}
	r.fxTick()
	if r.marked(effect.Changed) {
		t.Error("Changed stayed on a step after the speed went back")
	}
}

func TestEffects_TwoEffectsGrantingOneTagKeepItTillTheLast(t *testing.T) {
	var rage, fury effect.Effect
	r := newRig(t, true, func(r *rig) {
		r.fx.Define("rage", effect.Spec{effect.Lasts(2 * fxTick), effect.Grant(r.angry)})
		rage = r.fx.Named("rage")
		r.fx.Define("fury", effect.Spec{effect.Lasts(4 * fxTick), effect.Grant(r.angry)})
		fury = r.fx.Named("fury")
	})
	r.casting = func(cb *goke.CmdBuf) {
		r.fx.Cast(r.id, rage)
		r.fx.Cast(r.id, fury)
	}
	r.fxTick()
	r.fxTick()
	r.fxTick() // rage is over, fury runs
	if _, _, angry, _ := r.state(); !angry || r.marked(rage.Mark()) || !r.marked(fury.Mark()) {
		t.Errorf("rage over, fury on: angry %v, rage %v, fury %v; want true, false, true",
			angry, r.marked(rage.Mark()), r.marked(fury.Mark()))
	}
	r.fxTick()
	r.fxTick()
	if _, _, angry, _ := r.state(); angry || r.marked(fury.Mark()) {
		t.Errorf("both over: angry %v, fury %v; want false, false", angry, r.marked(fury.Mark()))
	}
}

func TestEffects_StackedCastsKeepTheMarkerTillTheLast(t *testing.T) {
	var sting effect.Effect
	r := newRig(t, true, func(r *rig) {
		r.fx.Define("sting", effect.Spec{effect.Lasts(2 * fxTick), effect.Stacking()})
		sting = r.fx.Named("sting")
	})
	r.cast(sting)
	r.fxTick()
	r.cast(sting)
	r.fxTick()
	r.fxTick() // the first is over
	if !r.marked(sting.Mark()) {
		t.Error("the marker went with the first of two stacked casts")
	}
	r.fxTick()
	if r.marked(sting.Mark()) {
		t.Error("the marker outlived the last stacked cast")
	}
}

func TestEffects_ThenFollowsWhenTheTimeIsUpNotOnDispel(t *testing.T) {
	var burn, ash effect.Effect
	r := newRig(t, true, func(r *rig) {
		r.fx.Define("ash", effect.Spec{effect.Lasts(2 * fxTick)})
		ash = r.fx.Named("ash")
		r.fx.Define("burn", effect.Spec{effect.Lasts(2 * fxTick), effect.Then(ash)})
		burn = r.fx.Named("burn")
	})
	r.cast(burn)
	r.fxTick()
	r.fxTick()
	r.fxTick() // its time is up
	if r.fx.Has(r.id, burn) || !r.fx.Has(r.id, ash) {
		t.Fatalf("burn over: burn %v, ash %v; want false, true", r.fx.Has(r.id, burn), r.fx.Has(r.id, ash))
	}
	r.fxTick()
	if !r.marked(ash.Mark()) {
		t.Error("the effect that follows never began")
	}
	for range 3 {
		r.fxTick()
	}
	if r.fx.Has(r.id, ash) {
		t.Fatal("the effect that follows outlived its time")
	}

	r.cast(burn)
	r.fxTick()
	r.fx.Dispel(r.id, burn)
	r.fxTick()
	r.fxTick()
	if r.fx.Has(r.id, burn) || r.fx.Has(r.id, ash) {
		t.Errorf("burn dispelled: burn %v, ash %v; want false, false", r.fx.Has(r.id, burn), r.fx.Has(r.id, ash))
	}
}

func TestEffects_ThenBeginsInThePassItsForerunnerRunsOut(t *testing.T) {
	var burn, ash effect.Effect
	r := newRig(t, true, func(r *rig) {
		r.fx.Define("ash", effect.Spec{effect.Lasts(3 * fxTick)})
		ash = r.fx.Named("ash")
		r.fx.Define("burn", effect.Spec{effect.Lasts(2 * fxTick), effect.Then(ash)})
		burn = r.fx.Named("burn")
	})
	r.cast(burn)
	r.fxTick()            // burn begins
	for tick := range 4 { // two of burn, then two of ash's three
		r.fxTick()
		b, a := r.marked(burn.Mark()), r.marked(ash.Mark())
		if !b && !a {
			t.Fatalf("tick %d: under neither burn nor ash; want no step without either", tick)
		}
	}
	r.fxTick() // ash's third
	if r.fx.Has(r.id, ash) {
		t.Error("ash outlived its three ticks")
	}
}

func TestEffects_ACastAfterDispelTakesTheSlotBack(t *testing.T) {
	var burn, ash effect.Effect
	r := newRig(t, true, func(r *rig) {
		r.fx.Define("ash", effect.Spec{effect.Lasts(fxTick)})
		ash = r.fx.Named("ash")
		r.fx.Define("burn", effect.Spec{effect.Lasts(2 * fxTick), effect.Then(ash)})
		burn = r.fx.Named("burn")
	})
	r.cast(burn)
	r.fxTick()
	r.fx.Dispel(r.id, burn)
	r.cast(burn) // in the same step, after the Dispel
	r.fxTick()
	if !r.fx.Has(r.id, burn) {
		t.Fatal("a cast after Dispel did not take the slot back")
	}
	r.fxTick() // its time is up, not dispelled: ash begins in that pass
	if !r.fx.Has(r.id, ash) {
		t.Error("the slot taken back still counted as dispelled")
	}
}

func TestEffects_TwoAltersOfOneComponentComposeAndEndApart(t *testing.T) {
	var haste, slow effect.Effect
	r := newRig(t, true, func(r *rig) {
		r.fx.Define("haste", effect.Spec{effect.Lasts(5 * fxTick), effect.Alter(func(s *steering.Steering) { s.MaxSpeed *= 2 })})
		haste = r.fx.Named("haste")
		r.fx.Define("slow", effect.Spec{effect.Lasts(2 * fxTick), effect.Alter(func(s *steering.Steering) { s.MaxSpeed *= 0.5 })})
		slow = r.fx.Named("slow")
	})
	r.casting = func(cb *goke.CmdBuf) {
		r.fx.Cast(r.id, haste)
	}
	r.fxTick()
	r.cast(slow)
	r.fxTick()
	r.fxTick()
	if speed, _, _, _ := r.state(); speed != 10 {
		t.Fatalf("both running: speed %v, want 10 (×2 × 0.5)", speed)
	}
	r.fxTick()
	r.fxTick()
	if speed, _, _, _ := r.state(); speed != 20 {
		t.Errorf("slow over, haste on: speed %v, want 20", speed)
	}
	for range 4 {
		r.fxTick()
	}
	if speed, _, _, active := r.state(); speed != 10 || active {
		t.Errorf("all over: speed %v active %v, want 10, false", speed, active)
	}
}

func TestEffects_RecastRefreshesUnlessStacking(t *testing.T) {
	var short, stacks effect.Effect
	r := newRig(t, true, func(r *rig) {
		r.fx.Define("short", effect.Spec{effect.Lasts(2 * fxTick), effect.Grant(r.angry)})
		short = r.fx.Named("short")
		r.fx.Define("stacks", effect.Spec{effect.Lasts(2 * fxTick), effect.Stacking(), effect.Alter(func(s *steering.Steering) { s.MaxSpeed++ })})
		stacks = r.fx.Named("stacks")
	})
	r.cast(short)
	r.fxTick()
	r.fxTick()
	r.cast(short)
	r.fxTick() // without the refresh it would have ended here
	if _, _, angry, _ := r.state(); !angry {
		t.Error("a refreshed effect ended on its first clock")
	}
	r.fxTick()
	if _, _, angry, _ := r.state(); angry {
		t.Error("a refreshed effect outlived its second clock")
	}

	r.cast(stacks)
	r.fxTick()
	r.cast(stacks)
	r.fxTick()
	if speed, _, _, _ := r.state(); speed != 12 {
		t.Errorf("two stacked casts: speed %v, want 12", speed)
	}
}

func TestEffects_ForeverLastsUntilDispel(t *testing.T) {
	var curse effect.Effect
	r := newRig(t, true, func(r *rig) {
		r.fx.Define("curse", effect.Spec{effect.Grant(r.angry)})
		curse = r.fx.Named("curse")
	})
	r.cast(curse)
	for range 30 {
		r.fxTick()
	}
	if _, _, angry, _ := r.state(); !angry {
		t.Fatal("an effect without Lasts ended on its own")
	}
	if !r.fx.Has(r.id, curse) {
		t.Error("Has says the curse is gone while it runs")
	}
	r.fx.Dispel(r.id, curse)
	r.fxTick()
	if _, _, angry, active := r.state(); angry || active {
		t.Errorf("after Dispel: angry %v active %v, want false, false", angry, active)
	}
}

func TestEffects_GrantAttachesAMissingFamily(t *testing.T) {
	var rage effect.Effect
	r := newRig(t, false, func(r *rig) {
		r.fx.Define("rage", effect.Spec{effect.Lasts(2 * fxTick), effect.Grant(r.angry)})
		rage = r.fx.Named("rage")
	})
	r.cast(rage)
	r.fxTick() // the family is attached for the next fxTick
	r.fxTick()
	if _, _, angry, _ := r.state(); !angry {
		t.Fatal("the granted tag never arrived on an entity without the family")
	}
	for range 3 {
		r.fxTick()
	}
	if _, _, angry, _ := r.state(); angry {
		t.Error("the granted tag stayed after the effect ended")
	}
}

func TestEffects_DefineRefusesOneEffectTooManyByName(t *testing.T) {
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 400, Height: 400},
		Entities: world.EntitiesCfg{MaxCount: 4, MinSize: 10, MaxSize: 10},
	})
	fx := w.Effects()
	for i := range tag.MaxTagsPerFamily - 1 {
		fx.Define(fmt.Sprint("e", i), effect.Spec{})
	}
	defer func() {
		if msg, _ := recover().(string); !strings.Contains(msg, `"last straw"`) {
			t.Errorf("panic %q, want one naming the effect", msg)
		}
	}()
	fx.Define("last straw", effect.Spec{})
}

func (r *rig) withCourse(fn func(c *steering.Course)) {
	for r.query.All(); r.query.Next(); {
		if cs := r.course.Slice(r.query.Cursor()); cs != nil {
			fn(&cs[0])
		}
	}
}

func TestEffects_AnAlterOfTheKnobsLeavesTheCourseAlone(t *testing.T) {
	var haste effect.Effect
	r := newRig(t, true, func(r *rig) {
		r.fx.Define("haste", effect.Spec{effect.Lasts(2 * fxTick), effect.Alter(func(s *steering.Steering) { s.MaxSpeed *= 2 })})
		haste = r.fx.Named("haste")
	})
	r.cast(haste)
	r.fxTick()
	north := geom.NewVec(0, 1)
	r.withCourse(func(c *steering.Course) { c.Want, c.WantSpeed = north, 15 })
	for range 3 {
		r.fxTick()
	}
	if speed, _, _, _ := r.state(); speed != 10 {
		t.Errorf("after the effect MaxSpeed is %v, want 10 put back", speed)
	}
	r.withCourse(func(c *steering.Course) {
		if c.Want != north || c.WantSpeed != 15 {
			t.Errorf("after the effect the course asks %v at %v, want north at 15 as asked while it ran", c.Want, c.WantSpeed)
		}
	})
}

func TestEffects_Named_IsTheEffectDefined(t *testing.T) {
	var burning effect.Effect
	r := newRig(t, true, func(r *rig) {
		r.fx.Define("wet", effect.Spec{})
		r.fx.Define("burning", effect.Spec{})
		burning = r.fx.Named("burning")
	})
	if got := r.fx.Named("burning"); got != burning {
		t.Errorf("Named(burning) = %+v, want %+v", got, burning)
	}
	defer func() {
		if recover() == nil {
			t.Error("Named of an unknown name did not panic")
		}
	}()
	r.fx.Named("frozen")
}

func TestEffects_AnEffectThatShowsChangesTheEntityAsItBeginsAndEnds(t *testing.T) {
	var snow effect.Effect
	r := newRig(t, true, func(r *rig) {
		r.fx.Define("snow", effect.Spec{effect.Lasts(2 * fxTick)})
		snow = r.fx.Named("snow")
		snow.Shows()
	})
	r.cast(snow)
	r.fxTick()
	if !r.marked(effect.Changed) {
		t.Error("the step it began left Changed off")
	}
	r.fxTick()
	if r.marked(effect.Changed) {
		t.Error("a step it ran on left Changed on")
	}
	r.fxTick()
	if !r.marked(effect.Changed) {
		t.Error("the step it ended left Changed off")
	}
}

func (c *fxInstallCtx) Hosts(h ...host.Host) { c.hosts = append(c.hosts, h...) }

func (c *fxInstallCtx) Deliver(rules ...act.Rule) error { return ceremony.Deliver(c.hosts, rules...) }
