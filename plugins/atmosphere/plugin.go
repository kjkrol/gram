package atmosphere

import (
	"fmt"
	"slices"
	"time"

	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/clock"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/atmosphere/air"
	"github.com/kjkrol/gram/plugins/atmosphere/aloft"
	"github.com/kjkrol/gram/plugins/atmosphere/backdrop"
	"github.com/kjkrol/gram/plugins/atmosphere/calendar"
	"github.com/kjkrol/gram/plugins/atmosphere/celestial"
	"github.com/kjkrol/gram/plugins/atmosphere/climate"
	"github.com/kjkrol/gram/plugins/atmosphere/precipitation"
	"github.com/kjkrol/gram/plugins/atmosphere/sky"
	"github.com/kjkrol/gram/plugins/atmosphere/weathering"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/ground"
	"github.com/kjkrol/gram/plugins/light"
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
	light       *light.Plugin

	freezes    command.Queue[Freeze]
	laters     command.Queue[Later]
	earliers   command.Queue[Earlier]
	changes    command.Queue[ChangeWeather]
	sets       command.Queue[SetWeather]
	clouds     command.Queue[Clouds]
	shades     command.Queue[HighShade]
	downpours  command.Queue[Downpour]
	calendar   *calendar.Calendar
	sky        *sky.Sky
	climate    *climate.Climate
	weathering *weathering.Weathering
	module     *module
	running    Running
	skies      []Sky
	vault      *backdrop.Renderer
	aloft      *aloft.Renderer // the low clouds: the high ones from above would veil the map
	look       CloudLook
	shadows    CloudShadows
}

var _ plugin.Plugin = (*Plugin)(nil)
var _ plugin.CommandHandler = (*Plugin)(nil)
var _ board.Atmosphere = (*Plugin)(nil)

// NewPlugin puts the atmosphere of cfg over worldPlugin, its sun shone through lightPlugin's
// mechanism: the atmosphere's own entity carries the sun's Source and its Shining of the hour.
// A world without heights panics.
func NewPlugin(worldPlugin *world.Plugin, lightPlugin *light.Plugin, cfg Config) *Plugin {
	if !worldPlugin.HasHeights() {
		panic("atmosphere: an atmosphere stands over a world with heights; set world.Config.Heights")
	}
	cal := calendar.New(worldPlugin.Clock(), cfg.Calendar)
	clim := climate.New(worldPlugin, cal, cfg.Climate)
	p := &Plugin{cfg: cfg, worldPlugin: worldPlugin, light: lightPlugin, calendar: cal,
		climate: clim, sky: sky.New(cal, cfg.Sky, clim.Zone().Latitude)}
	p.aloft = aloft.New(p.Air, air.Low)
	p.ShowClouds(AllBodies)
	p.shadows = CloudShadows{Ground: true, Figures: true, High: true}
	p.Self = world.NewSelf(worldPlugin, p, comp.Const(p.sky.Moonlight()),
		comp.Const(climate.DefaultSpread()), comp.Const(light.Source{Strength: 1, Ambient: 1}),
		comp.Const(light.Shining{}))
	clim.About(p.Entity)
	r := AllRunning()
	if cfg.Running != nil {
		r = *cfg.Running
	}
	r.Day = r.Day && !cfg.Sky.Frozen // a light the sky's Config freezes stays frozen
	p.SetRunning(r)
	return p
}

// Sun is the light of the day as it stands: the light plugin's, the sky's of the hour as its
// Source scales it; the sky's own without a light plugin.
func (p *Plugin) Sun() light.Distant {
	if p.light == nil {
		return p.sky.Sun()
	}
	return p.light.Distant()
}

// Heavens is where the sun, the moon and the stars stand at the hour of the light.
func (p *Plugin) Heavens() celestial.Heavens { return p.sky.Heavens() }

// Air is the weather as the last step of the simulation left it, its clouds shading as
// CloudShadows say.
func (p *Plugin) Air() air.Weather {
	w := p.climate.Air()
	w.HighShade, w.ShadeGround, w.ShadeFigures = p.shadows.High, p.shadows.Ground,
		p.shadows.Figures
	return w
}

