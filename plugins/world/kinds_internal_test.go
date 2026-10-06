package world

import (
	"github.com/kjkrol/gram/render"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/entity/kind/comp"
)

type spawnerTag struct{}

type spawnerStat struct{ HP int }

type propData struct{ x float64 }

func spawnerTestPos() Position {
	return Position{AABB: plane.NewAABB(geom.NewVec(0, 0), 10, 10)}
}

// statSpec is a kind whose rows are an int: the HP its one component spawns with.
func statSpec() kind.Spec {
	return kind.Spec{
		comp.Const(spawnerTestPos()),
		comp.Const(Velocity{}),
		comp.Load(func(hp int) spawnerStat { return spawnerStat{HP: hp} }),
	}
}

func setupWorld(wm *module, onInit func(si *goke.SysInit)) {
	goke.New().Setup(append(wm.SetupSystems(), goke.SystemFn{OnInit: onInit})...)
}

func testPlugin() *Plugin { return NewPlugin(testWorld().config) }

func TestPopulate_ConstAndLoadComponents(t *testing.T) {
	p := testPlugin()
	p.Kinds().NewSprite()
	kind.Define[int](p.Kinds(), "unit", append(statSpec(), comp.Const(spawnerTag{})))
	unit := kind.Named[int](p.Kinds(), "unit")
	p.Seed(unit.Entry(9), unit.Entry(4))
	if err := p.Populate(); err != nil {
		t.Fatalf("Populate: %v", err)
	}

	var appearance goke.Comp[render.Appearance]
	var stat goke.Comp[spawnerStat]
	var q *goke.Query
	setupWorld(p.module, func(si *goke.SysInit) {
		q = si.NewQueryBuilder(&appearance, &stat).Include(goke.Include[spawnerTag]()).Build()
	})

	var hps []int
	q.All()
	for q.Next() {
		cur := q.Cursor()
		appearances, stats := appearance.Slice(cur), stat.Slice(cur)
		for i := range cur.IDs {
			if appearances[i].SpriteID != unit.SpriteID() {
				t.Errorf("SpriteID = %v, want the kind's %v", appearances[i].SpriteID, unit.SpriteID())
			}
			hps = append(hps, stats[i].HP)
		}
	}
	if len(hps) != 2 || hps[0] != 9 || hps[1] != 4 {
		t.Errorf("HP per entity = %v, want [9 4] (read from each entity's row)", hps)
	}
}

func TestPopulate_KindsWithDifferentRowsAndComponents(t *testing.T) {
	p := testPlugin()
	kind.Define[int](p.Kinds(), "unit", statSpec())
	unit := kind.Named[int](p.Kinds(), "unit")
	kind.Define[propData](p.Kinds(), "prop", kind.Spec{
		comp.Load(func(d propData) Position {
			return Position{AABB: plane.NewAABB(geom.NewVec(d.x, 0), 10, 10)}
		}),
		comp.Const(Velocity{}),
		comp.Const(spawnerTag{}),
	})
	prop := kind.Named[propData](p.Kinds(), "prop")
	p.Seed(unit.Entry(5), prop.Entry(propData{x: 40}), unit.Entry(6))
	if err := p.Populate(); err != nil {
		t.Fatalf("Populate: %v", err)
	}

	var stat goke.Comp[spawnerStat]
	var tagBase goke.Comp[Base]
	var units int
	var propX []float64
	setupWorld(p.module, func(si *goke.SysInit) {
		uq := si.NewQueryBuilder(&stat).Build()
		for uq.All(); uq.Next(); {
			units += len(uq.Cursor().IDs)
		}
		pq := si.NewQueryBuilder(&tagBase).Include(goke.Include[spawnerTag]()).Build()
		for pq.All(); pq.Next(); {
			for _, b := range tagBase.Slice(pq.Cursor()) {
				propX = append(propX, b.Pos.TopLeft.X)
			}
		}
	})

	if units != 2 {
		t.Errorf("units = %d, want 2", units)
	}
	if len(propX) != 1 || propX[0] != 40 {
		t.Errorf("prop positions X = %v, want [40] (read from its own row type)", propX)
	}
}

func TestPlugin_Populate_EntryOfAKindThisWorldDoesNotHold_ErrorsWithoutSpawning(t *testing.T) {
	p := testPlugin()
	kind.Define[int](p.Kinds(), "unit", statSpec())
	unit := kind.Named[int](p.Kinds(), "unit")
	elsewhere := newKinds(false)
	kind.Define[int](elsewhere, "stranger", statSpec())
	stranger := kind.Named[int](elsewhere, "stranger")
	p.Seed(unit.Entry(1), stranger.Entry(1), kind.Entry{})

	if err := p.Populate(); err == nil {
		t.Fatal("Populate: expected an error for an entry of a kind this world was never given")
	}
	if n := len(p.module.SetupSystems()); n != 0 {
		t.Errorf("queued %d spawns, want 0", n)
	}
}
