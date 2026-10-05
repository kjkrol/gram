package climate

import (
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/atmosphere/air"
	"github.com/kjkrol/gram/plugins/atmosphere/calendar"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/uid"
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
	rules    plugin.StepRules[Weathering]
	about    func() uid.UID64 // whose entity a Weathering is about
	running  Running
}

// Running is which of the weather's workings go on: Changes, one weather following another as the
// climate throws them (off, the weather now stays, though Change and Set still change it); Wind,
// the wind blowing and carrying the clouds (off, the air stands still); Clouds, the clouds
// covering the sky (off, a clear sky); Falls, rain and snow falling (off, nothing falls). The air
// (Climate.Air) and the rules hosted with it have the weather as they leave it; the
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
	c.sys = newWeatherSystem(c.cfg, c.world, c.calendar, &c.change, &c.set, &c.rules, &c.running)
	c.sys.about = c.about
	return c.sys
}

// About says whose entity a Weathering is about — the atmosphere's own; the world's without it.
// Call it before System.
func (c *Climate) About(self func() uid.UID64) { c.about = self }

// LoadComps lists the weather's one component — see goke.CompProvider.
func (c *Climate) LoadComps() []goke.CompToken { return []goke.CompToken{goke.LoadComp[Weather]()} }

// Rules takes the rules of Weathering, fired every step with the weather: a moment of the world
// as a whole, so its rules are of a role the atmosphere plays.
func (c *Climate) Rules() plugin.Host { return &c.rules }

// Reporter is the weather's line for a render.TelemetryRenderer.
func (c *Climate) Reporter() render.Reporter { return &c.report }
