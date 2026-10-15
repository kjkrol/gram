package atmosphere

import (
	"strings"
	"testing"

	"github.com/kjkrol/gram/plugins/world"
)

func TestNewPlugin_RefusesAFlatWorld(t *testing.T) {
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 128, Height: 128},
		Entities: world.EntitiesCfg{MaxCount: 4, MinSize: 1, MaxSize: 20},
	})
	defer func() {
		if msg, _ := recover().(string); !strings.Contains(msg, "world.Config.Heights") {
			t.Errorf("panic %q, want one asking for world.Config.Heights", msg)
		}
	}()
	NewPlugin(w, Config{})
	t.Error("an atmosphere over a flat world: want a panic")
}
