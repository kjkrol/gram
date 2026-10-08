package topography

import (
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugin/section"
	"github.com/kjkrol/gram/plugins/atmosphere/air"
	"github.com/kjkrol/gram/plugins/atmosphere/sky"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/ground"
	"github.com/kjkrol/gram/plugins/board/look"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/topography/internal/billboards"
	icameras "github.com/kjkrol/gram/plugins/topography/internal/cameras"
	"github.com/kjkrol/gram/plugins/topography/internal/hexes"
	ipainter "github.com/kjkrol/gram/plugins/topography/internal/painter"
	irelief "github.com/kjkrol/gram/plugins/topography/internal/relief"
	"github.com/kjkrol/gram/plugins/topography/internal/terrain"
	"github.com/kjkrol/gram/plugins/topography/painter"
	"github.com/kjkrol/gram/plugins/topography/relief"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

// Atmosphere is the sky over the relief: the sun that lights it and casts its shadows, and the
// weather — the wind what sways leans in, the clouds whose shadows drift over the ground, the air
// that hazes the far off. plugins/atmosphere's Plugin is one; without one the relief stands under
// sky.DefaultSun in still, clear air.
type Atmosphere interface {
	Sun() sky.Sun
	Air() air.Weather
}

// stillSky is the sky of a relief given no atmosphere: the default sun in still, clear air.
type stillSky struct{}

func (stillSky) Sun() sky.Sun     { return sky.DefaultSun }
func (stillSky) Air() air.Weather { return air.Weather{} }

// Config is the topography: the isometric view — a Cell-sized square of the world is a TileW x
// TileH diamond, a height lifts a point HeightUnit screen units per world unit, Headroom is how
// far above the ground a camera looks for sprites; zero TileW, TileH, HeightUnit and Headroom are
// 64, 32, 1 and 64, Cell must be set — how flat the eye may look along the ground (MinPitch, degrees; 30 when zero, the 2:1
// view's, below which the near relief hides what lies behind it), whether View reaches a third
// view, in Perspective — an eye at a point of the world, placed by LookFrom, LookAt and a Ride inside a unit,
// seeing FieldOfView degrees from the top of the screen to the bottom (45 when zero) — how
// Raise, Lower and Level shape the ground (Shaping; zero: a quarter of a cell
// a step, any slope) and what slopes do to whoever goes over them (Climbing; zero:
// relief.DefaultClimbing).
type Config struct {
	Cell, TileW, TileH float32
	HeightUnit         float32
	Headroom           float32
	MinPitch           float32
	Perspective        bool
	FieldOfView        float32
	Shaping            Shaping
	Climbing           relief.Climbing
}

// Plugin is a map in relief over a board: the ground's heights, shaped by the player and pricing
// every slope; the tiles lit and shaded by the world's sun, their grounds blending, water glinting
// and running, ways and bridges over the relief, the clouds' shadows; and the views of it — from
// above, isometric and, when the game says so, in perspective, switched at play (View) — with the
// cameras turned, tilted, fastened behind a unit and, in perspective, put at a point of the world.
// It is the board's Map and the world's Ground.
type Plugin struct {
	*world.Self // its own entity: its knobs, the roles it plays, the effects it is under

	cfg         Config
	worldPlugin *world.Plugin
	boardPlugin *board.Plugin
	relief      *irelief.Relief
	painter     *ipainter.Painter
	sky         Atmosphere
	climbing    relief.Climbing
	shaper      *shaper
	seeded      func(p geom.Vec) float64

	// ground draws the ground on the GPU as a mesh of its heights, in place of the tiles; nil off
	// a square grid, where hexes draws it as prisms
	ground *terrain.Renderer
	hexes  *hexes.Ground

	cameras   *icameras.Control
	views     func(icameras.Config) cameras.Maker // a maker of the views starting as the Config says
	camQueues cameraQueues                        // the cameras' commands, which the cameras read as their Orders
	coarse    control.Queue[CoarseShadows]
	module    *module
}

var _ plugin.Plugin = (*Plugin)(nil)
var _ plugin.Populator = (*Plugin)(nil)
var _ plugin.CommandHandler = (*Plugin)(nil)
var _ board.Map = (*Plugin)(nil)

// NewPlugin puts boardPlugin, over worldPlugin, in relief as cfg says: from then on the board is
// drawn and priced by the topography, the world's ground is its heights and its entities stand as
// billboards in the isometric view; its cameras are Views, for the cameras plugin. Make it right
// after the world and the board; the world must have heights (world.Config.Heights) and may not
// wrap.
func NewPlugin(worldPlugin *world.Plugin, boardPlugin *board.Plugin, cfg Config) *Plugin {
	if !worldPlugin.HasHeights() {
		panic("topography: a map in relief needs a world with heights; set world.Config.Heights")
	}
	if edges := worldPlugin.Res.Config.Space.Edges; edges.WrapsX() || edges.WrapsY() {
		panic("topography: a world that wraps cannot be seen in relief")
	}
	brd := boardPlugin.Res.Logic.Board
	p := &Plugin{Self: world.NewSelf(worldPlugin, "gram.topography"), cfg: cfg, worldPlugin: worldPlugin, boardPlugin: boardPlugin, sky: stillSky{},
		climbing: cfg.Climbing}
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
	ground := func(x, y float32) float32 { return float32(p.topAt(geom.NewVec(float64(x), float64(y)))) }
	extent := func() (float32, float32) {
		low, high := p.relief.Extent()
		return float32(low), float32(high)
	}
	views := icameras.Config{Cell: cfg.Cell, TileW: cfg.TileW, TileH: cfg.TileH, HeightUnit: cfg.HeightUnit, Headroom: cfg.Headroom,
		MinPitch: cfg.MinPitch, Perspective: cfg.Perspective, FieldOfView: cfg.FieldOfView}
	p.cameras = icameras.NewControl(p.relief, p.topAt, cfg.Perspective)
	bend := float32(worldPlugin.Scale().Bend())
	p.views = func(start icameras.Config) cameras.Maker {
		views.Isometric = start.Isometric
		return cameras.Maker(p.cameras.Maker(views, ground, extent, bend))
	}
	if _, _, _, _, square := p.relief.Lattice(); square {
		p.ground = terrain.New(p.relief, boardSurface{p}, liveSky{p}, terrain.Config{Shadows: true, Scale: worldPlugin.Scale()})
	} else {
		p.hexes = hexes.New(worldPlugin, boardPlugin, p, p.relief, liveSky{p})
	}
	worldPlugin.SetLook(billboards.New(worldPlugin.FlatLook(), liveSky{p}, p.relief, p.ground))
	return p
}

// Start is the view a camera of a world in relief begins in: FromAbove or Isometrically.
type Start uint8

const (
	// FromAbove begins a camera looking down from above, heights not drawn.
	FromAbove Start = iota
	// Isometrically begins a camera in the isometric view.
	Isometrically
)

// Views make the cameras of a world in relief, for the cameras plugin, each beginning as start
// says: from above, isometrically and, given Config.Perspective, in perspective, View going round
// them. Every camera of such a world is one of these: what the topography draws asks for their
// lines of sight.
func (p *Plugin) Views(start Start) cameras.Maker {
	return p.views(icameras.Config{Isometric: start == Isometrically})
}

// boardSurface is the ground's look as the painter paints it out of the board's atlas — the board
// painted flat and its water, nothing before board.Plugin.WithRenderer — the way to the shore from
// every corner, the grid while the board's is on, and where water may lie: the terrain.Surface
// contract.
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
	if rs := s.p.boardPlugin.Res.Render; rs != nil && rs.ShowGridLines {
		out.Grid = look.MinGridCell
	}
	return out
}

