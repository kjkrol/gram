package topography

import (
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/command"
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
// weather. plugins/atmosphere's Plugin is one; given none the relief stands under sky.DefaultSun
// in still, clear air.
type Atmosphere interface {
	Sun() sky.Sun
	Air() air.Weather
}

type stillSky struct{}

func (stillSky) Sun() sky.Sun     { return sky.DefaultSun }
func (stillSky) Air() air.Weather { return air.Weather{} }

// Config is the topography: Cell (world units, must be set) is drawn isometrically as a TileW x
// TileH diamond (64 x 32 when zero), a height lifts a point HeightUnit screen units a world unit
// (1), Headroom is how far above the ground a camera looks for sprites (64), MinPitch the
// flattest the eye may look along the ground in degrees (30), Perspective whether View reaches
// a perspective view seeing FieldOfView degrees top to bottom (45). Shaping zero is a quarter
// of a cell a step and any slope, Climbing zero relief.DefaultClimbing.
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

// Plugin is a map in relief over a board: the ground's heights, lit and shaded, and the views of
// it from above, isometric and in perspective. It is the board's Map and the world's Ground.
type Plugin struct {
	*world.Self

	cfg         Config
	worldPlugin *world.Plugin
	boardPlugin *board.Plugin
	relief      *irelief.Relief
	painter     *ipainter.Painter
	sky         Atmosphere
	climbing    relief.Climbing
	shaper      *shaper
	seeded      func(p geom.Vec) float64

	// nil off a square grid, where hexes draws the ground
	ground *terrain.Renderer
	hexes  *hexes.Ground

	cameras   *icameras.Control
	views     func(icameras.Config) cameras.Maker
	camQueues cameraQueues
	coarse    command.Queue[CoarseShadows]
	module    *module
}

var _ plugin.Plugin = (*Plugin)(nil)
var _ plugin.Populator = (*Plugin)(nil)
var _ plugin.CommandHandler = (*Plugin)(nil)
var _ board.Map = (*Plugin)(nil)

// NewPlugin puts boardPlugin, over worldPlugin, in relief as cfg says: from then on the
// topography is the board's Map and the world's Ground and Look. It panics for a world without
// heights or one that wraps.
func NewPlugin(worldPlugin *world.Plugin, boardPlugin *board.Plugin, cfg Config) *Plugin {
	if !worldPlugin.HasHeights() {
		panic("topography: a map in relief needs a world with heights; set world.Config.Heights")
	}
	if edges := worldPlugin.Res.Config.Space.Edges; edges.WrapsX() || edges.WrapsY() {
		panic("topography: a world that wraps cannot be seen in relief")
	}
	brd := boardPlugin.Board()
	p := &Plugin{cfg: cfg, worldPlugin: worldPlugin, boardPlugin: boardPlugin, sky: stillSky{},
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
	ground := func(x, y float32) float32 {
		return float32(p.seenAt(geom.NewVec(float64(x), float64(y))))
	}
	extent := func() (float32, float32) {
		low, high := p.relief.Extent()
		return float32(low), float32(high)
	}
	views := icameras.Config{Cell: cfg.Cell, TileW: cfg.TileW, TileH: cfg.TileH, HeightUnit: cfg.HeightUnit, Headroom: cfg.Headroom,
		MinPitch: cfg.MinPitch, Perspective: cfg.Perspective, FieldOfView: cfg.FieldOfView,
		EdgeTurn:   TurnStep / cameras.DefaultScrollSpeed,
		EdgeTilt:   TiltStep / cameras.DefaultScrollSpeed,
		CornerTurn: CornerStep / cameras.DefaultScrollSpeed,
		Fine:       p.fineAt,
		Solid:      p.solidAt}
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

// Views make the cameras of a world in relief for the cameras plugin, each beginning as start
// says. Every camera of such a world must be one of them.
func (p *Plugin) Views(start Start) cameras.Maker {
	return p.views(icameras.Config{Isometric: start == Isometrically})
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
	if rs := s.p.boardPlugin.Res.Render; rs != nil && rs.ShowGridLines {
		out.Grid = look.MinGridCell
	}
	return out
}

type liveSky struct{ p *Plugin }

func (s liveSky) Sun() sky.Sun     { return s.p.sky.Sun() }
func (s liveSky) Air() air.Weather { return s.p.sky.Air() }

// WithAtmosphere puts the relief under a: its sun lights and shades it, its weather leans what
// sways, lays the clouds' shadows and hazes the far off. Call it before the first frame is drawn.
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

func (p *Plugin) fineAt(x, y float32) (float32, float32) {
	brd := p.boardPlugin.Board()
	if !brd.NearDeck(geom.NewVec(float64(x), float64(y))) {
		return 0, 0
	}
	return float32(brd.NarrowestDeck() / 2), float32(brd.ThinnestDeck())
}

func (p *Plugin) solidAt(x, y, z float32) (float32, bool) {
	at := geom.NewVec(float64(x), float64(y))
	if top := float32(p.topAt(at)); z < top {
		return top, true
	}
	top, in := float32(0), false
	p.boardPlugin.Board().DecksOver(at, func(c cell.ID, f cell.Floor) {
		deck := float32(p.relief.Altitude(c) + f.Height)
		if f.Height > 0 && z < deck && z >= deck-max(float32(f.Thickness), 1e-3) {
			top, in = max(top, deck), true
		}
	})
	return top, in
}

func (p *Plugin) seenAt(at geom.Vec) float64 {
	top := p.topAt(at)
	p.boardPlugin.Board().DecksOver(at, func(c cell.ID, f cell.Floor) {
		top = max(top, p.relief.Altitude(c)+f.Height)
	})
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

// WithShadows says whether the terrain casts shadows; on by default.
func (p *Plugin) WithShadows(on bool) *Plugin {
	if p.ground != nil {
		p.ground.Shadows(on)
	}
	return p
}

// WithCoarseShadows has the terrain's shadows baked half as fine a side, a quarter of the GPU's
// work, or fine again.
func (p *Plugin) WithCoarseShadows(on bool) *Plugin {
	if p.ground != nil {
		p.ground.Coarse(on)
	}
	return p
}

// ShadowsCoarse reports whether the terrain's shadows are baked coarse.
func (p *Plugin) ShadowsCoarse() bool { return p.ground != nil && p.ground.Coarsened() }

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
		cameras:  p.cameras.System(&p.camQueues),
		shaping:  p.shaper.system(),
		altitude: p.relief.AltitudeSystem(),
		clock:    p.worldPlugin.Clock(),
	}
	ctx.UseModule(p.module)
	return nil
}

// RunPlan carries out the views' and the shaping commands at once, in the tactical pause too,
// and puts every unit at its altitude in the simulation. Call it after the world has moved and
// before the players' RunPlan.
func (p *Plugin) RunPlan(ctx goke.RunCtx, d time.Duration) {
	p.coarse.Drain(func(command.Issued[CoarseShadows]) { p.WithCoarseShadows(!p.ShadowsCoarse()) })
	p.module.RunPlan(ctx, d)
}

// Renderer is the ground's renderer, a render.Direct at the Ground tier for the scene's composer.
func (p *Plugin) Renderer() render.Layer {
	if p.ground != nil {
		return p.ground
	}
	return p.hexes
}

// EventHandler is nil: the topography takes commands, not input.
func (p *Plugin) EventHandler() control.EventHandler { return nil }

// Serializable is nil: the heights live on the topography's entity and the cameras save
// themselves.
func (p *Plugin) Serializable() plugin.Serializable { return nil }
