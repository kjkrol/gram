package rule_test

import (
	"testing"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/uid"
)

// The toy facts.
type (
	alarm struct{}
	poke  struct{}
	mood  struct{ Calm bool }
)

func (m mood) calm() bool { return m.Calm }

// rig is one entity running a tree over a goke world: each tick the test's changes to its facts
// land, the trees run, the clock 100 ms on, and the toys carry out its commands.
type rig struct {
	t     *testing.T
	ecs   *goke.ECS
	now   time.Duration
	id    uid.UID64
	edits []func(cb *goke.CmdBuf)
	ids   map[string]goke.CompID
	toys  *toys
}

func newRig(t *testing.T, plan comp.Template[rule.Mind]) *rig {
	t.Helper()
	r := &rig{t: t, ecs: goke.New(), ids: map[string]goke.CompID{}, toys: newToys()}
	template := plan
	c := rule.New(func() time.Duration { return r.now }, nil, 0, nil, &r.toys.carrier)
	var mind goke.Comp[rule.Mind]
	r.ecs.Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		r.ids["alarm"], r.ids["poke"], r.ids["mood"] = si.RegComp[alarm](), si.RegComp[poke](), si.RegComp[mood]()
		r.toys.init(si)
		r.ids["done"] = r.toys.doneID
		f := si.NewFactory(&mind)
		f.Create(1)
		f.Next()
		r.id = f.Cursor.IDs[0]
		mind.Slice(&f.Cursor)[0] = template.Resolve(nil)
	}})
	edits := r.ecs.RegSys(goke.SystemFn{OnUpdate: func(cb *goke.CmdBuf, _ time.Duration) {
		for _, e := range r.edits {
			e(cb)
		}
		r.edits = r.edits[:0]
	}})
	run := r.ecs.RegSys(c.System())
	toys := r.ecs.RegSys(r.toys.system())
	r.ecs.SetPlan(func(ctx goke.RunCtx, d time.Duration) {
		ctx.Run(edits, d)
		ctx.Sync()
		ctx.Run(run, d)
		ctx.Sync()
		ctx.Run(toys, d)
		ctx.Sync()
		r.now += d
	})
	return r
}

// tick runs n ticks of 100 ms.
func (r *rig) tick(n int) {
	for range n {
		r.ecs.Tick(100 * time.Millisecond)
	}
}

// give puts the fact v, under name, on the entity from the next tick.
func give[F any](r *rig, name string, v F) {
	r.edits = append(r.edits, func(cb *goke.CmdBuf) { cb.AddOne(r.id, r.ids[name], v) })
}

// drop takes the fact name off the entity from the next tick.
func (r *rig) drop(name string) {
	r.edits = append(r.edits, func(cb *goke.CmdBuf) { cb.RemoveCompOne(r.id, r.ids[name]) })
}

// want checks what the entity does, "" nothing, and how many commands it gave so far.
func (r *rig) want(when, doing string, given int) {
	r.t.Helper()
	if got, n := r.toys.doing[r.id], r.toys.given[r.id]; got != doing || n != given {
		r.t.Fatalf("%s: doing %q after %d commands, want %q after %d", when, got, n, doing, given)
	}
}

// Then issues its commands one after another, each once what came of the one before is told —
// Until; a tree done begins again at the next tick.
func TestSteps_OrdersItsCommandsOneAfterAnother(t *testing.T) {
	r := newRig(t, rule.Plan("walk then jump", func(a *rule.Actor) rule.Step {
		return a.Steps(
			a.Order(walk{Far: 3}).Until[done](),
			a.Order(jump{}).Until[done]())
	}))
	r.tick(1)
	r.want("the first tick", "walk", 1)
	r.tick(3)
	r.want("the walk done, told so next tick", "", 1)
	r.tick(1)
	r.want("told the walk done", "jump", 2)
	r.tick(3)
	r.want("told the jump done: the tree is done this tick", "", 2)
	r.tick(1)
	r.want("the tree begun again", "walk", 3)
}

// First is reactive: a branch earlier in its list takes over as soon as it can, and gives way back
// when it can no more, the later one begun again.
func TestOneOf_AnEarlierBranchTakesOverAndGivesBack(t *testing.T) {
	r := newRig(t, rule.Plan("flee from alarms", func(a *rule.Actor) rule.Step {
		return a.OneOf(
			a.When[alarm]("alarmed", func(a *rule.Actor) rule.Step { return a.Order(flee{}).Stay() }),
			a.Order(walk{}).Stay(),
		)
	}))
	r.tick(1)
	r.want("calm", "walk", 1)
	give(r, "alarm", alarm{})
	r.tick(1)
	r.want("alarmed", "flee", 2)
	r.tick(1)
	r.want("still alarmed: given once", "flee", 2)
	r.drop("alarm")
	r.tick(1)
	r.want("the alarm over", "walk", 3)
}

// On holds to what began with a fact that lasted a tick, until it is done.
func TestOn_GoesOnAfterTheFactIsGone(t *testing.T) {
	r := newRig(t, rule.Plan("jump when poked", func(a *rule.Actor) rule.Step {
		return a.OneOf(
			a.On[poke]("poked", func(a *rule.Actor) rule.Step { return a.Order(jump{}).Until[done]() }),
			a.Idle(),
		)
	}))
	give(r, "poke", poke{})
	r.tick(1)
	r.want("poked", "jump", 1)
	r.drop("poke")
	r.tick(1)
	r.want("the poke gone, the jump goes on", "jump", 1)
	r.tick(3)
	r.want("the jump done", "", 1)
}

