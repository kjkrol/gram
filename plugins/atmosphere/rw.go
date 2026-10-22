package atmosphere

import (
	"slices"

	"github.com/kjkrol/gram/plugins/atmosphere/air"
	"github.com/kjkrol/gram/plugins/atmosphere/calendar"
	"github.com/kjkrol/gram/plugins/atmosphere/celestial"
	"github.com/kjkrol/gram/plugins/atmosphere/climate"
	"github.com/kjkrol/gram/plugins/atmosphere/sky"
	"github.com/kjkrol/gram/plugins/atmosphere/weathering"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/light"
	"github.com/kjkrol/gram/render"
)

// Read reads the atmosphere's state, the same on every goroutine.
type Read struct{ p *Plugin }

var _ board.Atmosphere = (*Read)(nil)

// Read reads the atmosphere's state on worker w of the game's pool, 0 the game's own goroutine.
func (p *Plugin) Read(int) *Read { return &p.read }

// Write changes the atmosphere's state, on the game's goroutine.
type Write struct{ p *Plugin }

// Write changes the atmosphere's state, on the game's goroutine.
func (p *Plugin) Write() *Write { return &p.write }

// Running is which of the atmosphere's workings go on now.
func (r *Read) Running() Running {
	running := r.p.running
	running.Day = !r.p.sky.Frozen()
	return running
}

// Sun is the light of the day as it stands: the light plugin's, the sky's of the hour as its
// Source scales it; the sky's own without a light plugin.
func (r *Read) Sun() light.Distant {
	p := r.p
	if p.light == nil {
		return p.sky.Sun()
	}
	return p.light.Read(0).Distant()
}

// Heavens is where the sun, the moon and the stars stand at the hour of the light.
func (r *Read) Heavens() celestial.Heavens { return r.p.sky.Heavens() }

// Air is the weather as the last step of the simulation left it, its clouds shading as
// CloudShadows say.
func (r *Read) Air() air.Weather {
	p := r.p
	w := p.climate.Air()
	w.HighShade, w.ShadeGround, w.ShadeFigures = p.shadows.High, p.shadows.Ground,
		p.shadows.Figures
	return w
}

// CloudShadows is whose shadows the clouds cast.
func (r *Read) CloudShadows() CloudShadows { return r.p.shadows }

// ShownClouds is how the clouds are drawn.
func (r *Read) ShownClouds() CloudLook { return r.p.Res.Settings.Clouds }

// CloudsAdaptive reports whether the clouds follow the frame budget.
func (r *Read) CloudsAdaptive() bool { return r.p.Res.Settings.AdaptiveClouds }

// CloudsDrawnAt is the coarse of the pixels each way the clouds are drawn at, 4 to start.
func (r *Read) CloudsDrawnAt() int {
	p := r.p
	if c := p.Res.Settings.CloudCoarse; c > 0 {
		return c
	}
	return coarses[0]
}

// CloudsReach is the share of the camera's sight the clouds are bodies out to, 1 everywhere.
func (r *Read) CloudsReach() float32 {
	p := r.p
	if r := p.Res.Settings.CloudReach; r > 0 {
		return r
	}
	return 1
}

// Calendar is the day, the season and the moon of the world's clock.
func (r *Read) Calendar() *calendar.Calendar { return r.p.calendar }

// Reporter is the atmosphere's lines for the telemetry panel: the daylight, how the clouds look
// and whose shadows they cast; the time, the season and the weather are the reporters of
// Calendar and Climate.
func (r *Read) Reporter() render.Reporter {
	return reporters{r.p.sky.Reporter(), cloudsReport{r.p}}
}

// SetRunning has the atmosphere's workings go on as r says, at once.
func (w *Write) SetRunning(r Running) {
	p := w.p
	p.running = r
	p.sky.SetFrozen(!r.Day)
	p.sky.SetMoon(r.Moon)
	p.climate.SetRunning(climate.Running{Changes: r.Weather, Wind: r.Wind, Clouds: r.Clouds,
		Falls: r.Falls})
	if p.weathering != nil {
		p.weathering.SetRunning(r.Weathering)
	}
}

// ShowClouds has the clouds drawn as look says (Settings.Clouds), AllBodies till then: a game's
// say as it is made, the Clouds command's as it goes.
func (w *Write) ShowClouds(look CloudLook) {
	p := w.p
	p.Res.Settings.Clouds = look
	p.aloft.Show(look != ShadowsAlone)
	p.aloft.Bodies(look == LowBodies || look == AllBodies)
}

// AdaptClouds has the clouds follow the cameras' frame budget, or not (Settings.AdaptiveClouds):
// drawn at fewer of the pixels first, as bodies to half the sight last.
func (w *Write) AdaptClouds(on bool) { w.p.Res.Settings.AdaptiveClouds = on }

// DrawCloudsAt has the clouds drawn at a coarse of the pixels each way, 4, 6 or 8
// (Settings.CloudCoarse); any other, 4.
func (w *Write) DrawCloudsAt(coarse int) {
	p := w.p
	if !slices.Contains(coarses, coarse) {
		coarse = coarses[0]
	}
	p.Res.Settings.CloudCoarse = coarse
	p.aloft.Coarse(coarse)
}

// ReachClouds has the clouds drawn as bodies out to share of the camera's sight from where it
// looks and flat beyond (Settings.CloudReach); 1 or more, or zero, everywhere.
func (w *Write) ReachClouds(share float32) {
	p := w.p
	if share <= 0 || share > 1 {
		share = 1
	}
	p.Res.Settings.CloudReach = share
	p.aloft.BodiesTo(share)
}

// Following is whether the clouds follow the frame budget (cameras.Lever).
func (w *Write) Following() bool { return w.p.read.CloudsAdaptive() }

// Coarser draws the clouds at fewer of the pixels, or at the fewest as bodies to half the sight,
// true where it could (cameras.Lever).
func (w *Write) Coarser() bool { return w.p.step(1) }

// Finer brings the clouds' bodies back to the whole sight, or draws them at more of the pixels,
// true where it could (cameras.Lever).
func (w *Write) Finer() bool { return w.p.step(-1) }

// Sky is the light of the day, to freeze and move by hand.
func (w *Write) Sky() *sky.Sky { return w.p.sky }

// Climate is the climate and its weather.
func (w *Write) Climate() *climate.Climate { return w.p.climate }

// Weathering is what the weather does to the board, nil without WithWeathering.
func (w *Write) Weathering() *weathering.Weathering { return w.p.weathering }
