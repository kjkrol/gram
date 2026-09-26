package sky

import (
	"math"
	"testing"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugins/world"
)

func near(a, b float32) bool { return math.Abs(float64(a-b)) < 1e-3 }

// defaults is the day of a zero Config, under a sky at the Latitude it has without a climate.
var defaults = Config{latitude: Latitude}

// firstDay is the sun of a zero Config's first day — the middle of spring, the year's second — at
// time t, as the sky sets it.
func firstDay(t float32) world.Sun { return defaults.SunAt((1+t)/8, t) }

func TestSunAt_RisesInTheEastStandsOverTheSouthAndSetsInTheWest(t *testing.T) {
	south := Config{NoonWay: South, latitude: Latitude}
	if s := south.SunAt(0, 0.25); s.Dir[0] < 0.99 || !near(s.Dir[2], 0) || s.Strength > 1e-6 {
		t.Errorf("at 6 the sun is %+v, want it on the eastern horizon, no strength yet", s)
	}
	if s := south.SunAt(0, 0.5); !near(s.Dir[1], 0.5) || !near(s.Dir[2], float32(math.Sin(math.Pi/3))) || s.Strength != sunStrength || s.Sky != daylight[3].sky || s.Color != daylight[3].sun {
		t.Errorf("at noon the sun is %+v, want it over the south 60° up at full strength", s)
	}
	if s := south.SunAt(0, 0.75); s.Dir[0] > -0.99 {
		t.Errorf("at 18 the sun is %+v, want it on the western horizon", s)
	}
	if s := south.SunAt(0, 0); s.Dir[2] >= 0 || s.Strength != 0 || s.Sky != daylight[0].sky || s.Ambient != daylight[0].ambient {
		t.Errorf("at midnight the sun is %+v, want it below the horizon, the night's ambient alone", s)
	}
	if morning, noonSun := south.SunAt(0, 0.3), south.SunAt(0, 0.5); morning.Dir[2] >= noonSun.Dir[2] {
		t.Error("the morning sun stands as high as the noon sun")
	}
}

func TestSunAt_StandsOverNoonWayAtNoonAndTurnsItsWholePathWithIt(t *testing.T) {
	northWest := defaults // the north-west by default
	s := northWest.SunAt(0, 0.5)
	if !near(s.Dir[0], s.Dir[1]) || s.Dir[0] >= 0 || !near(s.Dir[2], float32(math.Sin(math.Pi/3))) {
		t.Errorf("at noon the sun is %+v, want it over the north-west 60° up", s)
	}
	// a quarter turn round from noon at 6: south-west, as the east is from the south
	if s := northWest.SunAt(0, 0.25); !near(s.Dir[0], -s.Dir[1]) || s.Dir[1] <= 0 || !near(s.Dir[2], 0) {
		t.Errorf("at 6 the sun is %+v, want it on the horizon in the south-west", s)
	}
	if s, south := northWest.SunAt(0, 0.4), (Config{NoonWay: South, latitude: Latitude}).SunAt(0, 0.4); s.Strength != south.Strength || s.Ambient != south.Ambient {
		t.Errorf("turned round the sun lights %v/%v, want the same light as over the south %v/%v", s.Strength, s.Ambient, south.Strength, south.Ambient)
	}
}

// queues are the sky's commands as a test issues them.
type queues struct {
	forward control.Queue[Forward]
	back    control.Queue[Back]
	pause   control.Queue[Pause]
}

// skyOf runs a sky system over a new world, returning a tick and a look at its Day.
func skyOf(t *testing.T, cfg Config) (w *world.Plugin, tick func(time.Duration), day func() Day, q *queues) {
	t.Helper()
	w = world.NewPlugin(world.Config{Space: world.SpaceCfg{Width: 64, Height: 64}, Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 8}, Quasi3D: true})
	q = &queues{}
	cfg = cfg.withDefaults()
	cfg.latitude = Latitude // as NewPlugin has it without a climate
	sys := newSkySystem(cfg, w, &q.forward, &q.back, &q.pause)
	ecs := goke.New()
	ecs.Setup(goke.SystemFn{OnInit: sys.Init})
	tick = func(d time.Duration) { sys.Update(nil, d) }
	day = func() Day {
		for sys.query.All(); sys.query.Next(); {
			return sys.day.Slice(sys.query.Cursor())[0]
		}
		t.Fatal("no sky entity")
		return Day{}
	}
	return
}

