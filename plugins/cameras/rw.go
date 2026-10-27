package cameras

import (
	"fmt"
	"time"

	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/cameras/internal/stick"
	"github.com/kjkrol/gram/plugins/cameras/internal/topdown"
	"github.com/kjkrol/gram/render"
)

// Read reads the cameras' state, the same on every goroutine.
type Read struct{ p *Plugin }

// Read reads the cameras' state on worker w of the game's pool, 0 the game's own goroutine.
func (p *Plugin) Read(int) *Read { return &p.read }

// Write changes the cameras' state, on the game's goroutine.
type Write struct{ p *Plugin }

// Write changes the cameras' state, on the game's goroutine.
func (p *Plugin) Write() *Write { return &p.write }

// Strategy is how the resolution of profile's pictures follows the frame budget.
func (r *Read) Strategy(profile Profile) Strategy { return r.p.knobs(profile).Strategy }

// Layers reports whether profile's pictures are drawn in layers.
func (r *Read) Layers(profile Profile) bool { return r.p.knobs(profile).Layers }

// Resolution is the share of their pixels profile's pictures are drawn at, the centre layer's
// in layers.
func (r *Read) Resolution(profile Profile) float32 { return r.p.knobs(profile).Resolution }

// SharpFigures reports whether profile's pictures draw their figures at every pixel.
func (r *Read) SharpFigures(profile Profile) bool { return !r.p.knobs(profile).SoftFigures }

// LayerShares are the inner and the middle layer's shares of the frame in profile's pictures.
func (r *Read) LayerShares(profile Profile) (inner, middle float32) {
	p := r.p
	k := p.knobs(profile)
	return k.Inner, k.Middle
}

// LayerResolutions are the outer and the middle layer's resolutions in profile's pictures.
func (r *Read) LayerResolutions(profile Profile) (outer, middle float32) {
	p := r.p
	k := p.knobs(profile)
	return k.OuterRes, k.MiddleRes
}

// Thresholds are the shares of the budget's rate the following steps at: coarser under the
// first, finer over the second.
func (r *Read) Thresholds() (under, met float32) {
	p := r.p
	return p.Res.Settings.Under, p.Res.Settings.Met
}

// Sampling is how long the frame rate is averaged over before the pictures follow it.
func (r *Read) Sampling() time.Duration { return r.p.Res.Settings.Sampling }

// Budget is the time a frame may take, which the resolution follows.
func (r *Read) Budget() time.Duration { return r.p.Res.Settings.FrameBudget }

// LayersShown reports whether profile's layers' edges are shown.
func (r *Read) LayersShown(profile Profile) bool { return r.p.knobs(profile).ShowLayers }

// ZonesShown reports whether the scroll zones are shown.
func (r *Read) ZonesShown() bool { return r.p.Res.Settings.Zones }

// ZonesOpacity is how opaque the scroll zones are at the picture's edge, 0 to 1.
func (r *Read) ZonesOpacity() float32 { return r.p.Res.Settings.zonesOpacity() }

// EdgesScroll reports whether the cursor at a picture's edge scrolls the camera.
func (r *Read) EdgesScroll() bool { return !r.p.Res.Settings.StillEdges }

// EdgeSpeed is how fast the cursor at the very edge scrolls, 0 for the keys' own.
func (r *Read) EdgeSpeed() float32 { return r.p.Res.Settings.EdgeSpeed }

// Cameras are every camera made, in order.
func (r *Read) Cameras() []camera.Camera { return r.p.cameras }

// Reporter is the cameras' part of the telemetry panel, a line each: the lead camera's profile,
// the share of its pixels each picture is drawn at, layer by layer, the layers' shares in layers,
// the figures sharp where a coarser picture draws them at every pixel, and the budget it follows,
// or still, as the lead camera's profile says.
func (r *Read) Reporter() render.Reporter { return reporter{r.p} }

// SetStrategy has the resolution of profile's pictures follow the frame budget as s says, or
// stay (Knobs.Strategy): the pictures drawn in layers under InLayers, whole under Uniform.
func (w *Write) SetStrategy(profile Profile, s Strategy) {
	p := w.p
	k := p.knobs(profile)
	k.Strategy = s
	if s != Still {
		k.Layers = s == InLayers
	}
	p.redress()
}

// SetLayers has profile's pictures drawn in layers, or whole (Knobs.Layers); a Strategy saying
// otherwise stops (Still).
func (w *Write) SetLayers(profile Profile, on bool) {
	p := w.p
	k := p.knobs(profile)
	k.Layers = on
	if s := k.Strategy; s == InLayers && !on || s == Uniform && on {
		p.still(profile)
	}
	p.redress()
}

// SetResolution has profile's pictures drawn at r of their pixels, the centre layer's in
// layers, LeastResolution up to 1 (Knobs.Resolution), and stops the following (Still).
func (w *Write) SetResolution(profile Profile, r float32) {
	p := w.p
	p.knobs(profile).Resolution = clamped(r)
	p.still(profile)
	p.redress()
}

