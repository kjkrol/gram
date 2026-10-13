package plan_test

import (
	"testing"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/internal/steps"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/gram/rule/cond"
	"github.com/kjkrol/gram/rule/plan"
	"github.com/kjkrol/uid"
)

// pass is the ask of these tests, let me pass; reply its answer
type (
	pass  struct{ rule.From }
	reply struct {
		rule.From
		Yes bool
	}
)

func (pass) Command()  {}
func (reply) Command() {}

func (r reply) yes() bool { return r.Yes }

// need is the fact of wanting past Who; room of what stands beside: free, or an ally to ask on
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

// asker asks whom it needs past, walks on let through and flees turned away or unanswered
var asker = write("asker", plan.OneOf(
	plan.Loop[need]{
		Name: "ask to pass",
		When: cond.Self.If(cond.Always),
		Then: plan.Steps(
			plan.Other.Do(pass{}),
			plan.OneOf(
				plan.Loop[any]{
					When:    cond.Self.If(cond.Always),
					Then:    plan.Self.Idle(),
					Until:   cond.Self.If(reply{}),
					Timeout: 500 * time.Millisecond,
				},
				plan.Self.Do(flee{}),
			),
		),
	},
	plan.Loop[reply]{
		Name: "let through",
		When: cond.Self.If(cond.NewWord("yes", reply.yes)),
		Then: plan.Self.Do(walk{}),
	},
	plan.Loop[reply]{
		Name: "turned away",
		When: cond.Self.If(cond.Not(cond.NewWord("yes", reply.yes))),
		Then: plan.Self.Do(flee{}),
	},
	idle(),
))

// giver lets pass where it has room, jumping aside; asks its ally on where it has one and
// passes the answer back; refuses otherwise
var giver = write("giver", plan.OneOf(
	plan.Loop[pass]{
		Name: "asked to let pass",
		When: cond.Self.If(cond.Always),
		Then: plan.OneOf(
			plan.Loop[room]{
				When: cond.Self.If(cond.NewWord("free", room.free)),
				Then: plan.Steps(
					plan.Other.Do(reply{Yes: true}),
					plan.Self.Do(jump{High: true}),
				),
			},
			plan.Loop[room]{
				When: cond.Self.If(cond.NewWord("by an ally", room.byAlly)),
				Then: plan.Other.Do(pass{}),
			},
			plan.Other.Do(reply{}),
		),
	},
	plan.Loop[reply]{
		Name: "pass the answer on",
		When: cond.Self.If(cond.Always),
		Then: plan.Loop[pass]{
			When: cond.Self.If(cond.Always),
			Then: plan.Other.Do(reply{Yes: true}),
		},
	},
	idle(),
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
	asked  goke.OptComp[pass]
	toys   *toys
}

type mindless struct{ Size int }

// each a written plan, or nil for a body without a mind
func newTalk(t *testing.T, plans ...any) *talk {
	t.Helper()
	k := &talk{t: t, ecs: goke.New(), toys: newToys()}
	var minds []steps.Mind
	c := steps.NewPlans(func() time.Duration { return k.now }, 0, nil, &k.toys.carrier)
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

// ally -1: nobody beside
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

func (k *talk) askedBy(who int) uid.UID64 {
	id := k.ids[who]
	if !k.look.Seek(id) {
		k.t.Fatal("entity gone")
	}
	q := k.asked.At(k.look.Cursor())
	if q == nil {
		return 0
	}
	return q.By
}

func TestMessage_AYesRunsTheAskersBranchOnTheReply(t *testing.T) {
	k := newTalk(t, asker, giver)
	k.room(1, true, -1)
	k.need(0, 1)
	k.tick(4)
	if _, walking, fleeing, _ := k.is(0); !walking || fleeing {
		t.Errorf("the asker walks %v, flees %v; want let pass: walking", walking, fleeing)
	}
	if _, _, _, jumping := k.is(1); !jumping {
		t.Error("the one asked does not jump aside")
	}
	if by := k.askedBy(1); by != k.ids[0] {
		t.Errorf("the ask on the one asked is from %v; want the asker %v", by, k.ids[0])
	}
}

func TestMessage_ANoOrNoAnswerRunsTheOtherBranch(t *testing.T) {
	k := newTalk(t, asker, giver)
	k.room(1, false, -1)
	k.need(0, 1)
	k.tick(4)
	if _, walking, fleeing, _ := k.is(0); walking || !fleeing {
		t.Errorf("refused, the asker walks %v, flees %v; want fleeing", walking, fleeing)
	}

	k = newTalk(t, asker, write("deaf", idle()))
	k.need(0, 1)
	k.tick(4)
	if _, _, fleeing, _ := k.is(0); fleeing {
		t.Error("the asker gave up before its wait was over")
	}
	k.tick(3)
	if _, _, fleeing, _ := k.is(0); !fleeing {
		t.Error("no answer in time, and the asker does not give up")
	}

	k = newTalk(t, asker, nil)
	k.need(0, 1)
	k.tick(8)
	if asked, _, _, _ := k.is(1); asked {
		t.Error("an entity with no mind was given a message")
	}
	if _, walking, fleeing, _ := k.is(0); walking || fleeing {
		t.Errorf("asking one with no mind the asker walks %v, flees %v; want nothing",
			walking, fleeing)
	}
}

func TestMessage_PassedOnComesBackAlongTheChain(t *testing.T) {
	k := newTalk(t, asker, giver, giver)
	k.room(1, false, 2)
	k.room(2, true, -1)
	k.need(0, 1)
	k.tick(2)
	if asked, _, _, _ := k.is(2); !asked {
		t.Fatal("the ally beside was not asked on")
	}
	if by := k.askedBy(2); by != k.ids[1] {
		t.Errorf("the ask passed on is from %v; want the one who passed it on, %v", by, k.ids[1])
	}
	k.tick(2)
	if _, _, _, jumping := k.is(2); !jumping {
		t.Error("the ally asked on does not make way")
	}
	k.tick(2)
	if _, walking, _, _ := k.is(0); !walking {
		t.Error("the way made along the chain, the answer passed back, the asker does not walk on")
	}
}

func TestMessage_PassedOnEndsAtMaxChain(t *testing.T) {
	n := rule.MaxChain + 3
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
	if furthest != rule.MaxChain {
		t.Errorf("the ask got as far as entity %d, want %d: the asker's own and %d passed on",
			furthest, rule.MaxChain, rule.MaxChain-1)
	}
}

func TestMessage_IsDroppedAfterItsLife(t *testing.T) {
	k := newTalk(t, asker, write("deaf too", idle()))
	k.need(0, 1)
	k.tick(2)
	if asked, _, _, _ := k.is(1); !asked {
		t.Fatal("not asked")
	}
	k.tick(int(rule.AskLife/(100*time.Millisecond)) + 2)
	if asked, _, _, _ := k.is(1); asked {
		t.Error("a message nobody took up outlived AskLife")
	}
}
