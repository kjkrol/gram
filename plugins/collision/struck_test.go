package collision_test

import (
	"testing"
	"time"

	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugin/host"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/collision/internal/collisiontest"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

// A Struck comes to the two that touched, the tick after, and never to one that struck nothing.
func TestStruck_ComesOnlyToWhoStruckSomething(t *testing.T) {
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 1000, Height: 1000},
		Entities: world.EntitiesCfg{MaxCount: 3, MinSize: 10, MaxSize: 10},
	})
	c := collision.NewPlugin(w)
	told := map[uid.UID64]int{}
	if err := c.Hook(host.Every(func(_ plugin.Tick, s collision.Struck) {
		if len(s.Contacts) == 0 {
			t.Errorf("entity %v told it struck, with nothing struck", s.ID)
		}
		told[s.ID]++
	})); err != nil {
		t.Fatal(err)
	}
	town := kind.Define[float64](w.Kinds(), "town", kind.Spec{
		comp.Load(func(x float64) world.Position { return posAt(x, 100, 10, 10) }),
		comp.Const(world.Velocity{}),
		comp.Const(collision.Collider{}),
	})
	w.Seed(town.Entry(100), town.Entry(105), town.Entry(500))
	if err := w.Populate(); err != nil {
		t.Fatal(err)
	}
	ecs := collisiontest.Start(t, w, c)
	for range 2 {
		ecs.Tick(time.Second / 60)
	}
	if len(told) != 2 {
		t.Errorf("told %d entities they struck (%v), want the two touching", len(told), told)
	}
}
