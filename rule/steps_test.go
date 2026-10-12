package rule_test

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/gram/rule/effect"
	"github.com/kjkrol/uid"
)

func alone(n nudge) bool { return n.nobody }

func TestBranch_RunsItsStepsOnItsMoment(t *testing.T) {
	ids, dodged, braced := nudged(t, rule.Branch[nudge]{
		Name: "dodge or brace",
		On:   rule.Self,
		Do: rule.OneOf(
			rule.Branch[nudge]{When: rule.Not(alone), Do: rule.Apply(dodge{})},
			rule.Apply(brace{}),
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
		r    rule.Rule
		want string
	}{
		"Around on a moment that is not Placed": {rule.Branch[nudge]{
			Name: "lost",
			On:   rule.Around(1),
			Do:   rule.Apply(brace{}),
		}, "Placed"},
		"Here on a moment that is not Placed": {rule.Branch[nudge]{
			Name: "lost",
			On:   rule.Self,
			Do:   rule.Branch[any]{On: rule.Here, Do: rule.Apply(brace{})},
		}, "Placed"},
		"a condition on another moment": {rule.Branch[nudge]{
			Name: "lost",
			On:   rule.Self,
			Do: rule.Branch[token]{
				When: func(token) bool { return true },
				Do:   rule.Apply(brace{}),
			},
		}, "another moment"},
		"a step that lasts": {rule.Branch[nudge]{
			Name: "lost",
			On:   rule.Self,
			Do:   rule.Steps(rule.Apply(brace{}), rule.Wait(time.Second)),
		}, "lasts over ticks"},
		"a message": {rule.Branch[nudge]{
			Name: "lost",
			On:   rule.Other,
			Do:   rule.Apply(ask{}),
		}, "sends a message"},
		"no On at the top": {rule.Branch[nudge]{
			Name: "lost",
			Do:   rule.Apply(brace{}),
		}, "needs an On"},
		"no Name at the top": {rule.Branch[nudge]{
			On: rule.Self,
			Do: rule.Apply(brace{}),
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
			rule.Build(tc.r)
		})
	}
}

func TestToggle_IsACommandNotAStep(t *testing.T) {
	defer func() {
		if msg, _ := recover().(string); !strings.Contains(msg, "not a step") {
			t.Errorf("panic %q; want one saying Toggle is a command, not a step", msg)
		}
	}()
	rule.Build(rule.Branch[nudge]{Name: "toggle", On: rule.Self, Do: rule.Toggle(effect.Effect{})})
}
