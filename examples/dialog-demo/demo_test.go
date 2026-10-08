package main

import (
	"cmp"
	"slices"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/internal/hosts"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/dialog"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/uid"
)

// stageInit is a game.Initializer that drives the real Stage without a window; Scene.Layers() is
// left out.
type stageInit struct {
	hosts   []plugin.Host
	ecs     *goke.ECS
	world   *world.Plugin
	tracked []any
	pending []func() []goke.System
	tps     game.TPS
}

var _ game.Initializer = (*stageInit)(nil)

func (c *stageInit) UseModule(m goke.Module) {
	regSys := goke.SystemFn{OnInit: func(*goke.SysInit) { m.RegSystems(c.ecs) }}
	c.tracked = append(c.tracked, m)
	c.pending = append(c.pending, func() []goke.System { return append(m.SetupSystems(), regSys) })
}

func (c *stageInit) Setup(providers ...goke.SetupProvider) {
	for _, p := range providers {
		c.tracked = append(c.tracked, p)
		c.pending = append(c.pending, p.SetupSystems)
	}
}

func (c *stageInit) RegSys(factory func() goke.System) goke.Runnable { return c.ecs.RegSys(factory()) }
func (c *stageInit) ECS() *goke.ECS                                  { return c.ecs }
func (c *stageInit) TPS() *game.TPS                                  { return &c.tps }
func (c *stageInit) Hosts(h ...plugin.Host)                          { c.hosts = append(c.hosts, h...) }

func (c *stageInit) Use(p plugin.Plugin) error {
	c.tracked = append(c.tracked, p)
	return p.Install(c)
}

func (c *stageInit) Track(s plugin.Serializable) error {
	c.tracked = append(c.tracked, s)
	return nil
}

func (c *stageInit) UseWorld(cfg world.Config) *world.Plugin {
	c.world = world.NewPlugin(cfg)
	c.tracked = append(c.tracked, c.world)
	if err := c.world.Install(c); err != nil {
		panic(err)
	}
	return c.world
}

// testStage is the demo built fresh, without a window, its scene's layers initialised.
type testStage struct {
	*arena
	stage  game.Stage
	ecs    *goke.ECS
	base   goke.Comp[world.Base]
	units  *goke.Query
	layers []render.Layer // the scene's, initialised
}

func buildStage(t *testing.T) *testStage {
	t.Helper()
	s := &testStage{}
	s.arena, s.stage = newArena()
	ctx := &stageInit{ecs: goke.New()}
	if err := s.stage.Init(ctx); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := hosts.Deliver(ctx.hosts, ctx.world.Kinds().Played()...); err != nil { // as the engine does once Init returns
		t.Fatalf("roles: %v", err)
	}
	if err := s.stage.Spawn(); err != nil {
		t.Fatalf("Spawn: %v", err)
	}
	for _, v := range ctx.tracked {
		if p, ok := v.(plugin.Populator); ok {
			if err := p.Populate(); err != nil {
				t.Fatalf("Populate: %v", err)
			}
		}
	}
	ctx.ecs.SetPlan(func(rc goke.RunCtx, d time.Duration) { s.stage.Update(rc, d); s.world.Clock().Replay(rc, d) })
	var systems []goke.System
	for _, produce := range ctx.pending {
		systems = append(systems, produce()...)
	}
	systems = append(systems, goke.SystemFn{OnInit: func(si *goke.SysInit) { s.units = si.NewQueryBuilder(&s.base).Build() }})
	s.layers = s.scene.Layers()
	for _, l := range s.layers { // as the engine does, entering the Stage
		systems = append(systems, goke.SystemFn{OnInit: l.Init})
	}
	ctx.ecs.Setup(systems...)
	s.ecs = ctx.ecs
	return s
}

func (s *testStage) tick(n int) {
	for range n {
		s.ecs.Tick(time.Second / TPS)
	}
}

// units are the entities of kind k, top to bottom.
func (s *testStage) all(t *testing.T, k string) []uid.UID64 {
	t.Helper()
	id := kind.Named[unitRow](s.world.Kinds(), k).ID()
	type at struct {
		id uid.UID64
		y  float64
	}
	var found []at
	for s.units.All(); s.units.Next(); {
		cur := s.units.Cursor()
		for i, e := range cur.IDs {
			if b := s.base.Slice(cur)[i]; b.TypeID == id {
				found = append(found, at{e, b.Pos.TopLeft.Y})
			}
		}
	}
	slices.SortFunc(found, func(a, b at) int { return cmp.Compare(a.y, b.y) })
	var out []uid.UID64
	for _, f := range found {
		out = append(out, f.id)
	}
	if len(out) == 0 {
		t.Fatalf("no %s", k)
	}
	return out
}

