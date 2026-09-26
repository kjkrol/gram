package climate

import (
	"math"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugin/host"
	"github.com/kjkrol/gram/plugins/climate/weather"
	"github.com/kjkrol/gram/plugins/sky"
	"github.com/kjkrol/gram/plugins/world"
)

var _ goke.System = (*weatherSystem)(nil)

// weatherSystem moves the weather on every tick, in the sky's time: counts its state down and
// throws the next when it runs out, or when Change or Set asks; brings the wind, the clouds, what
// falls and the temperature towards the state's, the wind's way wandering; carries the clouds on
// the wind; sets the world's weather; and runs the behaviours hosted with it.
type weatherSystem struct {
	cfg    Config
	world  *world.Plugin
	change *control.Queue[Change]
	set    *control.Queue[Set]

	query   *goke.Query
	now     goke.Comp[Weather]
	spawn   goke.Comp[Weather]
	days    *goke.Query
	day     goke.Comp[sky.Day]
	host    *host.EachHost[Weathering]
	profile Profile                // the zone's climate in numbers
	about   func(i int) Weathering // what a behaviour hears, bound once so a tick allocates nothing
	told    Weathering
}

func newWeatherSystem(cfg Config, w *world.Plugin, change *control.Queue[Change], set *control.Queue[Set], behaviours *host.EachHost[Weathering]) *weatherSystem {
	s := &weatherSystem{cfg: cfg, world: w, change: change, set: set, host: behaviours, profile: cfg.Zone.Profile()}
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
	s.days = si.NewQueryBuilder(&s.day).Build()
	for s.query.All(); s.query.Next(); {
		return // a loaded game brought its weather
	}
	f := si.NewFactory(&s.spawn)
	f.Create(1)
	for f.Next() {
		// the state is thrown on the first tick, when the sky has its day and so its season
		s.spawn.Slice(&f.Cursor)[0] = Weather{State: unbegun, Dice: s.cfg.Seed}
	}
}

// unbegun is the state of a weather not yet begun.
const unbegun = -1

// begin has w begin in cfg's Start or, without one, in a state thrown as season has them, already
// at its wind, clouds, temperature and fall rather than coming round to them.
func (s *weatherSystem) begin(w *Weather, day sky.Day) {
	start := s.cfg.index(s.cfg.Start)
	if start < 0 {
		start = s.throw(w, day.Season(), func(int) float32 { return 1 })
	}
	s.enter(w, start)
	st := s.cfg.Weathers[start]
	w.Blow, w.Clouds, w.Temperature = w.Target, st.Clouds, s.warmth(st, day)
	w.Rain, w.Snow = falling(st.Falls, w.Temperature)
}

func (s *weatherSystem) Update(cb *goke.CmdBuf, d time.Duration) {
	day := s.today()
	season := day.Season()
	for s.query.All(); s.query.Next(); {
		cursor := s.query.Cursor()
		w := &s.now.Slice(cursor)[0]
		if w.State == unbegun {
			s.begin(w, day)
			w.SeenDate, w.SeenTime = day.Date, day.Time
		}
		d = s.passed(w, day, d)
		s.change.Drain(func(control.Issued[Change]) { s.enter(w, s.next(w, season)) })
		s.set.Drain(func(i control.Issued[Set]) {
			if at := s.cfg.index(i.Command.Name); at >= 0 {
				s.enter(w, at)
			}
		})
		dt := float32(d.Seconds())
		if w.Left -= dt; w.Left <= 0 {
			s.enter(w, s.next(w, season))
		}
		s.settle(w, dt, day)
		air := w.air()
		s.world.SetWeather(air)
		if !s.host.Empty() {
			s.told = Weathering{Weather: air, Season: season}
			s.host.Run(plugin.Tick{CmdBuf: cb, Now: time.Now(), Dt: d}, cursor, s.about)
		}
		return
	}
}

// today is the sky's day, a summer's afternoon without a sky.
func (s *weatherSystem) today() sky.Day {
	for s.days.All(); s.days.Next(); {
		return s.day.Slice(s.days.Cursor())[0]
	}
	return summer
}

