package atmosphere

import (
	"reflect"
	"testing"

	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/atmosphere/climate"
	"github.com/kjkrol/gram/plugins/world"
)

func TestPlugin_RunningSwitchesTheAtmospheresWorkings(t *testing.T) {
	w := world.NewPlugin(world.Config{Space: world.SpaceCfg{Width: 128, Height: 128}, Entities: world.EntitiesCfg{MaxCount: 4, MinSize: 1, MaxSize: 20}, Heights: true})
	r := AllRunning()
	r.Day, r.Moon, r.Wind, r.Falls = false, false, false, false
	a := NewPlugin(w, nil, Config{Running: &r})
	if !a.Sky().Frozen() || a.Sky().Moon() {
		t.Errorf("begun without the day and the moon, the light is frozen %v, moonlit %v", a.Sky().Frozen(), a.Sky().Moon())
	}
	if c := a.Climate().Running(); !c.Changes || c.Wind || !c.Clouds || c.Falls {
		t.Errorf("the climate runs %+v, want the changes and the clouds only", c)
	}
	if got := a.Running(); got != r {
		t.Errorf("the atmosphere runs %+v, want %+v", got, r)
	}
	a.SetRunning(AllRunning())
	if a.Sky().Frozen() || !a.Sky().Moon() || a.Climate().Running() != climate.AllRunning() || a.Running() != AllRunning() {
		t.Errorf("set going, the atmosphere runs %+v", a.Running())
	}
	other := world.NewPlugin(world.Config{Space: world.SpaceCfg{Width: 128, Height: 128}, Entities: world.EntitiesCfg{MaxCount: 4, MinSize: 1, MaxSize: 20}, Heights: true})
	if all := NewPlugin(other, nil, Config{}).Running(); all != AllRunning() {
		t.Errorf("a Config without Running runs %+v, want all of it", all)
	}
}

// riding in a unit, W with Shift sprints
func TestDefaultBindings_ChangeTheWeatherWithTheCameraFreeOnly(t *testing.T) {
	w := world.NewPlugin(world.Config{Space: world.SpaceCfg{Width: 128, Height: 128}, Entities: world.EntitiesCfg{MaxCount: 4, MinSize: 1, MaxSize: 20}, Heights: true})
	for _, b := range NewPlugin(w, nil, Config{}).DefaultBindings() {
		if b.Type() != reflect.TypeFor[ChangeWeather]() {
			continue
		}
		if !b.Holds(camera.Loose) || !b.Holds(camera.Centred) || b.Holds(camera.Inside) {
			t.Errorf("Shift+W holds free %v, riding %v; want free only", b.Holds(camera.Loose), b.Holds(camera.Inside))
		}
		return
	}
	t.Error("the atmosphere binds no key to ChangeWeather")
}
