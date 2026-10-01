package rule_test

import (
	"testing"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugin/host"
	"github.com/kjkrol/gram/plugins/world/rule"
	"github.com/kjkrol/uid"
)

// nudge is a made-up host's moment: an entity nudged by another, its Subject.
type nudge struct{ self, by uid.UID64 }

func (n nudge) Who() uid.UID64             { return n.self }
func (n nudge) Subject() (uid.UID64, bool) { return n.by, true }

// dodge is a command about whom to dodge, told as it is given.
type dodge struct{ Of uid.UID64 }

func (d *dodge) Aim(who uid.UID64) { d.Of = who }

// A rule aims a command at its moment's Subject: the one nudged gives itself a dodge of the
// one who nudged it.
func TestRule_AimsItsCommandAtTheMomentsSubject(t *testing.T) {
	var carrier control.Carrier
	var dodges control.Queue[dodge]
	if err := carrier.Carry(&dodges); err != nil {
		t.Fatal(err)
	}
	h := &host.EachHost[nudge]{}
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
			h.RunRows(plugin.Tick{Dt: time.Millisecond, Commands: &carrier}, cur, []int{0}, func(i int) nudge {
				return nudge{self: cur.IDs[i], by: cur.IDs[1-i]}
			})
		}
	}})
	var got []control.Issued[dodge]
	dodges.Drain(func(i control.Issued[dodge]) { got = append(got, i) })
	if len(got) != 1 || got[0].Entity != ids[0] || !got[0].ByEntity || got[0].Command.Of != ids[1] {
		t.Errorf("dodges given: %+v; want one, by %v, of %v", got, ids[0], ids[1])
	}
}

// token is the made-up host's component its entities carry.
type token struct{ N int }