func TestSky_TheDayGoesByAndTheSunFollowsItInSteps(t *testing.T) {
	w, tick, day, _ := skyOf(t, Config{Length: 24 * time.Second, Start: 0.5, Steps: 24})
	if d := day(); d.Time != 0.5 || d.Pace != 1 {
		t.Fatalf("a fresh sky is at %+v, want noon at pace 1", d)
	}
	tick(time.Second / 60)
	if w.Sun() != firstDay(0.5) {
		t.Errorf("the world's sun %+v, want noon's", w.Sun())
	}
	tick(time.Second / 2) // half an hour of a 24-second day: still the noon step
	if w.Sun() != firstDay(0.5) {
		t.Error("the sun moved within a step")
	}
	tick(time.Second) // past one: the next step
	if d := day(); !near(d.Time, 0.5+1.5/24+1.0/60/24) || w.Sun() != firstDay(13.0/24) {
		t.Errorf("an hour and a half on the day is at %v and the sun %+v, want the step at 13", d.Time, w.Sun())
	}
}

func TestSky_ForwardAndBackChangeThePaceOfADayGoingBy(t *testing.T) {
	_, tick, day, q := skyOf(t, Config{})
	q.forward.Add(0, Forward{})
	q.forward.Add(0, Forward{})
	tick(0)
	if p := day().Pace; p != 4 {
		t.Errorf("pace %v after two Forwards, want 4", p)
	}
	q.back.Add(0, Back{})
	tick(0)
	if p := day().Pace; p != 2 {
		t.Errorf("pace %v after a Back, want 2", p)
	}
}

// Stopped, the day stands still, Forward and Back move it by half an hour and the sun follows at
// once; let go again, it goes by at the pace it had.
func TestSky_AStoppedDayMovesByHalfAnHourAndGoesOnAtItsPace(t *testing.T) {
	w, tick, day, q := skyOf(t, Config{Length: 24 * time.Second, Start: 0.5, Steps: 24})
	q.forward.Add(0, Forward{}) // pace 2
	tick(0)
	q.pause.Add(0, Pause{})
	tick(time.Second)
	if d := day(); !d.Stopped || d.Time != 0.5 {
		t.Fatalf("after Pause the day is %+v, want it stopped at noon", d)
	}
	tick(time.Hour)
	if d := day(); d.Time != 0.5 {
		t.Errorf("a stopped day went on to %v", d.Time)
	}
	q.forward.Add(0, Forward{})
	q.forward.Add(0, Forward{})
	q.back.Add(0, Back{})
	tick(0)
	if d := day(); !near(d.Time, 12.5/24) || d.Pace != 2 {
		t.Errorf("two half hours on and one back the day is %+v, want 12:30 at pace 2", d)
	}
	q.forward.Add(0, Forward{})
	tick(0)
	if w.Sun() != firstDay(13.0/24) { // the sun moves by the day's steps, hours here
		t.Errorf("at 13:00 the sun is %+v, want the step's own at once", w.Sun())
	}
	q.back.Add(0, Back{})
	tick(0)
	q.back.Add(0, Back{})
	q.back.Add(0, Back{})
	tick(0)
	if d := day(); !near(d.Time, 11.5/24) {
		t.Errorf("two half hours back the day is at %v, want 11:30", d.Time)
	}
	q.pause.Add(0, Pause{})
	tick(time.Second)
	if d := day(); d.Stopped || !near(d.Time, 11.5/24+2.0/24) {
		t.Errorf("let go, a second of a 24-second day at pace 2 brings %+v, want 13:30 and going", d)
	}
}

func TestSky_BackBeforeMidnightIsTheDayBefore(t *testing.T) {
	_, tick, day, q := skyOf(t, Config{Start: 0.25 / 24})
	q.pause.Add(0, Pause{})
	q.back.Add(0, Back{})
	tick(0)
	if d := day(); !near(d.Time, 23.75/24) {
		t.Errorf("half an hour back from 00:15 the day is at %v, want 23:45", d.Time)
	}
}

