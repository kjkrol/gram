package sky

import (
	"math"
	"testing"
	"time"

	"github.com/kjkrol/gram/clock"
	"github.com/kjkrol/gram/plugins/atmosphere/calendar"
	"github.com/kjkrol/gram/plugins/atmosphere/celestial"
	"github.com/kjkrol/gram/render"
)

func near(a, b float32) bool { return math.Abs(float64(a-b)) < 1e-3 }

var defaults = Config{latitude: Latitude}

// the year's second day, the middle of spring
func firstDay(t float32) Sun { return defaults.SunAt((1+t)/8, t) }

func TestSunAt_RisesInTheEastStandsOverTheSouthAndSetsInTheWest(t *testing.T) {
	south := Config{NoonWay: celestial.South, latitude: Latitude}
	if s := south.SunAt(0, 0.25); s.Dir[0] < 0.99 || !near(s.Dir[2], 0) || s.Strength > 1e-6 {
		t.Errorf("at 6 the sun is %+v, want it on the eastern horizon, no strength yet", s)
	}
	if s := south.SunAt(0, 0.5); !near(s.Dir[1], 0.5) ||
		!near(s.Dir[2], float32(math.Sin(math.Pi/3))) || s.Strength != sunStrength ||
		s.Sky != earthDaylight[3].Sky || s.Color != earthDaylight[3].Sun {
		t.Errorf("at noon the sun is %+v, want it over the south 60° up at full strength", s)
	}
	if s := south.SunAt(0, 0.75); s.Dir[0] > -0.99 {
		t.Errorf("at 18 the sun is %+v, want it on the western horizon", s)
	}
	if s := south.SunAt(0, 0); s.Dir[2] >= 0 || s.Strength != 0 ||
		s.Sky != earthDaylight[0].Sky || s.Ambient != earthDaylight[0].Ambient {
		t.Errorf("at midnight the sun is %+v, want it below the horizon, the night's ambient alone", s)
	}
	if morning, noonSun := south.SunAt(0, 0.3), south.SunAt(0, 0.5); morning.Dir[2] >= noonSun.Dir[2] {
		t.Error("the morning sun stands as high as the noon sun")
	}
}

func TestSunAt_StandsOverNoonWayAtNoonAndTurnsItsWholePathWithIt(t *testing.T) {
	northWest := defaults
	s := northWest.SunAt(0, 0.5)
	if !near(s.Dir[0], s.Dir[1]) || s.Dir[0] >= 0 || !near(s.Dir[2], float32(math.Sin(math.Pi/3))) {
		t.Errorf("at noon the sun is %+v, want it over the north-west 60° up", s)
	}
	// at 6 a quarter turn round from noon: south-west, as the east is from the south
	if s := northWest.SunAt(0, 0.25); !near(s.Dir[0], -s.Dir[1]) || s.Dir[1] <= 0 || !near(s.Dir[2], 0) {
		t.Errorf("at 6 the sun is %+v, want it on the horizon in the south-west", s)
	}
	if s, south := northWest.SunAt(0, 0.4), (Config{NoonWay: celestial.South, latitude: Latitude}).SunAt(0, 0.4); s.Strength != south.Strength || s.Ambient != south.Ambient {
		t.Errorf("turned round the sun lights %v/%v, want the same light as over the south %v/%v", s.Strength, s.Ambient, south.Strength, south.Ambient)
	}
}

type rig struct {
	clk *clock.Clock
	sky *Sky
}

func skyOf(t *testing.T, cfg Config, cal calendar.Config) *rig {
	t.Helper()
	clk := clock.New(clock.Config{})
	return &rig{clk: clk, sky: New(calendar.New(clk, cal), cfg, Latitude)}
}

func (r *rig) tick(d time.Duration) {
	r.clk.Replay(nil, d)
	r.sky.Update()
}

func TestSky_TheSunFollowsTheCalendarInSteps(t *testing.T) {
	cfg := calendar.Config{DayRealTime: 24 * time.Second, Start: 12 * time.Hour}
	r := skyOf(t, Config{Steps: 24}, cfg)
	r.tick(time.Second / 60)
	if r.sky.Sun() != firstDay(0.5) {
		t.Errorf("the sky's sun %+v, want noon's", r.sky.Sun())
	}
	r.tick(time.Second / 2) // half an hour of a 24-second day: still the noon step
	if r.sky.Sun() != firstDay(0.5) {
		t.Error("the sun moved within a step")
	}
	r.tick(time.Second) // past one: the next step
	if r.sky.Sun() != firstDay(13.0/24) {
		t.Errorf("an hour and a half on the sun is %+v, want the step at 13", r.sky.Sun())
	}
}

