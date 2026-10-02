package climate

import (
	"math"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/atmosphere/air"
	"github.com/kjkrol/gram/plugins/atmosphere/calendar"
	"github.com/kjkrol/gram/plugins/atmosphere/climate/weather"
	"github.com/kjkrol/gram/plugins/world"
)

var _ goke.System = (*weatherSystem)(nil)

// weatherSystem moves the weather on every step of the simulation, on the calendar's day: counts
// its state down and throws the next when it runs out, or when Change or Set asks; brings the
// wind, the clouds, what falls and the temperature towards the state's, the wind's way wandering;
// carries the clouds on the wind; keeps the air as it stands; and runs the rules hosted with
// it.
type weatherSystem struct {
	cfg      Config
	world    *world.Plugin
	calendar *calendar.Calendar
	change   *control.Queue[Change]
	set      *control.Queue[Set]
	running  *Running // which of the workings go on, the Climate's

	query   *goke.Query
	now     goke.Comp[Weather]
	spawn   goke.Comp[Weather]
	host    *plugin.Rules[Weathering]
	profile Profile                // the zone's climate in numbers
	about   func(i int) Weathering // what a rule hears, bound once so a tick allocates nothing
	told    Weathering
	current air.Weather // the air as the last step left it, what Climate.Air gives
}

func newWeatherSystem(cfg Config, w *world.Plugin, cal *calendar.Calendar, change *control.Queue[Change], set *control.Queue[Set], rules *plugin.Rules[Weathering], running *Running) *weatherSystem {
	s := &weatherSystem{cfg: cfg, world: w, calendar: cal, change: change, set: set, running: running, host: rules, profile: cfg.Zone.Profile()}
	s.about = func(int) Weathering { return s.told }
	return s
}

// snowsBelow is the temperature, degrees Celsius, below which what falls comes down as snow.
const snowsBelow = 1

// wander is how fast the wind's way drifts, radians a second at most.
const wander = 0.05

func (s *weatherSystem) Init(si *goke.SysInit) {
	qb := si.NewQueryBuilder(&s.now)
	s.host.Bind(qb)
	s.query = qb.Build()
	for s.query.All(); s.query.Next(); {
		return // a loaded game brought its weather
	}
	f := si.NewFactory(&s.spawn)
	f.Create(1)
	for f.Next() {
		// the state is thrown in the first step, in the season the calendar has then
		s.spawn.Slice(&f.Cursor)[0] = Weather{State: unbegun, Dice: s.cfg.Seed}
	}
}

// unbegun is the state of a weather not yet begun.
const unbegun = -1

// begin has w begin in cfg's Start or, without one, in a state thrown as season has them, already
// at its wind, clouds, temperature and fall rather than coming round to them.
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

// Update moves the weather on by d, one step of the simulation.
func (s *weatherSystem) Update(cb *goke.CmdBuf, d time.Duration) {
	m := s.calendar.Now()
	season := m.Season()
	for s.query.All(); s.query.Next(); {
		cursor := s.query.Cursor()
		w := &s.now.Slice(cursor)[0]
		if w.State == unbegun {
			s.begin(w, m)
		}
		s.change.Drain(func(control.Issued[Change]) { s.enter(w, s.next(w, season)) })
		s.set.Drain(func(i control.Issued[Set]) {
			if at := s.cfg.index(i.Command.Name); at >= 0 {
				s.enter(w, at)
			}
		})
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
			t := s.world.Tick(cb, d)
			s.told = Weathering{Weather: now, Season: season, World: t.World}
			s.host.Run(t, cursor, s.about)
		}
		return
	}
}

// likely is how likely st comes in season in this climate: its own likelihood, and the zone's wet
// for the season when it brings rain or snow.
func (s *weatherSystem) likely(st weather.State, season calendar.Season) float32 {
	if st.Falls > 0 {
		return st.Likely(season) * s.profile.Wet[season%4]
	}
	return st.Likely(season)
}

// warmth is the temperature st brings at m: the year's warmth by how far through the year it is,
// the day's by the hour, and the state's own.
func (s *weatherSystem) warmth(st weather.State, m calendar.Moment) float32 {
	year := math.Sin(2 * math.Pi * (float64(m.OfYear()) - 0.125)) // highest at midsummer
	hour := math.Sin(2 * math.Pi * (float64(m.Time) - 0.375))     // highest mid-afternoon
	p := &s.profile
	return p.Mean + p.Year*float32(year) + p.Day*float32(hour) + st.Warmth
}

// falling is how much rain and how much snow fall of falls at temperature t.
func falling(falls, t float32) (rain, snow float32) {
	if t < snowsBelow {
		return 0, falls
	}
	return falls, 0
}

// enter has w begin state at: how long it lasts, the wind it blows up to, how much of the sky its
// clouds cover and how heaped they grow, thrown now.
func (s *weatherSystem) enter(w *Weather, at int32) {
	st := s.cfg.Weathers[at]
	w.State = at
	w.Left = float32(st.Lasts[0].Seconds()) + roll(&w.Dice)*float32((st.Lasts[1]-st.Lasts[0]).Seconds())
	w.Target = st.Wind[0] + roll(&w.Dice)*(st.Wind[1]-st.Wind[0])
	w.CloudsTo = st.Clouds[0] + roll(&w.Dice)*(st.Clouds[1]-st.Clouds[0])
	w.BillowTo = st.Billow[0] + roll(&w.Dice)*(st.Billow[1]-st.Billow[0])
}

// next throws the state to follow w's by its weights and how often each comes in season; w's own
// when none may.
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

// throw throws a weather of the climate, each as likely as weight says times how likely it comes in
// season — one that brings rain or snow the more the wetter the zone has the season; -1 when none
// may come.
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

// settle brings w over dt seconds towards its state's at m: the wind, the clouds and their heaps,
// the temperature, what falls — snow below snowsBelow, rain above — the wind's way wandering, the
// clouds carried on.
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

// wind is the wind w blows, world units a second along x and y.
func (w *Weather) wind() [2]float32 {
	s, c := math.Sincos(float64(w.Heading))
	return [2]float32{float32(c) * w.Blow, float32(s) * w.Blow}
}

// air is w as the air over a world of scale, with what r stops left out, seen as far through as the
// weather lets.
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

// roll throws the dice: a number from 0 up to 1, the dice moved on (xorshift).
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
