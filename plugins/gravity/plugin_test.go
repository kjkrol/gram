package gravity_test

import (
	"strings"
	"testing"

	"github.com/kjkrol/gram/plugins/board/ground"
	"github.com/kjkrol/gram/plugins/gravity"
	"github.com/kjkrol/gram/plugins/world"
)

func expectPanic(t *testing.T, want string, run func()) {
	t.Helper()
	defer func() {
		if msg, _ := recover().(string); !strings.Contains(msg, want) {
			t.Errorf("panic %q, want one mentioning %q", msg, want)
		}
	}()
	run()
	t.Errorf("no panic, want one mentioning %q", want)
}

func TestNewPlugin_RefusesAPullItsWorldCannotKeep(t *testing.T) {
	newWorld := func(heights bool) *world.Plugin {
		return world.NewPlugin(world.Config{
			Space:    world.SpaceCfg{Width: 40, Height: 40},
			Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10},
			Heights:  heights,
		})
	}
	expectPanic(t, "not both", func() {
		gravity.NewPlugin(newWorld(true), gravity.Config{Pull: gravity.Pull{Y: 10, Z: -10}})
	})
	expectPanic(t, "flat world", func() {
		gravity.NewPlugin(newWorld(false), gravity.Config{Pull: gravity.Pull{Z: -10}})
	})
	expectPanic(t, "no surfaces", func() {
		gravity.NewPlugin(newWorld(true), gravity.Config{Pull: gravity.Pull{Y: 10}}).
			WithSurfaces(func() ground.Surfaces { return nil })
	})
}
