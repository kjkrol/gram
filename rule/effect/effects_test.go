package effect_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/internal/hosts"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/steering"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/gram/rule/effect"
	"github.com/kjkrol/uid"
)

// installCtx is the plugin.Installer a Stage would hand over, minus the engine.
type installCtx struct {
	hosts   []plugin.Host // of the rules of the moments the plugins installed catch
	ecs     *goke.ECS
	pending []func() []goke.System
}

func (c *installCtx) UseModule(m goke.Module) {
	regSys := goke.SystemFn{OnInit: func(*goke.SysInit) { m.RegSystems(c.ecs) }}
	c.pending = append(c.pending, func() []goke.System { return append(m.SetupSystems(), regSys) })
}
func (c *installCtx) Setup(providers ...goke.SetupProvider) {
	for _, p := range providers {
		c.pending = append(c.pending, p.SetupSystems)
	}
}
func (c *installCtx) RegSys(factory func() goke.System) goke.Runnable { return c.ecs.RegSys(factory()) }
func (c *installCtx) ECS() *goke.ECS                                  { return c.ecs }

// moods is the family of the test's tags.
type moods struct{}

const tick = time.Second / 10

// rig is a world with the effects plugin and one entity carrying Steering, Appearance and a
// moods family, plus a casting hook run at the start of each tick.
type rig struct {
	t       *testing.T
	w       *world.Plugin
	fx      *effect.Effects
	ecs     *goke.ECS
	id      uid.UID64
	angry   tag.Tag[moods]
	query   *goke.Query
	base    goke.Comp[world.Base]
	steer   goke.Comp[steering.Steering]
	look    goke.Comp[world.Appearance]
	marks   goke.OptComp[tag.Tags[moods]]
	states  goke.OptComp[tag.Tags[effect.States]]
	course  goke.OptComp[steering.Course]
	casting func(cb *goke.CmdBuf)
	comps   []comp.Comp // more of the entity's kind: a plan
	rules   []rule.Rule // what the world's hosts are handed once it is installed
}

// newRig builds the rig; define adds effects before Install and may read the rig's tags. Without
// withFamily the entity carries neither moods nor the effects' markers.
func newRig(t *testing.T, withFamily bool, define func(r *rig)) *rig {
	t.Helper()
	r := &rig{t: t}
	r.w = world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 400, Height: 400},
		Entities: world.EntitiesCfg{MaxCount: 4, MinSize: 10, MaxSize: 10},
	})
	r.angry = r.w.Kinds().DefineTag[moods]("angry")
	r.fx = r.w.Effects()
	define(r)

	ctx := &installCtx{ecs: goke.New()}
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
		spec = append(spec, comp.Tagged[moods](), comp.Marks[effect.States]())
	}
	spec = append(spec, r.comps...)
	unit := kind.Define[struct{}](r.w.Kinds(), "unit", spec)
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

func (r *rig) tick() { r.ecs.Tick(tick) }

// cast queues a Cast for the next tick.
func (r *rig) cast(fx effect.Effect) {
	r.casting = func(cb *goke.CmdBuf) { r.fx.Cast(cb, r.id, fx) }
}

// state is the entity's speed, sprite, whether it is angry, and whether an effect is on it: the
// marker of one, besides Changed.
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

// marked reports whether the entity has the effects' marker t on right now.
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
			effect.Lasts(3 * tick),
			effect.Grant(r.angry),
			effect.Alter(func(s *steering.Steering) { s.MaxSpeed *= 2 }),
			effect.Alter(func(a *world.Appearance) { a.SpriteID = 7 }),
		})
		rage = r.fx.Named("rage")
	})
	r.cast(rage)
	r.tick() // cast lands and the slot begins, the effects pass running after the cast
	r.tick()
	if speed, sprite, angry, active := r.state(); speed != 20 || sprite != 7 || !angry || !active {
		t.Fatalf("running: speed %v sprite %d angry %v active %v, want 20, 7, true, true", speed, sprite, angry, active)
	}
	if !r.marked(rage.Mark()) {
		t.Error("rage's own marker is off while it runs")
	}
	for range 3 {
		r.tick()
	}
	if speed, sprite, angry, active := r.state(); speed != 10 || sprite != 0 || angry || active {
		t.Errorf("after its time: speed %v sprite %d angry %v active %v, want 10, 0, false, false", speed, sprite, angry, active)
	}
	if r.marked(rage.Mark()) {
		t.Error("rage's own marker stayed after it ended")
	}
}