// liveSky is the plugin's atmosphere as it stands, whenever it is set — the sky of the terrain,
// the painter and the billboards.
type liveSky struct{ p *Plugin }

func (s liveSky) Sun() sky.Sun     { return s.p.sky.Sun() }
func (s liveSky) Air() air.Weather { return s.p.sky.Air() }

// WithAtmosphere puts the relief under a: its sun lights and shades the terrain and the units,
// its weather leans what sways, lays the clouds' shadows and hazes the far off. Call it once the
// atmosphere is made, before the first frame is drawn.
func (p *Plugin) WithAtmosphere(a Atmosphere) *Plugin {
	p.sky = a
	return p
}

// topAt is the top of the cell under at as it is drawn: the ground there and its kind's Height.
func (p *Plugin) topAt(at geom.Vec) float64 {
	top := p.relief.GroundAt(at)
	brd := p.boardPlugin.Res.Logic.Board
	if c, ok := brd.CellAt(at); ok {
		top += brd.Kind(c).Height
	}
	return top
}

// Relief is the ground's heights as a game reads and sets them: the height at any point and how far
// apart it is sampled (ground.Heights), a cell's level, and all of them at once from a function
// (relief.MeanOfCells builds one from a height per cell). Raise, Lower and Level shape it.
type Relief interface {
	ground.Heights
	Altitude(c cell.ID) float64
	SetHeights(heights func(p geom.Vec) float64)
}

// Relief is the ground's heights, to read and set from a game's code.
func (p *Plugin) Relief() Relief { return p.relief }

