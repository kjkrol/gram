package dialog_test

import (
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/ceremony/effect"
	"github.com/kjkrol/ceremony/host"
	"github.com/kjkrol/ceremony/tag"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/dialog"
	"github.com/kjkrol/gram/plugins/players/owner"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/ui"
	"github.com/kjkrol/uid"
)

type installer struct {
	ecs     *goke.ECS
	pending []func() []goke.System
}

func (c *installer) UseModule(m goke.Module) {
	regSys := goke.SystemFn{OnInit: func(*goke.SysInit) { m.RegSystems(c.ecs) }}
	c.pending = append(c.pending, func() []goke.System { return append(m.SetupSystems(), regSys) })
}
func (c *installer) Setup(providers ...goke.SetupProvider) {
	for _, p := range providers {
		c.pending = append(c.pending, p.SetupSystems)
	}
}
func (c *installer) RegSys(factory func() goke.System) goke.Runnable { return c.ecs.RegSys(factory()) }
func (c *installer) ECS() *goke.ECS                                  { return c.ecs }
func (c *installer) Hosts(...host.Host)                              {}

const hosts = `
greet:
  speaker: Host
  say: ["Hello, traveller!"]
  choices:
    - text: "Hello to you too!"
      then: [warm]
      next: glad
    - text: "You again, friend!"
      if: friend
      next: glad
    - text: "Out of my way."
      then: [offend]
      next: end

glad:
  speaker: Host
  say: ["Glad to meet you.", "Come back any time!"]
  choices:
    - text: "Bye."
      then: [flour]
      do: [blush]
      next: end
`

type liking struct{ N int8 }

type sacks struct{ N int8 }

type who struct {
	x  float64
	by command.Issuer
}

type rig struct {
	t                     *testing.T
	w                     *world.Plugin
	d                     *dialog.Plugin
	ecs                   *goke.ECS
	host, guest, stranger uid.UID64
	talking, talked       effect.Effect
	blush                 effect.Effect
	carried               *goke.Query
	liking                goke.Comp[liking]
	sacks                 goke.Comp[sacks]
}

func define(file string) (*world.Plugin, *dialog.Plugin, error) {
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 1000, Height: 1000},
		Entities: world.EntitiesCfg{MaxCount: 4, MinSize: 10, MaxSize: 10},
	})
	d := dialog.NewPlugin(w, dialog.Config{})
	d.DefineEffects()
	w.Effects().Define("blush", effect.Spec{effect.Lasts(time.Minute)})
	w.Commands().Define("blush", ceremony.Put(w.Effects().Named("blush")).On(ui.It))
	d.Changes().Define("warm", dialog.Alter(func(l *liking, _ uid.UID64) { l.N += 40 }))
	d.Changes().Define("offend", dialog.Alter(func(l *liking, _ uid.UID64) { l.N -= 80 }))
	d.Changes().Define("flour", dialog.AlterListener(func(s *sacks, _ uid.UID64) { s.N++ }))
	d.Conditions().Define("friend", dialog.When(func(l *liking, _ uid.UID64) bool { return l.N >= 30 }))
	return w, d, d.Load(fstest.MapFS{"hosts.yaml": {Data: []byte(file)}}, "hosts.yaml")
}

