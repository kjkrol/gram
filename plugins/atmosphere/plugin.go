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

	freezes      command.Queue[Freeze]
	hours        command.Queue[Hour]
	changes      command.Queue[ChangeWeather]
	sets         command.Queue[SetWeather]
	clouds       command.Queue[Clouds]
	shades       command.Queue[HighShade]
	groundShades command.Queue[GroundShade]
	reaches      command.Queue[CloudReach]
	coarses      command.Queue[CloudsDrawnAt]
	figureShades command.Queue[FigureShade]
	adapts       command.Queue[AdaptClouds]
	downpours    command.Queue[Downpour]
	calendar     *calendar.Calendar
	sky          *sky.Sky
	climate      *climate.Climate
	weathering   *weathering.Weathering
	module       *module
	running      Running
	skies        []Sky
	vault        *backdrop.Renderer
	aloft        *aloft.Renderer // the low clouds: the high ones from above would veil the map
	shadows      CloudShadows
	kept         *plugin.Kept[Settings]
	published    render.Buffered[drawnSky]
	live         drawnSky // drawn before any was published
	swapped      bool

	Res   Resources
	read  Read
	write Write
}

var _ plugin.Plugin = (*Plugin)(nil)
var _ plugin.CommandHandler = (*Plugin)(nil)
var _ plugin.Tunable = (*Plugin)(nil)
var _ plugin.Restorer = (*Plugin)(nil)
var _ plugin.Publisher = (*Plugin)(nil)

// NewPlugin puts the atmosphere of cfg over worldPlugin, its sun shone through lightPlugin's
// mechanism: the atmosphere's own entity carries the sun's Source and its Shining of the hour.
// A world without heights panics.
func NewPlugin(worldPlugin *world.Plugin, lightPlugin *light.Plugin, cfg Config) *Plugin {
	if !worldPlugin.Read(0).HasHeights() {
		panic("atmosphere: an atmosphere stands over a world with heights; set world.Config.Heights")
	}
	cal := calendar.New(worldPlugin.Write().Clock(), cfg.Calendar)
	clim := climate.New(worldPlugin, cal, cfg.Climate)
	p := &Plugin{cfg: cfg, worldPlugin: worldPlugin, light: lightPlugin, calendar: cal,
		climate: clim, sky: sky.New(cal, cfg.Sky, clim.Zone().Latitude)}
	p.read.p, p.write.p = p, p
	p.aloft = aloft.New(p.drawnAir, air.Low)
	p.kept = plugin.Keep(&p.Res.Settings)
	p.write.ShowClouds(AllBodies)
	p.CastClouds(CloudShadows{Ground: true, Figures: true, High: true})
	p.Self = world.NewSelf(worldPlugin, p, comp.Const(p.sky.Moonlight()),
		comp.Const(climate.DefaultSpread()), comp.Const(light.Source{Strength: 1, Ambient: 1}),
		comp.Const(light.Shining{}))
	clim.About(p.Entity)
	r := AllRunning()
	if cfg.Running != nil {
		r = *cfg.Running
	}
	r.Day = r.Day && !cfg.Sky.Frozen // a light the sky's Config freezes stays frozen
	p.write.SetRunning(r)
	return p
}

// CloudShadows is whose shadows the clouds cast: on the Ground, on the Figures standing on it,
// and whether the High clouds' fall too, or the low ones' alone. A cloud shades itself only as a
// body lit from above, darker under its brow.
type CloudShadows struct{ Ground, Figures, High bool }

// CastClouds has the clouds' shadows fall as c says; all three as the atmosphere starts. A
// scene sets it as it begins.
func (p *Plugin) CastClouds(c CloudShadows) *Plugin {
	p.shadows = c
	p.Res.Settings.HighShade = c.High
	return p
}

func (p *Plugin) shadeHigh(on bool) {
	p.shadows.High = on
	p.Res.Settings.HighShade = on
}

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

// the pixels each way the clouds may be drawn at, from the most
var coarses = []int{4, 6, 8}

// coarseRung is the rung of coarses coarse stands on, or the next down
func coarseRung(coarse int) int {
	for k, c := range coarses {
		if coarse <= c {
			return k
		}
	}
	return len(coarses) - 1
}

// a rung of the budget's ladder: the pixels the clouds are drawn at and the bodies' reach
type rung struct {
	coarse int
	reach  float32
}

// the ladder the budget steps the clouds down and back up: fewer pixels first, the bodies' reach
// last and never under half the sight, the look never
var ladder = []rung{{4, 1}, {6, 1}, {8, 1}, {8, 0.5}}

func (p *Plugin) step(by int) bool {
	at := p.rung() + by
	if at < 0 || at >= len(ladder) {
		return false
	}
	p.write.DrawCloudsAt(ladder[at].coarse)
	p.write.ReachClouds(ladder[at].reach)
	return true
}

// rung is the ladder's rung the clouds stand on: by their pixels, the last with the reach cut
func (p *Plugin) rung() int {
	coarse := p.read.CloudsDrawnAt()
	last := len(ladder) - 1
	if coarse == ladder[last].coarse && p.read.CloudsReach() < 1 {
		return last
	}
	for k, r := range ladder {
		if r.coarse == coarse {
			return k
		}
	}
	return 0
}

// skyClouds is how the sky shows the clouds as drawn
func (p *Plugin) skyClouds() backdrop.Clouds {
	look := p.drawn().settings.Clouds
	low := look == LowBodies || look == AllBodies
	return backdrop.Clouds{Hidden: look == ShadowsAlone, Bodies: [2]bool{low, look == AllBodies},
		Before: low}
}

