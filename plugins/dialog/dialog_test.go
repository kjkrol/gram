package dialog_test

import (
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/dialog"
	"github.com/kjkrol/gram/plugins/players/owner"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/gram/rule/effect"
	"github.com/kjkrol/gram/ui"
	"github.com/kjkrol/uid"
)

// installer is the plugin.Installer a Stage would hand over, minus the engine.
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
func (c *installer) Hosts(...plugin.Host)                            {}

const hosts = `
greet:
  speaker: Host
  say: ["Hello, traveller!"]
  choices:
    - text: "Hello to you too!"
      mood: 40
      next: glad
    - text: "Good to see you again!"
      if: talked
      next: glad
    - text: "You again, friend!"
      if: friend
      next: glad
    - text: "Out of my way."
      mood: -80
      next: end

glad:
  speaker: Host
  say: ["Glad to meet you.", "Come back any time!"]
  choices:
    - text: "Bye."
      next: end
      do: [blush]
`

// who is a row of a test's entities: where it stands and whose it is.
type who struct {
	x  float64
	by control.PlayerID
}

// rig is a world of three: a host nobody owns, a guest player 1 owns and a stranger player 2 owns,
// each with a Script beginning at greet.
type rig struct {
	t                     *testing.T
	w                     *world.Plugin
	d                     *dialog.Plugin
	ecs                   *goke.ECS
	host, guest, stranger uid.UID64
	talking, talked       effect.Effect
	blush                 effect.Effect
}

// define makes the world and the dialog plugin and defines what the rig's tests share, loading
// file as the hosts' nodes; it hands back the world, the plugin and the error of the load.
func define(file string) (*world.Plugin, *dialog.Plugin, error) {
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 1000, Height: 1000},
		Entities: world.EntitiesCfg{MaxCount: 4, MinSize: 10, MaxSize: 10},
	})
	d := dialog.NewPlugin(w, dialog.Config{})
	d.DefineEffects()
	w.Effects().Define("blush", effect.Spec{effect.Lasts(time.Minute)})
	w.Commands().Define("blush", rule.Cast(w.Effects().Named("blush")).On(ui.It))
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
			if r.by == control.Nobody {
				return 0
			}
			return tag.Tags[owner.Family](0).With(owner.Of(r.by))
		}),
		comp.Const(d.Script("greet")),
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
	systems = append(systems, goke.SystemFn{OnInit: func(si *goke.SysInit) { q = si.NewQueryBuilder(&base).Build() }})
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

// begin has speaker begin to talk with listener, as a rule's Order on a moment naming it does.
func (r *rig) begin(speaker, listener uid.UID64) {
	b := dialog.Begin{}
	b.Aim(listener)
	r.w.Carrier().PutFrom(speaker, b)
	r.tick(2)
}

// choose has player by choose the answer at index in speaker's conversation, as the window's
// button does.
func (r *rig) choose(by control.PlayerID, speaker uid.UID64, index int) {
	r.w.Carrier().Put(by, dialog.Choose{Index: index, Speaker: speaker})
	r.tick(2)
}

// line is what the speaker says now; "" for nothing.
func (r *rig) line(speaker uid.UID64) string {
	s, _ := r.d.LineText().Text(speaker, true)
	return s
}

// offered are the answers the speaker offers now.
func (r *rig) offered(speaker uid.UID64) []string {
	var out []string
	for i := range dialog.MaxChoices {
		if s, ok := r.d.ChoiceText(i).Text(speaker, true); ok {
			out = append(out, s)
		}
	}
	return out
}

// chosen is a selection with one unit chosen for every player.
type chosen uid.UID64

func (c chosen) Chosen(control.PlayerID) (uid.UID64, bool) { return uid.UID64(c), true }

func TestConversation_BeginsIsAnsweredAndRemembered(t *testing.T) {
	r := newRig(t)
	r.begin(r.host, r.guest)
	if got := r.line(r.host); got != "Hello, traveller!" || !r.talking.On(r.host) {
		t.Fatalf("the host says %q, talking %v; want the greeting", got, r.talking.On(r.host))
	}
	if got := strings.Join(r.offered(r.host), "|"); got != "Hello to you too!|Out of my way." {
		t.Fatalf("offered %q, want the two answers that need nothing", got)
	}
	r.choose(1, r.host, 0)
	if got := r.line(r.host); got != "Glad to meet you.\nCome back any time!" {
		t.Fatalf("after the answer the host says %q, want the next node's lines", got)
	}
	if got, _ := r.d.Stance(chosen(r.guest), 1).Text(r.host, true); got != "Friend" {
		t.Fatalf("the host makes %q of the guest, want Friend", got)
	}
	r.choose(1, r.host, 0) // Bye
	if r.line(r.host) != "" || r.talking.On(r.host) || !r.talked.On(r.host) {
		t.Fatalf("after Bye: says %q, talking %v, talked %v; want it over and talked", r.line(r.host), r.talking.On(r.host), r.talked.On(r.host))
	}
	if !r.blush.On(r.host) {
		t.Error("Bye's command for ui.It did not reach the host")
	}
	r.begin(r.host, r.guest)
	if got := strings.Join(r.offered(r.host), "|"); got != "Hello to you too!|Good to see you again!|You again, friend!|Out of my way." {
		t.Errorf("meeting again offered %q, want those for one talked to and a friend too", got)
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
	r.w.Carrier().Put(control.Nobody, rule.Lift(r.talking).On(entity.ID(r.host)))
	r.tick(2)
	if r.line(r.host) != "" || r.talked.On(r.host) {
		t.Errorf("with talking taken off: says %q, talked %v; want it over, not talked", r.line(r.host), r.talked.On(r.host))
	}
}

func TestConversation_OneTalksWithOneAtATime(t *testing.T) {
	r := newRig(t)
	r.begin(r.host, r.guest)
	r.begin(r.stranger, r.guest)
	if r.talking.On(r.stranger) || r.line(r.stranger) != "" {
		t.Error("the stranger began to talk with a guest talking with the host")
	}
	r.begin(r.host, r.stranger)
	if r.line(r.host) != "Hello, traveller!" {
		t.Errorf("a second Begin moved the host's conversation to %q", r.line(r.host))
	}
}

func TestMood_MovesWhatTheSpeakerMakesOfWhomItIsAbout(t *testing.T) {
	r := newRig(t)
	m := dialog.Mood{By: -80}
	m.Aim(r.guest)
	r.w.Carrier().PutFrom(r.host, m)
	r.tick(2)
	if got, _ := r.d.Stance(chosen(r.guest), 1).Text(r.host, true); got != "Enemy" {
		t.Errorf("the host makes %q of the guest, want Enemy", got)
	}
	if _, ok := r.d.Stance(chosen(r.host), 1).Text(r.host, true); ok {
		t.Error("a stance towards itself is said, want nothing")
	}
}

func TestLoad_RefusesWhatItCannotRead(t *testing.T) {
	for name, tc := range map[string]struct{ file, want string }{
		"a misspelt field":     {"a:\n  speaker: X\n  choices:\n    - text: \"hi\"\n      nxt: end\n", "line 5"},
		"an unknown command":   {"a:\n  choices:\n    - text: \"hi\"\n      next: end\n      do: [shout]\n", `"shout"`},
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
