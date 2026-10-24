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
	// Levers is what else the frame budget turns, coarser before the pictures' resolution and
	// finer after it, in order: the topography's care.
	Levers []Lever
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
	edgeScrolls command.Queue[EdgeScroll]
	edgeSpeeds  command.Queue[EdgeSpeed]
	sights      command.Queue[Sight]
	resolutions command.Queue[Resolution]
	adapts      command.Queue[Adapt]
	pictures    command.Queue[Picture]
	sharps      command.Queue[SharpFigures]
	layers      command.Queue[Layers]
	layerShares command.Queue[LayerShares]
	layerRes    command.Queue[LayerResolutions]
	budgets     command.Queue[Budget]
	samplings   command.Queue[Sampling]
	thresholds  command.Queue[Thresholds]
	levers      []Lever
	showLayers  command.Queue[ShowLayers]
	orders      orders
	zonesView   *zonesView
	layersView  *layersView
	adapter     adapter
	worn        map[camera.Camera]dressed
	kept        *plugin.Kept[Settings]
	module      *module

	Res   Resources
	read  Read
	write Write
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
	p.read.p, p.write.p = p, p
	p.Self = world.NewSelf(worldPlugin, p)
	p.zonesView = &zonesView{drawnSettings{live: &p.Res.Settings}}
	p.layersView = &layersView{drawnSettings{live: &p.Res.Settings}}
	p.kept = plugin.Keep(&p.Res.Settings)
	p.levers = cfg.Levers
	p.Res.Settings = Settings{Tactical: cfg.Tactical.filled(), Riding: cfg.Riding.filled()}
	p.write.SetBudget(cfg.FrameBudget)
	p.write.SetSampling(DefaultSampling)
	p.write.SetThresholds(DefaultUnder, DefaultMet)
	p.write.SetStrategy(Tactical, cfg.Tactical.Strategy)
	p.write.SetStrategy(Riding, cfg.Riding.Strategy)
	worldPlugin.Write().Anchor(anchors{p})
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
	p.adapter.on = p.following()
	if f := p.adapter.step(time.Now(), render.ActualFPS()); f != 1 {
		p.redraw(f)
	}
	p.module.RunPlan(ctx, d)
	p.redressChanged()
}

// following reports whether anything follows the frame budget: a profile's pictures or a lever
func (p *Plugin) following() bool {
	if p.anyFollowing() {
		return true
	}
	for _, l := range p.levers {
		if l.Following() {
			return true
		}
	}
	return false
}

// redraw turns what follows the frame budget a step coarser (factor under 1) or finer: the
// levers first either way, the first first going coarser and the last first coming back, then
// the pictures; a step taken ends it. The levers give first and return first: the cheapest
// to the eye takes the rate's wavering, the pictures move under a lasting change alone.
func (p *Plugin) redraw(factor float32) {
	if factor < 1 {
		for _, l := range p.levers {
			if l.Following() && l.Coarser() {
				return
			}
		}
		p.step(factor)
		return
	}
	for i := len(p.levers) - 1; i >= 0; i-- {
		if l := p.levers[i]; l.Following() && l.Finer() {
			return
		}
	}
	p.step(factor)
}

// step has every picture but a minimap's whose profile follows the frame budget drawn at
// factor times its resolution, the Settings showing the lead camera's; true where one changed
func (p *Plugin) step(factor float32) bool {
	changed := false
	for _, cam := range p.cameras {
		if p.read.Strategy(ProfileOf(cam)) != Still && p.apply(cam, factor) {
			changed = true
		}
	}
	p.note(p.lead())
	return changed
}

// apply has cam's picture drawn at factor times its resolution, a layer at a time where it is
// drawn in layers, and is true where the resolutions changed: not for a minimap's, a camera
// without one or one at the end of its range
func (p *Plugin) apply(cam camera.Camera, factor float32) bool {
	r, ok := cam.(camera.Resolved)
	if !ok || fitsWhole(cam) {
		return false
	}
	before := resolutionsOf(cam)
	if c, ok := cam.(camera.Layered); ok && len(c.Layers()) > 0 {
		r.SetResolution(stepLayers(c, factor))
	} else {
		r.SetResolution(clamped(r.Resolution() * factor))
	}
	return !sameResolutions(before, resolutionsOf(cam))
}

type reporter struct{ p *Plugin }

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
	parts := []string{profile.String(), strings.Join(shares, ", ") + " of the pixels"}
	switch {
	case k.Layers:
		parts = append(parts,
			fmt.Sprintf("in layers at %.0f%% and %.0f%%", k.Middle*100, k.Inner*100))
	case coarse && !k.SoftFigures:
		parts = append(parts, "the figures sharp")
	}
	if k.Strategy == Still {
		parts = append(parts, "still")
	} else {
		budget := budgetRate(r.p.Res.Settings.FrameBudget)
		parts = append(parts, fmt.Sprintf("following a budget of %.0f FPS", budget))
	}
	line("Picture", strings.Join(parts, "\n"))
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
	p.adapter = adapter{}
	p.write.SetBudget(s.FrameBudget)
	p.write.SetSampling(s.Sampling)
	p.write.SetThresholds(s.Under, s.Met)
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
