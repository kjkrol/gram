package collision_test

import (
	"testing"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/collision"
)

func TestModule_LoadComps_ListsOwnedComponents(t *testing.T) {
	p := collision.New(testSpace(t), goke.New())

	tokens := p.LoadComps()
	if len(tokens) != 3 {
		t.Fatalf("LoadComps() returned %d tokens, want 3", len(tokens))
	}
}

func TestModule_RegSystems_IsIdempotent(t *testing.T) {
	ecs := goke.New()
	p := collision.New(testSpace(t), ecs)

	p.RegSystems(ecs)
	p.RegSystems(ecs)
}
