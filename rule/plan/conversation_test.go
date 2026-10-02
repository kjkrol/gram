package plan_test

import (
	"testing"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/gram/rule/plan"
	"github.com/kjkrol/uid"
)

// pass is the ask of the conversation tests: let me pass.
type pass struct{}

// need is the fact of wanting past Who; room of what stands beside: free, or an ally to ask on.
type (
	need struct{ Who uid.UID64 }
	room struct {
		Free, Beside bool
		Ally         uid.UID64
	}
)

func (n need) Subject() (uid.UID64, bool) { return n.Who, true }
func (r room) Subject() (uid.UID64, bool) { return r.Ally, r.Beside }
func (r room) free() bool                 { return r.Free }
func (r room) byAlly() bool               { return !r.Free && r.Beside }

// asker walks on once let pass, and flees when not.
var asker = plan.New("asker", func(a *plan.Actor) rule.Step {
	return a.OneOf(
		a.When[need]("needs to pass", func(a *plan.Actor) rule.Step {
			return a.Ask[pass]("let me pass", 500*time.Millisecond,
				a.Order(walk{}).Stay(),
				a.Order(flee{}).Stay())
		}),
		a.Idle(),
	)
})

// giver lets pass where it has room — it jumps aside — asks its ally on where it has one, and
// refuses otherwise.
var giver = plan.New("giver", func(a *plan.Actor) rule.Step {
	return a.OneOf(
		a.On[plan.Asked[pass]]("asked to let pass", func(a *plan.Actor) rule.Step {
			return a.OneOf(
				a.If(room.free, a.Steps(a.Agree[pass](), a.Order(jump{High: true}))),
				a.If(room.byAlly, a.Relay[pass]()),
				a.Refuse[pass](),
			)
		}),
		a.Idle(),
	)
})

// talk is entities each running a tree, their facts set by the test between ticks.
type talk struct {
	t      *testing.T
	ecs    *goke.ECS
	now    time.Duration
	ids    []uid.UID64
	edits  []func(cb *goke.CmdBuf)
	needID goke.CompID
	roomID goke.CompID
	look   *goke.Query
	asked  goke.OptComp[plan.Asked[pass]]
	toys   *toys
}

// body is what an entity with no mind is made of.
type body struct{ Size int }

// newTalk spawns an entity for each plan; nil spawns one without a mind.
func newTalk(t *testing.T, plans ...any) *talk {
	t.Helper()
	k := &talk{t: t, ecs: goke.New(), toys: newToys()}
	var minds []plan.Mind
	for _, p := range plans {
		if p == nil {
			minds = append(minds, plan.Mind{})
			continue
		}
		minds = append(minds, p.(comp.Template[plan.Mind]).Resolve(nil))
	}
	c := plan.NewPlans(func() time.Duration { return k.now }, nil, 0, nil, &k.toys.carrier)
	var mind goke.Comp[plan.Mind]
	var plain goke.Comp[body]
	k.ecs.Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		k.needID, k.roomID = si.RegComp[need](), si.RegComp[room]()
		k.toys.init(si)
		withMind, without := si.NewFactory(&mind), si.NewFactory(&plain)
		for _, m := range minds {
			f := withMind
			if m.Plan == 0 {
				f = without
			}
			f.Create(1)
			f.Next()
			k.ids = append(k.ids, f.Cursor.IDs[0])
			if m.Plan != 0 {
				mind.Slice(&f.Cursor)[0] = m
			}
		}
		k.look = si.NewQueryBuilder().Optional(&k.asked).Build()
	}})
	edits := k.ecs.RegSys(goke.SystemFn{OnUpdate: func(cb *goke.CmdBuf, _ time.Duration) {
		for _, e := range k.edits {
			e(cb)
		}
		k.edits = k.edits[:0]
	}})
	run := k.ecs.RegSys(c.System())
	toys := k.ecs.RegSys(k.toys.system())
	k.ecs.SetPlan(func(ctx goke.RunCtx, d time.Duration) {
		ctx.Run(edits, d)
		ctx.Sync()
		ctx.Run(run, d)
		ctx.Sync()
		ctx.Run(toys, d)
		ctx.Sync()
		k.now += d
	})
	return k
}

func (k *talk) tick(n int) {
	for range n {
		k.ecs.Tick(100 * time.Millisecond)
	}
}

func (k *talk) need(who, of int) {
	id := k.ids[who]
	v := need{Who: k.ids[of]}
	k.edits = append(k.edits, func(cb *goke.CmdBuf) { cb.AddOne(id, k.needID, v) })
}

// room gives who room beside, free, or taken by the ally of index ally (-1 none).
func (k *talk) room(who int, free bool, ally int) {
	id := k.ids[who]
	v := room{Free: free}
	if ally >= 0 {
		v.Ally, v.Beside = k.ids[ally], true
	}
	k.edits = append(k.edits, func(cb *goke.CmdBuf) { cb.AddOne(id, k.roomID, v) })
}

// is reports what entity who carries — an ask — and what it did last: a walk, a flight, a jump.
func (k *talk) is(who int) (asked, walking, fleeing, jumping bool) {
	id := k.ids[who]
	if !k.look.Seek(id) {
		k.t.Fatal("entity gone")
	}
	doing := k.toys.last[id]
	return k.asked.Present(k.look.Cursor()), doing == "walk", doing == "flee", doing == "jump"
}

