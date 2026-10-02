package rule_test

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/uid"
)

// nudge is a made-up host's moment: an entity nudged by another, its Subject, or by nobody.
type nudge struct {
	self, by uid.UID64
	nobody   bool
}

func (n nudge) Who() uid.UID64             { return n.self }
func (n nudge) Subject() (uid.UID64, bool) { return n.by, !n.nobody }

// dodge is a command about whom to dodge, told as it is given.
type dodge struct{ Of uid.UID64 }

func (d *dodge) Aim(who uid.UID64) { d.Of = who }

// A rule aims a command at its moment's Subject: each one nudged gives itself a dodge of the one
// who nudged it.
func TestRule_AimsItsCommandAtTheMomentsSubject(t *testing.T) {
	var carrier control.Carrier
	var dodges control.Queue[dodge]
	if err := carrier.Carry(&dodges); err != nil {
		t.Fatal(err)
	}
	h := &rule.EachHost[nudge]{}
	if err := h.Add(rule.On("dodge", rule.All, func(m *rule.Moment[nudge]) rule.Step { return m.Order(dodge{}) })); err != nil {
		t.Fatal(err)
	}
	var ids []uid.UID64
	ecs := goke.New()
	ecs.Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
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
			h.Run(rule.Tick{Dt: time.Millisecond, Commands: &carrier}, cur, func(i int) nudge {
				return nudge{self: cur.IDs[i], by: cur.IDs[1-i]}
			})
		}
	}})
	var got []control.Issued[dodge]
	dodges.Drain(func(i control.Issued[dodge]) { got = append(got, i) })
	if len(got) != 2 {
		t.Fatalf("dodges given: %+v; want two", got)
	}
	for i, g := range got {
		if g.Entity != ids[i] || !g.ByEntity || g.Command.Of != ids[1-i] {
			t.Errorf("dodge %d: %+v; want by %v, of %v", i, g, ids[i], ids[1-i])
		}
	}
}

// brace is a command about nobody.
type brace struct{}

// An Aimed command fails while the moment names nobody: the one nudged by nobody braces instead
// of dodging; the other dodges.
func TestRule_FailsACommandAimedAtNobody(t *testing.T) {
	var carrier control.Carrier
	var dodges control.Queue[dodge]
	var braces control.Queue[brace]
	if err := carrier.Carry(&dodges, &braces); err != nil {
		t.Fatal(err)
	}
	h := &rule.EachHost[nudge]{}
	if err := h.Add(rule.On("dodge or brace", rule.All, func(m *rule.Moment[nudge]) rule.Step {
		return m.OneOf(m.Order(dodge{}), m.Order(brace{}))
	})); err != nil {
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
			h.Run(rule.Tick{Dt: time.Millisecond, Commands: &carrier}, cur, func(i int) nudge {
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

// token is the made-up host's component its entities carry.
type token struct{ N int }

// Chance draws afresh at every step of the game, from the seed, the step's time and the entity:
// near its likelihood over many steps, the same for the same seed, else not.
func TestRule_ChanceIsTheSameForTheSameSeedAndTime(t *testing.T) {
	const steps, p = 2000, 0.3
	draws := func(seed uint64) []int {
		var carrier control.Carrier
		var dodges control.Queue[dodge]
		if err := carrier.Carry(&dodges); err != nil {
			t.Fatal(err)
		}
		h := &rule.EachHost[nudge]{}
		if err := h.Add(rule.On("dodge now and then", rule.All, func(m *rule.Moment[nudge]) rule.Step {
			return m.Chance(p, m.Order(dodge{}))
		})); err != nil {
			t.Fatal(err)
		}
		var got []int // 2×step + 0 or 1, the one who dodged
		ecs := goke.New()
		ecs.Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
			var tokens goke.Comp[token]
			f := si.NewFactory(&tokens)
			f.Create(2)
			f.Next()
			ids := f.Cursor.IDs
			qb := si.NewQueryBuilder(&tokens)
			h.Bind(qb)
			q := qb.Build()
			for k := range steps {
				tick := rule.Tick{Dt: time.Millisecond, Commands: &carrier, Time: time.Duration(k+1) * time.Millisecond, Seed: seed}
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

// Here and Around need a moment that is Placed: written for another, the rule is refused as it is
// made.
func TestRule_HereNeedsAPlacedMoment(t *testing.T) {
	defer func() {
		if msg, _ := recover().(string); !strings.Contains(msg, "Placed") {
			t.Errorf("panic %q, want one saying the moment must be Placed", msg)
		}
	}()
	rule.On("here", rule.All, func(m *rule.Moment[nudge]) rule.Step { return m.Here(m.Order(dodge{})) })
}
