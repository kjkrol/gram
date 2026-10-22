package topography

import (
	"fmt"
	"strconv"
	"strings"
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
// zero relief.DefaultClimbing, Care how many sectors out from the middle of the screen the
// ground is shaded with full care, and one care less every as many farther ([Plugin.SetCare]),
// zero every one.
type Config struct {
	Shaping  Shaping
	Climbing relief.Climbing
	Care     int
}

// Plugin is a map in relief over a board: the ground's heights, lit and shaded. It is the board's
// Map, the world's Ground and the cameras' Cage.
type Plugin struct {
	*world.Self

	Res Resources

	cfg         Config
	worldPlugin *world.Plugin
	boardPlugin *board.Plugin
	board       *board.BoardRead // the board's main read
	light       *light.Plugin
	relief      *irelief.Relief
	painter     *ipainter.Painter
	sky         Atmosphere
	climbing    relief.Climbing
	shaper      *shaper
	seeded      func(p geom.Vec) float64

	// nil off a square grid, where hexes draws the ground
	ground   *terrain.Renderer
	careView *careView
	hexes    *hexes.Ground

	hazing   command.Queue[Haze]
	caring   command.Queue[Care]
	adapting command.Queue[AdaptCare]
	showing  command.Queue[ShowCare]
	kept     *plugin.Kept[Settings]
	look     billboards.Look
	taking   []render.Taking // by cell, as the kinds last stood
	swaying  []float32
	takesAt  uint64
	module   *module
}

var _ plugin.Plugin = (*Plugin)(nil)
var _ plugin.Populator = (*Plugin)(nil)
var _ plugin.CommandHandler = (*Plugin)(nil)
var _ plugin.Tunable = (*Plugin)(nil)
var _ plugin.Restorer = (*Plugin)(nil)
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
	brd := boardPlugin.Read(0).Board()
	p := &Plugin{cfg: cfg, worldPlugin: worldPlugin, boardPlugin: boardPlugin, board: brd,
		light:    lightPlugin,
		sky:      stillSky{sight: air.Sight(worldPlugin.Read(0).Scale())},
		climbing: cfg.Climbing}
	p.kept = plugin.Keep(&p.Res.Settings)
	p.Self = world.NewSelf(worldPlugin, p)
	if p.climbing == (relief.Climbing{}) {
		p.climbing = relief.DefaultClimbing
	}
	shaping := cfg.Shaping
	if shaping.Step == 0 {
		w, h := brd.CellBounds()
		shaping.Step = min(w, h) / 4
	}
	p.relief = irelief.New(brd, boardPlugin.Write().Board().Touch)
	p.shaper = &shaper{relief: p.relief, cfg: shaping}
	p.painter = ipainter.New(brd, p.relief, liveSky{p}, true, map[cell.Name]painter.Style{}).WithKinds(boardPlugin.Read(0).CellKinds())
	boardPlugin.WithMap(p)
	if _, _, _, _, square := p.relief.Lattice(); square {
		p.ground = terrain.New(p.relief, boardSurface{p}, liveSky{p}, terrain.Config{
			Scale: worldPlugin.Read(0).Scale(),
			Seen:  worldPlugin.Read(0).ViewFor,
		})
		p.careView = &careView{p}
		p.SetCare(cfg.Care)
	} else {
		p.hexes = hexes.New(worldPlugin, boardPlugin, brd.Map(), p.relief, liveSky{p})
	}
	p.look = billboards.New(worldPlugin.Read(0).FlatLook(), liveSky{p}, p.relief)
	worldPlugin.Write().SetLook(p.look)
	if lightPlugin != nil {
		lightPlugin.OverRelief(p.relief)
	}
	return p
}

type boardSurface struct{ p *Plugin }

func (s boardSurface) Surface() terrain.Painted {
	var out terrain.Painted
	d := s.p.painter
	if atlas := s.p.boardPlugin.Read(0).Atlas(); atlas != nil {
		out.Albedo, out.Water, out.Px, out.WaterPx = d.Surface(atlas)
		if out.Albedo != nil {
			out.Wet, out.Wetness = d.Wet()
			out.Kinds = d.Kinds()
			out.Details, _ = d.Details()
			out.CellDetails, out.CellDetailsAt = d.CellDetails()
		}
	}
	out.Shores, out.Reach, out.Coast = d.Coast()
	out.Takes, out.Sway, out.Styled = s.p.takes()
	if line, width, shown := s.p.boardPlugin.Res.Render.Grid(); shown {
		out.Grid, out.GridLine, out.GridWidth = look.MinGridCell, line, width
	}
	return out
}

