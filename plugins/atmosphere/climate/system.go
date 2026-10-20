package climate

import (
	"math"
	"slices"
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
	nameNow  *string // the weather of the time by name, the Climate's

	query   *goke.Query
	now     goke.Comp[Weather]
	spawn   goke.Comp[Weather]
	patches *goke.Query
	patch   goke.Comp[air.Patch]
	newly   goke.Comp[air.Patch]
	host    *host.StepRules[Weathering]
	about   func() uid.UID64 // nil for the world's
	profile Profile
	current air.Weather

	dome   *air.Dome
	middle [2]float32
	lanes  int     // on each side of lane 0
	width  float32 // a lane's, world units
	kept   bool    // the dome's patches came from a save
	laid   bool
	stale  []uid.UID64 // patches of a save the dome has no place for
	part   int         // of the dome's patches, settling next
	setNow bool

	ground  func() (low, peaks float64) // nil over no ground in particular
	land    *air.Land                   // nil without
	places  *places
	spread  Spread
	spreads *goke.Query
	knob    goke.Comp[Spread]
	// the Default weathers', -1 for none
	named struct{ fair, cloudy, rain, storm int32 }
	// seconds since the land last worked on the air, and steps
	lifted float32
	lifts  int
	// as air.ReliefOf, from the first step on
	relief   [2]float32
	bases    [2]float32
	ceilings [3]float32 // coming to the weather's over the world's middle
	ceiled   bool
}

func newWeatherSystem(cfg Config, w *world.Plugin, cal *calendar.Calendar, asked *[]string,
	rules *host.StepRules[Weathering], running *Running, over *string) *weatherSystem {
	return &weatherSystem{cfg: cfg, world: w, calendar: cal, asked: asked, running: running,
		host: rules, profile: cfg.Zone.Profile(), nameNow: over}
}

// what falls from the weather of the state st
func (s *weatherSystem) falls(st int16) float32 {
	if st < 0 || int(st) >= len(s.cfg.Weathers) {
		return 0
	}
	return s.cfg.Weathers[st].Falls
}

// the ground the low clouds hang over and how high each layer hangs, worked out once
func (s *weatherSystem) hang() {
	if s.relief[1] > 0 {
		return
	}
	var low, peaks float64
	if s.ground != nil {
		low, peaks = s.ground()
	}
	scale := s.world.Scale()
	s.relief = air.ReliefOf(scale, low, peaks)
	s.bases = air.BasesOver(scale, s.relief)
}

// the shares of the low clouds' bands go towards the weather's over the world's middle, as fast
// as the air settles into it
func (s *weatherSystem) ceil(w *Weather, dt float32) {
	to := s.cfg.Weathers[w.Over].Ceilings
	if sum := to[0] + to[1] + to[2]; sum > 0 {
		to = [3]float32{to[0] / sum, to[1] / sum, to[2] / sum}
	} else {
		to = [3]float32{1.0 / 3, 1.0 / 3, 1.0 / 3}
	}
	if !s.ceiled {
		s.ceilings, s.ceiled = to, true
		return
	}
	k := s.share(dt)
	if w.Forced > 0 {
		k = 1 - float32(math.Exp(-float64(dt)/setting))
	}
	for i := range s.ceilings {
		s.ceilings[i] += (to[i] - s.ceilings[i]) * k
	}
}

const (
	wander      = 0.05 // radians a second at most
	settleParts = 15   // steps the dome takes to settle all its patches once
	setParts    = 4    // as many while a weather set now settles, so its patches go together
	setting     = 2.0  // seconds the air over the world takes to settle into a weather set now
	forcing     = 5    // settings a weather set now settles in for
	neighbourly = 2    // how much likelier a lane takes a neighbour's weather
	liftParts   = 4    // steps the land takes to work on the air once, as long as they took
)

