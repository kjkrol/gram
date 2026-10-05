package rule_test

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/uid"
)

// alone holds for a nudge by nobody.
func alone(n nudge) bool { return n.nobody }

// A rule written with Then and the package's steps runs as one written with On: the one nudged
// by nobody braces, the other, If(Not(alone)), dodges.
func TestThen_RunsThePackagesStepsOnItsMoment(t *testing.T) {
	var carrier control.Carrier
	var dodges control.Queue[dodge]
	var braces control.Queue[brace]
	if err := carrier.Carry(&dodges, &braces); err != nil {
		t.Fatal(err)
	}
	h := &plugin.Rules[nudge]{}
	if err := h.Add(rule.Then[nudge]("dodge or brace", rule.All, rule.OneOf(
		rule.If(rule.Not(alone), rule.Order(dodge{})),
		rule.Order(brace{}),
	))); err != nil {
		t.Fatal(err)
	}
	var ids []uid.UID64
	goke.New().Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		var tokens goke.Comp[token]
		f := si.NewFactory(&tokens)
		f.Create(2)
		f.Next()
		ids = append(ids, f.Cursor.IDs...)
		qb := si.NewQueryBuilder(&tokens)
		h.Bind(qb)
		q := qb.Build()
		for q.All(); q.Next(); {
			cur := q.Cursor()
			h.Run(plugin.Tick{Dt: time.Millisecond, Commands: &carrier}, cur, func(i int) nudge {
				return nudge{self: cur.IDs[i], by: cur.IDs[1-i], nobody: i == 0}
			})
		}
	}})
	var dodged, braced []uid.UID64
	dodges.Drain(func(i control.Issued[dodge]) { dodged = append(dodged, i.Entity) })
	braces.Drain(func(i control.Issued[brace]) { braced = append(braced, i.Entity) })
	if !slices.Equal(dodged, ids[1:]) || !slices.Equal(braced, ids[:1]) {
		t.Errorf("dodged %v, braced %v; want %v dodging, %v bracing", dodged, braced, ids[1], ids[0])
	}
}

// A step the rule's moment cannot run is refused as the rule is made, by the rule's name.
func TestThen_RefusesAStepItsMomentCannotRun(t *testing.T) {
	for name, tc := range map[string]struct {
		step rule.Step
		want string
	}{
		"Around on a moment that is not Placed": {rule.Around(1, rule.Order(brace{})), "Placed"},
		"Here on a moment that is not Placed":   {rule.Here(rule.Order(brace{})), "Placed"},
		"If over another moment":                {rule.If(func(token) bool { return true }, rule.Order(brace{})), "another moment"},
	} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				msg, _ := recover().(string)
				if !strings.Contains(msg, `"lost"`) || !strings.Contains(msg, tc.want) {
					t.Errorf("panic %q; want one naming the rule \"lost\" and %q", msg, tc.want)
				}
			}()
			rule.Then[nudge]("lost", rule.All, tc.step)
		})
	}
}
