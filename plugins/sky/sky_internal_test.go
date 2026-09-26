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

// defaults is the day of a zero Config.
var defaults Config

func TestSunAt_RisesInTheEastStandsOverTheSouthAndSetsInTheWest(t *testing.T) {
	if s := defaults.SunAt(0.25); s.Dir[0] < 0.99 || !near(s.Dir[2], 0) || s.Strength != 0 {
		t.Errorf("at 6 the sun is %+v, want it on the eastern horizon, no strength yet", s)
	}
	if s := defaults.SunAt(0.5); !near(s.Dir[1], 0.5) || !near(s.Dir[2], float32(math.Sin(math.Pi/3))) || s.Strength != sunStrength || s.Sky != daylight[3].sky || s.Color != daylight[3].sun {
		t.Errorf("at noon the sun is %+v, want it over the south 60° up at full strength", s)
	}
	if s := defaults.SunAt(0.75); s.Dir[0] > -0.99 {
		t.Errorf("at 18 the sun is %+v, want it on the western horizon", s)
	}
	if s := defaults.SunAt(0); s.Dir[2] >= 0 || s.Strength != 0 || s.Sky != daylight[0].sky || s.Ambient != daylight[0].ambient {
		t.Errorf("at midnight the sun is %+v, want it below the horizon, the night's ambient alone", s)
	}
	if morning, noonSun := defaults.SunAt(0.3), defaults.SunAt(0.5); morning.Dir[2] >= noonSun.Dir[2] {
		t.Error("the morning sun stands as high as the noon sun")
	}
}

func TestSunAt_StandsOverNoonWayAtNoonAndTurnsItsWholePathWithIt(t *testing.T) {
	northWest := Config{NoonWay: [2]float32{-1, -1}}
	s := northWest.SunAt(0.5)
	if !near(s.Dir[0], s.Dir[1]) || s.Dir[0] >= 0 || !near(s.Dir[2], float32(math.Sin(math.Pi/3))) {
		t.Errorf("at noon the sun is %+v, want it over the north-west 60° up", s)
	}
	// a quarter turn round from noon at 6: south-west, as the east is from the south
	if s := northWest.SunAt(0.25); !near(s.Dir[0], -s.Dir[1]) || s.Dir[1] <= 0 || !near(s.Dir[2], 0) {
		t.Errorf("at 6 the sun is %+v, want it on the horizon in the south-west", s)
	}
	if s, south := northWest.SunAt(0.4), defaults.SunAt(0.4); s.Strength != south.Strength || s.Ambient != south.Ambient {
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
	sys := newSkySystem(cfg.withDefaults(), w, &q.forward, &q.back, &q.pause)
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
	if w.Sun() != defaults.SunAt(0.5) {
		t.Errorf("the world's sun %+v, want noon's", w.Sun())
	}
	tick(time.Second / 2) // half an hour of a 24-second day: still the noon step
	if w.Sun() != defaults.SunAt(0.5) {
		t.Error("the sun moved within a step")
	}
	tick(time.Second) // past one: the next step
	if d := day(); !near(d.Time, 0.5+1.5/24+1.0/60/24) || w.Sun() != defaults.SunAt(13.0/24) {
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
	if w.Sun() != defaults.SunAt(13.0/24) { // the sun moves by the day's steps, hours here
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
	noon, dawn, night := defaults.SunAt(0.5), defaults.SunAt(0.25), defaults.SunAt(0)
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