// SetSharpFigures has profile's pictures draw their figures, all that lies over the ground, at
// every shown pixel while the ground is drawn at the resolution, or at the resolution too
// (Knobs.SoftFigures); a picture drawn in layers is drawn as it is.
func (w *Write) SetSharpFigures(profile Profile, on bool) {
	p := w.p
	p.knobs(profile).SoftFigures = !on
	p.redress()
}

// SetLayerShares has the inner layer of profile's pictures cover inner of the frame and the
// middle layer middle, at least a tenth over the inner (Knobs.Inner, Middle).
func (w *Write) SetLayerShares(profile Profile, inner, middle float32) {
	p := w.p
	k := p.knobs(profile)
	inner = min(max(inner, 0.1), 0.8)
	k.Inner = inner
	k.Middle = min(max(middle, inner+0.1), 0.9)
	p.redress()
}

// SetLayerResolutions has the outer layer of profile's pictures drawn at outer of its pixels and
// the middle layer at middle, LeastResolution up to 1 (Knobs.OuterRes, MiddleRes), and stops
// the following (Still).
func (w *Write) SetLayerResolutions(profile Profile, outer, middle float32) {
	p := w.p
	k := p.knobs(profile)
	k.OuterRes = clamped(outer)
	k.MiddleRes = clamped(middle)
	p.still(profile)
	p.redress()
}

// SetBudget sets the time a frame may take, which the resolution follows
// (Settings.FrameBudget); zero or less, a sixtieth of a second.
func (w *Write) SetBudget(d time.Duration) {
	p := w.p
	if d <= 0 {
		d = time.Second / 60
	}
	p.Res.Settings.FrameBudget = d
	p.adapter.budget = d
}

// SetThresholds sets the shares of the budget's rate the following steps at (Settings.Under,
// Settings.Met): coarser once a window's rate falls under the first, finer once two have met
// the second, each between a half and the whole, the second never under the first; zero or
// less, DefaultUnder and DefaultMet.
func (w *Write) SetThresholds(under, met float32) {
	p := w.p
	if under <= 0 {
		under = DefaultUnder
	}
	if met <= 0 {
		met = DefaultMet
	}
	under = min(max(under, 0.5), 1)
	met = min(max(met, under), 1)
	p.Res.Settings.Under, p.Res.Settings.Met = under, met
	p.adapter.underAt, p.adapter.metAt = float64(under), float64(met)
}

// SetSampling sets how long the frame rate is averaged over before the pictures follow it
// (Settings.Sampling), LeastSampling up to MostSampling; zero or less, DefaultSampling.
func (w *Write) SetSampling(d time.Duration) {
	p := w.p
	if d <= 0 {
		d = DefaultSampling
	}
	d = min(max(d, LeastSampling), MostSampling)
	p.Res.Settings.Sampling = d
	p.adapter.sampling = d
}

// ShowLayers shows the layers' edges over every picture drawn in layers by profile, or hides
// them (Knobs.ShowLayers).
func (w *Write) ShowLayers(profile Profile, shown bool) { w.p.knobs(profile).ShowLayers = shown }

// ShowZones shows the scroll zones over every picture, or hides them (Settings.Zones).
func (w *Write) ShowZones(shown bool) { w.p.Res.Settings.Zones = shown }

// SetZonesOpacity sets how opaque the scroll zones are at the picture's edge
// (Settings.ZonesOpacity), kept between LeastZonesOpacity and 1.
func (w *Write) SetZonesOpacity(opacity float32) {
	w.p.Res.Settings.ZonesOpacity = min(max(opacity, LeastZonesOpacity), 1)
}

// ScrollFromEdges has the cursor at a picture's edge scroll the camera, or not
// (Settings.StillEdges); on from the start.
func (w *Write) ScrollFromEdges(on bool) { w.p.Res.Settings.StillEdges = !on }

// SetEdgeSpeed sets how fast the cursor at the very edge scrolls, screen pixels a tick
// (Settings.EdgeSpeed); 0 or less, the keys' own.
func (w *Write) SetEdgeSpeed(speed float32) { w.p.Res.Settings.EdgeSpeed = max(speed, 0) }

// New is a camera over the world beginning as start says, configured by cfg, made where a scene
// shows it and sized by whoever shows it. It is saved with the game: one made after a Load is as
// the camera made in its place was saved.
func (w *Write) New(start Start, cfg camera.Config) camera.Camera {
	p := w.p
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
		bend := p.worldPlugin.Read(0).Scale().Bend()
		made := p.views.Maker(p.viewCfg, from, p.ground, p.extent, bend)
		cam = made(space.Width, space.Height, space.Edges, cfg)
	}
	cam.(seeing).SetDetail(p.cfg.Detail)
	p.worldPlugin.Write().NewView(cam)
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
