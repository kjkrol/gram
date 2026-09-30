package effect_test

import (
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/act"
	"github.com/kjkrol/gram/plugins/world/act/effect"
	"github.com/kjkrol/gram/plugins/world/entity/kind"
	"github.com/kjkrol/gram/plugins/world/entity/kind/comp"
	"github.com/kjkrol/gram/plugins/world/entity/tag"
	"github.com/kjkrol/gram/plugins/world/steering"
	"github.com/kjkrol/uid"
)

// installCtx is the plugin.Installer a Stage would hand over, minus the engine.
type installCtx struct {
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
	active  goke.OptComp[effect.Active]
	states  goke.OptComp[tag.Tags[effect.States]]
	casting func(cb *goke.CmdBuf)
	idled   []uid.UID64 // whom the hosted Idling behavior heard of, in order
}

// newRig builds the rig; define adds effects before Install and may read the rig's tags.
func newRig(t *testing.T, withFamily bool, define func(r *rig)) *rig {
	t.Helper()
	r := &rig{t: t}
	r.w = world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 400, Height: 400},
		Entities: world.EntitiesCfg{MaxCount: 4, MinSize: 10, MaxSize: 10},
	})
	r.angry = r.w.Kinds().DefineTag[moods]("angry")
	r.fx = r.w.Effects()
	if err := r.w.Hook(act.Trigger[effect.Idling]("idled").RunsOn(func(_ plugin.Tick, _ *steering.Steering, i effect.Idling) {
		r.idled = append(r.idled, i.ID)
	})); err != nil {
		t.Fatal(err)
	}
	define(r)

	ctx := &installCtx{ecs: goke.New()}
	if err := r.w.Install(ctx); err != nil {
		t.Fatal(err)
	}
	spec := kind.Spec{
		comp.Const(world.Position{AABB: plane.NewAABB(geom.NewVec(100, 100), 10, 10)}),
		comp.Const(world.Velocity{}),
		comp.Const(steering.Steering{MaxSpeed: 10}),
	}
	if withFamily {
		spec = append(spec, comp.Tagged[moods]())
	}
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
		r.query = si.NewQueryBuilder(&r.base, &r.steer, &r.look).Optional(&r.marks, &r.active, &r.states).Build()
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

// state reads the entity back: its speed, sprite, tags and whether it is under any effect.
// state is the entity's speed, sprite, whether it is angry, and whether an effect is on it — its
// Active stays once it came, empty when none is.
func (r *rig) state() (speed float64, sprite uint8, angry bool, active bool) {
	for r.query.All(); r.query.Next(); {
		cur := r.query.Cursor()
		speed = r.steer.Slice(cur)[0].MaxSpeed
		sprite = uint8(r.look.Slice(cur)[0].SpriteID)
		if m := r.marks.Slice(cur); m != nil {
			angry = m[0].Has(r.angry)
		}
		if a := r.active.Slice(cur); a != nil {
			for _, s := range a[0].Slots {
				active = active || s.State != effect.Empty
			}
		}
	}
	return
}

// idleMarked reports whether the entity's Idle is on right now.
func (r *rig) idleMarked() bool {
	marked := false
	for r.query.All(); r.query.Next(); {
		m := r.states.Slice(r.query.Cursor())
		marked = m != nil && m[0].Has(effect.Idle)
	}
	return marked
}

func TestEffects_GrantAndAlterHoldForLastsThenRevert(t *testing.T) {
	var rage effect.Effect
	r := newRig(t, true, func(r *rig) {
		rage = r.fx.Define("rage", effect.Spec{
			effect.Lasts(3 * tick),
			effect.Grant(r.angry),
			effect.Alter(func(s *steering.Steering) { s.MaxSpeed *= 2 }),
			effect.Alter(func(a *world.Appearance) { a.SpriteID = 7 }),
		})
	})
	r.cast(rage)
	r.tick() // cast lands and the slot begins, the effects pass running after the cast
	r.tick()
	if speed, sprite, angry, active := r.state(); speed != 20 || sprite != 7 || !angry || !active {
		t.Fatalf("running: speed %v sprite %d angry %v active %v, want 20, 7, true, true", speed, sprite, angry, active)
	}
	for range 3 {
		r.tick()
	}
	if speed, sprite, angry, active := r.state(); speed != 10 || sprite != 0 || angry || active {
		t.Errorf("after its time: speed %v sprite %d angry %v active %v, want 10, 0, false, false", speed, sprite, angry, active)
	}
	if len(r.idled) != 1 || r.idled[0] != r.id {
		t.Errorf("the Idling behavior heard %v, want the entity once", r.idled)
	}
}

