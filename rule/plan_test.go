package rule_test

import (
	"testing"
	"time"

	"github.com/kjkrol/gram/internal/steps"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/uid"
)

type (
	alarm struct{}
	prod  struct{}
	mood  struct{ Calm bool }
)

func (m mood) calm() bool { return m.Calm }

type rig struct {
	t     *testing.T
	ecs   *goke.ECS
	now   time.Duration
	id    uid.UID64
	edits []func(cb *goke.CmdBuf)
	ids   map[string]goke.CompID
	toys  *toys
}

type written struct{ plan rule.Planned }

func write(name string, root rule.Step) written { return written{rule.Plan(name, root)} }

func (w written) mind(c *steps.Plans) steps.Mind {
	w.plan.Define(c)
	return steps.Mind{Plan: steps.PlanID(w.plan.Name())}
}

func newRig(t *testing.T, given written) *rig {
	t.Helper()
	r := &rig{t: t, ecs: goke.New(), ids: map[string]goke.CompID{}, toys: newToys()}
	c := steps.NewPlans(func() time.Duration { return r.now }, nil, 0, nil, &r.toys.carrier)
	follows := given.mind(c)
	var mind goke.Comp[steps.Mind]
	r.ecs.Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		r.ids["alarm"], r.ids["prod"], r.ids["mood"] = si.RegComp[alarm](), si.RegComp[prod](), si.RegComp[mood]()
		r.toys.init(si)
		r.ids["done"] = r.toys.doneID
		f := si.NewFactory(&mind)
		f.Create(1)
		f.Next()
		r.id = f.Cursor.IDs[0]
		mind.Slice(&f.Cursor)[0] = follows
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

func (r *rig) tick(n int) {
	for range n {
		r.ecs.Tick(100 * time.Millisecond)
	}
}

func give[F any](r *rig, name string, v F) {
	r.edits = append(r.edits, func(cb *goke.CmdBuf) { cb.AddOne(r.id, r.ids[name], v) })
}

func (r *rig) drop(name string) {
	r.edits = append(r.edits, func(cb *goke.CmdBuf) { cb.RemoveCompOne(r.id, r.ids[name]) })
}

func (r *rig) want(when, doing string, given int) {
	r.t.Helper()
	if got, n := r.toys.doing[r.id], r.toys.given[r.id]; got != doing || n != given {
		r.t.Fatalf("%s: doing %q after %d commands, want %q after %d", when, got, n, doing, given)
	}
}

func TestSteps_OrdersItsCommandsOneAfterAnother(t *testing.T) {
	r := newRig(t, write("walk then jump", rule.Steps(
		rule.Order(walk{Far: 3}).Until[done](),
		rule.Order(jump{}).Until[done](),
	)))
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

func TestOneOf_AnEarlierBranchTakesOverAndGivesBack(t *testing.T) {
	r := newRig(t, write("flee from alarms", rule.OneOf(
		rule.Branch[alarm]{Name: "alarmed", Do: rule.Order(flee{}).Stay()},
		rule.Order(walk{}).Stay(),
	)))
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

func TestOn_GoesOnAfterTheFactIsGone(t *testing.T) {
	r := newRig(t, write("jump when poked", rule.OneOf(
		rule.Branch[prod]{Name: "poked", Through: true, Do: rule.Order(jump{}).Until[done]()},
		rule.Idle(),
	)))
	give(r, "prod", prod{})
	r.tick(1)
	r.want("poked", "jump", 1)
	r.drop("prod")
	r.tick(1)
	r.want("the prod gone, the jump goes on", "jump", 1)
	r.tick(3)
	r.want("the jump done", "", 1)
}

func TestIf_RunsWhileTheFactHolds(t *testing.T) {
	r := newRig(t, write("walk when calm", rule.OneOf(
		rule.Branch[mood]{When: mood.calm, Do: rule.Order(walk{}).Until[done]()},
		rule.Idle(),
	)))
	r.tick(1)
	r.want("no mood", "", 0)
	give(r, "mood", mood{Calm: true})
	r.tick(1)
	r.want("calm", "walk", 1)
	give(r, "mood", mood{Calm: false})
	r.tick(5)
	r.want("upset: nothing given again", "", 1)
}

func TestUntil_WaitsForTheFactToCome(t *testing.T) {
	r := newRig(t, write("walk, then jump", rule.Steps(
		rule.Order(walk{}).Until[done](),
		rule.Order(jump{}).Until[done](),
	)))
	give(r, "done", done{})
	r.tick(2)
	r.want("told done before the walk", "walk", 1)
	r.drop("done")
	r.tick(3)
	r.want("the walk done", "jump", 2)

	r = newRig(t, write("jump once calm", rule.Steps(rule.Until(mood.calm), rule.Order(jump{}))))
	give(r, "mood", mood{Calm: false})
	r.tick(2)
	r.want("upset", "", 0)
	give(r, "mood", mood{Calm: true})
	r.tick(1)
	r.want("calm", "jump", 1)
}

func TestWaitTimeoutAndCooldown_GoByTheClock(t *testing.T) {
	r := newRig(t, write("pause, then walk",
		rule.Steps(rule.Wait(250*time.Millisecond), rule.Order(walk{}))))
	r.tick(3)
	r.want("waiting", "", 0)
	r.tick(1)
	r.want("the wait over", "walk", 1)

	r = newRig(t, write("walk at most 150 ms", rule.OneOf(
		rule.Timeout(150*time.Millisecond, rule.Order(walk{}).Until[done]()),
		rule.Order(rest{}).Stay(),
	)))
	r.tick(2)
	r.want("in time", "walk", 1)
	r.tick(1)
	r.want("out of time: the next branch", "rest", 2)

	r = newRig(t, write("jump, then a second off", rule.OneOf(
		rule.Cooldown(time.Second, rule.Order(jump{})),
		rule.Order(walk{}).Stay(),
	)))
	r.tick(1)
	r.want("jumping", "jump", 1)
	r.tick(1)
	r.want("cooling down", "walk", 2)
	r.tick(9)
	r.want("cooled down", "jump", 3)
	r.tick(1)
	r.want("cooling down again", "walk", 4)
}

func TestPlan_OneNameIsOnePlanAWorld(t *testing.T) {
	walking := write("the same", rule.OneOf(rule.Order(walk{})))
	jumping := write("the same", rule.OneOf(rule.Order(jump{})))
	mine, theirs := steps.NewPlans(nil, nil, 0, nil, nil), steps.NewPlans(nil, nil, 0, nil, nil)
	if a, b := walking.mind(mine), jumping.mind(theirs); a != b {
		t.Errorf("two worlds know the plan of one name as %v and %v, want the name hashed alike", a, b)
	}
	walking.mind(mine) // the same plan again: known already
	defer func() {
		if recover() == nil {
			t.Error("a name defined twice among one world's plans was taken, want a refusal")
		}
	}()
	jumping.mind(mine)
}

type fly struct{ High bool }

func (fly) Command() {}

func TestOrder_ACommandNobodyHandlesPanics(t *testing.T) {
	r := newRig(t, write("fly", rule.OneOf(rule.Order(fly{}))))
	defer func() {
		if recover() == nil {
			t.Error("a command nobody handles was issued, want a panic")
		}
	}()
	r.tick(1)
}

func TestEither_TakesEachSideAboutHalfTheTime(t *testing.T) {
	plan := write("jump or walk", rule.OneOf(rule.Either(rule.Order(jump{}), rule.Order(walk{}))))
	jumps := func() (jumps, walks int) {
		r := newRig(t, plan)
		for range 1000 {
			r.tick(1)
			switch r.toys.last[r.id] {
			case "jump":
				jumps++
			case "walk":
				walks++
			}
		}
		return jumps, walks
	}
	j, w := jumps()
	if j < 400 || j > 600 || w < 400 || w > 600 {
		t.Errorf("in 1000 ticks jumped %d times and walked %d, want about 500 each", j, w)
	}
	if again, _ := jumps(); again != j {
		t.Errorf("a second run jumped %d times, the first %d", again, j)
	}
}

func TestChance_TakesItsBranchAsOftenAsItsLikelihood(t *testing.T) {
	plan := write("jump now and then",
		rule.OneOf(rule.Chance(0.25, rule.Order(jump{})), rule.Idle()))
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
