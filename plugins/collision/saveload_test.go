package collision_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/collision/internal/collisiontest"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/uid"
)

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

// the world the cycle runs in, a few colliders in it
const (
	screenWidth  = 1024
	screenHeight = 1024
	rectSize     = uint32(5)
)

// body is a collider's row: where it starts and how it moves.
type body struct {
	pos world.Position
	vel world.Velocity
}

// countCollidable reports how many entities the space offers as collision candidates.
func countCollidable(space *aabbworld.Space) int {
	box := geom.NewAABBAt(geom.NewVec(0, 0), screenWidth-1, screenHeight-1)
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
		Space:    world.SpaceCfg{Width: screenWidth, Height: screenHeight, Edges: aabbworld.Torus},
		Entities: world.EntitiesCfg{MaxCount: count, MinSize: rectSize, MaxSize: rectSize},
	}

	ecs := goke.New()
	wp := world.NewPlugin(cfg)
	placement := world.NewGridPlacement(screenWidth, screenHeight, rectSize)
	defineKinds := func(wp *world.Plugin) []kind.Entry {
		var entries []kind.Entry
		for i := range count {
			kind.Define[body](wp.Kinds(), fmt.Sprintf("k%d", i), kind.Spec{
				comp.Load(func(b body) world.Position { return b.pos }),
				comp.Load(func(b body) world.Velocity { return b.vel }),
				comp.Const(collision.Collider{}),
			})
			of := kind.Named[body](wp.Kinds(), fmt.Sprintf("k%d", i))
			entries = append(entries, of.Entry(body{pos: placement.Place(i, count), vel: world.Velocity{Dir: geom.NewVec(1, 0), Value: float64(10 * (i + 1))}}))
		}
		return entries
	}
	wp.Seed(defineKinds(wp)...)
	if err := wp.Populate(); err != nil {
		t.Fatalf("Populate: %v", err)
	}
	cm := collision.New(wp.Space(), ecs)

	ctx := collisiontest.NewInstallCtx(ecs)
	if err := wp.Install(ctx); err != nil {
		t.Fatalf("Install: %v", err)
	}

	var origIDs []uint64
	var origAppearance map[uint64]render.SpriteID
	systems := append(ctx.Systems(),
		goke.SystemFn{OnInit: func(si *goke.SysInit) {
			var posQ goke.Comp[world.Base]
			var appQ goke.Comp[render.Appearance]
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

	ctx2 := collisiontest.NewInstallCtx(ecs2)
	if err := plugin2.Install(ctx2); err != nil {
		t.Fatalf("Install: %v", err)
	}

	comps := goke.ProvidedComps(append([]any{cm2}, ctx2.Tracked()...)...)
	if err := ecs2.Load(path, eachOnce(comps)...); err != nil {
		t.Fatalf("Load: %v", err)
	}
	cm2.RegSystems(ecs2)

	var postLoad []goke.System
	for _, v := range append([]any{cm2}, ctx2.Tracked()...) {
		if pl, ok := v.(plugin.PostLoader); ok {
			postLoad = append(postLoad, pl.PostLoad())
		}
	}

	var loadedCount int
	postLoad = append(postLoad, goke.SystemFn{OnInit: func(si *goke.SysInit) {
		var posQ goke.Comp[world.Base]
		var appQ goke.Comp[render.Appearance]
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
