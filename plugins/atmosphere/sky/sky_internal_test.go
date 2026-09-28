package sky

import (
	"math"
	"testing"
	"time"

	"github.com/kjkrol/gram/plugins/atmosphere/calendar"
	"github.com/kjkrol/gram/plugins/world/clock"
)

func near(a, b float32) bool { return math.Abs(float64(a-b)) < 1e-3 }

// defaults is a zero Config at the Latitude the sky has without a climate.
var defaults = Config{latitude: Latitude}

// firstDay is the sun of a zero Config's first day — the middle of spring, the year's second — at
// time t, as the sky sets it.
func firstDay(t float32) Sun { return defaults.SunAt((1+t)/8, t) }

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

// rig is a sky on a clock the test moves on.
type rig struct {
	clk *clock.Clock
	sky *Sky
}

func skyOf(t *testing.T, cfg Config, cal calendar.Config) *rig {
	t.Helper()
	clk := clock.New(clock.Config{})
	return &rig{clk: clk, sky: New(calendar.New(clk, cal), cfg, Latitude)}
}

// tick has the clock move on by d, then the sky set the sun.
func (r *rig) tick(d time.Duration) {
	r.clk.Replay(nil, d)
	r.sky.Update()
}

func TestSky_TheSunFollowsTheCalendarInSteps(t *testing.T) {
	r := skyOf(t, Config{Steps: 24}, calendar.Config{Day: 24 * time.Second, Start: 0.5})
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

// Frozen, the light stands at its hour while the calendar goes on; Later and Earlier move it by
// half an hour and the sun follows at once; let go, it is the hour's again.
func TestSky_FrozenLightStandsWhileTheCalendarGoesOn(t *testing.T) {
	r := skyOf(t, Config{Steps: 24}, calendar.Config{Day: 24 * time.Second, Start: 0.5})
	r.sky.freeze.Add(0, Freeze{})
	r.tick(0)
	r.tick(6 * time.Second) // six hours of the day
	if !r.sky.Frozen() || r.sky.Hour() != 0.5 || r.sky.Sun() != firstDay(0.5) {
		t.Fatalf("frozen at noon the light is %+v at %v, want noon's still", r.sky.Sun(), r.sky.Hour())
	}
	if m := r.sky.calendar.Now(); !near(m.Time, 0.75) {
		t.Errorf("the calendar stands at %v, want 18:00: the day goes on under the frozen light", m.Time)
	}
	r.sky.later.Add(0, Later{})
	r.sky.later.Add(0, Later{})
	r.sky.earlier.Add(0, Earlier{})
	r.tick(0)
	if !near(r.sky.Hour(), 12.5/24) {
		t.Errorf("two half hours on and one back the light is at %v, want 12:30", r.sky.Hour())
	}
	r.sky.later.Add(0, Later{})
	r.tick(0)
	if r.sky.Sun() != firstDay(13.0/24) { // the sun moves by the day's steps, hours here
		t.Errorf("at 13:00 the sun is %+v, want the step's own at once", r.sky.Sun())
	}
	r.sky.freeze.Add(0, Freeze{})
	r.tick(0)
	if r.sky.Frozen() || r.sky.Sun() != firstDay(0.75) {
		t.Errorf("let go, the light is %+v, want the calendar's 18:00 at once", r.sky.Sun())
	}
}

func TestSky_EarlierBeforeMidnightIsTheEveningBefore(t *testing.T) {
	r := skyOf(t, Config{}, calendar.Config{Start: 0.25 / 24})
	r.sky.freeze.Add(0, Freeze{})
	r.sky.earlier.Add(0, Earlier{})
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
	r := skyOf(t, Config{Frozen: true, Hour: 18.0 / 24, Steps: 24}, calendar.Config{Day: 24 * time.Second, Start: 0.5})
	r.tick(0)
	if !r.sky.Frozen() || r.sky.Sun() != firstDay(0.75) {
		t.Errorf("begun frozen at 18:00 the light is %+v, want the evening's", r.sky.Sun())
	}
	if rep := (Config{Frozen: true}).withDefaults(); rep.Hour != 0.5 {
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
	if sky, _, _ := daylightAt(0.2); sky[0] >= daylight[2].sky[0] || sky[0] <= daylight[3].sky[0] {
		t.Errorf("between sunrise and day the sky is %v, want it blended between theirs", sky)
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
