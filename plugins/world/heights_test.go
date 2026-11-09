package world_test

import (
	"strings"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/world"
)

func TestHeights_AreTheConfigsChoice(t *testing.T) {
	cfg := world.Config{Space: world.SpaceCfg{Width: 100, Height: 100}, Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 10, MaxSize: 10}}
	if flat := world.NewPlugin(cfg); flat.Read(0).HasHeights() {
		t.Error("a world by default has heights, want it flat")
	}
	cfg.Heights = true
	if tall := world.NewPlugin(cfg); !tall.Read(0).HasHeights() {
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

func TestGroundAt_IsZeroWithoutAGroundAndTheGroundsWithOne(t *testing.T) {
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 100, Height: 100},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 10, MaxSize: 10},
		Heights:  true,
	})
	at := geom.NewVec(30, 70)
	if got := w.Read(0).GroundAt(at); got != 0 {
		t.Errorf("a world without a ground has it at %v, want 0", got)
	}
	w.Write().SetGround(func(p geom.Vec) float32 { return p.X + 2*p.Y })
	if got := w.Read(0).GroundAt(at); got != 170 {
		t.Errorf("the ground given lies at %v under %v, want 170, its own height there", got, at)
	}
}
