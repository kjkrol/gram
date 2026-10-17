package gravity_test

import (
	"strings"
	"testing"

	"github.com/kjkrol/gram/plugins/board/ground"
	"github.com/kjkrol/gram/plugins/gravity"
	"github.com/kjkrol/gram/plugins/motion"
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
	over := func(heights bool, cfg gravity.Config) *gravity.Plugin {
		w := world.NewPlugin(world.Config{
			Space:    world.SpaceCfg{Width: 40, Height: 40},
			Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10},
			Heights:  heights,
		})
		return gravity.NewPlugin(w, motion.NewPlugin(w), cfg)
	}
	expectPanic(t, "not both", func() {
		over(true, gravity.Config{Pull: gravity.Pull{Y: 10, Z: -10}})
	})
	expectPanic(t, "flat world", func() {
		over(false, gravity.Config{Pull: gravity.Pull{Z: -10}})
	})
	expectPanic(t, "no surfaces", func() {
		over(true, gravity.Config{Pull: gravity.Pull{Y: 10}}).
			WithSurfaces(func() ground.Surfaces { return nil })
	})
	expectPanic(t, "motion", func() {
		w := world.NewPlugin(world.Config{
			Space:    world.SpaceCfg{Width: 40, Height: 40},
			Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10},
		})
		gravity.NewPlugin(w, nil, gravity.Config{})
	})
}