// An ask answered yes runs the asker's agreed branch; the one asked did what its tree says.
func TestAsk_AYesRunsTheAgreedBranch(t *testing.T) {
	k := newTalk(t, asker, giver)
	k.room(1, true, -1)
	k.need(0, 1)
	k.tick(3)
	if _, walking, fleeing, _ := k.is(0); !walking || fleeing {
		t.Errorf("the asker walks %v, flees %v; want let pass: walking", walking, fleeing)
	}
	if asked, _, _, jumping := k.is(1); asked || !jumping {
		t.Errorf("the one asked still has the ask %v, jumps aside %v; want answered, jumping", asked, jumping)
	}
}

// An ask answered no, or not in time, or to one with no mind, runs the refused branch.
func TestAsk_ANoOrNoAnswerRunsTheRefusedBranch(t *testing.T) {
	k := newTalk(t, asker, giver) // no room: refused
	k.room(1, false, -1)
	k.need(0, 1)
	k.tick(3)
	if _, walking, fleeing, _ := k.is(0); walking || !fleeing {
		t.Errorf("refused, the asker walks %v, flees %v; want fleeing", walking, fleeing)
	}

	k = newTalk(t, asker, plan.New("deaf", func(a *plan.Actor) rule.Step { return a.Idle() })) // never answers
	k.need(0, 1)
	k.tick(4)
	if _, _, fleeing, _ := k.is(0); fleeing {
		t.Error("the asker gave up before its wait was over")
	}
	k.tick(3)
	if _, _, fleeing, _ := k.is(0); !fleeing {
		t.Error("no answer in time, and the asker does not flee")
	}

	k = newTalk(t, asker, nil) // no mind to ask
	k.need(0, 1)
	k.tick(2)
	if asked, _, _, _ := k.is(1); asked {
		t.Error("an entity with no mind was given an ask")
	}
	if _, _, fleeing, _ := k.is(0); !fleeing {
		t.Error("asking one with no mind, the asker does not flee at once")
	}
}

// An ask passed on keeps the asker waiting past its wait; the one asked on makes way, then the
// first one can too, and the asker walks on.
func TestRelay_PassesTheAskOnAndTheAskerWaits(t *testing.T) {
	k := newTalk(t, asker, giver, giver)
	k.room(1, false, 2)
	k.room(2, true, -1)
	k.need(0, 1)
	k.tick(2)
	if asked, _, _, _ := k.is(2); !asked {
		t.Fatal("the ally beside was not asked on")
	}
	k.tick(2)
	if _, _, _, jumping := k.is(2); !jumping {
		t.Error("the ally asked on does not make way")
	}
	k.tick(4) // past the asker's wait: it waits on, the ask passed on
	if _, walking, fleeing, _ := k.is(0); walking || fleeing {
		t.Errorf("the asker walks %v, flees %v while its ask is passed on; want waiting", walking, fleeing)
	}
	k.room(1, true, -1) // room beside now: the first one makes way and says yes
	k.tick(3)
	if _, walking, _, _ := k.is(0); !walking {
		t.Error("the way made along the chain, the asker does not walk on")
	}
}

// A no from the one asked on comes back along the chain to the asker.
func TestRelay_ANoComesBackAlongTheChain(t *testing.T) {
	k := newTalk(t, asker, giver, giver)
	k.room(1, false, 2)
	k.room(2, false, -1)
	k.need(0, 1)
	k.tick(5)
	if _, _, fleeing, _ := k.is(0); !fleeing {
		t.Error("refused further on, the asker does not flee")
	}
}

// A chain ends at MaxChain, and never comes back to one on it: the last one refuses, and the no
// comes back.
func TestRelay_TheChainEndsAtItsLimitAndNeverLoops(t *testing.T) {
	n := plan.MaxChain + 3
	plans := []any{asker}
	for range n {
		plans = append(plans, giver)
	}
	k := newTalk(t, plans...)
	for i := 1; i < n; i++ {
		k.room(i, false, i+1)
	}
	k.room(n, false, -1)
	k.need(0, 1)
	furthest := 0
	for range 3 * n {
		k.tick(1)
		for i := 1; i <= n; i++ {
			if asked, _, _, _ := k.is(i); asked && i > furthest {
				furthest = i
			}
		}
	}
	if furthest != plan.MaxChain+1 {
		t.Errorf("the ask got as far as entity %d, want %d: past the asker, %d links", furthest, plan.MaxChain+1, plan.MaxChain)
	}
	if _, _, fleeing, _ := k.is(0); !fleeing {
		t.Error("the chain ended, and the asker does not flee")
	}

	k = newTalk(t, asker, giver, giver) // round: the second one's ally is the first
	k.room(1, false, 2)
	k.room(2, false, 1)
	k.need(0, 1)
	k.tick(6)
	if _, _, fleeing, _ := k.is(0); !fleeing {
		t.Error("an ask going round was not refused")
	}
}

// An ask nobody takes up is dropped after AskLife.
func TestAsked_IsDroppedAfterItsLife(t *testing.T) {
	k := newTalk(t, asker, plan.New("deaf too", func(a *plan.Actor) rule.Step { return a.Idle() }))
	k.need(0, 1)
	k.tick(2)
	if asked, _, _, _ := k.is(1); !asked {
		t.Fatal("not asked")
	}
	k.tick(int(plan.AskLife/(100*time.Millisecond)) + 2)
	if asked, _, _, _ := k.is(1); asked {
		t.Error("an ask nobody took up outlived AskLife")
	}
}
