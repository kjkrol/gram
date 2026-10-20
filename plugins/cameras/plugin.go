package cameras

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/cameras/internal/stick"
	"github.com/kjkrol/gram/plugins/cameras/internal/topdown"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/sector"
	"github.com/kjkrol/gram/render"
)

// Config is what a game's cameras may do and how they see: Turn (Q, E), Tilt (R, F) and
// Perspective (Tab) say which keys a player has, the rest how a world in relief is drawn and where
// the cameras may be. The zero Config is the plain camera from above: wrapping on a wrapping axis,
// a world unit a pixel at zoom 1.
type Config struct {
	Turn        bool
	Tilt        bool
	Perspective bool
	// degrees: the flattest the eye may look (zero, 30) and how much it sees top to bottom in
	// perspective (zero, 45)
	MinPitch    float32
	FieldOfView float32
	// world units over the highest ground a camera looks for what stands there; zero, 64
	Headroom float32
	// a Cell-sized square of the world drawn isometrically as a TileW x TileH diamond (zero, 64 x
	// 32), a height lifting a point HeightUnit screen units a world unit (zero, 1)
	Cell       float32
	TileW      float32
	TileH      float32
	HeightUnit float32
	// where the cameras may be; nil, anywhere over level ground at sea level
	Cage Cage
	// how far every camera sees and how finely as it is made (Sight changes it); zero, as far as
	// the world goes, every bit
	Detail camera.Detail
	// how every picture but a minimap's is drawn from the start (Settings.Tactical, Riding): a
	// camera's free, over or behind an entity by Tactical, one's inside an entity by Riding, the
	// resolution following the FrameBudget where the Strategy says, the time a frame may take
	// (zero, a sixtieth of a second): a rate the vsync caps is met, never exceeded
	Tactical    Knobs
	Riding      Knobs
	FrameBudget time.Duration
}

// plain is the camera from above that neither turns nor tilts nor sees anything in relief
func (c Config) plain() bool { return c.Cage == nil && !c.Turn && !c.Tilt && !c.Perspective }

// Start is how a camera begins: FromAbove or Isometrically.
type Start uint8

const (
	// FromAbove begins a camera looking straight down, a screen pixel a world unit.
	FromAbove Start = iota
	// Isometrically begins a camera in the 2:1 view, a Cell a TileW-wide diamond; a plain Config
	// has none.
	Isometrically
)

// Plugin makes a world's cameras and moves them: Pan, Zoom and Follow, a camera fastened Centred
// over an entity kept over it as it goes, and in relief the views, turning, tilting and riding.
type Plugin struct {
	*world.Self

	worldPlugin *world.Plugin
	cfg         Config
	views       *stick.Control
	viewCfg     stick.Config
	ground      func(x, y float32) float32
	extent      func() (low, high float32)
	cameras     []camera.Camera
	saved       [][]byte
	loaded      bool
	starts      []starting
	pans        command.Queue[Pan]
	zooms       command.Queue[Zoom]
	follows     command.Queue[Follow]
	centerOns   command.Queue[CenterOn]
	centerAts   command.Queue[CenterAt]
	zones       command.Queue[Zones]
	sights      command.Queue[Sight]
	resolutions command.Queue[Resolution]
	adapts      command.Queue[Adapt]
	pictures    command.Queue[Picture]
	sharps      command.Queue[SharpFigures]
	layers      command.Queue[Layers]
	layerShares command.Queue[LayerShares]
	layerRes    command.Queue[LayerResolutions]
	budgets     command.Queue[Budget]
	showLayers  command.Queue[ShowLayers]
	orders      orders
	zonesView   *zonesView
	layersView  *layersView
	adapter     adapter
	worn        map[camera.Camera]dressed
	kept        *plugin.Kept[Settings]
	module      *module

	Res Resources
}

var _ plugin.Plugin = (*Plugin)(nil)
var _ plugin.CommandHandler = (*Plugin)(nil)
var _ plugin.Tunable = (*Plugin)(nil)
var _ plugin.Restorer = (*Plugin)(nil)

