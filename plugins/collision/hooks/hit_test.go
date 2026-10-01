package hooks_test

import (
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugin/host"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/collision/hooks"
	"github.com/kjkrol/gram/plugins/collision/internal/collisiontest"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/entity/kind"
	"github.com/kjkrol/gram/plugins/world/entity/kind/comp"
	"github.com/kjkrol/gram/plugins/world/rule/effect"
	"github.com/kjkrol/uid"
)

// box is a hit test's collider: where it starts and how fast it goes right.
type box struct{ x, vx float64 }

// hits is a world with collision, boxes at their places, the hit effect cast by ShowHits and a
// host of HitOverlay to draw with.
type hits struct {
	ecs     *goke.ECS
	w       *world.Plugin
	hit     effect.Effect
	ids     []uid.UID64
	drawing *host.EachHost[world.Drawing]
	drawn   *goke.Query
	base    goke.Comp[world.Base]
}

var flash = world.Appearance{SpriteID: 2}

func newHits(t *testing.T, boxes ...box) *hits {
	t.Helper()
	h := &hits{drawing: &host.EachHost[world.Drawing]{}}
	h.w = world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 1000, Height: 1000},
		Entities: world.EntitiesCfg{MaxCount: len(boxes), MinSize: 10, MaxSize: 10},
	})
	h.hit = hooks.Hit(h.w, 100*time.Millisecond)
	if err := h.drawing.Add(hooks.HitOverlay(h.hit, flash)); err != nil {
		t.Fatal(err)
	}
	c := collision.NewPlugin(h.w)
	if err := c.Hook(hooks.ShowHits(h.hit)); err != nil {
		t.Fatal(err)
	}
	unit := kind.Define[box](h.w.Kinds(), "box", kind.Spec{
		comp.Load(func(b box) world.Position { return world.Position{AABB: plane.NewAABB(geom.NewVec(b.x, 100), 10, 10)} }),
		comp.Load(func(b box) world.Velocity { return world.Velocity{Dir: geom.NewVec(1, 0), Value: b.vx} }),
		comp.Const(collision.Collider{}),
	})
	for _, b := range boxes {
		h.w.Seed(unit.Entry(b))
	}
	if err := h.w.Populate(); err != nil {
		t.Fatal(err)
	}
	h.ecs = collisiontest.Start(t, h.w, c, goke.SystemFn{OnInit: func(si *goke.SysInit) {
		qb := si.NewQueryBuilder(&h.base)
		h.drawing.Bind(qb)
		h.drawn = qb.Build()
	}})
	for h.drawn.All(); h.drawn.Next(); {
		h.ids = append(h.ids, h.drawn.Cursor().IDs...)
	}
	return h
}

func (h *hits) tick(n int) {
	for range n {
		h.ecs.Tick(time.Second / 60)
	}
}

// under reports which boxes are under the hit, in the order made.
func (h *hits) under() []bool {
	var out []bool
	for _, id := range h.ids {
		out = append(out, h.hit.On(id))
	}
	return out
}

// overlaid reports which boxes HitOverlay draws the flash on, in the order made.
func (h *hits) overlaid() []bool {
	on := map[uid.UID64]bool{}
	for h.drawn.All(); h.drawn.Next(); {
		cur := h.drawn.Cursor()
		bases := h.base.Slice(cur)
		layers := make([][]world.Appearance, len(cur.IDs))
		h.drawing.Run(plugin.Tick{}, cur, func(i int) world.Drawing {
			layers[i] = []world.Appearance{{SpriteID: 1}}
			return world.Drawing{ID: cur.IDs[i], Base: &bases[i], Layers: &layers[i]}
		})
		for i, id := range cur.IDs {
			on[id] = len(layers[i]) == 2 && layers[i][1] == flash
		}
	}
	var out []bool
	for _, id := range h.ids {
		out = append(out, on[id])
	}
	return out
}

// Two boxes striking each other are under the hit and, once it begins, drawn with the flash by its
// marker; one far off is not; parted, the hit lasts its while of game time and goes.
func TestShowHits_AStrikerIsUnderTheHitForItsWhile(t *testing.T) {
	h := newHits(t, box{x: 100, vx: 600}, box{x: 105}, box{x: 500})
	h.tick(2)
	if got := h.under(); !got[0] || !got[1] || got[2] {
		t.Fatalf("under the hit %v, want the two that struck, not the one far off", got)
	}
	h.tick(1) // the hit begins: its marker goes on
	if got := h.overlaid(); !got[0] || !got[1] || got[2] {
		t.Errorf("drawn with the flash %v, want the two under the hit", got)
	}
	h.tick(20) // parted 30 ms in, the hit lasts 100 ms
	if got := h.under(); got[0] || got[1] || got[2] {
		t.Errorf("under the hit %v a third of a second on, want none: it lasts 100 ms", got)
	}
	if got := h.overlaid(); got[0] || got[1] || got[2] {
		t.Errorf("drawn with the flash %v with no hit on, want none", got)
	}
}