// Two effects cast in one pass on an entity under none yet, carrying no family of theirs, both
// land: the Active on its way carries them both, the families attached carry both their tags,
// and both begin a tick later.
func TestEffects_TwoCastAtOnceOnAFreshEntityBothLand(t *testing.T) {
	var haste, mark effect.Effect
	r := newRig(t, false, func(r *rig) {
		r.fx.Define("haste", effect.Spec{effect.Lasts(3 * tick), effect.Alter(func(s *steering.Steering) { s.MaxSpeed *= 2 })})
		haste = r.fx.Named("haste")
		r.fx.Define("mark", effect.Spec{effect.Lasts(3 * tick), effect.Grant(r.angry)})
		mark = r.fx.Named("mark")
	})
	r.casting = func(cb *goke.CmdBuf) {
		r.fx.Cast(cb, r.id, haste)
		r.fx.Cast(cb, r.id, mark)
	}
	r.tick() // the Active lands with both; the markers' family is attached with both on
	if !r.marked(haste.Mark()) || !r.marked(mark.Mark()) {
		t.Errorf("haste %v mark %v after the first tick, want both markers on as their family is attached", r.marked(haste.Mark()), r.marked(mark.Mark()))
	}
	r.tick()
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
		r.fx.Define("haste", effect.Spec{effect.Lasts(3 * tick), effect.Alter(func(s *steering.Steering) { s.MaxSpeed *= 2 })})
		haste = r.fx.Named("haste")
		r.fx.Define("mark", effect.Spec{effect.Lasts(4 * tick), effect.Grant(r.angry)})
		mark = r.fx.Named("mark")
	})
	r.cast(haste)
	r.tick() // lands and begins: the speed is rewritten
	if !r.marked(effect.Changed) {
		t.Error("the step that began an Alter left Changed off")
	}
	r.tick() // runs on, nothing rewritten
	if r.marked(effect.Changed) {
		t.Error("a step that rewrote nothing left Changed on")
	}
	r.cast(mark)
	r.tick() // a Grant begins beside it: tags, no Alter
	if r.marked(effect.Changed) {
		t.Error("a Grant beginning turned Changed on")
	}
	r.tick() // haste's time is up: the speed goes back
	if speed, _, _, _ := r.state(); speed != 10 || !r.marked(effect.Changed) {
		t.Errorf("haste ended: speed %v, Changed %v; want 10 and on", speed, r.marked(effect.Changed))
	}
	r.tick()
	if r.marked(effect.Changed) {
		t.Error("Changed stayed on a step after the speed went back")
	}
}

func TestEffects_TwoEffectsGrantingOneTagKeepItTillTheLast(t *testing.T) {
	var rage, fury effect.Effect
	r := newRig(t, true, func(r *rig) {
		r.fx.Define("rage", effect.Spec{effect.Lasts(2 * tick), effect.Grant(r.angry)})
		rage = r.fx.Named("rage")
		r.fx.Define("fury", effect.Spec{effect.Lasts(4 * tick), effect.Grant(r.angry)})
		fury = r.fx.Named("fury")
	})
	r.casting = func(cb *goke.CmdBuf) {
		r.fx.Cast(cb, r.id, rage)
		r.fx.Cast(cb, r.id, fury)
	}
	r.tick()
	r.tick()
	r.tick() // rage is over, fury runs
	if _, _, angry, _ := r.state(); !angry || r.marked(rage.Mark()) || !r.marked(fury.Mark()) {
		t.Errorf("rage over, fury on: angry %v, rage %v, fury %v; want true, false, true",
			angry, r.marked(rage.Mark()), r.marked(fury.Mark()))
	}
	r.tick()
	r.tick()
	if _, _, angry, _ := r.state(); angry || r.marked(fury.Mark()) {
		t.Errorf("both over: angry %v, fury %v; want false, false", angry, r.marked(fury.Mark()))
	}
}

func TestEffects_StackedCastsKeepTheMarkerTillTheLast(t *testing.T) {
	var sting effect.Effect
	r := newRig(t, true, func(r *rig) {
		r.fx.Define("sting", effect.Spec{effect.Lasts(2 * tick), effect.Stacking()})
		sting = r.fx.Named("sting")
	})
	r.cast(sting)
	r.tick() // the first begins: two ticks left
	r.cast(sting)
	r.tick() // the second begins, the first has one left
	r.tick() // the first is over
	if !r.marked(sting.Mark()) {
		t.Error("the marker went with the first of two stacked casts")
	}
	r.tick()
	if r.marked(sting.Mark()) {
		t.Error("the marker outlived the last stacked cast")
	}
}