// put stands the entity id on cell (x, y).
func (s *testStage) put(id uid.UID64, x, y uint32) {
	at := s.board.Res.Logic.Board.CellCenter(s.board.Res.Logic.Board.CellIndex(x, y))
	if s.units.Seek(id) {
		pos := &s.base.At(s.units.Cursor()).Pos
		pos.AABB = plane.NewAABB(geom.NewVec(at.X-UnitSize/2, at.Y-UnitSize/2), UnitSize, UnitSize)
	}
}

// says is what host says now and the answers it offers; "" and none for nothing.
func (s *testStage) says(host uid.UID64) (string, []string) {
	line, _ := s.dialog.LineText().Text(host, true)
	var offered []string
	for i := range dialog.MaxChoices {
		if text, ok := s.dialog.ChoiceText(i).Text(host, true); ok {
			offered = append(offered, text)
		}
	}
	return line, offered
}

// answer has the player choose text among host's answers, as the window's button does.
func (s *testStage) answer(t *testing.T, host uid.UID64, text string) {
	t.Helper()
	_, offered := s.says(host)
	i := slices.Index(offered, text)
	if i < 0 {
		t.Fatalf("%q is not offered, only %q", text, offered)
	}
	if err := s.players.Issue(s.player, dialog.Choose{Index: i, Speaker: host}); err != nil {
		t.Fatal(err)
	}
	s.tick(2)
}

// stance is what host makes of the traveller, as the label under it says while pointed at.
func (s *testStage) stance(host uid.UID64) string {
	word, _ := s.dialog.Stance(s.selection, s.player.ID).Text(host, true)
	return word
}

// hosts are the miller, at (16, 4), and the smith, at (16, 10), and the traveller.
func (s *testStage) hosts(t *testing.T) (miller, smith, traveller uid.UID64) {
	t.Helper()
	h := s.all(t, HostKind)
	return h[0], h[1], s.all(t, TravellerKind)[0]
}

// Each host greets the traveller beside it with lines of its own, read from its file; walked off,
// the conversation is over.
func TestDemo_EachHostSaysLinesOfItsOwn(t *testing.T) {
	s := buildStage(t)
	s.tick(2)
	miller, smith, traveller := s.hosts(t)
	if line, _ := s.says(miller); line != "" {
		t.Fatalf("the miller says %q to a traveller far off", line)
	}
	s.put(traveller, 15, 4)
	s.tick(3)
	line, offered := s.says(miller)
	if line != "Hello, traveller! Fine weather for the mill." || !slices.Equal(offered, []string{"Hello to you too!", "Who are you?", "Out of my way."}) {
		t.Fatalf("the miller says %q offering %q", line, offered)
	}
	s.put(traveller, 15, 10)
	s.tick(3)
	if line, _ := s.says(miller); line != "" {
		t.Errorf("the miller still says %q with the traveller gone", line)
	}
	if line, _ := s.says(smith); line != "Hm. A traveller." {
		t.Errorf("the smith says %q, want its own greeting", line)
	}
}

// A kind answer makes the miller a friend: the label under it says so, it lets the traveller be for
// a while after, and then greets it as a friend.
func TestDemo_AKindAnswerMakesTheMillerAFriendWhoRemembersIt(t *testing.T) {
	s := buildStage(t)
	s.tick(2)
	miller, _, traveller := s.hosts(t)
	s.put(traveller, 15, 4)
	s.tick(3)
	if got := s.stance(miller); got != "Neutral" {
		t.Fatalf("before a word the miller makes %q of the traveller, want Neutral", got)
	}
	s.answer(t, miller, "Hello to you too!")
	if line, _ := s.says(miller); line != "Glad to meet you. Come back any time!" {
		t.Fatalf("after the answer the miller says %q", line)
	}
	s.answer(t, miller, "Bye.")
	if got := s.stance(miller); got != "Friend" {
		t.Fatalf("the miller makes %q of the traveller, want Friend", got)
	}
	s.tick(TPS)
	if line, _ := s.says(miller); line != "" {
		t.Fatalf("the miller talks again at once: %q", line)
	}
	s.tick(10 * TPS)
	if _, offered := s.says(miller); len(offered) == 0 || offered[0] != "Good to see you again, miller!" {
		t.Errorf("meeting again the miller offers %q, want a friend's answer first", offered)
	}
}

// A rude answer makes the smith an enemy, who offers the traveller a way to make up.
func TestDemo_ARudeAnswerMakesTheSmithAnEnemy(t *testing.T) {
	s := buildStage(t)
	s.tick(2)
	_, smith, traveller := s.hosts(t)
	s.put(traveller, 15, 10)
	s.tick(3)
	s.answer(t, smith, "Leave me alone.")
	if got := s.stance(smith); got != "Enemy" {
		t.Fatalf("the smith makes %q of the traveller, want Enemy", got)
	}
	s.tick(11 * TPS)
	if _, offered := s.says(smith); len(offered) == 0 || offered[0] != "Sorry about before." {
		t.Errorf("meeting again the smith offers %q, want a way to make up first", offered)
	}
}
