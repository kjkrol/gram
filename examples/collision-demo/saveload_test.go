package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/entity/kind"
	"github.com/kjkrol/gram/plugins/world/entity/kind/comp"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/uid"
)

// testInstallCtx is a minimal plugin.Installer for tests that call Install directly.
type testInstallCtx struct {
	ecs     *goke.ECS
	pending []func() []goke.System
	tracked []any
}

func (c *testInstallCtx) UseModule(m goke.Module) {
	regSys := goke.SystemFn{OnInit: func(si *goke.SysInit) { m.RegSystems(c.ecs) }}
	c.tracked = append(c.tracked, m)
	c.pending = append(c.pending, func() []goke.System { return append(m.SetupSystems(), regSys) })
}
func (c *testInstallCtx) Setup(providers ...goke.SetupProvider) {
	for _, p := range providers {
		c.tracked = append(c.tracked, p)
		c.pending = append(c.pending, p.SetupSystems)
	}
}
func (c *testInstallCtx) RegSys(factory func() goke.System) goke.Runnable {
	return c.ecs.RegSys(factory())
}
func (c *testInstallCtx) ECS() *goke.ECS { return c.ecs }

// eachOnce drops repeated tokens, as the engine does.
func eachOnce(tokens []goke.CompToken) []goke.CompToken {
	listed := map[string]bool{}
	var once []goke.CompToken
	for _, token := range tokens {
		if !listed[token.Name] {
			listed[token.Name] = true
			once = append(once, token)
		}
	}
	return once
}

// countCollidable reports how many entities the space offers as collision candidates.
func countCollidable(space *aabbworld.Space) int {
	box := geom.NewAABBAt(geom.NewVec(0, 0), ScreenWidth-1, ScreenHeight-1)
	seen := map[uid.UID64]struct{}{}
	space.Query(box, aabbworld.CanCollide, func(id uid.UID64) {
		seen[id] = struct{}{}
	})
	return len(seen)
}

func TestSaveLoadCycle(t *testing.T) {
	path := t.TempDir() + "/save.bin"

	const count = 5
	cfg := world.Config{
		Space:    world.SpaceCfg{Width: ScreenWidth, Height: ScreenHeight, Edges: aabbworld.Torus},
		Entities: world.EntitiesCfg{MaxCount: count, MinSize: RectSize, MaxSize: RectSize},
	}

	ecs := goke.New()
	wp := world.NewPlugin(cfg)
	placement := world.NewGridPlacement(ScreenWidth, ScreenHeight, RectSize)
	motion := newRandomVelocity(200, 50, 10)
	defineKinds := func(wp *world.Plugin) []kind.Entry {
		var entries []kind.Entry
		for i := range count {
			of := kind.Define[body](wp.Kinds(), fmt.Sprintf("k%d", i), kind.Spec{
				comp.Load(func(b body) world.Position { return b.pos }),
				comp.Load(func(b body) world.Velocity { return b.vel }),
				comp.Const(collision.Collider{}),
			})
			entries = append(entries, of.Entry(body{pos: placement.Place(i, count), vel: motion.initialVelocity(i)}))
		}
		return entries
	}
	wp.Seed(defineKinds(wp)...)
	if err := wp.Populate(); err != nil {
		t.Fatalf("Populate: %v", err)
	}
	cm := collision.New(wp.Space(), ecs)

	ctx := &testInstallCtx{ecs: ecs}
	if err := wp.Install(ctx); err != nil {
		t.Fatalf("Install: %v", err)
	}

	var origIDs []uint64
	var origAppearance map[uint64]render.SpriteID
	var systems []goke.System
	for _, produce := range ctx.pending {
		systems = append(systems, produce()...)
	}
	systems = append(systems,
		goke.SystemFn{OnInit: func(si *goke.SysInit) {
			var posQ goke.Comp[world.Base]
			var appQ goke.Comp[world.Appearance]
			q := si.NewQueryBuilder(&posQ, &appQ).Build()
			origAppearance = make(map[uint64]render.SpriteID)
			q.All()
			for q.Next() {
				cur := q.Cursor()
				appearances := appQ.Slice(cur)
				for i, id := range cur.IDs {
					origIDs = append(origIDs, uint64(id))
					origAppearance[uint64(id)] = appearances[i].SpriteID
				}
			}
		}},
	)
	ecs.Setup(systems...)
	cm.RegSystems(ecs)

	if len(origIDs) != count {
		t.Fatalf("spawned %d entities, want %d", len(origIDs), count)
	}

	ecs.SetPlan(cm.RunPlan)
	ecs.Tick(time.Millisecond)
	if got := countCollidable(wp.Space()); got != count {
		t.Errorf("%d of %d spawned entities can collide after a tick — the broad phase did not tell the index", got, count)
	}

	ecs.Pause()
	if err := ecs.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	ecs.Resume()

	ecs2 := goke.New()
	plugin2 := world.NewPlugin(cfg)
	defineKinds(plugin2)
	cm2 := collision.New(plugin2.Space(), ecs2)

	ctx2 := &testInstallCtx{ecs: ecs2}
	if err := plugin2.Install(ctx2); err != nil {
		t.Fatalf("Install: %v", err)
	}

	comps := goke.ProvidedComps(append([]any{cm2}, ctx2.tracked...)...)
	if err := ecs2.Load(path, eachOnce(comps)...); err != nil {
		t.Fatalf("Load: %v", err)
	}
	cm2.RegSystems(ecs2)

	var postLoad []goke.System
	for _, v := range append([]any{cm2}, ctx2.tracked...) {
		if pl, ok := v.(plugin.PostLoader); ok {
			postLoad = append(postLoad, pl.PostLoad())
		}
	}

	var loadedCount int
	postLoad = append(postLoad, goke.SystemFn{OnInit: func(si *goke.SysInit) {
		var posQ goke.Comp[world.Base]
		var appQ goke.Comp[world.Appearance]
		q := si.NewQueryBuilder(&posQ, &appQ).Build()
		q.All()
		for q.Next() {
			cur := q.Cursor()
			appearances := appQ.Slice(cur)
			for i, id := range cur.IDs {
				wantSprite, ok := origAppearance[uint64(id)]
				if !ok {
					t.Errorf("entity %d: not among originally spawned IDs", id)
				} else if appearances[i].SpriteID != wantSprite {
					t.Errorf("entity %d: SpriteID = %d, want %d", id, appearances[i].SpriteID, wantSprite)
				}
				loadedCount++
			}
		}
	}})
	ecs2.Setup(postLoad...)

	if loadedCount != count {
		t.Fatalf("loaded %d entities, want %d", loadedCount, count)
	}

	ecs2.SetPlan(cm2.RunPlan)
	ecs2.Tick(time.Millisecond)
	if got := countCollidable(plugin2.Space()); got != count {
		t.Errorf("%d of %d loaded entities can collide after a tick — PostLoad left them marked as already indexed", got, count)
	}
}
