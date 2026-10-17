package world_test

import (
	"strings"
	"testing"
	"time"

	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/world"
)

type stamp struct{ By string }

// asking is a plugin giving every unit a stamp of its name
type asking struct{ name string }

var _ plugin.KindTemplater = asking{}

func (a asking) Name() string                     { return a.name }
func (asking) Install(plugin.Installer) error     { return nil }
func (asking) RunPlan(goke.RunCtx, time.Duration) {}
func (asking) EventHandler() control.EventHandler { return nil }
func (asking) Serializable() plugin.Serializable  { return nil }
func (a asking) KindTemplates(r *kind.Roster) {
	r.Default(world.UnitTemplate, comp.Const(stamp{By: a.name}))
}

func TestRoster_AsksTheWorldAndEveryPluginMadeInTheOrderMade(t *testing.T) {
	w := world.NewPlugin(testWorldConfig())
	world.NewSelf(w, asking{"first"})
	world.NewSelf(w, asking{"second"})
	spec := w.Roster().Spec(world.UnitTemplate, comp.Const(world.Position{}))
	var stamps []string
	for _, c := range spec {
		if s, ok := c.(comp.Template[stamp]); ok {
			stamps = append(stamps, s.Resolve(nil).By)
		}
	}
	// asked in the order made, the first default of a type kept
	if len(stamps) != 1 || stamps[0] != "first" {
		t.Errorf("a unit is stamped %v, want by the plugin made first", stamps)
	}
	if len(spec) < 5 {
		t.Errorf("a unit's spec is %d components, want the world's own and the stamp", len(spec))
	}
	defer func() {
		if msg, _ := recover().(string); !strings.Contains(msg, `"third"`) {
			t.Errorf("a plugin asking of the templates made after they were read panics %q, "+
				"want it refused by name", msg)
		}
	}()
	world.NewSelf(w, asking{"third"})
}