func (s *weatherSystem) Init(si *goke.SysInit) {
	s.host.Bind()
	space := s.world.Res.Config.Space
	width, height := float32(space.Width), float32(space.Height)
	s.middle = [2]float32{width / 2, height / 2}
	scale := s.world.Scale()
	s.dome = air.NewDome(width, height, float32(scale.Units(s.cfg.Horizon)))
	s.lanesOf(scale)
	s.query = si.NewQueryBuilder(&s.now).Build()
	s.patches = si.NewQueryBuilder(&s.patch).Build()
	s.spreads = si.NewQueryBuilder(&s.knob).Build()
	s.places = newPlaces(width, height)
	s.named.fair, s.named.cloudy = s.cfg.index(weather.Fair), s.cfg.index(weather.Cloudy)
	s.named.rain, s.named.storm = s.cfg.index(weather.Rain), s.cfg.index(weather.Storm)
	kept := false
	for s.query.All(); s.query.Next(); {
		kept = true
	}
	if !kept {
		f := si.NewFactory(&s.spawn)
		f.Create(1)
		for f.Next() {
			// thrown in the first step, in the season the calendar has then
			w := Weather{Dice: s.cfg.Seed}
			w.Lanes[0].State = unbegun
			s.spawn.Slice(&f.Cursor)[0] = w
		}
	}
	s.keep(si, kept)
}

// the patches a save left held by the dome, those missing made
func (s *weatherSystem) keep(si *goke.SysInit, kept bool) {
	held := map[[2]int32]bool{}
	for s.patches.All(); s.patches.Next(); {
		c := s.patches.Cursor()
		ps := s.patch.Slice(c)
		for i := range ps {
			at := [2]int32{ps[i].Level, ps[i].Slot}
			if held[at] || !s.dome.Hold(&ps[i]) {
				s.stale = append(s.stale, c.IDs[i])
				continue
			}
			held[at] = true
		}
	}
	var missing [][2]int32
	for _, at := range s.dome.Slots() {
		if !held[at] {
			missing = append(missing, at)
		}
	}
	s.kept = kept && len(missing) == 0
	if len(missing) == 0 {
		return
	}
	f := si.NewFactory(&s.newly)
	f.Create(len(missing))
	n := 0
	for f.Next() {
		ps := s.newly.Slice(&f.Cursor)
		for i := range ps {
			ps[i] = air.Patch{Level: missing[n][0], Slot: missing[n][1]}
			n++
		}
	}
}

// how wide the lanes of a Patchy climate are and how many there are
func (s *weatherSystem) lanesOf(scale world.Scale) {
	if !s.cfg.Patchy {
		return
	}
	r := s.dome.Reach()
	half := float32(math.Hypot(float64(max(s.middle[0]-r[0], r[2]-s.middle[0])),
		float64(max(s.middle[1]-r[1], r[3]-s.middle[1]))))
	s.width = max(float32(scale.Units(air.ClearAir/2)), 2*half/(MaxLanes-1))
	s.lanes = min(int(math.Ceil(float64(half/s.width-0.5))), (MaxLanes-1)/2)
}

const unbegun = -1

func (s *weatherSystem) begin(w *Weather, m calendar.Moment) {
	start := s.cfg.index(s.cfg.Start)
	s.each(func(k int) {
		at := start
		if at < 0 {
			at = s.throw(w, m.Season(), func(int) float32 { return 1 })
		}
		s.enter(w, k, at)
		l := &w.Lanes[lane(k)]
		l.Clouds, l.Billow, l.Falls = l.CloudsTo, l.BillowTo, s.cfg.Weathers[at].Falls
	})
	w.Blow, w.Over = w.Target, w.Lanes[0].State
	w.Temperature = s.warmth(s.cfg.Weathers[w.Over], m)
}

// each lane, lane 0 first
func (s *weatherSystem) each(f func(k int)) {
	f(0)
	for k := 1; k <= s.lanes; k++ {
		f(-k)
		f(k)
	}
}

func lane(k int) int { return (k%MaxLanes + MaxLanes) % MaxLanes }

