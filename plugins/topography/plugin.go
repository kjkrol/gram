package topography

import (
	"fmt"
	"image/color"
	"math"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/atmosphere/air"
	"github.com/kjkrol/gram/plugins/atmosphere/sky"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/topography/heightfield"
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
// 64, 32, 1 and 64, Cell must be set — whether a fresh game begins Isometric rather than from
// above, how flat the eye may look along the ground (MinPitch, degrees; 30 when zero, the 2:1
// view's, below which the near relief hides what lies behind it), whether View reaches a third
// view, in Perspective — an eye at a point of the world, placed by LookFrom, LookAt and LookOut,
// seeing FieldOfView degrees from the top of the screen to the bottom (45 when zero) — how Raise,
// Lower and Level shape the ground (Shaping; zero: a quarter of a cell a step, any slope), what
// slopes do to whoever goes over them (Climbing; zero: DefaultClimbing), and whether G reaches
// the ground drawn on the GPU from its heightmap in place of the tiles (Heightfield; see
// topography/heightfield).
type Config struct {
	Cell, TileW, TileH float32
	HeightUnit         float32
	Headroom           float32
	Isometric          bool
	MinPitch           float32
	Perspective        bool
	FieldOfView        float32
	Shaping            Shaping
	Climbing           Climbing
	Heightfield        bool
}

// Plugin is a map in relief over a board: the ground's heights, shaped by the player and pricing
// every slope; the tiles lit and shaded by the world's sun, their grounds blending, water glinting
// and running, ways and bridges over the relief, the clouds' shadows; and the views of it — from
// above, isometric and, when the game says so, in perspective, switched at play (View) — with the
// cameras turned, tilted, fastened behind a unit and, in perspective, put at a point of the world.
// It is the board's Map and the world's Ground.
type Plugin struct {
	cfg         Config
	worldPlugin *world.Plugin
	boardPlugin *board.Plugin
	relief      *Relief
	dresser     *dresser
	sky         Atmosphere
	styles      map[board.Name]Style
	projection  projection
	climbing    Climbing
	shaping     shaping
	seeded      func(p geom.Vec) float64

	// field draws the ground from its heightmap while fieldOn, in place of the tiles; nil unless
	// Config.Heightfield
	field   *heightfield.Renderer
	fieldOn bool
	fields  control.Queue[Heightfield]

	turns     control.Queue[Turn]
	tilts     control.Queue[Tilt]
	follows   control.Queue[Follow]
	drives    control.Queue[Drive]
	views     control.Queue[View]
	lookFroms control.Queue[LookFrom]
	lookAts   control.Queue[LookAt]
	lookOuts  control.Queue[LookOut]
	looks     control.Queue[Look]
	selection *selection.Plugin
	module    *module
}

var _ plugin.Plugin = (*Plugin)(nil)
var _ plugin.Populator = (*Plugin)(nil)
var _ plugin.CommandHandler = (*Plugin)(nil)
var _ board.Map = (*Plugin)(nil)

// NewPlugin puts boardPlugin, over worldPlugin, in relief as cfg says: from then on the board is
// drawn and priced by the topography, the world's ground is its heights, its cameras are the
// topography's and its entities stand as billboards in the isometric view. Make it right after
// the world and the board, before anything asks for a camera; the world must have heights
// (world.Config.Heights) and may not wrap.
func NewPlugin(worldPlugin *world.Plugin, boardPlugin *board.Plugin, cfg Config) *Plugin {
	if !worldPlugin.HasHeights() {
		panic("topography: a map in relief needs a world with heights; set world.Config.Heights")
	}
	if edges := worldPlugin.Res.Config.Space.Edges; edges.WrapsX() || edges.WrapsY() {
		panic("topography: a world that wraps cannot be seen in relief")
	}
	brd := boardPlugin.Res.Logic.Board
	p := &Plugin{cfg: cfg, worldPlugin: worldPlugin, boardPlugin: boardPlugin, styles: map[board.Name]Style{}, sky: stillSky{},
		projection: projection{Cell: cfg.Cell, TileW: cfg.TileW, TileH: cfg.TileH, HeightUnit: cfg.HeightUnit, Headroom: cfg.Headroom, MinPitch: cfg.MinPitch * math.Pi / 180, flat: !cfg.Isometric}.withDefaults(),
		climbing:   cfg.Climbing}
	if p.climbing == (Climbing{}) {
		p.climbing = DefaultClimbing
	}
	p.shaping.cfg = cfg.Shaping
	if p.shaping.cfg.Step == 0 {
		w, h := brd.CellBounds()
		p.shaping.cfg.Step = min(w, h) / 4
	}
	p.relief = NewRelief(brd)
	p.dresser = newDresser(brd, p.relief, p.sky, true, p.styles)
	p.dresser.kinds = boardPlugin.CellKindDict()
	boardPlugin.WithMap(p)
	ground := func(x, y float32) float32 { return float32(p.topAt(geom.NewVec(float64(x), float64(y)))) }
	extent := func() (float32, float32) {
		low, high := p.relief.Extent()
		return float32(low), float32(high)
	}
	worldPlugin.SetCameras(func(width, height uint32, edges aabbworld.Edges, c camera.Config) camera.Camera {
		return newCamera(p.projection, width, height, edges, c, cfg.FieldOfView*math.Pi/180, cfg.Perspective, ground, extent, float32(worldPlugin.Scale().Bend()))
	})
	if cfg.Heightfield {
		p.field = heightfield.New(p.relief, kindColours{brd}, liveSky{p}, heightfield.Config{Shadows: true, Scale: worldPlugin.Scale()})
		p.field.Hide(true)
	}
	worldPlugin.SetLook(worldLook{flat: worldPlugin.FlatLook(), d: p.dresser, field: p.field})
	return p
}

// kindColours colours the heightfield's cells: each in its kind's Color, grey for a kind of none
// — the heightfield.Colours contract.
type kindColours struct{ b *board.Board }

func (k kindColours) Size() (cols, rows int) {
	sq, _ := k.b.Square()
	return int(sq.Cols), int(sq.Rows)
}

func (k kindColours) Colour(x, y int) color.RGBA {
	c, ok := k.b.CellIndex(uint32(x), uint32(y))
	if !ok {
		return color.RGBA{}
	}
	col := k.b.Kind(c).Color
	if col.A == 0 {
		return color.RGBA{R: 128, G: 128, B: 128, A: 255}
	}
	return col
}

func (k kindColours) Version() uint64 { return k.b.Changes() }

// liveSky is the plugin's atmosphere as it stands, whenever it is set — the heightfield.Sky
// contract.
type liveSky struct{ p *Plugin }

func (s liveSky) Sun() sky.Sun     { return s.p.sky.Sun() }
func (s liveSky) Air() air.Weather { return s.p.sky.Air() }

// ShowHeightfield draws the ground from its heightmap on the GPU (topography/heightfield) in place
// of the tiles, or the tiles again — what the Heightfield command toggles; nothing without
// Config.Heightfield.
func (p *Plugin) ShowHeightfield(on bool) {
	if p.field == nil {
		return
	}
	p.fieldOn = on
	p.field.Hide(!on)
}

// HeightfieldShown reports whether the ground is drawn from its heightmap rather than as tiles.
func (p *Plugin) HeightfieldShown() bool { return p.fieldOn }

// WithAtmosphere puts the relief under a: its sun lights and shades the terrain and the units,
// its weather leans what sways, lays the clouds' shadows and hazes the far off. Call it once the
// atmosphere is made, before the first frame is drawn.
func (p *Plugin) WithAtmosphere(a Atmosphere) *Plugin {
	p.sky, p.dresser.sky = a, a
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

// Relief is the ground's heights, to read and shape from a game's code.
func (p *Plugin) Relief() *Relief { return p.relief }

// Style sets how the kind named name looks in relief beyond its sprite; a kind without one is
// plain ground.
func (p *Plugin) Style(name string, s Style) *Plugin {
	p.styles[board.Named(name)] = s
	return p
}

// StyleOf is how the kind named name looks, as Style set it.
func (p *Plugin) StyleOf(name string) Style { return p.styles[board.Named(name)] }

// WithShadows says whether the terrain casts shadows — the ground and what stands on it hiding
// the sun from what lies behind; on by default.
func (p *Plugin) WithShadows(on bool) *Plugin {
	p.dresser.shadows = on
	return p
}

// WithSelection lets Follow fasten a camera behind the unit selectionPlugin has selected; call it
// before the plugin is installed.
func (p *Plugin) WithSelection(selectionPlugin *selection.Plugin) *Plugin {
	p.selection = selectionPlugin
	return p
}

// Seed sets the ground's heights applied when this Stage starts fresh — see Populate.
func (p *Plugin) Seed(heights func(p geom.Vec) float64) { p.seeded = heights }

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

// Look is how the board's cells lie on the screen: blocks in the isometric view, flat tiles from
// above; no tiles at all while the ground is drawn from its heightmap (board.Nothing).
func (p *Plugin) Look() board.Look {
	if p.fieldOn {
		return board.Nothing
	}
	return boardLook{d: p.dresser}
}

// Dressing is what lies over the tiles: the light on the relief and the terrain's shadows, the
// grounds blending, coasts, water, the ways, the clouds' shadows.
func (p *Plugin) Dressing() board.Dressing { return p.dresser }

// Heights is the relief: the ground's height at any point, for sight and navigation.
func (p *Plugin) Heights() board.Heights { return p.relief }

// Top is c's corners with its kind's Height standing on them, and its ground level.
func (p *Plugin) Top(c board.CellID) (corners [4]float32, level float32) {
	return p.relief.Top(c, p.boardPlugin.Res.Logic.Board.Kind(c).Height)
}

// Climb is how many times as long the step from one cell to its neighbour takes whoever moves in
// d as on the flat: the slope's, as Climbing says.
func (p *Plugin) Climb(from, to board.CellID, d board.Domain) float64 {
	return p.relief.Climb(from, to, d, p.climbing)
}

// Least is the smallest Climb for d: the quickest descent's.
func (p *Plugin) Least(d board.Domain) float64 {
	if !p.climbing.Feels(d) {
		return 1
	}
	return p.climbing.Least()
}

// Slope is how many times as long moving at at towards dir takes whoever moves in d: the slope
// under the entity, as Climbing says.
func (p *Plugin) Slope(at, dir geom.Vec, d board.Domain) float64 {
	if !p.climbing.Feels(d) {
		return 1
	}
	return p.climbing.Factor(p.relief.slopeAt(at, dir))
}

// Climbing is how slopes slow a climb and speed a descent.
func (p *Plugin) Climbing() Climbing { return p.climbing }

// =================================================================
// plugin.Plugin contract
// =================================================================

func (p *Plugin) Name() string { return "gram.topography" }

// Install wires the heights' entity, the cameras, the shaping and the altitudes.
func (p *Plugin) Install(ctx plugin.Installer) error {
	cams := &cameraSystem{turns: &p.turns, tilts: &p.tilts, follows: &p.follows, drives: &p.drives, views: &p.views,
		lookFroms: &p.lookFroms, lookAts: &p.lookAts, lookOuts: &p.lookOuts, looks: &p.looks, relief: p.relief, topAt: p.topAt}
	if p.selection != nil {
		cams.selected, cams.selecting = p.selection.Tags().Selected, true
	}
	p.module = &module{
		heights:  &heightsSystem{relief: p.relief},
		cameras:  cams,
		shaping:  &shapingSystem{relief: p.relief, shape: &p.shaping},
		altitude: newAltitudeSystem(p.relief),
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
	p.fields.Drain(func(control.Issued[Heightfield]) { p.ShowHeightfield(!p.fieldOn) })
	p.module.RunPlan(ctx, d)
}

// WithRenderer is a no-op: the topography draws through the board's and the world's renderers,
// and the heightfield's needs no atlas.
func (p *Plugin) WithRenderer(render.AtlasSource) {}

// Renderer is the heightfield's renderer, drawing the ground from its heightmap on the GPU while
// ShowHeightfield says so (topography/heightfield) — for the scene's composer, beside the board's
// and the world's; nil without Config.Heightfield.
func (p *Plugin) Renderer() render.Layer {
	if p.field == nil {
		return nil
	}
	return p.field
}

// EventHandler is nil: the topography takes commands, not input.
func (p *Plugin) EventHandler() control.EventHandler { return nil }

// Serializable is nil: the heights are the topography's entity and the cameras save themselves
// with the world.
func (p *Plugin) Serializable() plugin.Serializable { return nil }

// RegisterBehavior refuses every behavior: the topography hosts none.
func (p *Plugin) RegisterBehavior(behaviors ...plugin.Behavior) error {
	for _, b := range behaviors {
		return fmt.Errorf("%w: %T in %s", plugin.ErrUnhostedBehavior, b, p.Name())
	}
	return nil
}