// summer is the day of a world without a sky: the middle of summer, mid-afternoon, standing.
var summer = sky.Day{Date: 3, Time: 0.6}

// passed is how much of the sky's time has gone by since w last looked at the day — a tick at the
// day's pace, a jump the day was moved by, nothing while it stands or goes back — and has w look
// at it now. The weather goes by in the sky's time; without a sky, in the ticks' d.
func (s *weatherSystem) passed(w *Weather, day sky.Day, d time.Duration) time.Duration {
	if day.Length <= 0 {
		return d // no sky, or one not yet begun
	}
	days := float64(day.Date-w.SeenDate) + float64(day.Time-w.SeenTime)
	if year := float64(day.Calendar.Days()); days < -year/2 {
		days += year // the year came round
	}
	w.SeenDate, w.SeenTime = day.Date, day.Time
	if days <= 0 {
		return 0
	}
	return time.Duration(days * float64(day.Length))
}

// likely is how likely st comes in season in this climate: its own likelihood, and the zone's wet
// for the season when it brings rain or snow.
func (s *weatherSystem) likely(st weather.State, season sky.Season) float32 {
	if st.Falls > 0 {
		return st.Likely(season) * s.profile.Wet[season%4]
	}
	return st.Likely(season)
}

// warmth is the temperature st brings on day: the year's warmth by how far through the year it
// is, the day's by the hour, and the state's own.
func (s *weatherSystem) warmth(st weather.State, day sky.Day) float32 {
	year := math.Sin(2 * math.Pi * (float64(day.OfYear()) - 0.125)) // highest at midsummer
	hour := math.Sin(2 * math.Pi * (float64(day.Time) - 0.375))     // highest mid-afternoon
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

// enter has w begin state at: how long it lasts and the wind it blows up to, thrown now.
func (s *weatherSystem) enter(w *Weather, at int32) {
	st := s.cfg.Weathers[at]
	w.State = at
	w.Left = float32(st.Lasts[0].Seconds()) + roll(&w.Dice)*float32((st.Lasts[1]-st.Lasts[0]).Seconds())
	w.Target = st.Wind[0] + roll(&w.Dice)*(st.Wind[1]-st.Wind[0])
}

// next throws the state to follow w's by its weights and how often each comes in season; w's own
// when none may.
func (s *weatherSystem) next(w *Weather, season sky.Season) int32 {
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
func (s *weatherSystem) throw(w *Weather, season sky.Season, weight func(i int) float32) int32 {
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

// settle brings w over dt seconds towards its state's on day: the wind, the clouds, the
// temperature, what falls — snow below snowsBelow, rain above — the wind's way wandering, the
// clouds carried on.
func (s *weatherSystem) settle(w *Weather, dt float32, day sky.Day) {
	st := s.cfg.Weathers[w.State]
	k := 1 - float32(math.Exp(-float64(dt)/s.cfg.Blend.Seconds()))
	w.Temperature += (s.warmth(st, day) - w.Temperature) * k
	rain, snow := falling(st.Falls, w.Temperature)
	w.Blow += (w.Target - w.Blow) * k
	w.Clouds += (st.Clouds - w.Clouds) * k
	w.Rain += (rain - w.Rain) * k
	w.Snow += (snow - w.Snow) * k
	w.Heading += (roll(&w.Dice)*2 - 1) * wander * dt
	wind := w.wind()
	w.Drift[0] += wind[0] * dt
	w.Drift[1] += wind[1] * dt
}

// wind is the wind w blows, world units a second along x and y.
func (w *Weather) wind() [2]float32 {
	s, c := math.Sincos(float64(w.Heading))
	return [2]float32{float32(c) * w.Blow, float32(s) * w.Blow}
}

// air is w as the world's weather.
func (w *Weather) air() world.Weather {
	return world.Weather{Wind: w.wind(), Clouds: w.Clouds, Rain: w.Rain, Snow: w.Snow, Temperature: w.Temperature, Drift: w.Drift}
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
