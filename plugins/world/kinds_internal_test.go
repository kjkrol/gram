package world

import (
	"strings"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/world/entity/kind"
	"github.com/kjkrol/gram/plugins/world/entity/kind/comp"
	"github.com/kjkrol/uid"
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

// panicsWith runs f and reports what it panicked with, failing the test if it did not.
func panicsWith(t *testing.T, f func()) (msg string) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected a panic")
		}
		msg, _ = r.(string)
	}()
	f()
	return ""
}

func TestKinds_Define_AssignsSpriteIDsByOrder(t *testing.T) {
	kinds := newKinds(false)
	red := kind.Define[int](kinds, "red", statSpec())
	overlay := kinds.NewSprite()
	blue := kind.Define[int](kinds, "blue", statSpec())

	if red.SpriteID() != 0 || overlay != 1 || blue.SpriteID() != 2 {
		t.Errorf("sprites = red %v, overlay %v, blue %v, want 0, 1, 2 — issued in call order", red.SpriteID(), overlay, blue.SpriteID())
	}
	if got := kinds.entries["blue"].typeID; got != 1 {
		t.Errorf("blue's TypeID = %d, want 1 — a sprite with no kind takes no TypeID", got)
	}
}

func TestKinds_Define_NeedsOnePositionAndOneVelocity(t *testing.T) {
	for name, spec := range map[string]kind.Spec{
		"no Position":   {comp.Const(Velocity{})},
		"no Velocity":   {comp.Const(spawnerTestPos())},
		"two Positions": {comp.Const(spawnerTestPos()), comp.Const(spawnerTestPos()), comp.Const(Velocity{})},
	} {
		t.Run(name, func(t *testing.T) {
			msg := panicsWith(t, func() { kind.Define[int](newKinds(false), "unit", spec) })
			if !strings.Contains(msg, `"unit"`) {
				t.Errorf("panic %q does not name the kind", msg)
			}
		})
	}
}

func TestKinds_Define_RefusesANameTwice(t *testing.T) {
	kinds := newKinds(false)
	kind.Define[int](kinds, "unit", statSpec())

	panicsWith(t, func() { kind.Define[int](kinds, "unit", statSpec()) })
}

func TestPopulate_ConstAndLoadComponents(t *testing.T) {
	p := testPlugin()
	p.Kinds().NewSprite()
	unit := kind.Define[int](p.Kinds(), "unit", append(statSpec(), comp.Const(spawnerTag{})))
	p.Seed(unit.Entry(9), unit.Entry(4))
	if err := p.Populate(); err != nil {
		t.Fatalf("Populate: %v", err)
	}

	var appearance goke.Comp[Appearance]
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

func TestPopulate_WithEffect_RunsAfterWriteWithValueAndID(t *testing.T) {
	p := testPlugin()
	var gotHP, calls int
	var gotID uid.UID64
	unit := kind.Define[int](p.Kinds(), "unit", kind.Spec{
		comp.Const(spawnerTestPos()),
		comp.Const(Velocity{}),
		comp.Load(func(hp int) spawnerStat { return spawnerStat{HP: hp} }).
			WithEffect(func(v spawnerStat, id uid.UID64) {
				calls++
				gotHP, gotID = v.HP, id
			}),
	})
	p.Seed(unit.Entry(9))
	if err := p.Populate(); err != nil {
		t.Fatalf("Populate: %v", err)
	}

	var stat goke.Comp[spawnerStat]
	var wantID uid.UID64
	setupWorld(p.module, func(si *goke.SysInit) {
		q := si.NewQueryBuilder(&stat).Build()
		q.All()
		for q.Next() {
			for _, id := range q.Cursor().IDs {
				wantID = id
			}
		}
	})

	if calls != 1 || gotHP != 9 || gotID != wantID {
		t.Errorf("effect calls=%d HP=%d id=%v, want 1, 9, %v", calls, gotHP, gotID, wantID)
	}
}

func TestPopulate_KindsWithDifferentRowsAndComponents(t *testing.T) {
	p := testPlugin()
	unit := kind.Define[int](p.Kinds(), "unit", statSpec())
	prop := kind.Define[propData](p.Kinds(), "prop", kind.Spec{
		comp.Load(func(d propData) Position {
			return Position{AABB: plane.NewAABB(geom.NewVec(d.x, 0), 10, 10)}
		}),
		comp.Const(Velocity{}),
		comp.Const(spawnerTag{}),
	})
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
	unit := kind.Define[int](p.Kinds(), "unit", statSpec())
	stranger := kind.Define[int](newKinds(false), "stranger", statSpec())
	p.Seed(unit.Entry(1), stranger.Entry(1), kind.Entry{})

	if err := p.Populate(); err == nil {
		t.Fatal("Populate: expected an error for an entry of a kind this world was never given")
	}
	if n := len(p.module.SetupSystems()); n != 0 {
		t.Errorf("queued %d spawns, want 0", n)
	}
}

func TestKinds_LoadComps_ListsWhatItsKindsCarryEachOnce(t *testing.T) {
	kinds := newKinds(false)
	if got := kinds.LoadComps(); len(got) != 0 {
		t.Fatalf("an empty registry lists %d component types, want none", len(got))
	}

	for _, name := range []string{"first", "second"} {
		kind.Define[int](kinds, name, append(statSpec(), comp.Const(spawnerTag{})))
	}

	var got []string
	for _, token := range kinds.LoadComps() {
		got = append(got, token.Name)
	}
	want := []string{goke.LoadComp[spawnerStat]().Name, goke.LoadComp[spawnerTag]().Name}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("LoadComps() = %v, want %v — each type once, however many kinds carry it, and neither Position nor Velocity: those are Base's", got, want)
	}
}
