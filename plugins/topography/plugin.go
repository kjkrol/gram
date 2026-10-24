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
	"github.com/kjkrol/gram/plugins/atmosphere/air"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/ground"
	"github.com/kjkrol/gram/plugins/board/look"
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

// Atmosphere is the weather over the relief as the frame drawn reads it, what leans what sways,
// hazes the far off and casts the clouds' shadows: plugins/atmosphere's Read. Given none the
// relief stands in still, clear air. The light is the light plugin's; given none, light.White,
// which the relief hands the frame itself.
type Atmosphere interface {
	DrawnAir() air.Weather
}

// clear air seen as far as the world's scale lets one see
type stillSky struct{ sight float64 }

func (s stillSky) DrawnAir() air.Weather { return air.Weather{Sight: s.sight} }

// stillFrame hands the frame the default sun and the still air while no plugin gives them, giving
// way to what some other source has put on the frame, whichever composes first
type stillFrame struct{ p *Plugin }

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
// ground is shaded with full care, and one care less every as many farther ([Write.SetCare]),
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
	drawnBoard  board.Drawn      // the board as the frame drawn holds it
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
	// the relief and the settings as the frame drawn holds them
	drawn   render.Buffered[drawnTopography]
	swapped bool
	// the sectors the last frame drawn shaded with each care, handed over as it is swapped
	cared  [terrain.FullCare + 1]int
	module *module
	read   Read
	reads  []*Read
	write  Write
}

var _ plugin.Plugin = (*Plugin)(nil)
var _ plugin.Populator = (*Plugin)(nil)
var _ plugin.CommandHandler = (*Plugin)(nil)
var _ plugin.Tunable = (*Plugin)(nil)
var _ plugin.Restorer = (*Plugin)(nil)
var _ board.Map = (*Plugin)(nil)
var _ plugin.Publisher = (*Plugin)(nil)

// drawnTopography is the relief and the settings a frame is drawn by
type drawnTopography struct {
	relief   irelief.Relief
	settings Settings
	grid     look.RenderState // the board's
}

// Publish hands the frame drawn next the relief as it stands, its heights copied as they change,
// and the topography's settings.
func (p *Plugin) Publish() {
	b := p.drawn.Back()
	p.relief.CopyInto(&b.relief)
	b.settings = p.Res.Settings
	b.grid = look.RenderState{}
	if r := p.boardPlugin.Res.Render; r != nil {
		b.grid = *r
	}
}

// Swap has the relief and the settings published last drawn.
func (p *Plugin) Swap() {
	p.drawn.Swap()
	p.swapped = true
	if p.ground != nil {
		p.cared = p.ground.Cared()
	}
}

// drawnRelief is the relief the frame drawn holds, the live one before any was published
func (p *Plugin) drawnRelief() *irelief.Relief {
	if !p.swapped {
		return p.relief
	}
	return &p.drawn.Front().relief
}

// drawnSettings are the settings the frame drawn holds
func (p *Plugin) drawnSettings() Settings {
	if !p.swapped {
		return p.Res.Settings
	}
	return p.drawn.Front().settings
}

// drawnGrid is the board's grid as the frame drawn holds it
func (p *Plugin) drawnGrid() look.RenderState {
	if !p.swapped {
		if r := p.boardPlugin.Res.Render; r != nil {
			return *r
		}
		return look.RenderState{}
	}
	return p.drawn.Front().grid
}

// drawnMap is the board's cells standing on the relief as the frame drawn holds them
type drawnMap struct{ p *Plugin }

func (m drawnMap) Look() look.Look         { return m.p.Look() }
func (m drawnMap) Heights() ground.Heights { return m.p.drawnRelief() }
func (m drawnMap) Dressing() look.Dressing { return m.p.painter }

func (m drawnMap) Top(c cell.ID) (corners [4]float32, level float32) {
	return m.p.drawnRelief().Top(c, m.p.drawnBoard.Kind(c).Height)
}

// DrawnHeights is the ground's height as the frame drawn holds it (board.Read.DrawnHeights).
func (p *Plugin) DrawnHeights() ground.Heights { return drawnHeights{p} }

// drawnHeights is the frame's relief, for what draws on it
type drawnHeights struct{ p *Plugin }

func (h drawnHeights) At(at geom.Vec) float64 { return h.p.drawnRelief().At(at) }
func (h drawnHeights) Step() float64          { return h.p.drawnRelief().Step() }

// drawnGround is the relief the terrain draws, the frame's
type drawnGround struct{ p *Plugin }

func (g drawnGround) Lattice() (cols, rows int, cell float32, heights []float32, ok bool) {
	return g.p.drawnRelief().Lattice()
}

func (g drawnGround) Version() uint64 { return g.p.drawnRelief().Version() }

// lightRelief is the relief as the light takes it: its heights as they stand, for the shade it
// bakes as it publishes, and the ground the frame drawn holds, under the shadows cast in it
type lightRelief struct{ p *Plugin }

func (r lightRelief) Lattice() (cols, rows int, cell float32, heights []float32, ok bool) {
	return r.p.relief.Lattice()
}

func (r lightRelief) Version() uint64 { return r.p.relief.Version() }

