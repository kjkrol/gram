package climate

import (
	"github.com/kjkrol/ceremony/host"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/atmosphere/air"
	"github.com/kjkrol/gram/plugins/atmosphere/calendar"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/uid"
)

// Climate is the climate of a world: where it lies, and its weather going from one state to the
// next in the simulation's time.
type Climate struct {
	cfg      Config
	world    *world.Plugin
	calendar *calendar.Calendar
	sys      *weatherSystem
	asked    []string // a state's name, "" for the next
	report   report
	rules    host.StepRules[Weathering]
	about    func() uid.UID64
	running  Running
}

// Running is which of the weather's workings go on: one weather following another, the wind,
// the clouds, rain and snow. Off, the weather's entity goes on underneath; none of it is saved.
type Running struct {
	Changes, Wind, Clouds, Falls bool
}

// AllRunning is Running with all of it going on, the default.
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

// Air is the weather as the last step of the simulation left it; a calm, clear day before the
// first.
func (c *Climate) Air() air.Weather {
	if c.sys == nil {
		return air.Weather{}
	}
	return c.sys.current
}

// Zone is where the climate lies.
func (c *Climate) Zone() Zone { return c.cfg.Zone }

// System is the weather's system, to run in every step of the simulation. Call it once.
func (c *Climate) System() goke.System {
	c.sys = newWeatherSystem(c.cfg, c.world, c.calendar, &c.asked, &c.rules, &c.running)
	c.sys.about = c.about
	return c.sys
}

// Change has the weather go on to the next state at the next step of the simulation.
func (c *Climate) Change() { c.asked = append(c.asked, "") }

// Set has the weather go into the state named name at the next step of the simulation; a name
// the climate lacks changes nothing.
func (c *Climate) Set(name string) {
	if name != "" {
		c.asked = append(c.asked, name)
	}
}

// About says whose entity a Weathering is about, the world's without it. Call it before System.
func (c *Climate) About(self func() uid.UID64) { c.about = self }

// LoadComps lists the weather's components for the saves: its own and the air masses' patches.
func (c *Climate) LoadComps() []goke.CompToken {
	return []goke.CompToken{goke.LoadComp[Weather](), goke.LoadComp[air.Patch]()}
}

// Rules takes the rules of Weathering.
func (c *Climate) Rules() host.Host { return &c.rules }

// Reporter is the weather's line for the telemetry panel.
func (c *Climate) Reporter() render.Reporter { return &c.report }
