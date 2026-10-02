package kinds

import (
	"strings"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/entity/kind/comp"
)

type spawnerTag struct{}

type spawnerStat struct{ HP int }

func spawnerTestPos() entity.Position {
	return entity.Position{AABB: plane.NewAABB(geom.NewVec(0, 0), 10, 10)}
}

// statSpec is a kind whose rows are an int: the HP its one component spawns with.
func statSpec() kind.Spec {
	return kind.Spec{
		comp.Const(spawnerTestPos()),
		comp.Const(entity.Velocity{}),
		comp.Load(func(hp int) spawnerStat { return spawnerStat{HP: hp} }),
	}
}

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
	kinds := New(false)
	red := kind.Define[int](kinds, "red", statSpec())
	overlay := kinds.NewSprite()
	blue := kind.Define[int](kinds, "blue", statSpec())

	if red.SpriteID() != 0 || overlay != 1 || blue.SpriteID() != 2 {
		t.Errorf("sprites = red %v, overlay %v, blue %v, want 0, 1, 2 — issued in call order", red.SpriteID(), overlay, blue.SpriteID())
	}
	if got := kinds.entries["blue"].TypeID; got != 1 {
		t.Errorf("blue's TypeID = %d, want 1 — a sprite with no kind takes no TypeID", got)
	}
}

func TestKinds_Define_NeedsOnePositionAndOneVelocity(t *testing.T) {
	for name, spec := range map[string]kind.Spec{
		"no Position":   {comp.Const(entity.Velocity{})},
		"no Velocity":   {comp.Const(spawnerTestPos())},
		"two Positions": {comp.Const(spawnerTestPos()), comp.Const(spawnerTestPos()), comp.Const(entity.Velocity{})},
	} {
		t.Run(name, func(t *testing.T) {
			msg := panicsWith(t, func() { kind.Define[int](New(false), "unit", spec) })
			if !strings.Contains(msg, `"unit"`) {
				t.Errorf("panic %q does not name the kind", msg)
			}
		})
	}
}

func TestKinds_Define_RefusesANameTwice(t *testing.T) {
	kinds := New(false)
	kind.Define[int](kinds, "unit", statSpec())

	panicsWith(t, func() { kind.Define[int](kinds, "unit", statSpec()) })
}

func TestKinds_LoadComps_ListsWhatItsKindsCarryEachOnce(t *testing.T) {
	kinds := New(false)
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