// Over has the low clouds hang over brd's ground, their bands fitted to its relief: from its low
// ground, the sea's or the valleys', up to its peaks. Call it as the plugin is made;
// WithWeathering does.
func (p *Plugin) Over(brd *board.Plugin) *Plugin {
	p.climate.Over(func() (low, peaks float64) { return reliefOf(brd.Read(0).Heights(), brd.Read(0).Board()) })
	return p
}

// the low ground of b's cells under heights h, the tenth of them lowest, none under the sea's, and
// their highest
func reliefOf(h ground.Heights, b *board.BoardRead) (low, peaks float64) {
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
	w, err := weathering.New(brd, p.read.Air, p.worldPlugin.Write().Effects(), p.calendar, cfg)
	if err != nil {
		panic(err)
	}
	p.weathering = w
	w.SetRunning(p.running.Weathering)
	p.climate.OnLand(w.Land())
	return p
}

func (p *Plugin) Name() string { return "gram.atmosphere" }

// Install wires the sky's, the weather's and the weathering's systems, and draws the sky behind
// the world, looking as its Skies say, the clouds seen from above and what falls before it.
func (p *Plugin) Install(ctx plugin.Installer) error {
	p.module = &module{p: p, sky: p.sky.System(), climate: p.climate.System(), comps: p.climate.LoadComps(), clock: p.worldPlugin.Write().Clock()}
	p.module.step = p.module.simulate
	if p.weathering != nil {
		p.module.comps = append(p.module.comps, goke.LoadComp[weathering.Wring]())
	}
	ctx.UseModule(p.module)
	p.module.rises = p.sky.RiseSystem(p.worldPlugin.Read(0).Tick, p.Entity)
	ctx.Hosts(p.climate.Rules(), p.sky.Rules())
	if p.weathering != nil {
		ctx.Hosts(p.weathering.Rules())
	}
	p.vault = backdrop.New(p.worldPlugin.Res.Config.Space, p.drawnSun, p.drawnAir).
		WithHeavens(p.drawnHeavens).
		WithShown(p.drawnShown).WithClouds(p.skyClouds)
	ctx.Draws(p.vault, lowClouds{p.aloft, p}, precipitation.NewDrops(p.drawnSun, p.drawnAir))
	ctx.Looks(p.dress)
	return nil
}

// RunPlan runs the atmosphere's tick. Call it after the world's.
func (p *Plugin) RunPlan(ctx goke.RunCtx, d time.Duration) {
	p.carryOut()
	p.module.RunPlan(ctx, d)
}

// EventHandler is nil: the atmosphere takes commands, not input.
func (p *Plugin) EventHandler() control.EventHandler { return nil }

// Serializable is the atmosphere's Settings, kept whole; the weather and its air masses are
// entities, saved with the ECS.
func (p *Plugin) Serializable() plugin.Serializable { return p.kept }

// Restore takes the Settings loaded: the light frozen at its hour, the clouds and their shadows.
func (p *Plugin) Restore() {
	p.kept.Restore()
	s := p.Res.Settings
	p.sky.SetFrozen(s.Frozen)
	p.sky.SetHour(s.Hour)
	p.write.ShowClouds(s.Clouds)
	p.write.DrawCloudsAt(s.CloudCoarse)
	p.write.ReachClouds(s.CloudReach)
	p.shadeHigh(s.HighShade)
}

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
	step          clock.Block // bound once, so a tick allocates no method value
}

func (m *module) RegSystems(ecs *goke.ECS) {
	m.skyRun = ecs.RegSys(m.sky)
	m.shineRun = ecs.RegSys(&shining{p: m.p})
	m.climateRun = ecs.RegSys(m.climate)
	m.risesRun = ecs.RegSys(m.rises)
	if w := m.p.weathering; w != nil {
		m.weatheringRun = ecs.RegSys(w.System(m.clock, m.p.worldPlugin.Read(0).Tick))
	}
}

func (m *module) RunPlan(ctx goke.RunCtx, d time.Duration) {
	ctx.Run(m.skyRun, d)
	ctx.Run(m.shineRun, d)
	ctx.Sync()
	clock.Simulate(m.clock, ctx, d, m.step)
	// a weather asked for while the clock stands is entered at once, no time passing
	if m.clock.Read().State().Paused && m.p.climate.Asked() {
		ctx.Run(m.climateRun, 0)
		ctx.Sync()
	}
}

func (m *module) simulate(ctx goke.RunCtx, step time.Duration) {
	ctx.Run(m.climateRun, step)
	ctx.Run(m.risesRun, step)
	if m.weatheringRun != nil {
		ctx.Run(m.weatheringRun, step)
	}
	ctx.Sync()
}

func (m *module) SetupSystems() []goke.System { return nil }

func (m *module) LoadComps() []goke.CompToken { return m.comps }

type reporters []render.Reporter

type cloudsReport struct{ p *Plugin }

func (r cloudsReport) Report(line func(label, value string)) {
	look := r.p.Res.Settings.Clouds.String()
	if w := r.p.read.Air(); !w.OneLayer && w.Relief[1] > 0 {
		c := w.Ceilings
		look += fmt.Sprintf("\nunder the peaks %.0f%%\ncovering %.0f%%\nover %.0f%%",
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
		if b, ok := r.(plugin.Bound); ok {
			b.Init(si)
		}
	}
}

func (rs reporters) Report(line func(label, value string)) {
	for _, r := range rs {
		r.Report(line)
	}
}