func TestSky_ALoadedSkyKeepsItsTime(t *testing.T) {
	w := world.NewPlugin(world.Config{Space: world.SpaceCfg{Width: 64, Height: 64}, Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 8}, Quasi3D: true})
	q := &queues{}
	sys := newSkySystem(Config{}.withDefaults(), w, &q.forward, &q.back, &q.pause)
	var saved goke.Comp[Day]
	ecs := goke.New()
	ecs.Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		f := si.NewFactory(&saved) // what a load brings back
		f.Create(1)
		for f.Next() {
			saved.Slice(&f.Cursor)[0] = Day{Time: 0.9, Pace: 3}
		}
		sys.Init(si)
	}})
	n := 0
	for sys.query.All(); sys.query.Next(); {
		n += len(sys.query.Cursor().IDs)
		if d := sys.day.Slice(sys.query.Cursor())[0]; d.Time != 0.9 || d.Pace != 3 {
			t.Errorf("the sky is at %+v, want the saved 0.9 at pace 3", d)
		}
	}
	if n != 1 {
		t.Errorf("%d sky entities, want the loaded one alone", n)
	}
}

func TestClock_ReadsTheTimeOfDayAndThePaceWhenItIsNotOne(t *testing.T) {
	for _, c := range []struct {
		day  Day
		want string
	}{
		{Day{Time: 0, Pace: 1}, "00:00"},
		{Day{Time: 14.0/24 + 5.0/24/60, Pace: 1}, "14:05"},
		{Day{Time: 0.75, Pace: 4}, "18:00 (x4)"},
		{Day{Time: 0.5, Pace: 0.5}, "12:00 (x0.5)"},
		{Day{Time: 0.5, Pace: 4, Stopped: true}, "12:00 (stopped)"},
	} {
		if got := hourOf(c.day); got != c.want {
			t.Errorf("hourOf(%+v) = %q, want %q", c.day, got, c.want)
		}
	}
}

func TestSunAt_ColoursTheSkyAndItsLightThroughTheDay(t *testing.T) {
	noon, dawn, night := defaults.SunAt(0, 0.5), defaults.SunAt(0, 0.25), defaults.SunAt(0, 0)
	if noon.Sky[2] <= noon.Sky[0] || noon.Color[0] < 0.95 || noon.Color[2] < 0.9 {
		t.Errorf("at noon the sky is %v and the sun %v, want a blue sky and a nearly white sun", noon.Sky, noon.Color)
	}
	if dawn.Sky[0] <= dawn.Sky[2] || dawn.Color[2] >= dawn.Color[0]/2 {
		t.Errorf("at 6 the sky is %v and the sun %v, want both warm, the sun orange", dawn.Sky, dawn.Color)
	}
	if light := night.Light(0, 0, 1); light[2] <= light[0] || light[2] > 0.2 {
		t.Errorf("at midnight the ground is lit %v, want a dim blue", light)
	}
	if sky, _, _ := daylightAt(0.2); sky[0] >= daylight[2].sky[0] || sky[0] <= daylight[3].sky[0] {
		t.Errorf("between sunrise and day the sky is %v, want it blended between theirs", sky)
	}
}

func TestSky_TheYearGoesByDayByDayAndTheSeasonsWithIt(t *testing.T) {
	d := Day{Time: 0.9}
	d.passing(0.2)
	if d.Date != 1 || !near(d.Time, 0.1) {
		t.Errorf("past midnight the day is %+v, want the second of the year at 0.1", d)
	}
	d.passing(-0.3)
	if d.Date != 0 || !near(d.Time, 0.8) {
		t.Errorf("held back past midnight the day is %+v, want the first again at 0.8", d)
	}
	d = Day{Date: 7, Time: 0.9}
	if d.passing(0.2); d.Date != 0 {
		t.Errorf("past the last night of the year the date is %d, want 0: the year comes round", d.Date)
	}
	for date, want := range map[int32]Season{0: Spring, 2: Summer, 4: Autumn, 7: Winter} {
		if got := (Day{Date: date}).Season(); got != want {
			t.Errorf("day %d of 8 falls in %v, want %v", date, got, want)
		}
	}
}

func TestSunAt_StandsHigherAndLongerInSummerThanInWinter(t *testing.T) {
	summer, winter := defaults.SunAt(0.25, 0.5), defaults.SunAt(0.75, 0.5)
	if summer.Dir[2] <= winter.Dir[2] {
		t.Errorf("at noon the sun stands %v high in summer and %v in winter, want it higher in summer", summer.Dir[2], winter.Dir[2])
	}
	if early, late := defaults.SunAt(0.25, 0.25), defaults.SunAt(0.75, 0.25); early.Dir[2] <= 0 || late.Dir[2] >= 0 {
		t.Errorf("at 6 the sun stands %v in summer, %v in winter; want it up in summer, still down in winter", early.Dir[2], late.Dir[2])
	}
}

