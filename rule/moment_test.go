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

type nudge struct {
	self, by uid.UID64
	nobody   bool
}

func (n nudge) Who() uid.UID64             { return n.self }
func (n nudge) Subject() (uid.UID64, bool) { return n.by, !n.nobody }

type dodge struct{ Of uid.UID64 }

func (dodge) Command() {}

func (d *dodge) Aim(who uid.UID64) { d.Of = who }

type brace struct{}

func (brace) Command() {}

type token struct{ N int }

// nudged runs the rule once for each of two entities, each nudged by the other, the first by
// nobody when alone, and hands back whom each command was issued for.
func nudged(t *testing.T, r rule.Rule, alone bool, tick plugin.Tick) (ids []uid.UID64,
	dodged []control.Issued[dodge], braced []uid.UID64) {
	t.Helper()
	var carrier control.Carrier
	var dodges control.Queue[dodge]
	var braces control.Queue[brace]
	if err := carrier.Carry(&dodges, &braces); err != nil {
		t.Fatal(err)
	}
	h := &plugin.Rules[nudge]{}
	if err := h.Add(r); err != nil {
		t.Fatal(err)
	}
	tick.Commands = &carrier
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
			h.Run(tick, cur, func(i int) nudge {
				return nudge{self: cur.IDs[i], by: cur.IDs[1-i], nobody: alone && i == 0}
			})
		}
	}})
	dodges.Drain(func(i control.Issued[dodge]) { dodged = append(dodged, i) })
	braces.Drain(func(i control.Issued[brace]) { braced = append(braced, i.Entity) })
	return ids, dodged, braced
}

func TestRule_AimsItsCommandAtTheMomentsSubject(t *testing.T) {
	ids, got, _ := nudged(t, rule.Branch[nudge]{
		Name: "dodge",
		On:   rule.Self,
		Do:   rule.Apply(dodge{}),
	}, false, plugin.Tick{Dt: time.Millisecond})
	if len(got) != 2 {
		t.Fatalf("dodges given: %+v; want two", got)
	}
	for i, g := range got {
		if g.Entity != ids[i] || !g.ByEntity || g.Command.Of != ids[1-i] {
			t.Errorf("dodge %d: %+v; want by %v, of %v", i, g, ids[i], ids[1-i])
		}
	}
}

func TestRule_FailsACommandAimedAtNobody(t *testing.T) {
	ids, dodged, braced := nudged(t, rule.Branch[nudge]{
		Name: "dodge or brace",
		On:   rule.Self,
		Do:   rule.OneOf(rule.Apply(dodge{}), rule.Apply(brace{})),
	}, true, plugin.Tick{Dt: time.Millisecond})
	var who []uid.UID64
	for _, d := range dodged {
		who = append(who, d.Entity)
	}
	if !slices.Equal(who, ids[1:]) || !slices.Equal(braced, ids[:1]) {
		t.Errorf("dodged %v, braced %v; want %v dodging, %v bracing", who, braced, ids[1], ids[0])
	}
}

func TestRule_OnOtherActsForTheMomentsSubject(t *testing.T) {
	ids, _, braced := nudged(t, rule.Branch[nudge]{
		Name: "brace the other",
		On:   rule.Other,
		Do:   rule.Apply(brace{}),
	}, true, plugin.Tick{Dt: time.Millisecond})
	if !slices.Equal(braced, ids[:1]) {
		t.Errorf("braced %v; want %v alone: the one nudged by nobody acts for nobody",
			braced, ids[0])
	}
}

func TestRule_OnInsideADoSwitchesWhomTheStepsActFor(t *testing.T) {
	ids, dodged, braced := nudged(t, rule.Branch[nudge]{
		Name: "dodge, the other braces",
		On:   rule.Self,
		Do: rule.Steps(
			rule.Apply(dodge{}),
			rule.Branch[any]{On: rule.Other, Do: rule.Apply(brace{})},
		),
	}, false, plugin.Tick{Dt: time.Millisecond})
	var who []uid.UID64
	for _, d := range dodged {
		who = append(who, d.Entity)
	}
	slices.Sort(braced)
	if !slices.Equal(who, ids) || !slices.Equal(braced, ids) {
		t.Errorf("dodged %v, braced %v; want both dodging and both braced by the other",
			who, braced)
	}
}

func TestRule_ChanceIsTheSameForTheSameSeedAndTime(t *testing.T) {
	const steps, p = 2000, 0.3
	draws := func(seed uint64) []int {
		var carrier control.Carrier
		var dodges control.Queue[dodge]
		if err := carrier.Carry(&dodges); err != nil {
			t.Fatal(err)
		}
		h := &plugin.Rules[nudge]{}
		if err := h.Add(rule.Branch[nudge]{
			Name: "dodge now and then",
			When: rule.Chance(p),
			On:   rule.Self,
			Do:   rule.Apply(dodge{}),
		}); err != nil {
			t.Fatal(err)
		}
		var got []int // 2×step + 0 or 1, the one who dodged
		goke.New().Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
			var tokens goke.Comp[token]
			f := si.NewFactory(&tokens)
			f.Create(2)
			f.Next()
			ids := f.Cursor.IDs
			qb := si.NewQueryBuilder(&tokens)
			h.Bind(qb)
			q := qb.Build()
			for k := range steps {
				tick := plugin.Tick{Dt: time.Millisecond, Commands: &carrier,
					Time: time.Duration(k+1) * time.Millisecond, Seed: seed}
				for q.All(); q.Next(); {
					cur := q.Cursor()
					h.Run(tick, cur, func(i int) nudge { return nudge{self: cur.IDs[i], by: cur.IDs[1-i]} })
				}
				dodges.Drain(func(i control.Issued[dodge]) {
					who := 0
					if i.Entity == ids[1] {
						who = 1
					}
					got = append(got, 2*k+who)
				})
			}
		}})
		return got
	}
	a, b, other := draws(7), draws(7), draws(8)
	if n := len(a); n < 2*steps*p*0.85 || n > 2*steps*p*1.15 {
		t.Errorf("%d dodges in %d draws, want about %v", n, 2*steps, 2*steps*p)
	}
	if !slices.Equal(a, b) {
		t.Error("the same seed and times drew differently")
	}
	if slices.Equal(a, other) {
		t.Error("another seed drew the same: not drawn from the seed")
	}
}

func TestRule_HereNeedsAPlacedMoment(t *testing.T) {
	defer func() {
		if msg, _ := recover().(string); !strings.Contains(msg, "Placed") {
			t.Errorf("panic %q, want one saying the moment must be Placed", msg)
		}
	}()
	rule.Build(rule.Branch[nudge]{Name: "here", On: rule.Here, Do: rule.Apply(dodge{})})
}
