package atmosphere

import (
	"time"

	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/ceremony/comp"
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

// Config is the atmosphere: its calendar, sky, climate and which of its workings go on from the
// start (all of them when Running is nil). Zero fields take each package's defaults.
type Config struct {
	Calendar calendar.Config
	Sky      sky.Config
	Climate  climate.Config
	Running  *Running
}

// Plugin is the atmosphere over a world: the calendar, the light of the day, the climate and its
// weather, what falls, and what the weather does to a board.
type Plugin struct {
	*world.Self

	cfg         Config
	worldPlugin *world.Plugin

	freezes    command.Queue[Freeze]
	laters     command.Queue[Later]
	earliers   command.Queue[Earlier]
	changes    command.Queue[ChangeWeather]
	sets       command.Queue[SetWeather]
	calendar   *calendar.Calendar
	sky        *sky.Sky
	climate    *climate.Climate
	weathering *weathering.Weathering
	module     *module
	running    Running
}

var _ plugin.Plugin = (*Plugin)(nil)
var _ plugin.CommandHandler = (*Plugin)(nil)

// NewPlugin puts the atmosphere of cfg over worldPlugin.
func NewPlugin(worldPlugin *world.Plugin, cfg Config) *Plugin {
	cal := calendar.New(worldPlugin.Clock(), cfg.Calendar)
	clim := climate.New(worldPlugin, cal, cfg.Climate)
	p := &Plugin{cfg: cfg, worldPlugin: worldPlugin, calendar: cal, climate: clim,
		sky: sky.New(cal, cfg.Sky, clim.Zone().Latitude)}
	p.Self = world.NewSelf(worldPlugin, p, comp.Const(sky.DefaultMoon()))
	clim.About(p.Entity)
	r := AllRunning()
	if cfg.Running != nil {
		r = *cfg.Running
	}
	r.Day = r.Day && !cfg.Sky.Frozen // a light the sky's Config freezes stays frozen
	p.SetRunning(r)
	return p
}

// Sun is the light of the day as it stands.
func (p *Plugin) Sun() sky.Sun { return p.sky.Sun() }

// Heavens is where the sun, the moon and the stars stand at the hour of the light.
func (p *Plugin) Heavens() celestial.Heavens { return p.sky.Heavens() }

// Air is the weather as the last step of the simulation left it.
func (p *Plugin) Air() air.Weather { return p.climate.Air() }

// WithWeathering has the weather work on brd as cfg says: snow, ice, what sways. Call it as the
// plugin is made; a kind cfg names that the board lacks panics as the weather first touches it.
func (p *Plugin) WithWeathering(brd *board.Plugin, cfg weathering.Config) *Plugin {
	w, err := weathering.New(brd, p.Air, p.worldPlugin.Effects(), p.calendar, cfg)
	if err != nil {
		panic(err)
	}
	p.weathering = w
	w.SetRunning(p.running.Weathering)
	return p
}

// Calendar is the day, the season and the moon of the world's clock.
func (p *Plugin) Calendar() *calendar.Calendar { return p.calendar }

// Sky is the light of the day, to freeze and move by hand.
func (p *Plugin) Sky() *sky.Sky { return p.sky }

// Climate is the climate and its weather.
func (p *Plugin) Climate() *climate.Climate { return p.climate }

// Weathering is what the weather does to the board, nil without WithWeathering.
func (p *Plugin) Weathering() *weathering.Weathering { return p.weathering }

func (p *Plugin) Name() string { return "gram.atmosphere" }

// Install wires the sky's, the weather's and the weathering's systems.
func (p *Plugin) Install(ctx plugin.Installer) error {
	p.module = &module{p: p, sky: p.sky.System(), climate: p.climate.System(), comps: p.climate.LoadComps(), clock: p.worldPlugin.Clock()}
	ctx.UseModule(p.module)
	p.module.rises = p.sky.RiseSystem(p.worldPlugin.Tick, p.Entity)
	ctx.Hosts(p.climate.Rules(), p.sky.Rules())
	return nil
}

// RunPlan runs the atmosphere's tick. Call it after the world's.
func (p *Plugin) RunPlan(ctx goke.RunCtx, d time.Duration) {
	p.carryOut()
	p.module.RunPlan(ctx, d)
}

// WithRenderer is a no-op: the sky and what falls are drawn in plain colours.
func (p *Plugin) WithRenderer(render.AtlasSource) {}

// Renderer is the sky behind the world, a render.Source for a scene's Composer.
func (p *Plugin) Renderer() render.Layer {
	shown := func() (stars, moon bool) {
		r := p.Running()
		return r.Stars, r.Moon
	}
	return backdrop.New(p.worldPlugin.Res.Config.Space, p.worldPlugin.Scale(), p.Sun, p.Air).WithHeavens(p.Heavens).WithShown(shown)
}

// Precipitation is what falls, a render.Source for a scene's Composer on render.Air.
func (p *Plugin) Precipitation() render.Layer { return precipitation.New(p.Sun, p.Air) }

// Clouds is the clouds' shadows over a flat world, a render.Source for a scene's Composer. A
// world with heights has its terrain shadow itself.
func (p *Plugin) Clouds() render.Layer { return overcast.New(p.Sun, p.Air) }

// Reporter is the atmosphere's lines for the telemetry panel.
func (p *Plugin) Reporter() render.Reporter {
	return reporters{p.calendar.Reporter(), p.sky.Reporter(), p.climate.Reporter()}
}

// HUD is the calendar's screen layer: the hour, the date and the moon.
func (p *Plugin) HUD() render.Layer { return p.calendar.HUD() }

// EventHandler is nil: the atmosphere takes commands, not input.
func (p *Plugin) EventHandler() control.EventHandler { return nil }

// Serializable is nil: the weather is its own entity, saved with the ECS.
func (p *Plugin) Serializable() plugin.Serializable { return nil }

var _ goke.Module = (*module)(nil)

type module struct {
	sky, climate  goke.System
	comps         []goke.CompToken
	clock         *clock.Clock
	skyRun        goke.Runnable
	rises         goke.System
	risesRun      goke.Runnable
	climateRun    goke.Runnable
	p             *Plugin
	weatheringRun goke.Runnable
}

func (m *module) RegSystems(ecs *goke.ECS) {
	m.skyRun = ecs.RegSys(m.sky)
	m.climateRun = ecs.RegSys(m.climate)
	m.risesRun = ecs.RegSys(m.rises)
	if w := m.p.weathering; w != nil {
		m.weatheringRun = ecs.RegSys(w.System(m.clock))
	}
}

func (m *module) RunPlan(ctx goke.RunCtx, d time.Duration) {
	ctx.Run(m.skyRun, d)
	ctx.Sync()
	clock.Simulate(m.clock, ctx, d, func(ctx goke.RunCtx, step time.Duration) {
		ctx.Run(m.climateRun, step)
		ctx.Run(m.risesRun, step)
		if m.weatheringRun != nil {
			ctx.Run(m.weatheringRun, step)
		}
		ctx.Sync()
	})
}

func (m *module) SetupSystems() []goke.System { return nil }

func (m *module) LoadComps() []goke.CompToken { return m.comps }

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