func newRig(t *testing.T) *rig {
	t.Helper()
	w, d, err := define(hosts)
	if err != nil {
		t.Fatal(err)
	}
	kind.Define[who](w.Kinds(), "body", kind.Spec{
		comp.Load(func(r who) world.Position {
			return world.Position{AABB: plane.NewAABB(geom.NewVec(r.x, 100), 10, 10)}
		}),
		comp.Const(world.Velocity{}),
		comp.Load(func(r who) tag.Tags[owner.Family] {
			if r.by == command.Nobody {
				return 0
			}
			return tag.Tags[owner.Family](0).With(owner.Of(r.by))
		}),
		comp.Const(d.Script("greet")),
		comp.Const(liking{}),
		comp.Const(sacks{}),
	})
	body := kind.Named[who](w.Kinds(), "body")
	w.Seed(body.Entry(who{x: 100}), body.Entry(who{x: 300, by: 1}), body.Entry(who{x: 500, by: 2}))
	if err := w.Populate(); err != nil {
		t.Fatal(err)
	}
	ctx := &installer{ecs: goke.New()}
	for _, p := range []plugin.Plugin{w, d} {
		if err := p.Install(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Carry(d); err != nil {
		t.Fatal(err)
	}
	r := &rig{t: t, w: w, d: d, ecs: ctx.ecs,
		talking: w.Effects().Named(dialog.TalkingEf), talked: w.Effects().Named(dialog.TalkedEf), blush: w.Effects().Named("blush")}
	var base goke.Comp[world.Base]
	var q *goke.Query
	var systems []goke.System
	for _, produce := range ctx.pending {
		systems = append(systems, produce()...)
	}
	systems = append(systems, goke.SystemFn{OnInit: func(si *goke.SysInit) {
		q = si.NewQueryBuilder(&base).Build()
		r.carried = si.NewQueryBuilder(&r.liking, &r.sacks).Build()
	}})
	r.ecs.Setup(systems...)
	r.ecs.SetPlan(func(rc goke.RunCtx, dt time.Duration) {
		w.RunPlan(rc, dt)
		d.RunPlan(rc, dt)
		w.Clock().Replay(rc, dt)
		rc.Sync()
	})
	byX := map[float64]uid.UID64{}
	for q.All(); q.Next(); {
		cur := q.Cursor()
		for i, id := range cur.IDs {
			byX[base.Slice(cur)[i].Pos.TopLeft.X] = id
		}
	}
	r.host, r.guest, r.stranger = byX[100], byX[300], byX[500]
	return r
}

func (r *rig) tick(n int) {
	for range n {
		r.ecs.Tick(time.Second / 10)
	}
}

func (r *rig) begin(speaker, listener uid.UID64) {
	b := dialog.Begin{}
	b.Aim(listener)
	r.w.Carrier().PutFrom(speaker, b)
	r.tick(2)
}

func (r *rig) choose(by command.Issuer, speaker uid.UID64, index int) {
	r.w.Carrier().Put(by, dialog.Choose{Index: index, Speaker: speaker})
	r.tick(2)
}

func (r *rig) line(speaker uid.UID64) string {
	s, _ := r.d.LineText().Text(speaker, true)
	return s
}

func (r *rig) offered(speaker uid.UID64) []string {
	var out []string
	for i := range dialog.MaxChoices {
		if s, ok := r.d.ChoiceText(i).Text(speaker, true); ok {
			out = append(out, s)
		}
	}
	return out
}

func (r *rig) held(id uid.UID64) (liking, sacks) {
	r.t.Helper()
	if !r.carried.Seek(id) {
		r.t.Fatalf("entity %d carries no liking", id)
	}
	cur := r.carried.Cursor()
	return *r.liking.At(cur), *r.sacks.At(cur)
}

func TestConversation_BeginsIsAnsweredAndChangesTheGame(t *testing.T) {
	r := newRig(t)
	r.begin(r.host, r.guest)
	if got := r.line(r.host); got != "Hello, traveller!" || !r.w.Effects().Has(r.host, r.talking) {
		t.Fatalf("the host says %q, talking %v; want the greeting", got, r.w.Effects().Has(r.host, r.talking))
	}
	if got := strings.Join(r.offered(r.host), "|"); got != "Hello to you too!|Out of my way." {
		t.Fatalf("offered %q, want the answers that need no friend", got)
	}
	r.choose(1, r.host, 0)
	if got := r.line(r.host); got != "Glad to meet you.\nCome back any time!" {
		t.Fatalf("after the answer the host says %q, want the next node's lines", got)
	}
	if l, _ := r.held(r.host); l.N != 40 {
		t.Fatalf("the host's liking is %d, want 40: warm changed the speaker", l.N)
	}
	r.choose(1, r.host, 0) // Bye
	if r.line(r.host) != "" || r.w.Effects().Has(r.host, r.talking) || !r.w.Effects().Has(r.host, r.talked) {
		t.Fatalf("after Bye: says %q, talking %v, talked %v; want it over and talked", r.line(r.host), r.w.Effects().Has(r.host, r.talking), r.w.Effects().Has(r.host, r.talked))
	}
	if _, s := r.held(r.guest); s.N != 1 {
		t.Errorf("the guest holds %d sacks, want 1: flour changed the listener", s.N)
	}
	if !r.w.Effects().Has(r.host, r.blush) {
		t.Error("Bye's command for ui.It did not reach the host")
	}
	r.begin(r.host, r.guest)
	if got := strings.Join(r.offered(r.host), "|"); got != "Hello to you too!|You again, friend!|Out of my way." {
		t.Errorf("meeting again offered %q, want a friend's answer too: the game's condition holds", got)
	}
}

func TestConversation_OnlyAPlayerTheListenerObeysAnswers(t *testing.T) {
	r := newRig(t)
	r.begin(r.host, r.guest)
	r.choose(2, r.host, 0)
	if got := r.line(r.host); got != "Hello, traveller!" {
		t.Errorf("another player's answer moved the conversation to %q", got)
	}
}

func TestConversation_TakingTalkingOffEndsItWithoutTalked(t *testing.T) {
	r := newRig(t)
	r.begin(r.host, r.guest)
	r.w.Carrier().Put(command.Nobody, ceremony.Take(r.talking).On(ceremony.ID(r.host)))
	r.tick(2)
	if r.line(r.host) != "" || r.w.Effects().Has(r.host, r.talked) {
		t.Errorf("with talking taken off: says %q, talked %v; want it over, not talked", r.line(r.host), r.w.Effects().Has(r.host, r.talked))
	}
}

func TestConversation_OneTalksWithOneAtATime(t *testing.T) {
	r := newRig(t)
	r.begin(r.host, r.guest)
	r.begin(r.stranger, r.guest)
	if r.w.Effects().Has(r.stranger, r.talking) || r.line(r.stranger) != "" {
		t.Error("the stranger began to talk with a guest talking with the host")
	}
	r.begin(r.host, r.stranger)
	if r.line(r.host) != "Hello, traveller!" {
		t.Errorf("a second Begin moved the host's conversation to %q", r.line(r.host))
	}
}

func TestFeel_MakesTheGamesChangeTowardsWhomItIsAbout(t *testing.T) {
	r := newRig(t)
	f := dialog.Feel{Change: "offend"}
	f.Aim(r.guest)
	r.w.Carrier().PutFrom(r.host, f)
	r.tick(2)
	if l, _ := r.held(r.host); l.N != -80 {
		t.Errorf("the host's liking is %d, want -80", l.N)
	}
}

func TestLoad_RefusesWhatItCannotRead(t *testing.T) {
	for name, tc := range map[string]struct{ file, want string }{
		"a misspelt field":     {"a:\n  speaker: X\n  choices:\n    - text: \"hi\"\n      nxt: end\n", "line 5"},
		"an unknown command":   {"a:\n  choices:\n    - text: \"hi\"\n      next: end\n      do: [shout]\n", `"shout"`},
		"an unknown change":    {"a:\n  choices:\n    - text: \"hi\"\n      next: end\n      then: [hug]\n", `"hug"`},
		"an unknown if":        {"a:\n  choices:\n    - text: \"hi\"\n      if: angry\n      next: end\n", `"angry"`},
		"an answer to nowhere": {"a:\n  choices:\n    - text: \"hi\"\n", "leads nowhere"},
	} {
		t.Run(name, func(t *testing.T) {
			_, _, err := define(tc.file)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("Load: %v, want an error saying %s", err, tc.want)
			}
		})
	}
}

func TestSetup_PanicsForAnAnswerLeadingToAnUndefinedNode(t *testing.T) {
	w, d, err := define("a:\n  choices:\n    - text: \"hi\"\n      next: b\n")
	if err != nil {
		t.Fatal(err)
	}
	ctx := &installer{ecs: goke.New()}
	for _, p := range []plugin.Plugin{w, d} {
		if err := p.Install(ctx); err != nil {
			t.Fatal(err)
		}
	}
	defer func() {
		if r := recover(); r == nil || !strings.Contains(r.(string), `"b"`) {
			t.Errorf("set up with an answer leading to b: %v, want a panic naming it", r)
		}
	}()
	var systems []goke.System
	for _, produce := range ctx.pending {
		systems = append(systems, produce()...)
	}
	ctx.ecs.Setup(systems...)
}

func TestConditions_RefuseANameAnEffectBears(t *testing.T) {
	_, d, err := define("a:\n  say: [\"hi\"]\n")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if r := recover(); r == nil {
			t.Error("a condition named as the effect blush was defined, want a panic")
		}
	}()
	d.Conditions().Define("blush", dialog.When(func(*liking, uid.UID64) bool { return true }))
}