func TestSky_TheSunGoesOnWithTheCalendar(t *testing.T) {
	r := skyOf(t, Config{}, calendar.Config{DayRealTime: 24 * time.Second, Start: 12 * time.Hour})
	r.tick(0)
	noon := r.sky.Sun()
	r.tick(time.Second / 60)
	if r.sky.Sun() == noon || r.sky.Sun() != firstDay(0.5+1.0/60/24) {
		t.Errorf("a tick on the sun is %+v, want the moment's own, moved on from noon's", r.sky.Sun())
	}
}

func TestSky_FrozenLightStandsWhileTheCalendarGoesOn(t *testing.T) {
	cfg := calendar.Config{DayRealTime: 24 * time.Second, Start: 12 * time.Hour}
	r := skyOf(t, Config{Steps: 24}, cfg)
	r.sky.SetFrozen(!r.sky.Frozen())
	r.tick(0)
	r.tick(6 * time.Second) // six hours of the day
	if !r.sky.Frozen() || r.sky.Hour() != 0.5 || r.sky.Sun() != firstDay(0.5) {
		t.Fatalf("frozen at noon the light is %+v at %v, want noon's still", r.sky.Sun(), r.sky.Hour())
	}
	if m := r.sky.calendar.Now(); !near(m.Time, 0.75) {
		t.Errorf("the calendar stands at %v, want 18:00: the day goes on under the frozen light", m.Time)
	}
	r.sky.Shift(HalfHour)
	r.sky.Shift(HalfHour)
	r.sky.Shift(-HalfHour)
	r.tick(0)
	if !near(r.sky.Hour(), 12.5/24) {
		t.Errorf("two half hours on and one back the light is at %v, want 12:30", r.sky.Hour())
	}
	r.sky.Shift(HalfHour)
	r.tick(0)
	if r.sky.Sun() != firstDay(13.0/24) { // the sun moves by the day's steps, hours here
		t.Errorf("at 13:00 the sun is %+v, want the step's own at once", r.sky.Sun())
	}
	r.sky.SetFrozen(!r.sky.Frozen())
	r.tick(0)
	if r.sky.Frozen() || r.sky.Sun() != firstDay(0.75) {
		t.Errorf("let go, the light is %+v, want the calendar's 18:00 at once", r.sky.Sun())
	}
}

func TestSky_EarlierBeforeMidnightIsTheEveningBefore(t *testing.T) {
	r := skyOf(t, Config{}, calendar.Config{Start: 15 * time.Minute})
	r.sky.SetFrozen(!r.sky.Frozen())
	r.sky.Shift(-HalfHour)
	r.tick(0)
	if !near(r.sky.Hour(), 23.75/24) {
		t.Errorf("half an hour back from 00:15 the light is at %v, want 23:45", r.sky.Hour())
	}
	r.sky.Shift(0.5 / 24)
	if !near(r.sky.Hour(), 0.25/24) {
		t.Errorf("half an hour on from 23:45 the light is at %v, want 00:15", r.sky.Hour())
	}
}

func TestSky_BeginsFrozenAtTheConfigsHour(t *testing.T) {
	cfg := calendar.Config{DayRealTime: 24 * time.Second, Start: 12 * time.Hour}
	r := skyOf(t, Config{Frozen: true, Hour: 18 * time.Hour, Steps: 24}, cfg)
	r.tick(0)
	if !r.sky.Frozen() || r.sky.Sun() != firstDay(0.75) {
		t.Errorf("begun frozen at 18:00 the light is %+v, want the evening's", r.sky.Sun())
	}
	if rep := (Config{Frozen: true}).withDefaults(); rep.Hour != 12*time.Hour {
		t.Errorf("a frozen light of no hour is at %v, want noon", rep.Hour)
	}
}

