package sky

import (
	"github.com/kjkrol/ceremony/effect"
	"time"

	"github.com/kjkrol/ceremony/cond"
	"github.com/kjkrol/ceremony/host"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/atmosphere/celestial"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/uid"
)

// Moon is the moon as a knob on the atmosphere's entity: the colour and the strength of the light
// a full one casts high in the sky, and the tint of its face.
type Moon struct {
	Color    render.Light
	Strength float32
	Face     render.Light
}

// Knob: an effect may turn it (effect.Knob).
func (Moon) Knob() {}

var _ effect.Knob = Moon{}

// DefaultMoon is a pale blue light and the face as it was photographed.
func DefaultMoon() Moon {
	return Moon{Color: moonColor, Strength: moonStrength, Face: render.Light{1, 1, 1}}
}

// Moonrise is the moment the moon comes over the horizon, about the atmosphere's own entity.
// Phase is how far the moon is round from new, 0 up to 1, a half full.
type Moonrise struct {
	Phase float32
	Self  uid.UID64
}

// Who is the atmosphere's own entity.
func (m Moonrise) Who() uid.UID64 { return m.Self }

// FullMoon is the word of the moon rising full or nearly.
func FullMoon() cond.Word[Moonrise] {
	return cond.NewWord("full moon",
		func(m Moonrise) bool { return celestial.Phase(m.Phase) >= 0.75 })
}

// Rules takes the rules of a Moonrise.
func (s *Sky) Rules() host.Host { return &s.rises }

// RiseSystem tells the rules of a Moonrise as the moon comes up, about the entity about gives.
// The first step tells nothing, so a loaded game hears of no rise it was saved after.
func (s *Sky) RiseSystem(tick host.TickSource, about func() uid.UID64) goke.System {
	var last float32
	begun := false
	return goke.SystemFn{
		OnInit: func(*goke.SysInit) { s.rises.Bind() },
		OnUpdate: func(cb *goke.CmdBuf, d time.Duration) {
			m := s.calendar.Now()
			up := s.cfg.place().MoonAt(m.OfYear(), m.Time, m.Moon())[2]
			if begun && last <= 0 && up > 0 && !s.rises.Empty() {
				s.rises.Run(tick.Of(cb, d), Moonrise{Phase: m.Moon(), Self: about()})
			}
			last, begun = up, true
		},
	}
}

var (
	_ host.About = Moonrise{}
)