// If runs its node only while its fact holds as it says.
func TestIf_RunsWhileTheFactHolds(t *testing.T) {
	r := newRig(t, rule.Plan("walk when calm", func(a *rule.Actor) rule.Step {
		return a.OneOf(
			a.If(mood.calm, a.Order(walk{}).Until[done]()),
			a.Idle(),
		)
	}))
	r.tick(1)
	r.want("no mood", "", 0)
	give(r, "mood", mood{Calm: true})
	r.tick(1)
	r.want("calm", "walk", 1)
	give(r, "mood", mood{Calm: false})
	r.tick(5)
	r.want("upset: nothing given again", "", 1)
}

// Until counts a fact given afresh, not one left from before it began; with a condition, the fact
// coming to hold it.
func TestUntil_WaitsForTheFactToCome(t *testing.T) {
	r := newRig(t, rule.Plan("walk, then jump", func(a *rule.Actor) rule.Step {
		return a.Steps(
			a.Order(walk{}).Until[done](), a.Order(jump{}).Until[done]())
	}))
	give(r, "done", done{})
	r.tick(2)
	r.want("told done before the walk", "walk", 1)
	r.drop("done")
	r.tick(3)
	r.want("the walk done", "jump", 2)

	r = newRig(t, rule.Plan("jump once calm", func(a *rule.Actor) rule.Step { return a.Steps(a.Until(mood.calm), a.Order(jump{})) }))
	give(r, "mood", mood{Calm: false})
	r.tick(2)
	r.want("upset", "", 0)
	give(r, "mood", mood{Calm: true})
	r.tick(1)
	r.want("calm", "jump", 1)
}

// Wait, Timeout and Cooldown go by the clock.
func TestWaitTimeoutAndCooldown_GoByTheClock(t *testing.T) {
	r := newRig(t, rule.Plan("pause, then walk", func(a *rule.Actor) rule.Step { return a.Steps(a.Wait(250*time.Millisecond), a.Order(walk{})) }))
	r.tick(3)
	r.want("waiting", "", 0)
	r.tick(1)
	r.want("the wait over", "walk", 1)

	r = newRig(t, rule.Plan("walk at most 150 ms", func(a *rule.Actor) rule.Step {
		return a.OneOf(
			a.Timeout(150*time.Millisecond, a.Order(walk{}).Until[done]()),
			a.Order(rest{}).Stay(),
		)
	}))
	r.tick(2)
	r.want("in time", "walk", 1)
	r.tick(1)
	r.want("out of time: the next branch", "rest", 2)

	r = newRig(t, rule.Plan("jump, then a second off", func(a *rule.Actor) rule.Step {
		return a.OneOf(
			a.Cooldown(time.Second, a.Order(jump{})),
			a.Order(walk{}).Stay(),
		)
	}))
	r.tick(1)
	r.want("jumping", "jump", 1)
	r.tick(1)
	r.want("cooling down", "walk", 2)
	r.tick(9)
	r.want("cooled down", "jump", 3)
	r.tick(1)
	r.want("cooling down again", "walk", 4)
}

// One name is one tree: registered again alike it is taken, unlike it is refused.
func TestPlan_OneNameIsOnePlan(t *testing.T) {
	rule.Plan("the same", func(a *rule.Actor) rule.Step { return a.OneOf(a.Order(walk{})) })
	rule.Plan("the same", func(a *rule.Actor) rule.Step { return a.OneOf(a.Order(walk{})) })
	defer func() {
		if recover() == nil {
			t.Error("a different tree under a known name was taken, want a refusal")
		}
	}()
	rule.Plan("the same", func(a *rule.Actor) rule.Step { return a.OneOf(a.Order(jump{})) })
}

// A command no plugin handles is a mistake of the game's, and panics.
func TestOrder_ACommandNobodyHandlesPanics(t *testing.T) {
	type fly struct{ High bool }
	r := newRig(t, rule.Plan("fly", func(a *rule.Actor) rule.Step { return a.OneOf(a.Order(fly{})) }))
	defer func() {
		if recover() == nil {
			t.Error("a command nobody handles was issued, want a panic")
		}
	}()
	r.tick(1)
}

// Chance draws afresh at every tick from the clock's time: a branch taken about as often as its
// likelihood, the same in a second run.
func TestChance_TakesItsBranchAsOftenAsItsLikelihood(t *testing.T) {
	plan := rule.Plan("jump now and then", func(a *rule.Actor) rule.Step {
		return a.OneOf(a.Chance(0.25, a.Order(jump{})), a.Idle())
	})
	jumps := func() int {
		r := newRig(t, plan)
		r.tick(1000)
		return r.toys.given[r.id]
	}
	n := jumps()
	if n < 200 || n > 300 {
		t.Errorf("jumped %d times in 1000 ticks, want about 250", n)
	}
	if again := jumps(); again != n {
		t.Errorf("a second run jumped %d times, the first %d", again, n)
	}
}
