package sky

import (
	"fmt"
	"math"
	"time"

	"github.com/kjkrol/ceremony/host"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/atmosphere/calendar"
	"github.com/kjkrol/gram/plugins/atmosphere/celestial"
)

// Sky is the light of the day over a world: the sun of the calendar's hour, or of a frozen one.
// A frozen light is a look at the world, changing in the tactical pause too and not saved.
type Sky struct {
	cfg      Config
	calendar *calendar.Calendar
	sun      Sun
	heavens  celestial.Heavens

	frozen   bool
	moonless bool
	moon     Moon
	rises    host.StepRules[Moonrise]
	hour     float32 // the frozen light's part of the day, 0 to 1
	step     int     // -1 before the first
	at       calendar.Moment
}

// New is the sky over cal as cfg says, the sun's path at latitude degrees from the equator,
// north or south alike; a Daylight whose rows do not rise panics.
func New(cal *calendar.Calendar, cfg Config, latitude float64) *Sky {
	cfg = cfg.withDefaults()
	cfg.latitude = latitude
	for i := 1; i < len(cfg.Daylight); i++ {
		if cfg.Daylight[i].Height <= cfg.Daylight[i-1].Height {
			panic(fmt.Sprintf("sky: daylight row %d at %v does not rise over row %d at %v", i,
				cfg.Daylight[i].Height, i-1, cfg.Daylight[i-1].Height))
		}
	}
	s := &Sky{cfg: cfg, calendar: cal, frozen: cfg.Frozen, hour: dayPart(cfg.Hour), step: -1,
		moon: cfg.Moon}
	m := cal.Now()
	if s.frozen {
		m.Time = s.hour
	}
	s.light(m)
	return s
}

// Sun is the light of the day as it stands.
func (s *Sky) Sun() Sun { return s.sun }

// Heavens is where the sun, the moon and the stars stand at the hour of the light.
func (s *Sky) Heavens() celestial.Heavens { return s.heavens }

// Moonlight is the moon the sky lights the night with, the atmosphere's knob.
func (s *Sky) Moonlight() Moon { return s.moon }

func (s *Sky) light(m calendar.Moment) {
	if s.moonless {
		s.sun = s.cfg.SunAt(m.OfYear(), m.Time)
	} else {
		s.sun = s.cfg.LightAt(m.OfYear(), m.Time, m.Moon(), s.moon)
	}
	s.heavens = s.cfg.place().HeavensAt(m.OfYear(), m.Time, m.Moon())
	s.heavens.MoonTint = s.moon.Face
	_, s.heavens.SunColor, _ = s.cfg.daylightAt(s.heavens.Sun[2])
}

func (s *Sky) relight() {
	m := s.calendar.Now()
	if s.frozen {
		m.Time = s.hour
	}
	s.step, s.at = -1, m
	s.light(m)
}

// SetMoon has the moon light the night, or not.
func (s *Sky) SetMoon(on bool) {
	if s.moonless != on {
		return
	}
	s.moonless = !on
	s.relight()
}

// Moon reports whether the moon lights the night.
func (s *Sky) Moon() bool { return !s.moonless }

// Frozen reports whether the light stands rather than going with the calendar.
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

// HalfHour is half an hour as a part of the day, for Shift.
const HalfHour = float32(1) / 48

// Update sets the light to the hour's, as it goes or whenever it moves onto another step.
func (s *Sky) Update() {
	m := s.calendar.Now()
	if s.frozen {
		m.Time = s.hour
	}
	if s.cfg.Steps <= 0 {
		if m != s.at {
			s.at = m
			s.light(m)
		}
		return
	}
	// a hair over, so an hour moved onto a step by halves is on it, not a rounding short of it
	step := int(m.Time*float32(s.cfg.Steps)+1e-3) % s.cfg.Steps
	if at := int(m.Date)*s.cfg.Steps + step; at != s.step {
		s.step = at
		m.Time = float32(step) / float32(s.cfg.Steps)
		s.light(m)
	}
}

// System is the sky as a goke.System, run once a tick.
func (s *Sky) System() goke.System { return &system{s: s} }

type system struct {
	s     *Sky
	moons *goke.Query
	moon  goke.Comp[Moon]
}

func (y *system) Init(si *goke.SysInit) { y.moons = si.NewQueryBuilder(&y.moon).Build() }

func (y *system) Update(*goke.CmdBuf, time.Duration) {
	for y.moons.All(); y.moons.Next(); {
		if m := y.moon.Slice(y.moons.Cursor())[0]; m != y.s.moon {
			y.s.moon = m
			y.s.relight()
		}
		break
	}
	y.s.Update()
}

func dayPart(at time.Duration) float32 {
	f := float64(at) / float64(24*time.Hour)
	return float32(f - math.Floor(f))
}
