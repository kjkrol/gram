package main

import (
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/internal/hosts"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/uid"
)

// stageInit is a game.Initializer that drives the real Stage without a window;
// Scene.Layers() is left out.
type stageInit struct {
	hosts   []plugin.Host // of the rules of the moments the plugins installed catch
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

func (c *stageInit) Use(p plugin.Plugin) error {
	c.tracked = append(c.tracked, p)
	return p.Install(c)
}

func (c *stageInit) Track(s plugin.Serializable) error {
	c.tracked = append(c.tracked, s)
	return nil
}

func (c *stageInit) UseWorld(cfg world.Config) *world.Plugin {
	cfg.Camera.ViewportWidth = ScreenWidth
	cfg.Camera.ViewportHeight = ScreenHeight
	c.world = world.NewPlugin(cfg)
	c.tracked = append(c.tracked, c.world)
	if err := c.world.Install(c); err != nil {
		panic(err)
	}
	return c.world
}

// buildStage runs the fresh-spawn half of entering a Stage: Init, Spawn, Populate, Setup.
func buildStage(t *testing.T) (*goke.ECS, *mainStage) {
	t.Helper()

	stage := newStage()
	ctx := &stageInit{ecs: goke.New()}
	if err := stage.Init(ctx); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := ctx.Deliver(ctx.world.Kinds().Played()...); err != nil { // as the engine does once Init returns
		t.Fatalf("roles: %v", err)
	}
	if err := stage.Spawn(); err != nil {
		t.Fatalf("Spawn: %v", err)
	}
	for _, v := range ctx.tracked {
		if p, ok := v.(plugin.Populator); ok {
			if err := p.Populate(); err != nil {
				t.Fatalf("Populate: %v", err)
			}
		}
	}
	ctx.ecs.SetPlan(func(rc goke.RunCtx, d time.Duration) { stage.Update(rc, d); stage.world.Clock().Replay(rc, d) })

	var systems []goke.System
	for _, produce := range ctx.pending {
		systems = append(systems, produce()...)
	}
	ctx.ecs.Setup(systems...)

	return ctx.ecs, stage
}

func TestStage_HunterEatsWhatItCatches(t *testing.T) {
	ecs, stage := buildStage(t)
	view := bodies(ecs, stage)

	if len(view.preyIDs()) != PreyCount {
		t.Fatalf("stage spawned %d prey, want %d", len(view.preyIDs()), PreyCount)
	}
	caught := placeOnPrey(t, stage, view)

	for range 3 {
		ecs.Tick(time.Second / TPS)
	}

	if left := view.preyIDs(); left[caught] {
		t.Errorf("prey %v survived being caught", caught)
	} else if len(left) != PreyCount-1 {
		t.Errorf("%d prey left, want %d — exactly the caught one gone", len(left), PreyCount-1)
	}
	if got, want := stage.world.Res.Telemetry.Count, PreyCount; got != want {
		t.Errorf("Telemetry.Count = %d, want %d (the prey and the hunter, one prey short)", got, want)
	}
}

// placeOnPrey drops the hunter onto the first prey, in the ECS and the index, and returns its id.
func placeOnPrey(t *testing.T, stage *mainStage, view bodyView) uid.UID64 {
	t.Helper()
	var target world.Position
	var caught uid.UID64
	found := false
	view.eachPrey(func(id uid.UID64, b *world.Base) {
		if !found {
			target, caught, found = b.Pos, id, true
		}
	})
	if !found {
		t.Fatal("no prey to place the hunter on")
	}
	view.eachHunter(func(_ uid.UID64, b *world.Base) { stage.world.Space().MoveTo(&b.Pos.AABB, target.TopLeft) })
	return caught
}

// bodyView is a live view of the hunters and the prey — one query, told apart by the roles they play.
type bodyView struct {
	query        *goke.Query
	base         goke.Comp[world.Base]
	marks        goke.Comp[tag.Tags[rule.Roles]]
	prey, hunter tag.Tag[rule.Roles]
}

func bodies(ecs *goke.ECS, s *mainStage) bodyView {
	view := bodyView{prey: s.world.Roles().Named(PreyRole).Tag(), hunter: s.world.Roles().Named(PredatorRole).Tag()}
	ecs.RegSys(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		view.query = si.NewQueryBuilder(&view.base, &view.marks).Build()
	}})
	return view
}

