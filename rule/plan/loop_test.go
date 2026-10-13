package plan_test

import (
	"strings"
	"testing"

	"github.com/kjkrol/gram/rule/act"
	"github.com/kjkrol/gram/rule/cond"
	"github.com/kjkrol/gram/rule/plan"
)

func TestLoop_UntilEndsAsTheFactComesAfresh(t *testing.T) {
	r := newRig(t, write("walk till done, then jump", plan.Steps(
		plan.Loop[any]{
			Name:  "walk",
			When:  cond.Self.If(cond.Always),
			Then:  plan.Self.Do(walk{}),
			Until: cond.Self.If(done{}),
		},
		plan.Loop[any]{Name: "jump", When: cond.Self.If(cond.Always), Then: plan.Self.Do(jump{})},
		idle(),
	)))
	give(r, "done", done{})
	r.tick(2)
	r.want("done carried from before the loop", "walk", 1)
	r.tick(3)
	r.want("the walk over, done still carried", "", 1)
	r.drop("done")
	r.tick(1)
	give(r, "done", done{})
	r.tick(1)
	r.want("done come afresh", "jump", 2)
}

func TestLoop_UntilHoldingAtTheStartEndsBeforeTheSteps(t *testing.T) {
	r := newRig(t, write("calm already", plan.Steps(
		plan.Loop[any]{
			Name:  "walk unless calm",
			When:  cond.Self.If(cond.Always),
			Then:  plan.Self.Do(walk{}),
			Until: cond.Self.If(cond.NewWord("calm", mood.calm)),
		},
		plan.Loop[any]{Name: "jump", When: cond.Self.If(cond.Always), Then: plan.Self.Do(jump{})},
		idle(),
	)))
	give(r, "mood", mood{Calm: true})
	r.tick(2)
	r.want("calm from the start: no walk", "jump", 1)
}

func TestLoop_AWordOfAnyFactReadsTheEntity(t *testing.T) {
	r := newRig(t, write("jump while calm", plan.OneOf(
		plan.Loop[any]{
			Name: "jump",
			When: cond.Self.If(cond.NewWord("calm", mood.calm)),
			Then: plan.Self.Do(jump{}),
		},
		idle(),
	)))
	r.tick(2)
	r.want("no mood", "", 0)
	give(r, "mood", mood{Calm: true})
	r.tick(1)
	r.want("calm", "jump", 1)
}

func TestLoop_RefusesWhatAPlanCannotSay(t *testing.T) {
	a := Role("a")
	for name, c := range map[string]struct {
		root plan.Part
		want string
	}{
		"a fact in a When": {plan.Loop[any]{
			Name: "fact",
			When: cond.Self.If(done{}),
			Then: plan.Self.Do(walk{}),
		}, "a loop's type"},
		"an Until of the other": {plan.Loop[any]{
			Name:  "other",
			When:  cond.Self.If(cond.Always),
			Then:  plan.Self.Do(walk{}),
			Until: cond.Other.If(done{}),
		}, "Self alone"},
		"a place": {plan.Loop[any]{
			Name: "place",
			When: cond.Around(1).If(a),
			Then: plan.Self.Do(walk{}),
		}, "a rule's"},
		"a place in a branch": {act.Branch[any]{
			Name: "place",
			When: cond.Self.If(cond.Always),
			Then: act.Around(1).Do(walk{}),
		}, "a rule's"},
	} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				msg, _ := recover().(string)
				if !strings.Contains(msg, c.want) {
					t.Errorf("panic %q; want one saying %q", msg, c.want)
				}
			}()
			plan.New("checked", c.root)
		})
	}
}
