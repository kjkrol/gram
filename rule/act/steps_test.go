package act_test

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/gram/rule/act"
	"github.com/kjkrol/gram/rule/cond"
	"github.com/kjkrol/gram/rule/effect"
	"github.com/kjkrol/uid"
)

func alone(n nudge) bool { return n.nobody }

func TestBranch_RunsItsStepsOnItsMoment(t *testing.T) {
	ids, dodged, braced := nudged(t, act.Branch[nudge]{
		Name: "dodge or brace",
		When: cond.Self.If(cond.Always),
		Then: act.OneOf(
			act.Branch[nudge]{
				When: cond.Self.If(cond.Not(cond.NewWord("alone", alone))),
				Then: act.Self.Do(dodge{}),
			},
			act.Self.Do(brace{}),
		),
	}, true, plugin.Tick{Dt: time.Millisecond})
	var who []uid.UID64
	for _, d := range dodged {
		who = append(who, d.Entity)
	}
	if !slices.Equal(who, ids[1:]) || !slices.Equal(braced, ids[:1]) {
		t.Errorf("dodged %v, braced %v; want %v dodging, %v bracing", who, braced, ids[1], ids[0])
	}
}

type ask struct{ rule.From }

func (ask) Command() {}

func TestBranch_RefusesWhatItsMomentCannotRun(t *testing.T) {
	for name, tc := range map[string]struct {
		r    act.Rule
		want string
	}{
		"Around on a moment that is not Placed": {act.Branch[nudge]{
			Name: "lost",
			When: cond.Self.If(cond.Always),
			Then: act.Around(1).Do(brace{}),
		}, "Placed"},
		"Here on a moment that is not Placed": {act.Branch[nudge]{
			Name: "lost",
			When: cond.Self.If(cond.Always),
			Then: act.Branch[any]{
				When: cond.Self.If(cond.Always),
				Then: act.Around(0).Do(brace{}),
			},
		}, "Placed"},
		"a condition on another moment": {act.Branch[nudge]{
			Name: "lost",
			When: cond.Self.If(cond.Always),
			Then: act.Branch[token]{
				When: cond.Self.If(cond.NewWord("any", func(token) bool { return true })),
				Then: act.Self.Do(brace{}),
			},
		}, "a word of"},
		"a message": {act.Branch[nudge]{
			Name: "lost",
			When: cond.Self.If(cond.Always),
			Then: act.Other.Do(ask{}),
		}, "sends a message"},
		"no When": {act.Branch[nudge]{
			Name: "lost",
			Then: act.Self.Do(brace{}),
		}, "needs a When"},
		"no Name at the top": {act.Branch[nudge]{
			When: cond.Self.If(cond.Always),
			Then: act.Self.Do(brace{}),
		}, "needs a Name"},
	} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				msg, _ := recover().(string)
				if !strings.Contains(msg, tc.want) {
					t.Errorf("panic %q; want one saying %q", msg, tc.want)
				}
				if tc.want != "needs a Name" && !strings.Contains(msg, `"lost"`) {
					t.Errorf("panic %q; want one naming the rule \"lost\"", msg)
				}
			}()
			act.Build(tc.r)
		})
	}
}

func TestToggle_IsACommandNotAStep(t *testing.T) {
	defer func() {
		if msg, _ := recover().(string); !strings.Contains(msg, "names nobody") {
			t.Errorf("panic %q; want one saying Toggle names nobody", msg)
		}
	}()
	act.Build(act.Branch[nudge]{
		Name: "toggle",
		When: cond.Self.If(cond.Always),
		Then: act.Self.Do(rule.Toggle(effect.Effect{})),
	})
}
