package vision_test

import (
	"testing"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/vision"
)

func TestModule_DeclaresEveryComponentItOwns(t *testing.T) {
	p := vision.NewPlugin(testWorldPlugin())
	ctx := &installCtx{ecs: goke.New()}
	if err := p.Install(ctx); err != nil {
		t.Fatalf("Install: %v", err)
	}

	got := len(goke.ProvidedComps(ctx.tracked...))
	if got != 4 {
		t.Errorf("vision declares %d components, want 4 (Sight, Sighted, SightOutline, Transparency)", got)
	}
}

func TestModule_RegSystemsIsIdempotent(t *testing.T) {
	p := vision.NewPlugin(testWorldPlugin())
	ctx := &installCtx{ecs: goke.New()}
	if err := p.Install(ctx); err != nil {
		t.Fatalf("Install: %v", err)
	}

	for _, m := range ctx.tracked {
		mod := m.(goke.Module)
		mod.RegSystems(ctx.ecs)
		mod.RegSystems(ctx.ecs)
	}
}
