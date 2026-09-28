package world_test

import (
	"github.com/kjkrol/aabbworld"
	"testing"

	"github.com/kjkrol/gram/plugins/world"
)

func TestPlugin_Res_PublishesConfig(t *testing.T) {
	cfg := world.Config{
		Space:    world.SpaceCfg{Width: 100, Height: 100, Edges: aabbworld.Torus},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10},
	}
	plugin := world.NewPlugin(cfg)

	if got := plugin.Res.Config; got.Space != cfg.Space || got.Entities != cfg.Entities || got.Camera != cfg.Camera || got.Heights != cfg.Heights {
		t.Errorf("Res.Config = %+v, want %+v", plugin.Res.Config, cfg)
	}
	if plugin.Res.Telemetry.Count != 0 {
		t.Errorf("Res.Telemetry.Count = %d, want 0 (nothing populated)", plugin.Res.Telemetry.Count)
	}
}