// NewPlugin keeps and moves the cameras of worldPlugin's world as cfg says; it makes none itself.
// A Config seeing in relief (any but the plain one) panics without a Cell or over a world that
// wraps.
func NewPlugin(worldPlugin *world.Plugin, cfg Config) *Plugin {
	p := &Plugin{worldPlugin: worldPlugin, cfg: cfg, worn: map[camera.Camera]dressed{}}
	p.Self = world.NewSelf(worldPlugin, p)
	p.zonesView = &zonesView{settings: &p.Res.Settings}
	p.layersView = &layersView{settings: &p.Res.Settings}
	p.kept = plugin.Keep(&p.Res.Settings)
	p.Res.Settings = Settings{Tactical: cfg.Tactical.filled(), Riding: cfg.Riding.filled()}
	p.SetBudget(cfg.FrameBudget)
	p.SetStrategy(Tactical, cfg.Tactical.Strategy)
	p.SetStrategy(Riding, cfg.Riding.Strategy)
	worldPlugin.Anchor(anchors{p})
	if !cfg.plain() {
		if edges := worldPlugin.Res.Config.Space.Edges; edges.WrapsX() || edges.WrapsY() {
			panic("cameras: a world that wraps cannot be seen in relief")
		}
		if cfg.Cell <= 0 {
			panic("cameras: seeing in relief needs the world size of a cell, Config.Cell")
		}
		views := stick.Config{
			Cell:        cfg.Cell,
			TileW:       cfg.TileW,
			TileH:       cfg.TileH,
			HeightUnit:  cfg.HeightUnit,
			Headroom:    cfg.Headroom,
			MinPitch:    cfg.MinPitch,
			Perspective: cfg.Perspective,
			FieldOfView: cfg.FieldOfView,
		}
		p.viewCfg, p.ground, p.extent = stick.Over(views, cfg.Cage)
		var free func(x, y, z float32) bool
		if cfg.Cage != nil {
			free = cfg.Cage.Free
		}
		p.views = stick.NewControl(free, cfg.Perspective)
	}
	return p
}

// New is a camera over the world beginning as start says, configured by cfg, made where a scene
// shows it and sized by whoever shows it. It is saved with the game: one made after a Load is as
// the camera made in its place was saved.
func (p *Plugin) New(start Start, cfg camera.Config) camera.Camera {
	space := p.worldPlugin.Res.Config.Space
	var cam camera.Camera
	switch {
	case p.views == nil && start == Isometrically:
		panic("cameras: a plain Config has no isometric view")
	case p.views == nil:
		cam = topdown.NewFromSpaceWithConfig(space.Width, space.Height, space.Edges, cfg)
	default:
		from := stick.FromAbove
		if start == Isometrically {
			from = stick.Isometrically
		}
		bend := float32(p.worldPlugin.Scale().Bend())
		made := p.views.Maker(p.viewCfg, from, p.ground, p.extent, bend)
		cam = made(space.Width, space.Height, space.Edges, cfg)
	}
	cam.(seeing).SetDetail(p.cfg.Detail)
	at := len(p.cameras)
	p.cameras = append(p.cameras, cam)
	switch {
	case p.loaded && at < len(p.saved):
		p.restore(at)
	case cfg.Follow != "":
		f, ok := cam.(camera.Fastenable)
		if !ok {
			panic(fmt.Sprintf("cameras: a %T cannot be fastened, so cannot follow %q", cam,
				cfg.Follow))
		}
		p.starts = append(p.starts, starting{cam: f, whom: ceremony.Named(cfg.Follow)})
	}
	p.dress(cam)
	return cam
}

// a camera of the plugin's, seeing as far as it is told
type seeing interface {
	SetDetail(d camera.Detail)
	See(factor float64)
}

// fitsWhole reports whether cam keeps the whole world in view: a minimap's
func fitsWhole(cam camera.Camera) bool {
	if f, ok := cam.(camera.Fitting); ok {
		_, _, whole := f.Fits()
		return whole
	}
	return false
}

// the world lives round the point each camera observes, a minimap's none
type anchors struct{ p *Plugin }

var _ sector.Anchors = anchors{}

func (a anchors) Anchors(at func(x, y float32)) {
	for _, cam := range a.p.cameras {
		if fitsWhole(cam) {
			continue
		}
		if f, ok := cam.(camera.Focused); ok {
			x, y, _ := f.Focus()
			at(x, y)
		}
	}
}

type starting struct {
	cam  camera.Fastenable
	whom ceremony.Whom
}

// Cameras are every camera made, in order.
func (p *Plugin) Cameras() []camera.Camera { return p.cameras }

func (p *Plugin) Name() string { return "gram.cameras" }

func (p *Plugin) Install(ctx plugin.Installer) error {
	p.module = &module{p: p}
	ctx.UseModule(p.module)
	ctx.Draws(p.zonesView, p.layersView)
	return nil
}

// RunPlan carries out the cameras' commands and keeps every camera fastened to its entity, at
// once, in the tactical pause too, each dressed by its profile; call it after whatever moves the
// entities.
func (p *Plugin) RunPlan(ctx goke.RunCtx, d time.Duration) {
	p.switchZones()
	if f := p.adapter.step(time.Now(), render.ActualFPS()); f != 1 {
		p.redraw(f)
	}
	p.module.RunPlan(ctx, d)
	p.redressChanged()
}

