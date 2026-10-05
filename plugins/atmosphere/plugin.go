package atmosphere

import (
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/clock"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/atmosphere/air"
	"github.com/kjkrol/gram/plugins/atmosphere/backdrop"
	"github.com/kjkrol/gram/plugins/atmosphere/calendar"
	"github.com/kjkrol/gram/plugins/atmosphere/celestial"
	"github.com/kjkrol/gram/plugins/atmosphere/climate"
	"github.com/kjkrol/gram/plugins/atmosphere/overcast"
	"github.com/kjkrol/gram/plugins/atmosphere/precipitation"
	"github.com/kjkrol/gram/plugins/atmosphere/sky"
	"github.com/kjkrol/gram/plugins/atmosphere/weathering"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

// Config is the atmosphere: the Calendar's days and year, the Sky's sun and light, the Climate's
// zone and weathers, and which of its workings go on from the start (Running; all of them when
// nil). Zero fields take each package's defaults; the sun goes at the climate's zone's latitude.
type Config struct {
	Calendar calendar.Config
	Sky      sky.Config
	Climate  climate.Config
	Running  *Running
}

// Plugin is the atmosphere over a world: the calendar of the world's clock, the light of the day
// over it, its climate and weather, what falls, and — laid on a board — what the weather does to
// the ground.
type Plugin struct {
	*world.Self // its own entity: its knobs, the roles it plays, the effects it is under

	cfg         Config
	worldPlugin *world.Plugin
	calendar    *calendar.Calendar
	sky         *sky.Sky
	climate     *climate.Climate
	weathering  *weathering.Weathering
	module      *module
	running     Running
}

var _ plugin.Plugin = (*Plugin)(nil)
var _ plugin.CommandHandler = (*Plugin)(nil)

// NewPlugin puts the atmosphere of cfg over worldPlugin.
func NewPlugin(worldPlugin *world.Plugin, cfg Config) *Plugin {
	cal := calendar.New(worldPlugin.Clock(), cfg.Calendar)
	clim := climate.New(worldPlugin, cal, cfg.Climate)
	p := &Plugin{Self: world.NewSelf(worldPlugin, "gram.atmosphere"), cfg: cfg, worldPlugin: worldPlugin, calendar: cal, climate: clim,
		sky: sky.New(cal, cfg.Sky, clim.Zone().Latitude)}
	clim.About(p.Entity)
	r := AllRunning()
	if cfg.Running != nil {
		r = *cfg.Running
	}
	r.Day = r.Day && !cfg.Sky.Frozen // a light the sky's Config freezes stays frozen
	p.SetRunning(r)
	return p
}

// Sun is the light of the day as it stands: what lights the world and casts its shadows.
func (p *Plugin) Sun() sky.Sun { return p.sky.Sun() }

// Heavens is where the sun, the moon and the stars stand at the hour of the light.
func (p *Plugin) Heavens() celestial.Heavens { return p.sky.Heavens() }

// Air is the weather as the last step of the simulation left it: the wind, the clouds, what
// falls, how far one sees.
func (p *Plugin) Air() air.Weather { return p.climate.Air() }

// WithWeathering has the weather work on brd as cfg says: snow, ice, what sways; call it in Init,
// once the kinds cfg names are in brd's dictionary — where the effects are defined, for it defines
// three. A Config the board cannot take panics.
func (p *Plugin) WithWeathering(brd *board.Plugin, cfg weathering.Config) *Plugin {
	w, err := weathering.New(brd, p.Air, p.worldPlugin.Effects(), p.calendar, cfg)
	if err != nil {
		panic(err)
	}
	p.weathering = w
	w.SetRunning(p.running.Weathering)
	return p
}

// Calendar is the atmosphere's: the day, the season and the moon of the world's clock.
func (p *Plugin) Calendar() *calendar.Calendar { return p.calendar }

// Sky is the light of the day, to freeze and move by hand.
func (p *Plugin) Sky() *sky.Sky { return p.sky }

// Climate is the climate and its weather.
func (p *Plugin) Climate() *climate.Climate { return p.climate }

// Weathering is what the weather does to the board, nil without WithWeathering.
func (p *Plugin) Weathering() *weathering.Weathering { return p.weathering }

// =================================================================
// plugin.Plugin contract
// =================================================================

func (p *Plugin) Name() string { return "gram.atmosphere" }

// Install wires the sky's and the weather's systems, and lays the weathering on the world's
// schedule.
func (p *Plugin) Install(ctx plugin.Installer) error {
	p.module = &module{p: p, sky: p.sky.System(), climate: p.climate.System(), comps: p.climate.LoadComps(), clock: p.worldPlugin.Clock()}
	ctx.UseModule(p.module)
	ctx.Hosts(p.climate.Rules())
	return nil
}

// RunPlan runs the atmosphere's tick: the light at once, the weather in every step of the
// simulation. Call it after the world's, before the world is drawn.
func (p *Plugin) RunPlan(ctx goke.RunCtx, d time.Duration) { p.module.RunPlan(ctx, d) }

// WithRenderer is a no-op: the sky and what falls are drawn in plain colours.
func (p *Plugin) WithRenderer(render.AtlasSource) {}

// Renderer is the sky behind the world, a render.Source for a scene's Composer: the viewport in
// the sky's colour under everything, wherever the ground does not cover it.
func (p *Plugin) Renderer() render.Layer {
	shown := func() (stars, moon bool) {
		r := p.Running()
		return r.Stars, r.Moon
	}
	return backdrop.New(p.worldPlugin.Res.Config.Space, p.worldPlugin.Scale(), p.Sun, p.Air).WithHeavens(p.Heavens).WithShown(shown)
}

// Precipitation is what falls — rain, snow — a render.Source for a scene's Composer, on render.Air.
func (p *Plugin) Precipitation() render.Layer { return precipitation.New(p.Sun, p.Air) }

// Clouds is the clouds' shadows over a flat world, a render.Source for a scene's Composer: laid
// over the ground under every pixel on the GPU, over what stands on it, under the overlays. A
// world with heights has its terrain shadow itself.
func (p *Plugin) Clouds() render.Layer { return overcast.New(p.Sun, p.Air) }

// Reporter is the atmosphere's lines for a render.TelemetryRenderer: the time of day and the date,
// the light, the weather.
func (p *Plugin) Reporter() render.Reporter {
	return reporters{p.calendar.Reporter(), p.sky.Reporter(), p.climate.Reporter()}
}

// HUD is the calendar's screen layer: the hour, the date and the moon in the bottom-right corner.
func (p *Plugin) HUD() render.Layer { return p.calendar.HUD() }

// EventHandler is nil: the atmosphere takes commands, not input.
func (p *Plugin) EventHandler() control.EventHandler { return nil }

// Serializable is nil: the weather is its own entity, saved with the ECS; the calendar is the
// clock's; the light's freeze is a look, not saved.
func (p *Plugin) Serializable() plugin.Serializable { return nil }

// =================================================================
// plugin.CommandHandler contract
// =================================================================

// Queues are the sky's and the climate's.
func (p *Plugin) Queues() []control.CommandQueue {
	return append(p.sky.Queues(), p.climate.Queues()...)
}

// DefaultBindings: P freezes the light, Shift+] and Shift+[ move a frozen light half an hour,
// Shift+W changes the weather.
func (p *Plugin) DefaultBindings() []control.Binding {
	return append(p.sky.DefaultBindings(), p.climate.DefaultBindings()...)
}

// =================================================================
// module
// =================================================================

var _ goke.Module = (*module)(nil)

// module runs the sky once a tick and the weather in every step of the simulation.
type module struct {
	sky, climate  goke.System
	comps         []goke.CompToken
	clock         *clock.Clock
	skyRun        goke.Runnable
	climateRun    goke.Runnable
	p             *Plugin // its weathering, given any time before the Stage is set up
	weatheringRun goke.Runnable
}

func (m *module) RegSystems(ecs *goke.ECS) {
	m.skyRun = ecs.RegSys(m.sky)
	m.climateRun = ecs.RegSys(m.climate)
	if w := m.p.weathering; w != nil {
		m.weatheringRun = ecs.RegSys(w.System(m.clock))
	}
}

func (m *module) RunPlan(ctx goke.RunCtx, d time.Duration) {
	ctx.Run(m.skyRun, d)
	ctx.Sync()
	clock.Simulate(m.clock, ctx, d, func(ctx goke.RunCtx, step time.Duration) {
		ctx.Run(m.climateRun, step)
		if m.weatheringRun != nil {
			ctx.Run(m.weatheringRun, step)
		}
		ctx.Sync()
	})
}

// SetupSystems is empty: the weather finds or makes its entity in its own Init.
func (m *module) SetupSystems() []goke.System { return nil }

// LoadComps lists the weather's component — see goke.CompProvider.
func (m *module) LoadComps() []goke.CompToken { return m.comps }

// =================================================================
// reporters
// =================================================================

// reporters is several reporters as one.
type reporters []render.Reporter

func (rs reporters) Init(si *goke.SysInit) {
	for _, r := range rs {
		r.Init(si)
	}
}

func (rs reporters) Report(line func(label, value string)) {
	for _, r := range rs {
		r.Report(line)
	}
}
