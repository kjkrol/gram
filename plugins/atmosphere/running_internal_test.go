package atmosphere

import (
	"testing"

	"github.com/kjkrol/gram/plugins/atmosphere/climate"
	"github.com/kjkrol/gram/plugins/world"
)

func TestPlugin_RunningSwitchesTheAtmospheresWorkings(t *testing.T) {
	w := world.NewPlugin(world.Config{Space: world.SpaceCfg{Width: 128, Height: 128}, Entities: world.EntitiesCfg{MaxCount: 4, MinSize: 1, MaxSize: 20}, Heights: true})
	r := AllRunning()
	r.Day, r.Moon, r.Wind, r.Falls = false, false, false, false
	a := NewPlugin(w, nil, Config{Running: &r})
	if !a.write.Sky().Frozen() || a.write.Sky().Moon() {
		t.Errorf("begun without the day and the moon, the light is frozen %v, moonlit %v", a.write.Sky().Frozen(), a.write.Sky().Moon())
	}
	if c := a.write.Climate().Running(); !c.Changes || c.Wind || !c.Clouds || c.Falls {
		t.Errorf("the climate runs %+v, want the changes and the clouds only", c)
	}
	if got := a.read.Running(); got != r {
		t.Errorf("the atmosphere runs %+v, want %+v", got, r)
	}
	a.write.SetRunning(AllRunning())
	if a.write.Sky().Frozen() || !a.write.Sky().Moon() || a.write.Climate().Running() != climate.AllRunning() || a.read.Running() != AllRunning() {
		t.Errorf("set going, the atmosphere runs %+v", a.read.Running())
	}
	other := world.NewPlugin(world.Config{Space: world.SpaceCfg{Width: 128, Height: 128}, Entities: world.EntitiesCfg{MaxCount: 4, MinSize: 1, MaxSize: 20}, Heights: true})
	if all := NewPlugin(other, nil, Config{}).read.Running(); all != AllRunning() {
		t.Errorf("a Config without Running runs %+v, want all of it", all)
	}
}