func TestReport_SaysWhetherTheLightIsFrozen(t *testing.T) {
	r := skyOf(t, Config{}, calendar.Config{})
	var got string
	r.sky.Reporter().Report(func(label, value string) { got = label + ": " + value })
	if got != "Light: the hour's" {
		t.Errorf("the report says %q, want the hour's light", got)
	}
	r.sky.SetFrozen(true)
	r.sky.Shift(14.0/24 - r.sky.Hour())
	r.sky.Reporter().Report(func(label, value string) { got = label + ": " + value })
	if got != "Light: frozen at 14:00" {
		t.Errorf("the report says %q, want the light frozen at 14:00", got)
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
	if sky, _, _ := defaults.withDefaults().daylightAt(0.2); sky[0] >= earthDaylight[2].Sky[0] ||
		sky[0] <= earthDaylight[3].Sky[0] {
		t.Errorf("between sunrise and day the sky is %v, want it blended between theirs", sky)
	}
}

func TestDaylight_AGamesOwnColoursTheSkyItsLightAndTheSun(t *testing.T) {
	if got := EarthDaylight(); len(got) != 4 || got[3] != earthDaylight[3] {
		t.Fatalf("the Earth's daylight is %v, want the four rows of night, twilight, sunrise "+
			"and day", got)
	}
	earth := Config{NoonWay: celestial.South, latitude: Latitude}
	own := earth
	own.Daylight = EarthDaylight()
	if earth.SunAt(0, 0.5) != own.SunAt(0, 0.5) || earth.SunAt(0, 0.3) != own.SunAt(0, 0.3) {
		t.Error("the Earth's daylight given by hand lights the world otherwise than none given")
	}
	red := render.Light{1, 0, 0}
	mars := earth
	mars.Daylight = []Daylight{{Height: 0, Sky: red, Sun: red, Ambient: 0.5}}
	if sun := mars.SunAt(0, 0.5); sun.Sky != red || sun.Color != red || sun.Ambient != 0.5 {
		t.Errorf("under a red daylight the sun is %+v, want its sky and light red", sun)
	}
	if c := skyOf(t, mars, calendar.Config{}).sky.Heavens().SunColor; c != red {
		t.Errorf("under a red daylight the heavens' sun is %v, want red", c)
	}
}

func TestHeavens_TheSunsColourIsTheDaylightsWhereItStands(t *testing.T) {
	noon := defaults
	noon.Frozen = true
	if c := skyOf(t, noon, calendar.Config{}).sky.Heavens().SunColor; c[2] < 0.9 {
		t.Errorf("at noon the sun is %v, want it nearly white", c)
	}
	night := noon
	night.Hour = 30 * time.Minute
	if c := skyOf(t, night, calendar.Config{}).sky.Heavens().SunColor; c != earthDaylight[0].Sun {
		t.Errorf("at night the heavens' sun is %v, want the daylight's at its height, not the "+
			"moon's", c)
	}
}

func TestNew_ADaylightThatDoesNotRisePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("a daylight falling from row to row was taken")
		}
	}()
	c := defaults
	c.Daylight = []Daylight{{Height: 0.3}, {Height: 0.1}}
	skyOf(t, c, calendar.Config{})
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

func TestLightAt_TheMoonLightsTheNightAsFullAsItIs(t *testing.T) {
	c := defaults
	if day := c.LightAt(0, 0.5, 0.5, DefaultMoon()); day != c.SunAt(0, 0.5) {
		t.Errorf("by day the light is %+v, want the sun", day)
	}
	full, new := c.LightAt(0, 0, 0.5, DefaultMoon()), c.LightAt(0, 0, 0, DefaultMoon())
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

func TestHeavensAt_StandTheSunTheMoonAndThePole(t *testing.T) {
	c := Config{NoonWay: celestial.South, latitude: 50}
	h, l := c.place().HeavensAt(0.2, 0.5, 0.5), c.LightAt(0.2, 0.5, 0.5, DefaultMoon())
	if h.Sun != l.Dir {
		t.Errorf("at noon the heavens' sun is %v, the light's %v", h.Sun, l.Dir)
	}
	h, l = c.place().HeavensAt(0.2, 0.02, 0.5), c.LightAt(0.2, 0.02, 0.5, DefaultMoon())
	if h.Moon != l.Dir || h.Full < 0.99 {
		t.Errorf("at night the heavens' moon is %v, full %v; the light's %v, want it, full", h.Moon, h.Full, l.Dir)
	}
	h = c.place().HeavensAt(0.2, 0.5, 0)
	if up := float64(h.Pole[2]); math.Abs(up-math.Sin(50*math.Pi/180)) > 1e-5 || h.Pole[1] > -0.5 {
		t.Errorf("the pole is %v, want 50° up over the north, away from the noon sun", h.Pole)
	}
}
