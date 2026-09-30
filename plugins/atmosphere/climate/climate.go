package climate

import (
	"fmt"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugin/host"
	"github.com/kjkrol/gram/plugins/atmosphere/air"
	"github.com/kjkrol/gram/plugins/atmosphere/calendar"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

// Climate is the climate of a world: where in the world it lies, and its weather going from one
// of the climate's weathers to the next in the simulation's time, saved with the game, the
// world's weather — wind, clouds, rain, snow, the temperature — following it.
type Climate struct {
	cfg      Config
	world    *world.Plugin
	calendar *calendar.Calendar
	sys      *weatherSystem
	change   control.Queue[Change]
	set      control.Queue[Set]
	report   report
	triggers host.EachHost[Weathering]
	running  Running
}

// Running is which of the weather's workings go on: Changes, one weather following another as the
// climate throws them (off, the weather now stays, though Change and Set still change it); Wind,
// the wind blowing and carrying the clouds (off, the air stands still); Clouds, the clouds
// covering the sky (off, a clear sky); Falls, rain and snow falling (off, nothing falls). The air
// (Climate.Air) and the triggers hosted with it have the weather as they leave it; the
// weather's own entity goes on underneath and is saved as it is. They are not saved.
type Running struct {
	Changes, Wind, Clouds, Falls bool
}

// AllRunning is Running with all of it going on, as a Climate begins.
func AllRunning() Running { return Running{Changes: true, Wind: true, Clouds: true, Falls: true} }

// SetRunning has the weather's workings go on as r says, from the next step.
func (c *Climate) SetRunning(r Running) { c.running = r }

// Running is which of the weather's workings go on.
func (c *Climate) Running() Running { return c.running }

// New puts w in the climate of cfg, its seasons cal's.
func New(w *world.Plugin, cal *calendar.Calendar, cfg Config) *Climate {
	cfg = cfg.withDefaults()
	c := &Climate{cfg: cfg, world: w, calendar: cal, running: AllRunning()}
	for _, s := range cfg.Weathers {
		c.report.names = append(c.report.names, s.Name)
	}
	return c
}

// Config is the climate's, with its defaults filled in.
func (c *Climate) Config() Config { return c.cfg }

// Air is the weather as the last step of the simulation left it: a calm, clear day before the
// first.
func (c *Climate) Air() air.Weather {
	if c.sys == nil {
		return air.Weather{}
	}
	return c.sys.current
}

// Zone is where the climate lies.
func (c *Climate) Zone() Zone { return c.cfg.Zone }

// System is the weather's system, to run in every step of the simulation; it finds or makes the
// weather's entity in its own Init. Call it once.
func (c *Climate) System() goke.System {
	c.sys = newWeatherSystem(c.cfg, c.world, c.calendar, &c.change, &c.set, &c.triggers, &c.running)
	return c.sys
}

// LoadComps lists the weather's one component — see goke.CompProvider.
func (c *Climate) LoadComps() []goke.CompToken { return []goke.CompToken{goke.LoadComp[Weather]()} }

// Host hosts a trigger of Weathering, fired every step with the weather; call before the
// system's Init.
func (c *Climate) Host(b plugin.Trigger) error {
	if err := c.triggers.Add(b); err != nil {
		return fmt.Errorf("%w in the climate — it takes a trigger of Weathering", err)
	}
	return nil
}

// Reporter is the weather's line for a render.TelemetryRenderer.
func (c *Climate) Reporter() render.Reporter { return &c.report }
