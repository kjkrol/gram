package sky

import (
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugins/atmosphere/calendar"
)

// Sky is the light of the day over a world: the sun of the calendar's hour, or of a frozen one,
// set into the world once a tick, as it goes or in steps. The frozen light is a look at the world, like the
// camera's turn: it changes at once, in the tactical pause too, and is not saved.
type Sky struct {
	cfg      Config
	calendar *calendar.Calendar
	sun      Sun // the light of the hour, as Update last set it

	frozen  bool
	hour    float32         // the frozen light's time of day
	step    int             // the step whose light the world has; -1 before the first
	at      calendar.Moment // going on: the moment whose light the world has
	freeze  control.Queue[Freeze]
	later   control.Queue[Later]
	earlier control.Queue[Earlier]
}

// New is the sky of the day and the season cal's, as cfg says, the sun going as it goes latitude
// degrees from the equator — north or south alike — high and with long summer days near the
// equator, low and with short winter days towards the pole, all day or none at all past the polar
// circle. Its light is the calendar's hour's from the start.
func New(cal *calendar.Calendar, cfg Config, latitude float64) *Sky {
	cfg = cfg.withDefaults()
	cfg.latitude = latitude
	s := &Sky{cfg: cfg, calendar: cal, frozen: cfg.Frozen, hour: cfg.Hour, step: -1}
	m := cal.Now()
	if s.frozen {
		m.Time = s.hour
	}
	s.sun = cfg.LightAt(m.OfYear(), m.Time, m.Moon())
	return s
}

// Sun is the light of the day as it stands: the hour's, or the frozen one's.
func (s *Sky) Sun() Sun { return s.sun }

// Frozen reports whether the light stands at Hour rather than going with the calendar.
func (s *Sky) Frozen() bool { return s.frozen }

// Hour is the frozen light's time of day, 0 to 1.
func (s *Sky) Hour() float32 { return s.hour }

// SetFrozen freezes the light where it stands now, or lets it go with the calendar again.
func (s *Sky) SetFrozen(frozen bool) {
	if frozen && !s.frozen {
		s.hour = s.calendar.Now().Time
	}
	s.frozen = frozen
}

// Shift moves the frozen light by part of a day, round midnight; it does nothing while the light
// goes with the calendar.
func (s *Sky) Shift(by float32) {
	if !s.frozen {
		return
	}
	s.hour += by
	for s.hour < 0 {
		s.hour++
	}
	for s.hour >= 1 {
		s.hour--
	}
}

// halfHour is what Later and Earlier move the frozen light by.
const halfHour = float32(1) / 48

// Update carries out the commands and sets the light to the hour's, by the calendar's or the
// frozen one: as it goes, or whenever it moves onto another step.
func (s *Sky) Update() {
	s.freeze.Drain(func(control.Issued[Freeze]) { s.SetFrozen(!s.frozen) })
	s.later.Drain(func(control.Issued[Later]) { s.Shift(halfHour) })
	s.earlier.Drain(func(control.Issued[Earlier]) { s.Shift(-halfHour) })
	m := s.calendar.Now()
	if s.frozen {
		m.Time = s.hour
	}
	if s.cfg.Steps <= 0 { // going on: the moment's light, whenever the moment has moved
		if m != s.at {
			s.at = m
			s.sun = s.cfg.LightAt(m.OfYear(), m.Time, m.Moon())
		}
		return
	}
	// a hair over, so an hour moved onto a step by halves is on it, not a rounding short of it
	step := int(m.Time*float32(s.cfg.Steps)+1e-3) % s.cfg.Steps
	if at := int(m.Date)*s.cfg.Steps + step; at != s.step {
		s.step = at
		m.Time = float32(step) / float32(s.cfg.Steps)
		s.sun = s.cfg.LightAt(m.OfYear(), m.Time, m.Moon())
	}
}

// System is the sky as a goke.System, run once a tick in the interface part of an atmosphere's
// plan.
func (s *Sky) System() goke.System { return system{s} }

type system struct{ s *Sky }

func (system) Init(*goke.SysInit)                   {}
func (y system) Update(*goke.CmdBuf, time.Duration) { y.s.Update() }

// Freeze stops the light at the hour it stands, or lets a frozen light go with the calendar
// again; Later and Earlier move a frozen light half an hour on or back.
type (
	Freeze  struct{}
	Later   struct{}
	Earlier struct{}
)

// Queues are where Freeze, Later and Earlier land.
func (s *Sky) Queues() []control.CommandQueue {
	return []control.CommandQueue{&s.freeze, &s.later, &s.earlier}
}

// DefaultBindings: P freezes the light and lets it go, Shift+] and Shift+[ move a frozen light half
// an hour on and back.
func (s *Sky) DefaultBindings() []control.Binding {
	shift := control.Mods{Shift: true}
	return []control.Binding{
		control.Command(control.KeyPress{Key: control.KeyP}, "Freeze the light of the day", func(control.Context) (Freeze, bool) { return Freeze{}, true }),
		control.Command(control.KeyPress{Key: control.KeyBracketRight, Mods: shift}, "Frozen light half an hour later", func(control.Context) (Later, bool) { return Later{}, true }),
		control.Command(control.KeyPress{Key: control.KeyBracketLeft, Mods: shift}, "Frozen light half an hour earlier", func(control.Context) (Earlier, bool) { return Earlier{}, true }),
	}
}
