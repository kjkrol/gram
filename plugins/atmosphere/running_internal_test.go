package atmosphere

import (
	"reflect"
	"testing"

	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/atmosphere/climate"
	"github.com/kjkrol/gram/plugins/world"
)

// The atmosphere's switches reach its parts: begun with the day, the moon and the weathering
// stopped, the light stands, the night is not moonlit and the climate's changes, wind, clouds and
// falls go as the Running says; set going again, the light goes with the calendar.
func TestPlugin_RunningSwitchesTheAtmospheresWorkings(t *testing.T) {
	w := world.NewPlugin(world.Config{Space: world.SpaceCfg{Width: 128, Height: 128}, Entities: world.EntitiesCfg{MaxCount: 4, MinSize: 1, MaxSize: 20}})
	r := AllRunning()
	r.Day, r.Moon, r.Wind, r.Falls = false, false, false, false
	a := NewPlugin(w, Config{Running: &r})
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
	other := world.NewPlugin(world.Config{Space: world.SpaceCfg{Width: 128, Height: 128}, Entities: world.EntitiesCfg{MaxCount: 4, MinSize: 1, MaxSize: 20}})
	if all := NewPlugin(other, Config{}).Running(); all != AllRunning() {
		t.Errorf("a Config without Running runs %+v, want all of it", all)
	}
}

// Shift+W changes the weather with the camera free only: riding in a unit, W with Shift sprints.
func TestDefaultBindings_ChangeTheWeatherWithTheCameraFreeOnly(t *testing.T) {
	w := world.NewPlugin(world.Config{Space: world.SpaceCfg{Width: 128, Height: 128}, Entities: world.EntitiesCfg{MaxCount: 4, MinSize: 1, MaxSize: 20}})
	for _, b := range NewPlugin(w, Config{}).DefaultBindings() {
		if b.Command() != reflect.TypeFor[ChangeWeather]() {
			continue
		}
		if !b.Holds(camera.Loose) || !b.Holds(camera.Centred) || b.Holds(camera.Inside) {
			t.Errorf("Shift+W holds free %v, riding %v; want free only", b.Holds(camera.Loose), b.Holds(camera.Inside))
		}
		return
	}
	t.Error("the atmosphere binds no key to ChangeWeather")
}