func TestEffects_ThenFollowsWhenTheTimeIsUpNotOnDispel(t *testing.T) {
	var burn, ash effect.Effect
	r := newRig(t, true, func(r *rig) {
		r.fx.Define("ash", effect.Spec{effect.Lasts(2 * tick)})
		ash = r.fx.Named("ash")
		r.fx.Define("burn", effect.Spec{effect.Lasts(2 * tick), effect.Then(ash)})
		burn = r.fx.Named("burn")
	})
	r.cast(burn)
	r.tick() // begins
	r.tick()
	r.tick() // its time is up: ash is queued
	if r.fx.Has(r.id, burn) || !r.fx.Has(r.id, ash) {
		t.Fatalf("burn over: burn %v, ash %v; want false, true", r.fx.Has(r.id, burn), r.fx.Has(r.id, ash))
	}
	r.tick() // ash begins
	if !r.marked(ash.Mark()) {
		t.Error("the effect that follows never began")
	}
	for range 3 {
		r.tick()
	}
	if r.fx.Has(r.id, ash) {
		t.Fatal("the effect that follows outlived its time")
	}

	r.cast(burn)
	r.tick()
	r.fx.Dispel(r.id, burn)
	r.tick()
	r.tick()
	if r.fx.Has(r.id, burn) || r.fx.Has(r.id, ash) {
		t.Errorf("burn dispelled: burn %v, ash %v; want false, false", r.fx.Has(r.id, burn), r.fx.Has(r.id, ash))
	}
}

func TestEffects_ACastAfterDispelTakesTheSlotBack(t *testing.T) {
	var burn, ash effect.Effect
	r := newRig(t, true, func(r *rig) {
		r.fx.Define("ash", effect.Spec{effect.Lasts(tick)})
		ash = r.fx.Named("ash")
		r.fx.Define("burn", effect.Spec{effect.Lasts(2 * tick), effect.Then(ash)})
		burn = r.fx.Named("burn")
	})
	r.cast(burn)
	r.tick()
	r.fx.Dispel(r.id, burn)
	r.cast(burn) // cast again in the same step, after the Dispel: the cause goes on
	r.tick()
	if !r.fx.Has(r.id, burn) {
		t.Fatal("a cast after Dispel did not take the slot back")
	}
	r.tick()
	r.tick() // its time is up, not dispelled: ash follows
	if !r.fx.Has(r.id, ash) {
		t.Error("the slot taken back still counted as dispelled")
	}
}

func TestEffects_TwoAltersOfOneComponentComposeAndEndApart(t *testing.T) {
	var haste, slow effect.Effect
	r := newRig(t, true, func(r *rig) {
		r.fx.Define("haste", effect.Spec{effect.Lasts(5 * tick), effect.Alter(func(s *steering.Steering) { s.MaxSpeed *= 2 })})
		haste = r.fx.Named("haste")
		r.fx.Define("slow", effect.Spec{effect.Lasts(2 * tick), effect.Alter(func(s *steering.Steering) { s.MaxSpeed *= 0.5 })})
		slow = r.fx.Named("slow")
	})
	r.casting = func(cb *goke.CmdBuf) {
		r.fx.Cast(cb, r.id, haste)
	}
	r.tick()
	r.cast(slow)
	r.tick()
	r.tick()
	if speed, _, _, _ := r.state(); speed != 10 {
		t.Fatalf("both running: speed %v, want 10 (×2 × 0.5)", speed)
	}
	r.tick()
	r.tick()
	if speed, _, _, _ := r.state(); speed != 20 {
		t.Errorf("slow over, haste on: speed %v, want 20", speed)
	}
	for range 4 {
		r.tick()
	}
	if speed, _, _, active := r.state(); speed != 10 || active {
		t.Errorf("all over: speed %v active %v, want 10, false", speed, active)
	}
}

func TestEffects_RecastRefreshesUnlessStacking(t *testing.T) {
	var short, stacks effect.Effect
	r := newRig(t, true, func(r *rig) {
		r.fx.Define("short", effect.Spec{effect.Lasts(2 * tick), effect.Grant(r.angry)})
		short = r.fx.Named("short")
		r.fx.Define("stacks", effect.Spec{effect.Lasts(2 * tick), effect.Stacking(), effect.Alter(func(s *steering.Steering) { s.MaxSpeed++ })})
		stacks = r.fx.Named("stacks")
	})
	r.cast(short)
	r.tick() // begins with two ticks left
	r.tick() // one left
	r.cast(short)
	r.tick() // refreshed to two, one left — without the refresh it would have ended here
	if _, _, angry, _ := r.state(); !angry {
		t.Error("a refreshed effect ended on its first clock")
	}
	r.tick()
	if _, _, angry, _ := r.state(); angry {
		t.Error("a refreshed effect outlived its second clock")
	}

	r.cast(stacks)
	r.tick()
	r.cast(stacks)
	r.tick()
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
		r.tick()
	}
	if _, _, angry, _ := r.state(); !angry {
		t.Fatal("an effect without Lasts ended on its own")
	}
	if !r.fx.Has(r.id, curse) {
		t.Error("Has says the curse is gone while it runs")
	}
	r.fx.Dispel(r.id, curse)
	r.tick()
	if _, _, angry, active := r.state(); angry || active {
		t.Errorf("after Dispel: angry %v active %v, want false, false", angry, active)
	}
}

