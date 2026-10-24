package light

import (
	"fmt"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/render"
)

// Read reads the light's state, the same on every goroutine: the sources as the clock's last
// Replay left them.
type Read struct{ p *Plugin }

// Read reads the light's state on worker w of the game's pool, 0 the game's own goroutine.
func (p *Plugin) Read(int) *Read { return &p.read }

// Write changes the light's state and readies it for a picture, on the game's goroutine.
type Write struct{ p *Plugin }

// Write changes the light's state, on the game's goroutine.
func (p *Plugin) Write() *Write { return &p.write }

// Distant is the distant light this tick, the sun's; White without one.
func (r *Read) Distant() Distant { return r.p.distant }

// Lamps are the lamps shining this tick.
func (r *Read) Lamps() []Lamp { return r.p.lamps }

// ShadowsCoarse reports whether the shadows are baked coarse.
func (r *Read) ShadowsCoarse() bool { return r.p.Res.Settings.Coarse }

// ShadowsAdaptive reports whether the shadows follow the frame budget.
func (r *Read) ShadowsAdaptive() bool { return r.p.Res.Settings.AdaptiveShadows }

// LampShadows is how many lamps nearest a picture's camera cast shadows.
func (r *Read) LampShadows() int { return r.p.Res.Settings.LampShadows }

// Reporter is the light's part of the telemetry panel, as the frame drawn is lit: the distant
// light's strength, the lamps and how many cast shadows.
func (r *Read) Reporter() render.Reporter { return reporter{r.p} }

type reporter struct{ p *Plugin }

func (r reporter) Report(line func(label, value string)) {
	var lit render.Lights
	if l := r.p.drawn(); l != nil {
		lit = *l
	}
	line("Light", fmt.Sprintf("sun %.2f\nlamps %d\nshadows of %d", lit.Sun.Strength,
		len(lit.Lamps), min(len(lit.Lamps), lit.LampShadows)))
}

// SetCoarseShadows has the shadows baked half as fine a side, a quarter of the GPU's work, or
// fine again (Settings.Coarse).
func (w *Write) SetCoarseShadows(on bool) { w.p.Res.Settings.Coarse = on }

// AdaptShadows has the shadows follow the cameras' frame budget, or not
// (Settings.AdaptiveShadows).
func (w *Write) AdaptShadows(on bool) { w.p.Res.Settings.AdaptiveShadows = on }

// SetLampShadows has the n lamps nearest a picture's camera cast shadows, 0 none
// (Settings.LampShadows).
func (w *Write) SetLampShadows(n int) { w.p.Res.Settings.LampShadows = n }

// Following is whether the shadows follow the frame budget (cameras.Lever).
func (w *Write) Following() bool { return w.p.Res.Settings.AdaptiveShadows }

// Coarser drops the lamps' shadows, or bakes the shadows coarse once they are gone, true where
// it could (cameras.Lever).
func (w *Write) Coarser() bool {
	switch {
	case w.p.Res.Settings.LampShadows > 0:
		w.SetLampShadows(0)
	case !w.p.Res.Settings.Coarse:
		w.SetCoarseShadows(true)
	default:
		return false
	}
	return true
}

// Finer bakes the shadows fine, or brings the lamps' shadows back once they are, as many as the
// Config says, true where it could (cameras.Lever).
func (w *Write) Finer() bool {
	switch {
	case w.p.Res.Settings.Coarse:
		w.SetCoarseShadows(false)
	case w.p.Res.Settings.LampShadows < w.p.cfg.LampShadows:
		w.SetLampShadows(w.p.cfg.LampShadows)
	default:
		return false
	}
	return true
}

// lighting is what the world asks of the light as it publishes its entities
type lighting struct{ p *Plugin }

func (l lighting) Join(qb *goke.QueryBuilder) { qb.Optional(&l.p.shadow) }

// Mark is the entity's Shadow; a Patch taking all for none.
func (l lighting) Mark(cur *goke.Cursor, i int) render.Shadowing {
	if s := l.p.shadow.Slice(cur); s != nil {
		return render.Shadowing{Cast: s[i].Cast, Take: s[i].Take}
	}
	return render.Shadowing{}
}
