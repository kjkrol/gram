package collision_test

import (
	"testing"
	"time"

	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/collision/internal/collisiontest"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/uid"
)

// A Struck comes to the two that touched, the tick after, and never to one that struck nothing.
func TestStruck_ComesOnlyToWhoStruckSomething(t *testing.T) {
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 1000, Height: 1000},
		Entities: world.EntitiesCfg{MaxCount: 3, MinSize: 10, MaxSize: 10},
	})
	var orders heards
	if err := w.Carry(&orders); err != nil {
		t.Fatal(err)
	}
	c := collision.NewPlugin(w)
	if err := c.Hook(rule.Then[collision.Struck]("struck", rule.All, rule.OneOf(
		rule.If(func(s collision.Struck) bool { return len(s.Contacts) == 0 }, rule.Order(heard{Rule: "nothing struck"})),
		rule.Order(heard{Rule: "struck"}),
	))); err != nil {
		t.Fatal(err)
	}
	kind.Define[float64](w.Kinds(), "town", kind.Spec{
		comp.Load(func(x float64) world.Position { return posAt(x, 100, 10, 10) }),
		comp.Const(world.Velocity{}),
		comp.Const(collision.Collider{}),
	})
	town := kind.Named[float64](w.Kinds(), "town")
	w.Seed(town.Entry(100), town.Entry(105), town.Entry(500))
	if err := w.Populate(); err != nil {
		t.Fatal(err)
	}
	ecs := collisiontest.Start(t, w, c)
	for range 2 {
		ecs.Tick(time.Second / 60)
	}
	told := map[uid.UID64]int{}
	for _, h := range orders.given() {
		if h.Command.Rule != "struck" {
			t.Errorf("entity %v told it struck, with nothing struck", h.Entity)
		}
		told[h.Entity]++
	}
	if len(told) != 2 {
		t.Errorf("told %d entities they struck (%v), want the two touching", len(told), told)
	}
}
