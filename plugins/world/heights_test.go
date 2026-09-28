package world_test

import (
	"strings"
	"testing"

	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/kind"
	"github.com/kjkrol/gram/plugins/world/kind/comp"
)

func TestHeights_AreTheConfigsChoice(t *testing.T) {
	cfg := world.Config{Space: world.SpaceCfg{Width: 100, Height: 100}, Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 10, MaxSize: 10}}
	if flat := world.NewPlugin(cfg); flat.HasHeights() {
		t.Error("a world by default has heights, want it flat")
	}
	cfg.Heights = true
	if tall := world.NewPlugin(cfg); !tall.HasHeights() {
		t.Error("a world given heights has none")
	}
}

func TestKinds_RefuseAZInAFlatWorld(t *testing.T) {
	w := world.NewPlugin(world.Config{Space: world.SpaceCfg{Width: 100, Height: 100}, Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 10, MaxSize: 10}})
	defer func() {
		if msg, _ := recover().(string); !strings.Contains(msg, "Heights") {
			t.Errorf("panic %q, want one pointing at world.Config.Heights", msg)
		}
	}()
	kind.Define[struct{}](w.Kinds(), "tower", kind.Spec{
		comp.Const(world.Position{}), comp.Const(world.Velocity{}), comp.Const(world.Z{Height: 3}),
	})
	t.Error("a flat world took a kind carrying a Z")
}
