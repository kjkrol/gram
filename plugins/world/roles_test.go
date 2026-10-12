package world_test

import (
	"strings"
	"testing"

	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/rule"
)

func TestRoles_AreTheStagesOwn(t *testing.T) {
	forest, arena := world.NewPlugin(testWorldConfig()), world.NewPlugin(testWorldConfig())
	forest.Roles().Define("mortal",
		rule.Branch[world.Moving]{Name: "drown", Do: rule.Order(world.Despawn{})})
	arena.Roles().Define("mortal",
		rule.Branch[world.Moving]{Name: "fall", Do: rule.Order(world.Despawn{})},
		rule.Branch[world.Leaving]{Name: "leave", Do: rule.Order(world.Despawn{})})

	a, b := forest.Roles().Named("mortal"), arena.Roles().Named("mortal")
	if len(a.Rules()) != 1 || len(b.Rules()) != 2 {
		t.Errorf("the forest's mortal obeys %d rules and the arena's %d; want 1 and 2, each its own", len(a.Rules()), len(b.Rules()))
	}
	if forest.Roles().Named("mortal") != a {
		t.Error("Named twice hands back two roles, want the one defined")
	}
	if got := a.String(); got != "the role mortal" {
		t.Errorf("the role says %q", got)
	}
}

func TestRoles_TagsAreHandedOutByTheWorld(t *testing.T) {
	w := world.NewPlugin(testWorldConfig())
	w.Roles().Define("first")
	w.Roles().Define("second")
	other := world.NewPlugin(testWorldConfig())
	other.Roles().Define("second")
	if f, s := w.Roles().Named("first").Tag(), w.Roles().Named("second").Tag(); f == s {
		t.Errorf("two roles of one world share the tag %d", f)
	}
	if got := other.Roles().Named("second").Tag(); got != w.Roles().Named("first").Tag() {
		t.Errorf("another world's first role has the tag %d, want its own first", got)
	}
}

func TestRoles_RefuseWhatIsNotDefinedOnce(t *testing.T) {
	w := world.NewPlugin(testWorldConfig())
	w.Roles().Define("mortal")
	for name, c := range map[string]struct {
		do   func()
		want string
	}{
		"defined twice": {func() { w.Roles().Define("mortal") }, `"mortal" is defined already`},
		"unknown":       {func() { w.Roles().Named("hasty") }, `no role is defined as "hasty"`},
		"no name":       {func() { w.Roles().Define("") }, "needs a name"},
	} {
		if msg := panicMessage(t, c.do); !strings.Contains(msg, c.want) {
			t.Errorf("%s: panic %q, want it to say %s", name, msg, c.want)
		}
	}
}