func (r lightRelief) GroundAt(at geom.Vec) float64 { return r.p.drawnRelief().GroundAt(at) }

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
		drawnBoard: boardPlugin.Read(0).Drawn(),
		light:      lightPlugin,
		sky:        stillSky{sight: air.Sight(worldPlugin.Read(0).Scale())},
		climbing:   cfg.Climbing}
	p.kept = plugin.Keep(&p.Res.Settings)
	p.read, p.write = Read{p: p}, Write{p: p}
	p.reads = []*Read{&p.read}
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
	p.painter = ipainter.New(p.drawnBoard, p.drawnRelief, true, map[cell.Name]painter.Style{}).
		WithKinds(boardPlugin.Read(0).CellKinds())
	boardPlugin.WithMap(p)
	if _, _, _, _, square := p.relief.Lattice(); square {
		p.ground = terrain.New(drawnGround{p}, boardSurface{p}, liveSky{p}, terrain.Config{
			Scale: worldPlugin.Read(0).Scale(),
			Seen:  worldPlugin.Read(0).ViewFor,
			Drawn: func() (int, bool) {
				s := p.drawnSettings()
				return s.Care, s.Haze
			},
		})
		p.careView = &careView{p}
		p.write.SetCare(cfg.Care)
	} else {
		p.hexes = hexes.New(worldPlugin, boardPlugin, p.drawnBoard, drawnMap{p}, p.drawnRelief,
			func() (bool, look.RenderState) { return p.drawnSettings().Haze, p.drawnGrid() },
			liveSky{p}, p.painter)
	}
	p.look = billboards.New(worldPlugin.Read(0).FlatLook(), liveSky{p},
		func() (*irelief.Relief, bool) { return p.drawnRelief(), p.drawnSettings().Haze })
	worldPlugin.Write().SetLook(p.look)
	if lightPlugin != nil {
		lightPlugin.OverRelief(lightRelief{p})
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
	out.Shores, out.Reach, out.Coast = d.Shores(s.p.boardPlugin.Read(0).Atlas())
	out.Takes, out.Sway, out.Styled = s.p.takes()
	grid := s.p.drawnGrid()
	if line, width, shown := grid.Grid(); shown {
		out.Grid, out.GridLine, out.GridWidth = look.MinGridCell, line, width
	}
	return out
}

// the shade every cell takes and how much it sways, by cell row by row, read anew as the kinds
// change, with the board's count of those changes
func (p *Plugin) takes() ([]render.Taking, []float32, uint64) {
	brd := p.drawnBoard
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

// the sun the frame drawn is lit by, the light plugin's, and the weather the atmosphere gives
type liveSky struct{ p *Plugin }

func (s liveSky) Distant() light.Distant {
	if l := s.p.worldPlugin.Read(0).Lights(); l != nil {
		return l.Sun
	}
	return light.White
}
func (s liveSky) Air() air.Weather { return s.p.sky.DrawnAir() }

// WithAtmosphere puts the relief under a: its weather leans what sways, lays the clouds' shadows
// and hazes the far off. Call it before the first frame is drawn.
func (p *Plugin) WithAtmosphere(a Atmosphere) *Plugin {
	p.sky = a
	return p
}

// Relief is the ground's heights as a game reads and sets them: ground.Heights, a cell's level
// and all of them at once from a function.
type Relief interface {
	ground.Heights
	Altitude(c cell.ID) float64
	SetHeights(heights func(p geom.Vec) float64)
}

// Style sets how the kind named name looks in relief beyond its sprite; a kind without one is
// plain ground.
func (p *Plugin) Style(name string, s painter.Style) *Plugin {
	p.painter.Style(name, s)
	return p
}

// WithShading styles the light on the relief as s says: the sun's light in bands, the shade
// tinted, the sun warmed. Nothing on a hex board.
func (p *Plugin) WithShading(s painter.Shading) *Plugin {
	if p.ground != nil {
		p.ground.SetShading(s)
	}
	return p
}

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
	return careLadder[min(max(careRung(p.read.Care())+step, 0), len(careLadder)-1)]
}

// the cares by what they keep, the least first (terrain's ladder)
var careNames = [terrain.FullCare + 1]string{
	"least",
	"no shade",
	"flat mirror",
	"no shore",
	"everything",
}

type reporter struct{ p *Plugin }

func (r reporter) Report(line func(label, value string)) {
	if r.p.read.Care() == 0 {
		line("Ground", "care everywhere")
		return
	}
	cared := r.p.cared
	lines := []string{fmt.Sprintf("care within %d sectors", r.p.read.Care())}
	for c := terrain.FullCare; c >= terrain.LeastCare; c-- {
		lines = append(lines, careNames[c]+" "+strconv.Itoa(cared[c]))
	}
	line("Ground", strings.Join(lines, "\n"))
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
	for w := len(p.reads); w < ctx.Pool().Size(); w++ {
		p.reads = append(p.reads, &Read{p: p, w: w})
	}
	return nil
}

// RunPlan carries out the shaping commands at once, in the tactical pause too, and puts every
// unit at its altitude in the simulation. Call it after the world has moved and before the
// cameras' RunPlan.
func (p *Plugin) RunPlan(ctx goke.RunCtx, d time.Duration) {
	p.hazing.Drain(func(i command.Issued[Haze]) { p.write.SetHaze(i.Command.On) })
	p.caring.Drain(func(i command.Issued[Care]) { p.write.SetCare(i.Command.Rings) })
	p.adapting.Drain(func(i command.Issued[AdaptCare]) { p.write.AdaptCare(i.Command.On) })
	p.showing.Drain(func(i command.Issued[ShowCare]) { p.write.ShowCare(i.Command.Shown) })
	p.module.RunPlan(ctx, d)
}

// EventHandler is nil: the topography takes commands, not input.
func (p *Plugin) EventHandler() control.EventHandler { return nil }

// Serializable is the topography's Settings, kept whole; the heights live on its entity.
func (p *Plugin) Serializable() plugin.Serializable { return p.kept }

// Restore takes the Settings loaded and shades as they say.
func (p *Plugin) Restore() {
	p.kept.Restore()
	p.write.SetHaze(p.Res.Settings.Haze)
	p.write.SetCare(p.Res.Settings.Care)
}