func TestEffects_GrantAttachesAMissingFamily(t *testing.T) {
	var rage effect.Effect
	r := newRig(t, false, func(r *rig) {
		r.fx.Define("rage", effect.Spec{effect.Lasts(2 * tick), effect.Grant(r.angry)})
		rage = r.fx.Named("rage")
	})
	r.cast(rage)
	r.tick() // Active attached, the family attached for next tick
	r.tick() // begun
	if _, _, angry, _ := r.state(); !angry {
		t.Fatal("the granted tag never arrived on an entity without the family")
	}
	for range 3 {
		r.tick()
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

// withCourse hands fn the entity's Course, as a plugin steering it would write it.
func (r *rig) withCourse(fn func(c *steering.Course)) {
	for r.query.All(); r.query.Next(); {
		if cs := r.course.Slice(r.query.Cursor()); cs != nil {
			fn(&cs[0])
		}
	}
}

// An Alter turns the steering's knobs and puts them back when it ends, leaving the course asked
// meanwhile as it is: the knobs and the state are components apart.
func TestEffects_AnAlterOfTheKnobsLeavesTheCourseAlone(t *testing.T) {
	var haste effect.Effect
	r := newRig(t, true, func(r *rig) {
		r.fx.Define("haste", effect.Spec{effect.Lasts(2 * tick), effect.Alter(func(s *steering.Steering) { s.MaxSpeed *= 2 })})
		haste = r.fx.Named("haste")
	})
	r.cast(haste)
	r.tick() // begins
	north := geom.NewVec(0, 1)
	r.withCourse(func(c *steering.Course) { c.Want, c.WantSpeed = north, 15 })
	for range 3 {
		r.tick() // ends: the knobs put back
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

// An effect is found again by its name, and an unknown name panics.
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

// A look is a slot of its own a sprite, the same asked again, listed with the effect's marker;
// an effect's first look after the looks were taken panics.
func TestEffect_Look_IssuesASlotASpriteUnderTheEffect(t *testing.T) {
	var burning, wet effect.Effect
	r := newRig(t, true, func(r *rig) {
		r.fx.Define("wet", effect.Spec{})
		wet = r.fx.Named("wet")
		r.fx.Define("burning", effect.Spec{})
		burning = r.fx.Named("burning")
	})
	next := render.SpriteID(10)
	r.fx.Sprites(func() render.SpriteID { next++; return next - 1 })

	a, b := burning.Look(1), burning.Look(2)
	if a != 10 || b != 11 || burning.Look(1) != a {
		t.Errorf("looks of 1, 2, 1 again: %d, %d, %d; want 10, 11, 10", a, b, burning.Look(1))
	}
	var listed int
	r.fx.Looks(func(mark tag.Tag[effect.States], twins map[render.SpriteID]render.SpriteID) {
		listed++
		if mark != burning.Mark() || len(twins) != 2 || twins[1] != a || twins[2] != b {
			t.Errorf("listed %v with %v; want burning's marker with 1→%d, 2→%d", mark, twins, a, b)
		}
	})
	if listed != 1 {
		t.Errorf("%d effects listed with looks, want burning alone", listed)
	}
	if c := burning.Look(3); c != 12 {
		t.Errorf("a further look of burning = %d, want 12", c)
	}
	defer func() {
		if recover() == nil {
			t.Error("the first look of wet after the looks were taken did not panic")
		}
	}()
	wet.Look(1)
}

// An effect that Shows turns Changed on as it begins and as it ends, though it alters nothing:
// whoever draws by its marker draws anew.
func TestEffects_AnEffectThatShowsChangesTheEntityAsItBeginsAndEnds(t *testing.T) {
	var snow effect.Effect
	r := newRig(t, true, func(r *rig) {
		r.fx.Define("snow", effect.Spec{effect.Lasts(2 * tick)})
		snow = r.fx.Named("snow")
		snow.Shows()
	})
	r.cast(snow)
	r.tick()
	if !r.marked(effect.Changed) {
		t.Error("the step it began left Changed off")
	}
	r.tick()
	if r.marked(effect.Changed) {
		t.Error("a step it ran on left Changed on")
	}
	r.tick()
	if !r.marked(effect.Changed) {
		t.Error("the step it ended left Changed off")
	}
}

// Hosts keeps the hosts of the rules of the moments a plugin catches.
func (c *installCtx) Hosts(h ...plugin.Host) { c.hosts = append(c.hosts, h...) }

// Deliver hands rules — a role's, each of its own — to the hosts of their moments, as the engine
// does with the roles played once a Stage's Init returns.
func (c *installCtx) Deliver(rules ...rule.Rule) error { return hosts.Deliver(c.hosts, rules...) }
