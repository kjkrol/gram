package sky

import (
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/atmosphere/celestial"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/uid"
)

// Moon is the moon as a knob: the colour and the strength of the light a full one casts high in
// the sky, and the tint of its face. The atmosphere's own entity carries it and the sky only
// reads it, so an effect's Alter turns it — a blood moon — and its end gives the moon back.
type Moon struct {
	Color    render.Light
	Strength float32
	Face     render.Light
}

// DefaultMoon is the moon as it is: a pale blue light, far brighter than the real one's so the
// night shows, and a face as it was photographed.
func DefaultMoon() Moon {
	return Moon{Color: moonColor, Strength: moonStrength, Face: render.Light{1, 1, 1}}
}

// Moonrise is the moment the moon comes over the horizon, by the calendar: Phase is how far it is
// round from new, 0 up to 1, a half full. It is about the atmosphere's own entity, so its rules
// fire while the atmosphere plays their role, and an effect one applies is a state of the
// atmosphere.
type Moonrise struct {
	Phase float32
	Self  uid.UID64
}

// Who is the atmosphere's own entity: whose moment it is, for a rule.
func (m Moonrise) Who() uid.UID64 { return m.Self }

// Full reports that the moon rising is full, or nearly: a condition for a rule's If. A moon is
// full at one rise at least every time round.
func (m Moonrise) Full() bool { return celestial.Phase(m.Phase) >= 0.75 }

// Rules takes the rules of a Moonrise.
func (s *Sky) Rules() plugin.Host { return &s.rises }

// RiseSystem tells the rules of a Moonrise as the moon comes up, in every step of the simulation:
// about is the entity the moment is of, the atmosphere's own. The first step tells nothing, so a
// loaded game hears of no rise it was saved after.
func (s *Sky) RiseSystem(tick plugin.TickSource, about func() uid.UID64) goke.System {
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