// CloudShadows is whose shadows the clouds cast: on the Ground, on the Figures standing on it,
// and whether the High clouds' fall too, or the low ones' alone. A cloud shades itself only as a
// body lit from above, darker under its brow.
type CloudShadows struct{ Ground, Figures, High bool }

// CastClouds has the clouds' shadows fall as c says; all three as the atmosphere starts. A
// scene sets it as it begins.
func (p *Plugin) CastClouds(c CloudShadows) *Plugin {
	p.shadows = c
	return p
}

// CloudShadows is whose shadows the clouds cast.
func (p *Plugin) CloudShadows() CloudShadows { return p.shadows }

// CloudLook is how the clouds are drawn: FlatClouds as flat sheets on the sky and the low ones
// from above, LowBodies the low ones as bodies, heaped from under them and half see-through from
// above, AllBodies the high ones heaped on the sky too, ShadowsAlone none, only their shadows on
// the ground; the high ones never over the map from above, where they veiled it.
type CloudLook uint8

const (
	FlatClouds CloudLook = iota
	LowBodies
	AllBodies
	ShadowsAlone
)

// String is the look's name: flat, the low as bodies, all as bodies, shadows alone.
func (l CloudLook) String() string {
	switch l {
	case FlatClouds:
		return "flat"
	case LowBodies:
		return "the low as bodies"
	case AllBodies:
		return "all as bodies"
	case ShadowsAlone:
		return "shadows alone"
	}
	return "?"
}

// ShowClouds has the clouds drawn as look says, AllBodies till then; a scene's say as it starts,
// Clouds going on to the next look as the game goes.
func (p *Plugin) ShowClouds(look CloudLook) {
	p.look = look
	p.aloft.Show(look != ShadowsAlone)
	p.aloft.Bodies(look == LowBodies || look == AllBodies)
}

// ShownClouds is how the clouds are drawn.
func (p *Plugin) ShownClouds() CloudLook { return p.look }

func (p *Plugin) skyClouds() backdrop.Clouds {
	low := p.look == LowBodies || p.look == AllBodies
	return backdrop.Clouds{
		Hidden: p.look == ShadowsAlone,
		Bodies: [2]bool{low, p.look == AllBodies},
		Before: low,
	}
}

// Over has the low clouds hang over brd's ground, their bands fitted to its relief: from its low
// ground, the sea's or the valleys', up to its peaks. Call it as the plugin is made;
// WithWeathering does.
func (p *Plugin) Over(brd *board.Plugin) *Plugin {
	p.climate.Over(func() (low, peaks float64) { return reliefOf(brd.Heights(), brd.Board()) })
	return p
}

// the low ground of b's cells under heights h, the tenth of them lowest, none under the sea's, and
// their highest
func reliefOf(h ground.Heights, b *board.Board) (low, peaks float64) {
	if h == nil {
		return 0, 0
	}
	all := make([]float64, 0, b.CellCount())
	b.EachCell(func(c cell.ID) { all = append(all, h.At(b.CellCenter(c))) })
	if len(all) == 0 {
		return 0, 0
	}
	slices.Sort(all)
	return max(all[len(all)/10], 0), all[len(all)-1]
}

