package topography

import (
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugin/section"
	"github.com/kjkrol/gram/plugins/atmosphere/air"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/ground"
	"github.com/kjkrol/gram/plugins/board/look"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/light"
	"github.com/kjkrol/gram/plugins/topography/internal/billboards"
	"github.com/kjkrol/gram/plugins/topography/internal/hexes"
	ipainter "github.com/kjkrol/gram/plugins/topography/internal/painter"
	irelief "github.com/kjkrol/gram/plugins/topography/internal/relief"
	"github.com/kjkrol/gram/plugins/topography/internal/terrain"
	"github.com/kjkrol/gram/plugins/topography/painter"
	"github.com/kjkrol/gram/plugins/topography/relief"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

// Atmosphere is the weather over the relief, what leans what sways, hazes the far off and casts
// the clouds' shadows: plugins/atmosphere's Plugin. Given none the relief stands in still, clear
// air. The light is the light plugin's; given none, light.White, which the relief hands the
// frame itself.
type Atmosphere interface {
	Air() air.Weather
}

// clear air seen as far as the world's scale lets one see
type stillSky struct{ sight float64 }

func (s stillSky) Air() air.Weather { return air.Weather{Sight: s.sight} }

// stillFrame hands the frame the default sun and the still air while no plugin gives them, giving
// way to what some other source has put on the frame, whichever composes first
type stillFrame struct{ p *Plugin }

func (stillFrame) Init(*goke.SysInit) {}

func (s stillFrame) Compose(f *render.Frame, _ camera.Camera) {
	if _, still := s.p.sky.(stillSky); !still || f.Uniformed("LightWay") {
		return
	}
	if s.p.light == nil {
		light.White.Frame(f)
	}
	air.Weather{}.Frame(f, light.White, 0, 0)
}

// Config is the topography: Shaping zero is a quarter of a cell a step and any slope, Climbing
// zero relief.DefaultClimbing.
type Config struct {
	Shaping  Shaping
	Climbing relief.Climbing
}

// Plugin is a map in relief over a board: the ground's heights, lit and shaded. It is the board's
// Map, the world's Ground and the cameras' Cage.
type Plugin struct {
	*world.Self

	cfg         Config
	worldPlugin *world.Plugin
	boardPlugin *board.Plugin
	light       *light.Plugin
	relief      *irelief.Relief
	painter     *ipainter.Painter
	sky         Atmosphere
	climbing    relief.Climbing
	shaper      *shaper
	seeded      func(p geom.Vec) float64

	// nil off a square grid, where hexes draws the ground
	ground *terrain.Renderer
	hexes  *hexes.Ground

	hazing  command.Queue[Haze]
	hazed   bool
	look    billboards.Look
	taking  []render.Taking // by cell, as the kinds last stood
	takesAt uint64
	module  *module
}

var _ plugin.Plugin = (*Plugin)(nil)
var _ plugin.Populator = (*Plugin)(nil)
var _ plugin.CommandHandler = (*Plugin)(nil)
var _ board.Map = (*Plugin)(nil)
var _ cameras.Cage = (*Plugin)(nil)
var _ cameras.Grained = (*Plugin)(nil)

// NewPlugin puts boardPlugin, over worldPlugin, in relief as cfg says, lit by lightPlugin (nil,
// light.White), whose shadows fall on the relief from then on: the topography is the
// board's Map and the world's Ground and Look. It panics for a world that wraps.
func NewPlugin(worldPlugin *world.Plugin, boardPlugin *board.Plugin, lightPlugin *light.Plugin,
	cfg Config) *Plugin {
	if edges := worldPlugin.Res.Config.Space.Edges; edges.WrapsX() || edges.WrapsY() {
		panic("topography: a world that wraps cannot be seen in relief")
	}
	brd := boardPlugin.Board()
	p := &Plugin{cfg: cfg, worldPlugin: worldPlugin, boardPlugin: boardPlugin, light: lightPlugin,
		sky:      stillSky{sight: air.Sight(worldPlugin.Scale())},
		climbing: cfg.Climbing}
	p.Self = world.NewSelf(worldPlugin, p)
	if p.climbing == (relief.Climbing{}) {
		p.climbing = relief.DefaultClimbing
	}
	shaping := cfg.Shaping
	if shaping.Step == 0 {
		w, h := brd.CellBounds()
		shaping.Step = min(w, h) / 4
	}
	p.relief = irelief.New(brd)
	p.shaper = &shaper{relief: p.relief, cfg: shaping}
	p.painter = ipainter.New(brd, p.relief, liveSky{p}, true, map[cell.Name]painter.Style{}).WithKinds(boardPlugin.CellKinds())
	boardPlugin.WithMap(p)
	if _, _, _, _, square := p.relief.Lattice(); square {
		p.ground = terrain.New(p.relief, boardSurface{p}, liveSky{p}, terrain.Config{
			Scale: worldPlugin.Scale(),
			Seen:  worldPlugin.ViewFor,
		})
	} else {
		p.hexes = hexes.New(worldPlugin, boardPlugin, p, p.relief, liveSky{p})
	}
	p.look = billboards.New(worldPlugin.FlatLook(), liveSky{p}, p.relief)
	worldPlugin.SetLook(p.look)
	if lightPlugin != nil {
		lightPlugin.OverRelief(p.relief)
	}
	return p
}

type boardSurface struct{ p *Plugin }

func (s boardSurface) Surface() terrain.Painted {
	var out terrain.Painted
	d := s.p.painter
	if atlas := s.p.boardPlugin.Atlas(); atlas != nil {
		out.Albedo, out.Water, out.Px, out.WaterPx = d.Surface(atlas)
		if out.Albedo != nil {
			out.Wet, out.Wetness = d.Wet()
		}
	}
	out.Shores, out.Reach, out.Coast = d.Coast()
	out.Takes, out.Kinds = s.p.takes()
	if line, width, shown := s.p.boardPlugin.Res.Render.Grid(); shown {
		out.Grid, out.GridLine, out.GridWidth = look.MinGridCell, line, width
	}
	return out
}

// the shade every cell takes, by cell row by row, read anew as the kinds change, with the
// board's count of those changes
func (p *Plugin) takes() ([]render.Taking, uint64) {
	brd := p.boardPlugin.Board()
	if v := brd.Version() + 1; p.takesAt != v {
		p.takesAt = v
		p.taking = p.taking[:0]
		if shape, ok := brd.Shape(); ok && !shape.Hex {
			for y := range shape.Rows {
				for x := range shape.Cols {
					p.taking = append(p.taking, brd.Kind(brd.At(x, y)).Takes)
				}
			}
		}
	}
	return p.taking, brd.Version()
}

// the sun the light plugin shines, and the weather the atmosphere gives, as they stand
type liveSky struct{ p *Plugin }

func (s liveSky) Distant() light.Distant {
	if s.p.light == nil {
		return light.White
	}
	return s.p.light.Distant()
}
func (s liveSky) Air() air.Weather { return s.p.sky.Air() }

// WithAtmosphere puts the relief under a: its weather leans what sways, lays the clouds' shadows
// and hazes the far off. Call it before the first frame is drawn.
func (p *Plugin) WithAtmosphere(a Atmosphere) *Plugin {
	p.sky = a
	return p
}

func (p *Plugin) topAt(at geom.Vec) float64 {
	top := p.relief.GroundAt(at)
	brd := p.boardPlugin.Board()
	if c, ok := brd.CellAt(at); ok {
		top += brd.Kind(c).Height
	}
	return top
}

// Relief is the ground's heights as a game reads and sets them: ground.Heights, a cell's level
// and all of them at once from a function.
type Relief interface {
	ground.Heights
	Altitude(c cell.ID) float64
	SetHeights(heights func(p geom.Vec) float64)
}

// Relief is the ground's heights, to read and set from a game's code.
func (p *Plugin) Relief() Relief { return p.relief }

// Flatten lays the cell c flat at h, the ground round it following within the Shaping's
// MaxStep, and reports whether it did. Nothing moves where a road would be left lying as
// Transport Tycoon would not lay it.
func (p *Plugin) Flatten(c cell.ID, h float64) bool {
	return p.relief.KeepRoads(func() { p.relief.Level(c, h, p.cfg.Shaping.MaxStep) })
}

var _ ground.Shaper = (*Plugin)(nil)

// Style sets how the kind named name looks in relief beyond its sprite; a kind without one is
// plain ground.
func (p *Plugin) Style(name string, s painter.Style) *Plugin {
	p.painter.Style(name, s)
	return p
}

// StyleOf is how the kind named name looks, as Style set it.
func (p *Plugin) StyleOf(name string) painter.Style { return p.painter.StyleOf(name) }

// WithHaze has the air haze the ground and what stands on it as thick as the weather makes it,
// through any camera: far off from an eye, over the ground from above; off by default, all clear.
func (p *Plugin) WithHaze(on bool) *Plugin {
	p.hazed = on
	if p.ground != nil {
		p.ground.Haze(on)
	}
	if p.hexes != nil {
		p.hexes.Haze(on)
	}
	p.look.Haze(on)
	return p
}

// Hazes reports whether the air hazes the ground.
func (p *Plugin) Hazes() bool { return p.hazed }

// Seed sets the ground's heights raised when the Stage starts fresh.
func (p *Plugin) Seed(heights func(p geom.Vec) float64) {
	if err := p.worldPlugin.InSection("the ground's heights seeded", section.Spawn); err != nil {
		panic("topography: " + err.Error())
	}
	p.seeded = heights
}

// Populate raises the seeded heights.
func (p *Plugin) Populate() error {
	if p.seeded != nil {
		p.relief.SetHeights(p.seeded)
		p.seeded = nil
	}
	return nil
}

// Look is look.Nothing: the ground is drawn on the GPU, not as tiles.
func (p *Plugin) Look() look.Look { return look.Nothing }

// Dressing is the painter: the light, the blends, the coasts, the water and the ways over the
// tiles.
func (p *Plugin) Dressing() look.Dressing { return p.painter }

// Heights is the relief: the ground's height at any point, for sight and navigation.
func (p *Plugin) Heights() ground.Heights { return p.relief }

// Top is c's corners with its kind's Height standing on them, and its ground level.
func (p *Plugin) Top(c cell.ID) (corners [4]float32, level float32) {
	return p.relief.Top(c, p.boardPlugin.Board().Kind(c).Height)
}

// Climb is how many times as long the step from one cell to its neighbour takes whoever moves
// in d as on the flat, as the Climbing says.
func (p *Plugin) Climb(from, to cell.ID, d cell.Domain) float64 {
	return p.relief.Climb(from, to, d, p.climbing)
}

// Least is the smallest Climb for d: the quickest descent's.
func (p *Plugin) Least(d cell.Domain) float64 {
	if !p.climbing.Feels(d) {
		return 1
	}
	return p.climbing.Least()
}

// Slope is how many times as long moving at at towards dir takes whoever moves in d as on the
// flat, as the Climbing says.
func (p *Plugin) Slope(at, dir geom.Vec, d cell.Domain) float64 {
	if !p.climbing.Feels(d) {
		return 1
	}
	return p.climbing.Factor(p.relief.SlopeAt(at, dir))
}

// Climbing is what slopes do to whoever goes over them, as the Config set it.
func (p *Plugin) Climbing() relief.Climbing { return p.climbing }

func (p *Plugin) Name() string { return "gram.topography" }

// Install registers the plugin's systems.
func (p *Plugin) Install(ctx plugin.Installer) error {
	p.module = &module{
		heights:  p.relief.HeightsSystem(),
		shaping:  p.shaper.system(),
		altitude: p.relief.AltitudeSystem(),
		clock:    p.worldPlugin.Clock(),
	}
	ctx.UseModule(p.module)
	if p.ground != nil {
		ctx.Draws(p.ground)
	} else {
		ctx.Draws(p.hexes)
	}
	ctx.Draws(stillFrame{p})
	return nil
}

// RunPlan carries out the shaping commands at once, in the tactical pause too, and puts every
// unit at its altitude in the simulation. Call it after the world has moved and before the
// cameras' RunPlan.
func (p *Plugin) RunPlan(ctx goke.RunCtx, d time.Duration) {
	p.hazing.Drain(func(command.Issued[Haze]) { p.WithHaze(!p.Hazes()) })
	p.module.RunPlan(ctx, d)
}

// EventHandler is nil: the topography takes commands, not input.
func (p *Plugin) EventHandler() control.EventHandler { return nil }

// Serializable is nil: the heights live on the topography's entity.
func (p *Plugin) Serializable() plugin.Serializable { return nil }