func (s *weatherSystem) Update(cb *goke.CmdBuf, d time.Duration) {
	s.hold()
	s.spread = DefaultSpread()
	for s.spreads.All(); s.spreads.Next(); {
		s.spread = s.knob.Slice(s.spreads.Cursor())[0]
	}
	for _, id := range s.stale {
		cb.RemoveOne(id)
	}
	s.stale = nil
	m := s.calendar.Now()
	season := m.Season()
	for s.query.All(); s.query.Next(); {
		cursor := s.query.Cursor()
		w := &s.now.Slice(cursor)[0]
		if w.Lanes[0].State == unbegun {
			s.begin(w, m)
		}
		for _, name := range *s.asked {
			at := s.cfg.index(name)
			if name == "" {
				at = s.next(w, 0, season)
			}
			if at >= 0 {
				s.enter(w, 0, at)
				l := &w.Lanes[0]
				l.Clouds, l.Billow, l.Falls = l.CloudsTo, l.BillowTo, s.cfg.Weathers[at].Falls
				w.Forced = forcing * setting
				s.setNow = true
			}
		}
		*s.asked = (*s.asked)[:0]
		dt := float32(d.Seconds())
		if s.running.Changes {
			s.each(func(k int) {
				if l := &w.Lanes[lane(k)]; l.Left-dt <= 0 {
					s.enter(w, k, s.next(w, k, season))
				} else {
					l.Left -= dt
				}
			})
		}
		*s.nameNow = s.cfg.Weathers[w.Lanes[0].State].Name
		s.blend(w, dt)
		s.carry(w, dt)
		over := s.over(w, dt, m)
		s.hang()
		s.ceil(w, dt)
		now := w.air(s.world.Scale(), *s.running, s.dome, over)
		now.Bases, now.Relief, now.Ceilings = s.bases, s.relief, s.ceilings
		now.OneLayer = s.cfg.OneLayer
		s.lifted, s.lifts = s.lifted+dt, (s.lifts+1)%liftParts
		if s.land != nil && now.Dome != nil && s.lifts == 0 {
			s.dome.Lift(now, s.land, s.falls, s.lifted)
			s.lifted = 0
		}
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

// the dome keeps its cells in the patches where the world holds them now
func (s *weatherSystem) hold() {
	for s.patches.All(); s.patches.Next(); {
		c := s.patches.Cursor()
		ps := s.patch.Slice(c)
		for i := range ps {
			if len(s.stale) == 0 || !slices.Contains(s.stale, c.IDs[i]) {
				s.dome.Hold(&ps[i])
			}
		}
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

func (s *weatherSystem) enter(w *Weather, k int, at int32) {
	st := s.cfg.Weathers[at]
	l := &w.Lanes[lane(k)]
	l.State = at
	l.Left = float32(st.Lasts[0].Seconds()) +
		roll(&w.Dice)*float32((st.Lasts[1]-st.Lasts[0]).Seconds())
	if k == 0 {
		w.Target = st.Wind[0] + roll(&w.Dice)*(st.Wind[1]-st.Wind[0])
	}
	l.Left = max(l.Left, s.crossing(w))
	l.CloudsTo = st.Clouds[0] + roll(&w.Dice)*(st.Clouds[1]-st.Clouds[0])
	l.BillowTo = st.Billow[0] + roll(&w.Dice)*(st.Billow[1]-st.Billow[0])
}

func (s *weatherSystem) next(w *Weather, k int, season calendar.Season) int32 {
	was := w.Lanes[lane(k)].State
	st := s.cfg.Weathers[was]
	at := s.throw(w, season, func(i int) float32 {
		if int32(i) == was {
			return 0
		}
		weight := float32(1)
		if len(st.Next) > 0 {
			weight = st.Next[s.cfg.Weathers[i].Name]
		}
		for _, n := range [2]int{k - 1, k + 1} {
			if n >= -s.lanes && n <= s.lanes && w.Lanes[lane(n)].State == int32(i) {
				weight *= neighbourly
			}
		}
		return weight
	})
	if at < 0 {
		return was
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

// how far towards a new weather the air settles in dt seconds
func (s *weatherSystem) share(dt float32) float32 {
	return 1 - float32(math.Exp(-float64(dt)/s.cfg.Blend.Seconds()))
}

func (s *weatherSystem) blend(w *Weather, dt float32) {
	k := s.share(dt)
	w.Blow += (w.Target - w.Blow) * k
	w.Heading += (roll(&w.Dice)*2 - 1) * wander * dt
	s.each(func(i int) {
		l := &w.Lanes[lane(i)]
		l.Clouds += (l.CloudsTo - l.Clouds) * k
		l.Billow += (l.BillowTo - l.Billow) * k
		l.Falls += (s.cfg.Weathers[l.State].Falls - l.Falls) * k
	})
}

// The wind carries the dome on, the horizon bringing in what the lanes have now; without wind, or
// over the world and upwind of it while a weather set now is forced, the air settles in place
// into what the lanes are coming to.
func (s *weatherSystem) carry(w *Weather, dt float32) {
	if s.running.Wind {
		wind := w.wind()
		w.Drift[0] += wind[0] * dt
		w.Drift[1] += wind[1] * dt
	}
	switch {
	case !s.laid && s.kept:
		s.dome.Resume(w.Drift)
	case !s.laid:
		s.dome.Carry(w.Drift, s.born(w, false), true)
	case s.running.Wind:
		s.dome.Carry(w.Drift, s.born(w, false), false)
	}
	s.laid = true
	w.Forced = max(w.Forced-dt, 0)
	if s.running.Wind && w.Forced <= 0 {
		return
	}
	// a part of the patches a step, all of them at once as a weather is set
	parts, k := settleParts, s.share(dt*settleParts)
	if w.Forced > 0 {
		parts = setParts
		k = 1 - float32(math.Exp(-float64(dt*setParts)/setting))
	}
	s.part = (s.part + 1) % parts
	part := s.part
	if s.setNow {
		part, parts, s.setNow = 0, 1, false
	}
	if !s.running.Wind {
		s.dome.Settle(s.born(w, true), func(float32, float32) float32 { return k }, part, parts)
		return
	}
	space := s.world.Res.Config.Space
	sin, cos := math.Sincos(float64(w.Heading))
	// the world's far side downwind, past which the old weather sails off
	reach := float32(math.Abs(cos)*float64(space.Width)+math.Abs(sin)*float64(space.Height)) / 2
	s.dome.Settle(s.born(w, true), func(x, y float32) float32 {
		along := (x-s.middle[0])*float32(cos) + (y-s.middle[1])*float32(sin)
		return k * min(max((reach+air.BoardCell-along)/air.BoardCell, 0), 1)
	}, part, parts)
}

// the mass the lanes bring in at the world point (x, y), or the one they are coming to
func (s *weatherSystem) born(w *Weather, to bool) func(x, y float32) (air.Mass, int16) {
	var parts [MaxLanes][]part
	var laid [MaxLanes]bool
	at := func(k int, x, y float32) (air.Mass, int16) {
		l := &w.Lanes[k]
		if !laid[k] {
			parts[k], laid[k] = s.partsOf(l, to), true
		}
		if len(parts[k]) == 1 {
			return parts[k][0].mass, parts[k][0].state
		}
		return spreadAt(parts[k], s.places.rank(x, y, w.Drift))
	}
	if s.lanes == 0 {
		return func(x, y float32) (air.Mass, int16) { return at(0, x, y) }
	}
	sin, cos := math.Sincos(float64(w.Heading))
	return func(x, y float32) (air.Mass, int16) {
		// across the wind, in lanes from lane 0's middle; between two lanes' middles, both
		u := ((s.middle[0]-x)*float32(sin) + (y-s.middle[1])*float32(cos)) / s.width
		u = min(max(u, -float32(s.lanes)), float32(s.lanes))
		k := int(math.Floor(float64(u)))
		t := u - float32(k)
		ma, sa := at(lane(k), x, y)
		mb, sb := at(lane(k+1), x, y)
		st := sa
		if t >= 0.5 {
			st = sb
		}
		return air.Mass{
			Cover:  ma.Cover + (mb.Cover-ma.Cover)*t,
			Billow: ma.Billow + (mb.Billow-ma.Billow)*t,
			Falls:  ma.Falls + (mb.Falls-ma.Falls)*t,
			Damp:   ma.Damp + (mb.Damp-ma.Damp)*t,
		}, st
	}
}

// the parts of the world the weather of the lane l comes in, by rank (places): a storm amid rain
// amid clouds, rain amid clouds, clouds alone, each with fair breaks, as the Spread has them; any
// other weather, or one whose climate lacks those round it, over all of it
func (s *weatherSystem) partsOf(l *Lane, to bool) []part {
	st := &s.cfg.Weathers[l.State]
	own := air.Mass{Cover: l.Clouds, Billow: l.Billow, Falls: l.Falls, Damp: st.Damp}
	if to {
		own = air.Mass{
			Cover:  l.CloudsTo,
			Billow: l.BillowTo,
			Falls:  st.Falls,
			Damp:   st.Damp,
		}
	}
	whole := []part{{mass: own, state: int16(l.State)}}
	n := s.named
	round := func(at int32, from float32) part {
		return part{from: from, mass: middling(s.cfg.Weathers[at]), state: int16(at)}
	}
	sp := s.spread
	rain := min(max(sp.Rain, 0), 1)
	storm := min(max(sp.Storm, 0), rain)
	// the cloudier the weather was thrown, the fewer breaks it leaves
	cloudier := float32(0.5)
	if lo, hi := st.Clouds[0], st.Clouds[1]; hi > lo {
		cloudier = min(max((l.CloudsTo-lo)/(hi-lo), 0), 1)
	}
	fair := min(max(sp.Fair, 0), 1-rain) * (1 - cloudier)
	var parts []part
	if n.fair >= 0 && fair > 0 {
		parts = append(parts, round(n.fair, 0))
	}
	switch l.State {
	case n.storm:
		if n.rain < 0 || n.cloudy < 0 {
			return whole
		}
		parts = append(parts, round(n.cloudy, fair), round(n.rain, 1-rain),
			part{from: 1 - storm, mass: own, state: int16(l.State)})
	case n.rain:
		if n.cloudy < 0 {
			return whole
		}
		parts = append(parts, round(n.cloudy, fair),
			part{from: 1 - rain, mass: own, state: int16(l.State)})
	case n.cloudy:
		parts = append(parts, part{from: fair, mass: own, state: int16(l.State)})
	default:
		return whole
	}
	if len(parts) > 0 {
		parts[0].from = 0
	}
	return parts
}

// the weather over the world's middle now, the temperature coming to be its
func (s *weatherSystem) over(w *Weather, dt float32, m calendar.Moment) air.Mass {
	mass, st := s.dome.At(s.middle[0], s.middle[1])
	w.Over = int32(st)
	w.Temperature += (s.warmth(s.cfg.Weathers[w.Over], m) - w.Temperature) * s.share(dt)
	w.Rain, w.Snow = air.Falling(mass.Falls, w.Temperature)
	return mass
}

// seconds the wind takes to carry the weather across the world, as fast as it comes to blow
func (s *weatherSystem) crossing(w *Weather) float32 {
	if w.Target <= 0 {
		return 0
	}
	space := s.world.Res.Config.Space
	sin, cos := math.Sincos(float64(w.Heading))
	along := math.Abs(cos)*float64(space.Width) + math.Abs(sin)*float64(space.Height)
	return float32(along) / w.Target
}

// world units a second
func (w *Weather) wind() [2]float32 {
	s, c := math.Sincos(float64(w.Heading))
	return [2]float32{float32(c) * w.Blow, float32(s) * w.Blow}
}

func (w *Weather) air(scale world.Scale, r Running, d *air.Dome, over air.Mass) air.Weather {
	a := air.Weather{Wind: w.wind(), Clouds: over.Cover, Billow: over.Billow, Rain: w.Rain,
		Snow: w.Snow, Temperature: w.Temperature, Drift: w.Drift, Sight: air.Sight(scale),
		Dome: d, Dry: !r.Falls}
	if !r.Wind {
		a.Wind = [2]float32{}
	}
	if !r.Clouds {
		a.Clouds, a.Dome = 0, nil
	}
	if !r.Falls {
		a.Rain, a.Snow = 0, 0
	}
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