func TestSky_BeginsInTheMiddleOfItsSeason(t *testing.T) {
	for season, want := range map[Season]int32{Spring: 1, Summer: 3, Autumn: 5, Winter: 7} {
		_, _, day, _ := skyOf(t, Config{Season: season})
		if d := day(); d.Date != want || d.Season() != season {
			t.Errorf("a sky of %v begins on day %d (%v), want day %d", season, d.Date, d.Season(), want)
		}
	}
}

func TestLightAt_TheMoonLightsTheNightAsFullAsItIs(t *testing.T) {
	c := defaults
	if day := c.LightAt(0, 0.5, 0.5); day != c.SunAt(0, 0.5) {
		t.Errorf("by day the light is %+v, want the sun", day)
	}
	full, new := c.LightAt(0, 0, 0.5), c.LightAt(0, 0, 0)
	if full.Strength <= 0.1 || full.Color != moonColor || full.Dir[2] <= 0 {
		t.Errorf("at midnight under a full moon the light is %+v, want the moon high, bright and pale", full)
	}
	if new.Strength != 0 {
		t.Errorf("at midnight under a new moon the light has strength %v, want none", new.Strength)
	}
	if full.Ambient != c.SunAt(0, 0).Ambient || full.Sky != c.SunAt(0, 0).Sky {
		t.Error("under the moon the night sky and its light changed, want the night's")
	}
}

func TestCalendar_AnEarthYearHasMonthsAndALongerMoon(t *testing.T) {
	earth := Day{Calendar: EarthYear}
	if got := earth.Written(); got != "20 March" {
		t.Errorf("the first of spring of an EarthYear is written %q, want 20 March", got)
	}
	if got := (Day{Calendar: EarthYear, Date: 365 - 78}).Written(); got != "1 January" {
		t.Errorf("an EarthYear's day %d is written %q, want 1 January", 365-78, got)
	}
	if got := (Day{Date: 2}).Written(); got != "day 3 of 8" {
		t.Errorf("a GameYear's third day is written %q", got)
	}
	if m := (Day{Calendar: EarthYear, Date: 14, Time: 0.765}).Moon(); !near(m, 0.5) {
		t.Errorf("fourteen and three quarter days into an EarthYear the moon is %v round, want full", m)
	}
	if m := (Day{Date: 2}).Moon(); m != 0.5 {
		t.Errorf("two days into a GameYear the moon is %v round, want full", m)
	}
	if got := moonOf(0.5); got != "full moon" {
		t.Errorf("the moon half round is %q", got)
	}
}

func TestPath_StandsAsHighAsTheLatitudeLetsAndLongerInSummer(t *testing.T) {
	temperate := Config{NoonWay: South, latitude: 55}
	if up := temperate.path(0, 0.5)[2]; !near(up, float32(math.Sin(35*math.Pi/180))) {
		t.Errorf("at 55° at noon at the equinox the sun stands %v up, want sin 35°", up)
	}
	if summer, winter := temperate.path(0.25, 0.5)[2], temperate.path(0.75, 0.5)[2]; !near(summer, float32(math.Sin((35+23.44)*math.Pi/180))) || winter >= summer {
		t.Errorf("at 55° the midsummer noon sun stands %v up and the midwinter %v, want 23.44° higher in summer", summer, winter)
	}
	// at 6 in the morning: up in summer, still down in winter, just rising at the equinox
	if summer, winter, equinox := temperate.path(0.25, 0.25)[2], temperate.path(0.75, 0.25)[2], temperate.path(0, 0.25)[2]; summer <= 0 || winter >= 0 || !near(equinox, 0) {
		t.Errorf("at 55° at 6 the sun stands %v in summer, %v in winter, %v at the equinox; want up, down and rising", summer, winter, equinox)
	}
}

func TestPath_PastThePolarCircleTheSunStaysUpInSummerAndDownInWinter(t *testing.T) {
	polar := Config{latitude: 78}
	for _, hour := range []float32{0, 0.25, 0.5, 0.75} {
		if up := polar.path(0.25, hour)[2]; up <= 0 {
			t.Errorf("at 78° at midsummer at %v of the day the sun stands %v, want above the horizon all day", hour, up)
		}
		if up := polar.path(0.75, hour)[2]; up >= 0 {
			t.Errorf("at 78° at midwinter at %v of the day the sun stands %v, want below the horizon all day", hour, up)
		}
	}
}