// each calls fn for every entity carrying tag.
func (v *bodyView) each(tag tag.Tag[rule.Roles], fn func(id uid.UID64, b *world.Base)) {
	v.query.All()
	for v.query.Next() {
		cursor := v.query.Cursor()
		bases := v.base.Slice(cursor)
		marks := v.marks.Slice(cursor)
		for i, id := range cursor.IDs {
			if marks[i].Has(tag) {
				fn(id, &bases[i])
			}
		}
	}
}

func (v *bodyView) eachPrey(fn func(id uid.UID64, b *world.Base))   { v.each(v.prey, fn) }
func (v *bodyView) eachHunter(fn func(id uid.UID64, b *world.Base)) { v.each(v.hunter, fn) }

// preyIDs is every prey alive.
func (v *bodyView) preyIDs() map[uid.UID64]bool {
	found := map[uid.UID64]bool{}
	v.eachPrey(func(id uid.UID64, _ *world.Base) { found[id] = true })
	return found
}

func TestStage_PreyTurnsAwayFromTheHunterItSees(t *testing.T) {
	ecs, stage := buildStage(t)
	view := bodies(ecs, stage)

	watched, course := placeHunterAhead(t, stage, view, 60)

	const ticks = 14
	for range ticks {
		ecs.Tick(time.Second / TPS)
	}

	now, alive := headingOf(view, watched)
	if !alive {
		t.Fatalf("the watched prey was gone within %d ticks — it never got the chance to run", ticks)
	}
	if along := now.X*course.X + now.Y*course.Y; along > 0.5 {
		t.Errorf("heading %v after %d ticks of looking at the hunter, started %v — want it well into turning away", now, ticks, course)
	}
}

// placeHunterAhead puts the hunter dead ahead of the first prey; returns its id and course.
func placeHunterAhead(t *testing.T, stage *mainStage, view bodyView, distance float64) (uid.UID64, geom.Vec) {
	t.Helper()
	var watched uid.UID64
	var from world.Position
	var course geom.Vec
	found := false
	view.eachPrey(func(id uid.UID64, b *world.Base) {
		if !found {
			watched, from, course, found = id, b.Pos, b.Vel.Dir, true
		}
	})
	if !found {
		t.Fatal("no prey to put the hunter in front of")
	}

	ahead := geom.NewVec(
		float64(from.TopLeft.X)+course.X*distance,
		float64(from.TopLeft.Y)+course.Y*distance,
	)

	view.eachHunter(func(_ uid.UID64, b *world.Base) { stage.world.Space().MoveTo(&b.Pos.AABB, ahead) })
	return watched, course
}

func headingOf(view bodyView, id uid.UID64) (dir geom.Vec, alive bool) {
	view.eachPrey(func(got uid.UID64, b *world.Base) {
		if got == id {
			dir, alive = b.Vel.Dir, true
		}
	})
	return dir, alive
}

// The prey flee from the start; A has the player take the fleeing off the world, and put it back.
func TestStage_ASwitchesTheFleeingOffAndOn(t *testing.T) {
	ecs, stage := buildStage(t)
	fleeing := func() bool {
		return stage.world.Effects().Has(stage.world.Clock().Entity(), stage.world.Effects().Named(FleeingEf))
	}
	ecs.Tick(time.Second / TPS)
	if !fleeing() {
		t.Fatal("no fleeing on the world from the start")
	}
	stage.world.Carrier().Put(stage.player.ID, stage.world.Commands().Named(FleeCmd)) // what the A key gives
	ecs.Tick(time.Second / TPS)
	if fleeing() {
		t.Fatal("the fleeing still on after A")
	}
	stage.world.Carrier().Put(stage.player.ID, stage.world.Commands().Named(FleeCmd)) // what the A key gives
	ecs.Tick(time.Second / TPS)
	if !fleeing() {
		t.Error("no fleeing after A again")
	}
}

// Hosts keeps the hosts of the rules of the moments a plugin catches.
func (c *stageInit) Hosts(h ...plugin.Host) { c.hosts = append(c.hosts, h...) }

// Deliver hands rules — a role's, each of its own — to the hosts of their moments, as the engine
// does with the roles played once a Stage's Init returns.
func (c *stageInit) Deliver(rules ...rule.Rule) error { return hosts.Deliver(c.hosts, rules...) }