// redraw has every picture but a minimap's whose profile follows the frame budget drawn at
// factor times its resolution, the Settings showing the lead camera's
func (p *Plugin) redraw(factor float32) {
	for _, cam := range p.cameras {
		if p.Strategy(ProfileOf(cam)) != Still {
			p.apply(cam, factor)
		}
	}
	p.note(p.lead())
}

// apply has cam's picture drawn at factor times its resolution, a layer at a time where it is
// drawn in layers, and is false for a minimap's or a camera without one
func (p *Plugin) apply(cam camera.Camera, factor float32) bool {
	r, ok := cam.(camera.Resolved)
	if !ok || fitsWhole(cam) {
		return false
	}
	if c, ok := cam.(camera.Layered); ok && len(c.Layers()) > 0 {
		r.SetResolution(stepLayers(c, factor))
		return true
	}
	r.SetResolution(clamped(r.Resolution() * factor))
	return true
}

// Reporter is the cameras' part of the telemetry panel: the lead camera's profile, the share of
// its pixels each picture is drawn at, layer by layer in layers with the layers' shares, the
// figures sharp where a coarser picture draws them at every pixel, and the budget it follows, or
// still, as the lead camera's profile says.
func (p *Plugin) Reporter() render.Reporter { return reporter{p} }

type reporter struct{ p *Plugin }

func (reporter) Init(*goke.SysInit) {}

func (r reporter) Report(line func(label, value string)) {
	lead := r.p.lead()
	if lead == nil {
		return
	}
	var shares []string
	coarse := false
	for _, cam := range r.p.cameras {
		if _, ok := cam.(camera.Resolved); ok && !fitsWhole(cam) {
			shares = append(shares, share(cam))
			coarse = coarse || camera.ResolutionOf(cam) < 1
		}
	}
	profile := ProfileOf(lead)
	k := r.p.knobs(profile)
	value := profile.String() + ", " + strings.Join(shares, ", ") + " of the pixels"
	switch {
	case k.Layers:
		value += fmt.Sprintf(" in layers at %.0f%% and %.0f%%", k.Middle*100, k.Inner*100)
	case coarse && !k.SoftFigures:
		value += ", the figures sharp"
	}
	if k.Strategy == Still {
		value += ", still"
	} else {
		budget := budgetRate(r.p.Res.Settings.FrameBudget)
		value += fmt.Sprintf(", following a budget of %.0f FPS", budget)
	}
	line("Picture", value)
}

// share is the resolution cam's picture is drawn at, its layers' from the outermost in layers
func share(cam camera.Camera) string {
	layers := camera.LayersOf(cam)
	if len(layers) == 0 {
		return fmt.Sprintf("%.2f", camera.ResolutionOf(cam))
	}
	parts := make([]string, len(layers))
	for i, layer := range layers {
		parts[i] = fmt.Sprintf("%.2f", layer.Resolution)
	}
	return strings.Join(parts, "/")
}

// EventHandler is nil: a player's bindings issue the commands.
func (p *Plugin) EventHandler() control.EventHandler { return nil }

// Serializable is every camera's state, its window, zoom and fastening, in the order made, and
// the Settings.
func (p *Plugin) Serializable() plugin.Serializable { return persisted{p} }

// Restore gives the cameras made so far the state a Load wrote, those made later take theirs as
// they are made, and takes the Settings loaded, the pictures drawn as they say.
func (p *Plugin) Restore() {
	p.kept.Restore()
	s := &p.Res.Settings
	s.Tactical = s.Tactical.filled()
	s.Riding = s.Riding.filled()
	p.adapter = adapter{on: p.anyFollowing()}
	p.SetBudget(s.FrameBudget)
	p.loaded = true
	for at := range min(len(p.cameras), len(p.saved)) {
		p.restore(at)
	}
	p.redress()
}

func (p *Plugin) restore(at int) {
	cam := p.cameras[at]
	dec := gob.NewDecoder(bytes.NewReader(p.saved[at]))
	for _, t := range cam.Persisted() {
		if err := dec.Decode(t); err != nil {
			log.Printf("cameras: camera %d is not as it was saved: %v", at, err)
			return
		}
	}
	cam.Restore()
}

type persisted struct{ p *Plugin }

func (s persisted) Persisted() []any {
	p := s.p
	p.saved = p.saved[:0]
	for _, cam := range p.cameras {
		var buf bytes.Buffer
		enc := gob.NewEncoder(&buf)
		for _, t := range cam.Persisted() {
			if err := enc.Encode(t); err != nil {
				panic(fmt.Sprintf("cameras: a %T cannot be saved: %v", cam, err))
			}
		}
		p.saved = append(p.saved, buf.Bytes())
	}
	return append([]any{&p.saved}, p.kept.Persisted()...)
}
