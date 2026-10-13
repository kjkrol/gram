package act_test

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/rule/act"
	"github.com/kjkrol/gram/rule/cond"
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
// nobody when alone, and hands back whom each command was issued for; out, given, gets the ids
// before the rule runs.
func nudged(t *testing.T, r act.Rule, alone bool, tick plugin.Tick, out ...*[]uid.UID64) (
	ids []uid.UID64, dodged []control.Issued[dodge], braced []uid.UID64) {
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
	var all []uid.UID64
	ids = runRule(t, h, tick, func(id uid.UID64) nudge {
		i := slices.Index(all, id)
		return nudge{self: id, by: all[1-i], nobody: alone && i == 0}
	}, func(made []uid.UID64) {
		all = made
		for _, o := range out {
			*o = made
		}
	})
	dodges.Drain(func(i control.Issued[dodge]) { dodged = append(dodged, i) })
	braces.Drain(func(i control.Issued[brace]) { braced = append(braced, i.Entity) })
	return ids, dodged, braced
}

// runRule runs the host's rules once over two entities, the first playing the role a and the
// second b, about each as about says; made, given, is told the ids first.
func runRule[P any](t *testing.T, h *plugin.Rules[P], tick plugin.Tick, about func(uid.UID64) P,
	made ...func([]uid.UID64)) (ids []uid.UID64) {
	t.Helper()
	goke.New().Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		var tokens goke.Comp[token]
		var roles goke.Comp[tag.Tags[cond.Roles]]
		f := si.NewFactory(&tokens, &roles)
		f.Create(2)
		f.Next()
		ids = append(ids, f.Cursor.IDs...)
		played := roles.Slice(&f.Cursor)
		played[0], played[1] = played[0].With(Role("a").Tag()), played[1].With(Role("b").Tag())
		for _, m := range made {
			m(ids)
		}
		qb := si.NewQueryBuilder(&tokens)
		h.Bind(qb)
		q := qb.Build()
		for q.All(); q.Next(); {
			cur := q.Cursor()
			h.Run(tick, cur, func(i int) P { return about(cur.IDs[i]) })
		}
	}})
	return ids
}

func TestRule_AimsItsCommandAtTheMomentsSubject(t *testing.T) {
	ids, got, _ := nudged(t, act.Branch[nudge]{
		Name: "dodge",
		When: cond.Self.If(cond.Always),
		Then: act.Self.Do(dodge{}),
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
	ids, dodged, braced := nudged(t, act.Branch[nudge]{
		Name: "dodge or brace",
		When: cond.Self.If(cond.Always),
		Then: act.OneOf(act.Self.Do(dodge{}), act.Self.Do(brace{})),
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
	ids, _, braced := nudged(t, act.Branch[nudge]{
		Name: "brace the other",
		When: cond.Self.If(cond.Always),
		Then: act.Other.Do(brace{}),
	}, true, plugin.Tick{Dt: time.Millisecond})
	if !slices.Equal(braced, ids[:1]) {
		t.Errorf("braced %v; want %v alone: the one nudged by nobody acts for nobody",
			braced, ids[0])
	}
}

func TestRule_OnInsideADoSwitchesWhomTheStepsActFor(t *testing.T) {
	ids, dodged, braced := nudged(t, act.Branch[nudge]{
		Name: "dodge, the other braces",
		When: cond.Self.If(cond.Always),
		Then: act.Steps(
			act.Self.Do(dodge{}),
			act.Branch[any]{When: cond.Self.If(cond.Always), Then: act.Other.Do(brace{})},
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
		if err := h.Add(act.Branch[nudge]{
			Name: "dodge now and then",
			When: cond.Self.If(cond.Chance(p)),
			Then: act.Self.Do(dodge{}),
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
	act.Build(act.Branch[nudge]{
		Name: "here",
		When: cond.Self.If(cond.Always),
		Then: act.Around(0).Do(dodge{}),
	})
}
