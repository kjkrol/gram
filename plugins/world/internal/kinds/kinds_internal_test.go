package kinds

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/entity/tag"
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

// Two families of tags of the test's own.
type (
	spawnerFamily struct{}
	otherFamily   struct{}
)

func TestKinds_Define_RefusesAComponentTypeTwice(t *testing.T) {
	for name, c := range map[string]struct {
		twice []comp.Comp
		typ   reflect.Type
	}{
		"two Tagged of one family": {
			[]comp.Comp{comp.Tagged(tag.Tag[spawnerFamily](0)), comp.Tagged(tag.Tag[spawnerFamily](1))},
			reflect.TypeFor[tag.Tags[spawnerFamily]](),
		},
		"a Tagged and the Marks of one family": {
			[]comp.Comp{comp.Tagged(tag.Tag[spawnerFamily](0)), comp.Marks[spawnerFamily]()},
			reflect.TypeFor[tag.Tags[spawnerFamily]](),
		},
		"two Const of one type": {
			[]comp.Comp{comp.Const(spawnerTag{}), comp.Const(spawnerTag{})},
			reflect.TypeFor[spawnerTag](),
		},
		"a Const of a type the Spec Loads": {
			[]comp.Comp{comp.Const(spawnerStat{HP: 1})},
			reflect.TypeFor[spawnerStat](),
		},
	} {
		t.Run(name, func(t *testing.T) {
			kinds := New(false)
			msg := panicsWith(t, func() { kind.Define[int](kinds, "unit", append(statSpec(), c.twice...)) })
			if !strings.Contains(msg, `"unit"`) || !strings.Contains(msg, c.typ.String()+" twice") {
				t.Errorf("panic %q does not name the kind and %v twice", msg, c.typ)
			}
			if _, defined := kinds.Kind("unit"); defined {
				t.Error("the refused kind was registered anyway")
			}
		})
	}
}

func TestKinds_Define_TakesOneOfEachComponentType(t *testing.T) {
	kinds := New(false)
	spec := append(statSpec(),
		comp.Tagged(tag.Tag[spawnerFamily](0), tag.Tag[spawnerFamily](3)),
		comp.Tagged(tag.Tag[otherFamily](1)),
		comp.Const(spawnerTag{}),
	)
	kind.Define[int](kinds, "unit", spec)

	r, ok := kinds.Kind("unit")
	if !ok {
		t.Fatal("the kind was not registered")
	}
	var got []reflect.Type
	for _, c := range r.Comps {
		got = append(got, comp.TypeOf(c))
	}
	want := []reflect.Type{
		reflect.TypeFor[spawnerStat](),
		reflect.TypeFor[tag.Tags[spawnerFamily]](),
		reflect.TypeFor[tag.Tags[otherFamily]](),
		reflect.TypeFor[spawnerTag](),
	}
	if !slices.Equal(got, want) {
		t.Errorf("components = %v, want %v — one of each type, families apart", got, want)
	}
	if tags := r.Comps[1].(comp.Template[tag.Tags[spawnerFamily]]).Resolve(0); tags != 0b1001 {
		t.Errorf("the family's tags = %b, want 1001: both bits of the one Tagged", tags)
	}
}