// Style sets how the kind named name looks in relief beyond its sprite; a kind without one is
// plain ground.
func (p *Plugin) Style(name string, s painter.Style) *Plugin {
	p.painter.Style(name, s)
	return p
}

// StyleOf is how the kind named name looks, as Style set it.
func (p *Plugin) StyleOf(name string) painter.Style { return p.painter.StyleOf(name) }

// WithShadows says whether the terrain casts shadows — the ground and what stands on it hiding
// the sun from what lies behind; on by default.
func (p *Plugin) WithShadows(on bool) *Plugin {
	if p.ground != nil {
		p.ground.Shadows(on)
	}
	return p
}

// WithCoarseShadows has the terrain's shadows baked coarser — half as fine a side, a quarter of the
// work the GPU does while the sun goes on — or fine again (H switches).
func (p *Plugin) WithCoarseShadows(on bool) *Plugin {
	if p.ground != nil {
		p.ground.Coarse(on)
	}
	return p
}

// ShadowsCoarse reports whether the terrain's shadows are baked coarse.
func (p *Plugin) ShadowsCoarse() bool { return p.ground != nil && p.ground.Coarsened() }

// Seed sets the ground's heights applied when this Stage starts fresh — see Populate.
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

// =================================================================
// board.Map contract
// =================================================================

// Look is how the board's cells lie on the screen: no tiles at all, the ground drawn on the GPU
// (look.Nothing) — a mesh of the heights over a square grid, prisms over a hex one.
func (p *Plugin) Look() look.Look { return look.Nothing }

// Dressing is what lies over the tiles: the light on the relief and the terrain's shadows, the
// grounds blending, coasts, water, the ways, the clouds' shadows.
func (p *Plugin) Dressing() look.Dressing { return p.painter }

// Heights is the relief: the ground's height at any point, for sight and navigation.
func (p *Plugin) Heights() ground.Heights { return p.relief }

// Top is c's corners with its kind's Height standing on them, and its ground level.
func (p *Plugin) Top(c cell.ID) (corners [4]float32, level float32) {
	return p.relief.Top(c, p.boardPlugin.Res.Logic.Board.Kind(c).Height)
}

// Climb is how many times as long the step from one cell to its neighbour takes whoever moves in
// d as on the flat: the slope's, as relief.Climbing says.
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

// Slope is how many times as long moving at at towards dir takes whoever moves in d: the slope
// under the entity, as relief.Climbing says.
func (p *Plugin) Slope(at, dir geom.Vec, d cell.Domain) float64 {
	if !p.climbing.Feels(d) {
		return 1
	}
	return p.climbing.Factor(p.relief.SlopeAt(at, dir))
}

// relief.Climbing is how slopes slow a climb and speed a descent.
func (p *Plugin) Climbing() relief.Climbing { return p.climbing }

// =================================================================
// plugin.Plugin contract
// =================================================================

func (p *Plugin) Name() string { return "gram.topography" }

// Install wires the heights' entity, the cameras, the shaping and the altitudes.
func (p *Plugin) Install(ctx plugin.Installer) error {
	p.module = &module{
		heights:  p.relief.HeightsSystem(),
		cameras:  p.cameras.System(&p.camQueues),
		shaping:  p.shaper.system(),
		altitude: p.relief.AltitudeSystem(),
		clock:    p.worldPlugin.Clock(),
	}
	ctx.UseModule(p.module)
	return nil
}

// RunPlan carries out the view's and the shaping commands at once — a player turns the view and
// shapes the ground in the tactical pause too — and puts every unit at its altitude in the
// simulation; call it after the world has moved, before the world is drawn and before the players'
// RunPlan.
func (p *Plugin) RunPlan(ctx goke.RunCtx, d time.Duration) {
	p.coarse.Drain(func(control.Issued[CoarseShadows]) { p.WithCoarseShadows(!p.ShadowsCoarse()) })
	p.module.RunPlan(ctx, d)
}

// WithRenderer is a no-op: the topography draws through the board's and the world's renderers,
// and its ground is painted from the board's atlas, reached through its plugin.
func (p *Plugin) WithRenderer(render.AtlasSource) {}

// Renderer is the ground's renderer, drawing it on the GPU — as a mesh of its heights over a square
// grid (its terrain), as prisms over a hex one — for the scene's composer, beside the
// board's and the world's.
func (p *Plugin) Renderer() render.Layer {
	if p.ground != nil {
		return p.ground
	}
	return p.hexes
}

// EventHandler is nil: the topography takes commands, not input.
func (p *Plugin) EventHandler() control.EventHandler { return nil }

// Serializable is nil: the heights are the topography's entity and the cameras save themselves
// with the world.
func (p *Plugin) Serializable() plugin.Serializable { return nil }
