package climate

import (
	"math"
	"time"

	"github.com/kjkrol/ceremony/host"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/atmosphere/air"
	"github.com/kjkrol/gram/plugins/atmosphere/calendar"
	"github.com/kjkrol/gram/plugins/atmosphere/climate/weather"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

var _ goke.System = (*weatherSystem)(nil)

type weatherSystem struct {
	cfg      Config
	world    *world.Plugin
	calendar *calendar.Calendar
	asked    *[]string
	running  *Running

	query   *goke.Query
	now     goke.Comp[Weather]
	spawn   goke.Comp[Weather]
	host    *host.StepRules[Weathering]
	about   func() uid.UID64 // nil for the world's
	profile Profile
	current air.Weather
}

func newWeatherSystem(cfg Config, w *world.Plugin, cal *calendar.Calendar, asked *[]string, rules *host.StepRules[Weathering], running *Running) *weatherSystem {
	return &weatherSystem{cfg: cfg, world: w, calendar: cal, asked: asked, running: running, host: rules, profile: cfg.Zone.Profile()}
}

// degrees Celsius
const snowsBelow = 1

// radians a second at most
const wander = 0.05

func (s *weatherSystem) Init(si *goke.SysInit) {
	s.host.Bind()
	s.query = si.NewQueryBuilder(&s.now).Build()
	for s.query.All(); s.query.Next(); {
		return
	}
	f := si.NewFactory(&s.spawn)
	f.Create(1)
	for f.Next() {
		// thrown in the first step, in the season the calendar has then
		s.spawn.Slice(&f.Cursor)[0] = Weather{State: unbegun, Dice: s.cfg.Seed}
	}
}

const unbegun = -1

func (s *weatherSystem) begin(w *Weather, m calendar.Moment) {
	start := s.cfg.index(s.cfg.Start)
	if start < 0 {
		start = s.throw(w, m.Season(), func(int) float32 { return 1 })
	}
	s.enter(w, start)
	st := s.cfg.Weathers[start]
	w.Blow, w.Clouds, w.Billow, w.Temperature = w.Target, w.CloudsTo, w.BillowTo, s.warmth(st, m)
	w.Rain, w.Snow = falling(st.Falls, w.Temperature)
}

func (s *weatherSystem) Update(cb *goke.CmdBuf, d time.Duration) {
	m := s.calendar.Now()
	season := m.Season()
	for s.query.All(); s.query.Next(); {
		cursor := s.query.Cursor()
		w := &s.now.Slice(cursor)[0]
		if w.State == unbegun {
			s.begin(w, m)
		}
		for _, name := range *s.asked {
			if name == "" {
				s.enter(w, s.next(w, season))
			} else if at := s.cfg.index(name); at >= 0 {
				s.enter(w, at)
			}
		}
		*s.asked = (*s.asked)[:0]
		dt := float32(d.Seconds())
		if s.running.Changes {
			if w.Left -= dt; w.Left <= 0 {
				s.enter(w, s.next(w, season))
			}
		}
		s.settle(w, dt, m)
		now := w.air(s.world.Scale(), *s.running)
		s.current = now
		if !s.host.Empty() {
			t := s.world.Tick(d)
			about := t.World
			if s.about != nil {
				about = s.about()
			}
			s.host.Run(t, Weathering{Weather: now, Season: season, Self: about})
		}
		return
	}
}

func (s *weatherSystem) likely(st weather.State, season calendar.Season) float32 {
	if st.Falls > 0 {
		return st.Likely(season) * s.profile.Wet[season%4]
	}
	return st.Likely(season)
}

func (s *weatherSystem) warmth(st weather.State, m calendar.Moment) float32 {
	year := math.Sin(2 * math.Pi * (float64(m.OfYear()) - 0.125)) // highest at midsummer
	hour := math.Sin(2 * math.Pi * (float64(m.Time) - 0.375))     // highest mid-afternoon
	p := &s.profile
	return p.Mean + p.Year*float32(year) + p.Day*float32(hour) + st.Warmth
}

func falling(falls, t float32) (rain, snow float32) {
	if t < snowsBelow {
		return 0, falls
	}
	return falls, 0
}

func (s *weatherSystem) enter(w *Weather, at int32) {
	st := s.cfg.Weathers[at]
	w.State = at
	w.Left = float32(st.Lasts[0].Seconds()) + roll(&w.Dice)*float32((st.Lasts[1]-st.Lasts[0]).Seconds())
	w.Target = st.Wind[0] + roll(&w.Dice)*(st.Wind[1]-st.Wind[0])
	w.CloudsTo = st.Clouds[0] + roll(&w.Dice)*(st.Clouds[1]-st.Clouds[0])
	w.BillowTo = st.Billow[0] + roll(&w.Dice)*(st.Billow[1]-st.Billow[0])
}

func (s *weatherSystem) next(w *Weather, season calendar.Season) int32 {
	st := s.cfg.Weathers[w.State]
	at := s.throw(w, season, func(i int) float32 {
		if int32(i) == w.State {
			return 0
		}
		if len(st.Next) == 0 {
			return 1
		}
		return st.Next[s.cfg.Weathers[i].Name]
	})
	if at < 0 {
		return w.State
	}
	return at
}

// -1 when none may come
func (s *weatherSystem) throw(w *Weather, season calendar.Season, weight func(i int) float32) int32 {
	total := float32(0)
	for i, st := range s.cfg.Weathers {
		total += weight(i) * s.likely(st, season)
	}
	if total <= 0 {
		return -1
	}
	pick := roll(&w.Dice) * total
	for i, st := range s.cfg.Weathers {
		if p := weight(i) * s.likely(st, season); p > 0 {
			if pick -= p; pick < 0 {
				return int32(i)
			}
		}
	}
	return -1
}

func (s *weatherSystem) settle(w *Weather, dt float32, m calendar.Moment) {
	st := s.cfg.Weathers[w.State]
	k := 1 - float32(math.Exp(-float64(dt)/s.cfg.Blend.Seconds()))
	w.Temperature += (s.warmth(st, m) - w.Temperature) * k
	rain, snow := falling(st.Falls, w.Temperature)
	w.Blow += (w.Target - w.Blow) * k
	w.Clouds += (w.CloudsTo - w.Clouds) * k
	w.Billow += (w.BillowTo - w.Billow) * k
	w.Rain += (rain - w.Rain) * k
	w.Snow += (snow - w.Snow) * k
	w.Heading += (roll(&w.Dice)*2 - 1) * wander * dt
	if s.running.Wind {
		wind := w.wind()
		w.Drift[0] += wind[0] * dt
		w.Drift[1] += wind[1] * dt
	}
}

// world units a second
func (w *Weather) wind() [2]float32 {
	s, c := math.Sincos(float64(w.Heading))
	return [2]float32{float32(c) * w.Blow, float32(s) * w.Blow}
}

func (w *Weather) air(scale world.Scale, r Running) air.Weather {
	a := air.Weather{Wind: w.wind(), Clouds: w.Clouds, Billow: w.Billow, Rain: w.Rain, Snow: w.Snow, Temperature: w.Temperature, Drift: w.Drift}
	if !r.Wind {
		a.Wind = [2]float32{}
	}
	if !r.Clouds {
		a.Clouds = 0
	}
	if !r.Falls {
		a.Rain, a.Snow = 0, 0
	}
	a.Visibility = air.Visibility(scale, a)
	return a
}

// xorshift; 0 up to 1
func roll(dice *uint64) float32 {
	x := *dice
	if x == 0 {
		x = 1
	}
	x ^= x << 13
	x ^= x >> 7
	x ^= x << 17
	*dice = x
	return float32(x>>40) / float32(1<<24)
}