// the shade every cell takes and how much it sways, by cell row by row, read anew as the kinds
// change, with the board's count of those changes
func (p *Plugin) takes() ([]render.Taking, []float32, uint64) {
	brd := p.board
	if v := brd.Version() + 1; p.takesAt != v {
		p.takesAt = v
		p.taking, p.swaying = p.taking[:0], p.swaying[:0]
		if shape, ok := brd.Shape(); ok && !shape.Hex {
			for y := range shape.Rows {
				for x := range shape.Cols {
					k := brd.Kind(brd.At(x, y))
					p.taking = append(p.taking, k.Takes)
					p.swaying = append(p.swaying, float32(k.Sway))
				}
			}
		}
	}
	return p.taking, p.swaying, brd.Version()
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
	brd := p.board
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

// WithShading styles the light on the relief as s says: the sun's light in bands, the shade
// tinted, the sun warmed. Nothing on a hex board.
func (p *Plugin) WithShading(s painter.Shading) *Plugin {
	if p.ground != nil {
		p.ground.SetShading(s)
	}
	return p
}

// WithHaze has the air haze the ground and what stands on it as thick as the weather makes it,
// through any camera: far off from an eye, over the ground from above; off by default, all clear.
func (p *Plugin) WithHaze(on bool) *Plugin {
	p.Res.Settings.Haze = on
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
func (p *Plugin) Hazes() bool { return p.Res.Settings.Haze }

// SetCare has the ground within rings sectors of the one under the middle of the screen (and of
// the one under the eye) shaded with full care and one care less every rings sectors farther,
// down the five cares (the package's doc, "Drawing and commands"); 0, every sector with full
// care. Nothing on a hex board.
func (p *Plugin) SetCare(rings int) {
	if p.ground != nil {
		p.ground.SetCare(rings)
		p.Res.Settings.Care = p.ground.Care()
	}
}

// Care is how many sectors out from the middle of the screen the ground is shaded with full
// care, 0 every one.
func (p *Plugin) Care() int {
	if p.ground == nil {
		return 0
	}
	return p.Res.Settings.Care
}

// AdaptCare has the full care's reach follow the cameras' frame budget, or not
// (Settings.AdaptiveCare): the topography is a cameras.Lever, stepped down the care's ladder
// as the rate falls under the budget and back up as it stays over.
func (p *Plugin) AdaptCare(on bool) { p.Res.Settings.AdaptiveCare = on }

// CareAdaptive reports whether the full care's reach follows the frame budget.
func (p *Plugin) CareAdaptive() bool { return p.Res.Settings.AdaptiveCare }

// Following is whether the care follows the frame budget (cameras.Lever).
func (p *Plugin) Following() bool { return p.CareAdaptive() }

// Coarser steps the full care's reach a rung down the ladder, true where it could
// (cameras.Lever).
func (p *Plugin) Coarser() bool { return p.stepCare(-1) }

// Finer steps the full care's reach a rung up the ladder, every sector the last, true where it
// could (cameras.Lever).
func (p *Plugin) Finer() bool { return p.stepCare(1) }

func (p *Plugin) stepCare(step int) bool {
	if p.ground == nil {
		return false
	}
	before := p.Care()
	p.SetCare(p.careStepped(step))
	return p.Care() != before
}

// ShowCare has the care's bands drawn over the ground, or not, as the ShowCare command switches
// it (Settings.ShowCare): the boundaries between sectors of different care. Nothing on a hex
// board, nor with every sector shaded with full care.
func (p *Plugin) ShowCare(shown bool) { p.Res.Settings.ShowCare = shown }

// CareShown reports whether the care's bands are drawn.
func (p *Plugin) CareShown() bool { return p.Res.Settings.ShowCare }

// the rings of full care the frame budget steps through, the last every sector
var careLadder = []int{1, 2, 3, 0}

// careRung is the ladder's rung care stands on, or the next up
func careRung(care int) int {
	if care <= 0 {
		return len(careLadder) - 1
	}
	for k, r := range careLadder[:len(careLadder)-1] {
		if care <= r {
			return k
		}
	}
	return len(careLadder) - 1
}

// careStepped is the care step rungs from the one now, within the ladder
func (p *Plugin) careStepped(step int) int {
	return careLadder[min(max(careRung(p.Care())+step, 0), len(careLadder)-1)]
}

// Reporter is the topography's part of the telemetry panel: how far out the ground is shaded
// with full care and how many sectors took each care in the last frame, a line each.
func (p *Plugin) Reporter() render.Reporter { return reporter{p} }

// the cares by what they keep, the least first (terrain's ladder)
var careNames = [terrain.FullCare + 1]string{
	"least",
	"no shade",
	"flat mirror",
	"no shore",
	"everything",
}

type reporter struct{ p *Plugin }

func (reporter) Init(*goke.SysInit) {}

func (r reporter) Report(line func(label, value string)) {
	if r.p.Care() == 0 {
		line("Ground", "care everywhere")
		return
	}
	cared := r.p.ground.Cared()
	lines := []string{fmt.Sprintf("care within %d sectors", r.p.Care())}
	for c := terrain.FullCare; c >= terrain.LeastCare; c-- {
		lines = append(lines, careNames[c]+" "+strconv.Itoa(cared[c]))
	}
	line("Ground", strings.Join(lines, "\n"))
}

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

// Top is c's corners with height standing on them, and its ground level.
func (p *Plugin) Top(c cell.ID, height float64) (corners [4]float32, level float32) {
	return p.relief.Top(c, height)
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
		altitude: p.relief.AltitudeSystem(p.boardPlugin.Read(0).Footing()),
		clock:    p.worldPlugin.Write().Clock(),
	}
	p.module.step = p.module.simulate
	ctx.UseModule(p.module)
	if p.ground != nil {
		ctx.Draws(p.ground, p.careView)
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
	p.hazing.Drain(func(i command.Issued[Haze]) { p.WithHaze(i.Command.On) })
	p.caring.Drain(func(i command.Issued[Care]) { p.SetCare(i.Command.Rings) })
	p.adapting.Drain(func(i command.Issued[AdaptCare]) { p.AdaptCare(i.Command.On) })
	p.showing.Drain(func(i command.Issued[ShowCare]) { p.ShowCare(i.Command.Shown) })
	p.module.RunPlan(ctx, d)
}

// EventHandler is nil: the topography takes commands, not input.
func (p *Plugin) EventHandler() control.EventHandler { return nil }

// Serializable is the topography's Settings, kept whole; the heights live on its entity.
func (p *Plugin) Serializable() plugin.Serializable { return p.kept }

// Restore takes the Settings loaded and shades as they say.
func (p *Plugin) Restore() {
	p.kept.Restore()
	p.WithHaze(p.Res.Settings.Haze)
	p.SetCare(p.Res.Settings.Care)
}