func TestEffects_IdleMarksTheEntityForOneTickAfterItsLastEffect(t *testing.T) {
	var blink effect.Effect
	r := newRig(t, false, func(r *rig) {
		blink = r.fx.Define("blink", effect.Spec{effect.Lasts(tick), effect.Alter(func(a *world.Appearance) { a.SpriteID = 3 })})
	})
	r.cast(blink)
	r.tick() // lands and begins
	if r.idleMarked() {
		t.Fatal("Idle while the effect runs")
	}
	r.tick() // its time is up: Active empties, Idle goes on
	if _, _, _, active := r.state(); active || !r.idleMarked() {
		t.Fatalf("after the effect: under an effect %v, idle %v; want none and Idle on", active, r.idleMarked())
	}
	if len(r.idled) != 0 {
		t.Errorf("heard %v before the Idle pass, want nothing yet", r.idled)
	}
	r.tick() // the Idle pass: behaviors hear, the mark goes
	if r.idleMarked() {
		t.Error("Idle still on after its tick")
	}
	if len(r.idled) != 1 {
		t.Errorf("heard %v, want the entity once", r.idled)
	}
	r.tick()
	if len(r.idled) != 1 {
		t.Errorf("heard %v after another tick, want still once", r.idled)
	}
}

// altered reads the entity's Active.Altered; false without an Active.
func (r *rig) altered() bool {
	for r.query.All(); r.query.Next(); {
		if a := r.active.Slice(r.query.Cursor()); a != nil {
			return a[0].Altered
		}
	}
	return false
}

func TestEffects_AlteredMarksThePassThatRewroteAComponent(t *testing.T) {
	var haste, mark effect.Effect
	r := newRig(t, true, func(r *rig) {
		haste = r.fx.Define("haste", effect.Spec{effect.Lasts(4 * tick), effect.Alter(func(s *steering.Steering) { s.MaxSpeed *= 2 })})
		mark = r.fx.Define("mark", effect.Spec{effect.Lasts(4 * tick), effect.Grant(r.angry)})
	})
	r.cast(haste)
	r.tick() // lands and begins: the speed is rewritten
	if !r.altered() {
		t.Error("the pass that began an Alter left Altered off")
	}
	r.tick() // runs on, nothing rewritten
	if r.altered() {
		t.Error("a pass that rewrote nothing left Altered on")
	}
	r.cast(mark)
	r.tick() // a Grant begins beside it: tags, no Alter
	if r.altered() {
		t.Error("a Grant beginning turned Altered on")
	}
}

func TestEffects_TwoAltersOfOneComponentComposeAndEndApart(t *testing.T) {
	var haste, slow effect.Effect
	r := newRig(t, true, func(r *rig) {
		haste = r.fx.Define("haste", effect.Spec{effect.Lasts(5 * tick), effect.Alter(func(s *steering.Steering) { s.MaxSpeed *= 2 })})
		slow = r.fx.Define("slow", effect.Spec{effect.Lasts(2 * tick), effect.Alter(func(s *steering.Steering) { s.MaxSpeed *= 0.5 })})
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
		short = r.fx.Define("short", effect.Spec{effect.Lasts(2 * tick), effect.Grant(r.angry)})
		stacks = r.fx.Define("stacks", effect.Spec{effect.Lasts(2 * tick), effect.Stacking(), effect.Alter(func(s *steering.Steering) { s.MaxSpeed++ })})
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
		curse = r.fx.Define("curse", effect.Spec{effect.Grant(r.angry)})
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
		rage = r.fx.Define("rage", effect.Spec{effect.Lasts(2 * tick), effect.Grant(r.angry)})
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