// WithWeathering has the weather work on brd as cfg says, snow, ice and what sways, and brd's
// ground on the air over it: the low clouds hang over it (Over), held back by what stands up into
// them. Call it as the plugin is made; a kind cfg names that the board lacks panics as the
// weather first touches it.
func (p *Plugin) WithWeathering(brd *board.Plugin, cfg weathering.Config) *Plugin {
	p.Over(brd)
	w, err := weathering.New(brd, p.Air, p.worldPlugin.Effects(), p.calendar, cfg)
	if err != nil {
		panic(err)
	}
	p.weathering = w
	w.SetRunning(p.running.Weathering)
	p.climate.OnLand(w.Land())
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

// Install wires the sky's, the weather's and the weathering's systems, and draws the sky behind
// the world, looking as its Skies say, the clouds seen from above and what falls before it.
func (p *Plugin) Install(ctx plugin.Installer) error {
	p.module = &module{p: p, sky: p.sky.System(), climate: p.climate.System(), comps: p.climate.LoadComps(), clock: p.worldPlugin.Clock()}
	if p.weathering != nil {
		p.module.comps = append(p.module.comps, goke.LoadComp[weathering.Wring]())
	}
	ctx.UseModule(p.module)
	p.module.rises = p.sky.RiseSystem(p.worldPlugin.Tick, p.Entity)
	ctx.Hosts(p.climate.Rules(), p.sky.Rules())
	if p.weathering != nil {
		ctx.Hosts(p.weathering.Rules())
	}
	shown := func() (stars, moon bool) {
		r := p.Running()
		return r.Stars, r.Moon
	}
	p.vault = backdrop.New(p.worldPlugin.Res.Config.Space, p.Sun, p.Air).WithHeavens(p.Heavens).
		WithShown(shown).WithClouds(p.skyClouds)
	ctx.Draws(p.vault, p.aloft, precipitation.NewDrops(p.Sun, p.Air))
	ctx.Looks(p.dress)
	return nil
}

// RunPlan runs the atmosphere's tick. Call it after the world's.
func (p *Plugin) RunPlan(ctx goke.RunCtx, d time.Duration) {
	p.carryOut()
	p.module.RunPlan(ctx, d)
}

// Reporter is the atmosphere's lines for the telemetry panel.
func (p *Plugin) Reporter() render.Reporter {
	return reporters{p.calendar.Reporter(), p.sky.Reporter(), p.climate.Reporter(), cloudsReport{p}}
}

// HUD is the calendar's screen renderer: the hour, the date and the moon, for a ui.Drawer.
func (p *Plugin) HUD() render.Renderer { return p.calendar.HUD() }

// EventHandler is nil: the atmosphere takes commands, not input.
func (p *Plugin) EventHandler() control.EventHandler { return nil }

// Serializable is nil: the weather and its air masses are entities, saved with the ECS.
func (p *Plugin) Serializable() plugin.Serializable { return nil }

var _ goke.Module = (*module)(nil)

type module struct {
	sky, climate  goke.System
	comps         []goke.CompToken
	clock         *clock.Clock
	skyRun        goke.Runnable
	shineRun      goke.Runnable
	rises         goke.System
	risesRun      goke.Runnable
	climateRun    goke.Runnable
	p             *Plugin
	weatheringRun goke.Runnable
}

func (m *module) RegSystems(ecs *goke.ECS) {
	m.skyRun = ecs.RegSys(m.sky)
	m.shineRun = ecs.RegSys(&shining{p: m.p})
	m.climateRun = ecs.RegSys(m.climate)
	m.risesRun = ecs.RegSys(m.rises)
	if w := m.p.weathering; w != nil {
		m.weatheringRun = ecs.RegSys(w.System(m.clock, m.p.worldPlugin.Tick))
	}
}

func (m *module) RunPlan(ctx goke.RunCtx, d time.Duration) {
	ctx.Run(m.skyRun, d)
	ctx.Run(m.shineRun, d)
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

type cloudsReport struct{ p *Plugin }

func (cloudsReport) Init(*goke.SysInit) {}

func (r cloudsReport) Report(line func(label, value string)) {
	look := r.p.look.String()
	if w := r.p.Air(); !w.OneLayer && w.Relief[1] > 0 {
		c := w.Ceilings
		look += fmt.Sprintf("\nunder the peaks %.0f%%\ncovering %.0f%%, over %.0f%%",
			100*c[0], 100*c[1], 100*c[2])
	}
	line("Clouds", look)
	line("Shadows", r.p.shadows.String())
}

// String says whose shadows fall: "both layers' on the ground and the figures".
func (c CloudShadows) String() string {
	layers := "the low clouds'"
	if c.High {
		layers = "both layers'"
	}
	switch {
	case c.Ground && c.Figures:
		return layers + " on the ground and the figures"
	case c.Ground:
		return layers + " on the ground"
	case c.Figures:
		return layers + " on the figures"
	}
	return "none"
}

// shining writes the sun of the hour beside the atmosphere's Source each tick, for the light
// plugin to read
type shining struct {
	p     *Plugin
	q     *goke.Query
	shine goke.Comp[light.Shining]
}

func (s *shining) Init(si *goke.SysInit) { s.q = si.NewQueryBuilder(&s.shine).Build() }

func (s *shining) Update(*goke.CmdBuf, time.Duration) {
	if s.q.Seek(s.p.Entity()) {
		s.shine.At(s.q.Cursor()).Light = s.p.sky.Sun()
	}
}

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
