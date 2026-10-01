package collision_test

import (
	"testing"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugin/host"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/collision/internal/collisiontest"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/entity/kind"
	"github.com/kjkrol/gram/plugins/world/entity/kind/comp"
	"github.com/kjkrol/gram/plugins/world/entity/tag"
	"github.com/kjkrol/uid"
)

func TestCollider_AttachedAndDetachedMidGame(t *testing.T) {
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 1000, Height: 1000},
		Entities: world.EntitiesCfg{MaxCount: 8, MinSize: 10, MaxSize: 10},
	})
	c := collision.NewPlugin(w)
	contacts := 0
	if err := c.Hook(host.Pair(tag.Any, tag.Any, func(plugin.Tick, collision.Meeting) { contacts++ })); err != nil {
		t.Fatalf("Hook: %v", err)
	}

	town := kind.Define[float64](w.Kinds(), "town", kind.Spec{
		comp.Load(func(x float64) world.Position { return posAt(x, 100, 10, 10) }),
		comp.Const(world.Velocity{}),
		comp.Const(collision.Collider{}),
	})
	w.Seed(town.Entry(100), town.Entry(105))
	if err := w.Populate(); err != nil {
		t.Fatalf("Populate: %v", err)
	}

	var first uid.UID64
	var edit func(cb *goke.CmdBuf)
	ecs := collisiontest.Start(t, w, c, goke.SystemFn{OnInit: func(si *goke.SysInit) {
		var base goke.Comp[world.Base]
		q := si.NewQueryBuilder(&base).Build()
		q.All()
		q.Next()
		first = q.Cursor().IDs[0]
	}})

	editor := ecs.RegSys(goke.SystemFn{OnUpdate: func(cb *goke.CmdBuf, _ time.Duration) {
		if edit != nil {
			edit(cb)
			edit = nil
		}
	}})
	step := collisiontest.Step(w, c)
	ecs.SetPlan(func(rc goke.RunCtx, d time.Duration) {
		rc.Run(editor, d)
		rc.Sync()
		step(rc, d)
	})

	collidable := func() int {
		return w.Space().Query(geom.NewAABBAt(geom.NewVec(90, 90), 40, 40), aabbworld.CanCollide, func(uid.UID64) {})
	}
	tick := func() int {
		contacts = 0
		ecs.Tick(time.Second / 60)
		return contacts
	}

	if got := tick(); got != 1 {
		t.Fatalf("%d contacts on the first tick, want 1 — carrying Collider is all it should take", got)
	}

	edit = func(cb *goke.CmdBuf) { w.Detach[collision.Collider](cb, first) }
	if got := tick(); got != 0 {
		t.Errorf("%d contacts on the tick Collider came off, want 0", got)
	}
	if got := collidable(); got != 1 {
		t.Errorf("the space lists %d collidable entities after the tick Collider came off, want the other one", got)
	}
	if got := tick(); got != 0 {
		t.Errorf("%d contacts a tick later, want 0", got)
	}

	edit = func(cb *goke.CmdBuf) { w.Attach(cb, first, collision.Collider{}) }
	if got := tick(); got != 1 {
		t.Errorf("%d contacts on the tick Collider went back on, want 1", got)
	}
}
