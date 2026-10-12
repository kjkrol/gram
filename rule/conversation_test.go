package rule_test

import (
	"github.com/kjkrol/gram/internal/steps"
	"testing"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/uid"
)

type pass struct{}

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

var asker = write("asker", rule.OneOf(
	rule.Branch[need]{
		Name: "needs to pass",
		Do: rule.Ask[pass]("let me pass", 500*time.Millisecond,
			rule.Order(walk{}).Stay(),
			rule.Order(flee{}).Stay()),
	},
	rule.Idle(),
))

var giver = write("giver", rule.OneOf(
	rule.Branch[rule.Asked[pass]]{
		Name: "asked to let pass",
		Do: rule.OneOf(
			rule.Branch[room]{
				When: room.free,
				Do:   rule.Steps(rule.Agree[pass](), rule.Order(jump{High: true})),
			},
			rule.Branch[room]{When: room.byAlly, Do: rule.Relay[pass]()},
			rule.Refuse[pass](),
		),
	},
	rule.Idle(),
))

type talk struct {
	t      *testing.T
	ecs    *goke.ECS
	now    time.Duration
	ids    []uid.UID64
	edits  []func(cb *goke.CmdBuf)
	needID goke.CompID
	roomID goke.CompID
	look   *goke.Query
	asked  goke.OptComp[steps.Asked[pass]]
	toys   *toys
}

type mindless struct{ Size int }

// plans: each a written plan, or nil for a body without a mind.
func newTalk(t *testing.T, plans ...any) *talk {
	t.Helper()
	k := &talk{t: t, ecs: goke.New(), toys: newToys()}
	var minds []steps.Mind
	c := steps.NewPlans(func() time.Duration { return k.now }, nil, 0, nil, &k.toys.carrier)
	defined := map[string]steps.Mind{}
	for _, p := range plans {
		if p == nil {
			minds = append(minds, steps.Mind{})
			continue
		}
		w := p.(written)
		if _, ok := defined[w.plan.Name()]; !ok {
			defined[w.plan.Name()] = w.mind(c)
		}
		minds = append(minds, defined[w.plan.Name()])
	}
	var mind goke.Comp[steps.Mind]
	var plain goke.Comp[mindless]
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

func (k *talk) room(who int, free bool, ally int) {
	id := k.ids[who]
	v := room{Free: free}
	if ally >= 0 {
		v.Ally, v.Beside = k.ids[ally], true
	}
	k.edits = append(k.edits, func(cb *goke.CmdBuf) { cb.AddOne(id, k.roomID, v) })
}

func (k *talk) is(who int) (asked, walking, fleeing, jumping bool) {
	id := k.ids[who]
	if !k.look.Seek(id) {
		k.t.Fatal("entity gone")
	}
	doing := k.toys.last[id]
	return k.asked.Present(k.look.Cursor()), doing == "walk", doing == "flee", doing == "jump"
}

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

func TestAsk_ANoOrNoAnswerRunsTheRefusedBranch(t *testing.T) {
	k := newTalk(t, asker, giver) // no room: refused
	k.room(1, false, -1)
	k.need(0, 1)
	k.tick(3)
	if _, walking, fleeing, _ := k.is(0); walking || !fleeing {
		t.Errorf("refused, the asker walks %v, flees %v; want fleeing", walking, fleeing)
	}

	k = newTalk(t, asker, write("deaf", rule.Idle())) // never answers
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
	k.tick(4) // past the asker's wait
	if _, walking, fleeing, _ := k.is(0); walking || fleeing {
		t.Errorf("the asker walks %v, flees %v while its ask is passed on; want waiting", walking, fleeing)
	}
	k.room(1, true, -1) // room beside now
	k.tick(3)
	if _, walking, _, _ := k.is(0); !walking {
		t.Error("the way made along the chain, the asker does not walk on")
	}
}

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

func TestRelay_TheChainEndsAtItsLimitAndNeverLoops(t *testing.T) {
	n := steps.MaxChain + 3
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
	if furthest != steps.MaxChain+1 {
		t.Errorf("the ask got as far as entity %d, want %d: past the asker, %d links", furthest, steps.MaxChain+1, steps.MaxChain)
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

func TestAsked_IsDroppedAfterItsLife(t *testing.T) {
	k := newTalk(t, asker, write("deaf too", rule.Idle()))
	k.need(0, 1)
	k.tick(2)
	if asked, _, _, _ := k.is(1); !asked {
		t.Fatal("not asked")
	}
	k.tick(int(steps.AskLife/(100*time.Millisecond)) + 2)
	if asked, _, _, _ := k.is(1); asked {
		t.Error("an ask nobody took up outlived AskLife")
	}
}
