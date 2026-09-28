package climate

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/atmosphere/calendar"
	"github.com/kjkrol/gram/plugins/atmosphere/climate/weather"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/clock"
)

func near(a, b, tolerance float32) bool { return math.Abs(float64(a-b)) <= float64(tolerance) }

type rig struct {
	w     *world.Plugin
	clk   *clock.Clock
	c     *Climate
	sys   *weatherSystem
	heard []Weathering
}

// weatherOf runs a weather system of cfg over a new world, on a calendar of 4-minute days begun in
// the middle of season.
func weatherOf(t *testing.T, cfg Config, season calendar.Season) *rig {
	t.Helper()
	r := &rig{w: world.NewPlugin(world.Config{Space: world.SpaceCfg{Width: 640, Height: 480}, Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 8}}), clk: clock.New(clock.Config{})}
	cal := calendar.New(r.clk, calendar.Config{Day: 4 * time.Minute, Season: season})
	r.c = New(r.w, cal, cfg)
	if err := r.c.Host(Every(func(_ plugin.Tick, w Weathering) { r.heard = append(r.heard, w) })); err != nil {
		t.Fatal(err)
	}
	r.sys = r.c.System().(*weatherSystem)
	ecs := goke.New()
	ecs.Setup(goke.SystemFn{OnInit: r.sys.Init})
	return r
}

// tick is one step of the simulation over d: the weather moves on, then the clock.
func (r *rig) tick(d time.Duration) {
	r.sys.Update(nil, d)
	r.clk.Replay(nil, d)
}

// run ticks for d in steps of a tenth of a second.
func (r *rig) run(d time.Duration) {
	for range int(d / (100 * time.Millisecond)) {
		r.tick(100 * time.Millisecond)
	}
}

func (r *rig) now() Weather {
	for r.sys.query.All(); r.sys.query.Next(); {
		return r.sys.now.Slice(r.sys.query.Cursor())[0]
	}
	return Weather{}
}

// twoStates is a temperate climate of a clear sky lasting a second, then clouds lasting a minute.
func twoStates() Config {
	return Config{Weathers: []weather.State{
		{Name: "clear", Wind: [2]float32{10, 10}, Lasts: [2]time.Duration{time.Second, time.Second}, Next: map[string]float32{"cloudy": 1}},
		{Name: "cloudy", Wind: [2]float32{20, 20}, Clouds: 0.8, Falls: 0.5, Lasts: [2]time.Duration{time.Minute, time.Minute}, Next: map[string]float32{"clear": 1}},
	}, Zone: Temperate, Start: "clear", Blend: time.Second}
}

func TestWeather_BeginsInItsStartLastsAndGoesOnAsItsWeightsSay(t *testing.T) {
	r := weatherOf(t, twoStates(), calendar.Summer)
	if w := r.now(); w.State != unbegun {
		t.Fatalf("a fresh weather is %+v, want it not begun before its first step", w)
	}
	r.tick(time.Millisecond)
	if w := r.now(); w.State != 0 || w.Blow != 10 || w.Clouds != 0 {
		t.Fatalf("a begun weather is %+v, want its Start at its own wind and clouds", w)
	}
	r.tick(time.Second / 2)
	if air := r.c.Air(); !near(float32(math.Hypot(float64(air.Wind[0]), float64(air.Wind[1]))), 10, 0.01) {
		t.Errorf("the world's wind is %v, want the weather's 10", air.Wind)
	}
	r.run(time.Second)
	if w := r.now(); w.State != 1 {
		t.Errorf("after its second the weather is in state %d, want the clouds", w.State)
	}
	r.run(3 * time.Second)
	if w := r.now(); !near(w.Clouds, 0.8, 0.05) || !near(w.Blow, 20, 1) || !near(w.Rain, 0.5, 0.05) {
		t.Errorf("three blends on the weather is %+v, want the clouds' 0.8, wind 20 and rain 0.5", w)
	}
}

func TestWeather_ChangeAndSetGoOnAtOnce(t *testing.T) {
	r := weatherOf(t, twoStates(), calendar.Summer)
	r.c.change.Add(control.Nobody, Change{})
	r.tick(time.Millisecond)
	if r.now().State != 1 {
		t.Errorf("after Change the state is %d, want the next", r.now().State)
	}
	r.c.set.Add(control.Nobody, Set{Name: "clear"})
	r.c.set.Add(control.Nobody, Set{Name: "fog"})
	r.tick(time.Millisecond)
	if r.now().State != 0 {
		t.Errorf("after Set clear (and an unknown fog) the state is %d, want clear", r.now().State)
	}
}

func TestWeather_TheWindCarriesTheClouds(t *testing.T) {
	r := weatherOf(t, twoStates(), calendar.Summer)
	r.c.set.Add(control.Nobody, Set{Name: "cloudy"})
	r.run(5 * time.Second) // blown up to 20
	before := r.now().Drift
	r.run(10 * time.Second)
	after := r.now().Drift
	if d := float32(math.Hypot(float64(after[0]-before[0]), float64(after[1]-before[1]))); !near(d, 200, 10) {
		t.Errorf("in 10 seconds of a wind of 20 the clouds drifted %v, want about 200", d)
	}
}

func TestWeather_IsColdInWinterWarmInSummerAndSnowsWhenCold(t *testing.T) {
	winter := weatherOf(t, twoStates(), calendar.Winter)
	winter.c.set.Add(control.Nobody, Set{Name: "cloudy"})
	winter.run(20 * time.Second)
	if w := winter.now(); w.Temperature >= 0 || w.Rain > 0.01 || w.Snow < 0.4 {
		t.Errorf("in midwinter it is %v°, rain %v, snow %v; want below freezing and snowing", w.Temperature, w.Rain, w.Snow)
	}
	summer := weatherOf(t, twoStates(), calendar.Summer)
	summer.c.set.Add(control.Nobody, Set{Name: "cloudy"})
	summer.run(20 * time.Second)
	if w := summer.now(); w.Temperature < 10 || w.Snow > 0.01 || w.Rain < 0.4 {
		t.Errorf("in midsummer it is %v°, rain %v, snow %v; want warm and raining", w.Temperature, w.Rain, w.Snow)
	}
	if air := summer.c.Air(); air.Temperature != summer.now().Temperature {
		t.Errorf("the world's temperature is %v, want the weather's %v", air.Temperature, summer.now().Temperature)
	}
}

func TestWeather_TellsItsBehavioursEveryStep(t *testing.T) {
	r := weatherOf(t, twoStates(), calendar.Winter)
	r.tick(time.Millisecond)
	r.tick(time.Millisecond)
	if len(r.heard) != 2 || r.heard[1].Season != calendar.Winter || r.heard[1].Weather != r.c.Air() {
		t.Errorf("the behaviour heard %+v, want the world's weather and the winter, every step", r.heard)
	}
}

func TestWeather_AStateComesOnlyInItsSeasonsAndTheSameSeedGoesTheSameWay(t *testing.T) {
	cfg := twoStates()
	cfg.Weathers = append(cfg.Weathers, weather.State{Name: "blizzard", Lasts: [2]time.Duration{time.Second, time.Second}, Often: [4]float32{calendar.Winter: 1}})
	cfg.Weathers[0].Next = nil // after the clear sky, any other alike
	seen := map[int32]bool{}
	r := weatherOf(t, cfg, calendar.Summer)
	for range 40 {
		r.c.set.Add(control.Nobody, Set{Name: "clear"})
		r.tick(time.Millisecond)
		r.c.change.Add(control.Nobody, Change{})
		r.tick(time.Millisecond)
		seen[r.now().State] = true
	}
	if seen[2] || !seen[1] {
		t.Errorf("in summer the weather went to %v, want the clouds and never the winter's blizzard", seen)
	}
	a, b := weatherOf(t, cfg, calendar.Winter), weatherOf(t, cfg, calendar.Winter)
	for range 20 {
		a.c.change.Add(control.Nobody, Change{})
		b.c.change.Add(control.Nobody, Change{})
		a.tick(time.Millisecond)
		b.tick(time.Millisecond)
		if a.now().State != b.now().State {
			t.Fatal("two weathers of one seed went apart")
		}
	}
}

func TestReport_SaysTheWeatherTheWindAndTheSnow(t *testing.T) {
	r := weatherOf(t, twoStates(), calendar.Winter)
	rep := &report{names: []string{"clear", "cloudy"}, query: r.sys.query, now: r.sys.now}
	r.c.set.Add(control.Nobody, Set{Name: "cloudy"})
	r.run(20 * time.Second)
	var got string
	rep.Report(func(label, value string) { got = label + ": " + value })
	if !strings.HasPrefix(got, "Weather: cloudy (snow), -") || !strings.Contains(got, "°C, wind 20") {
		t.Errorf("the report says %q, want the clouds snowing, the frost and the wind of 20", got)
	}
}

func TestWeather_BeginsAsTheSeasonHasIt(t *testing.T) {
	cfg := Config{Weathers: []weather.State{
		{Name: "sunny", Lasts: [2]time.Duration{time.Minute, time.Minute}, Often: [4]float32{calendar.Summer: 1}},
		{Name: "wet", Clouds: 0.8, Falls: 0.5, Lasts: [2]time.Duration{time.Minute, time.Minute}, Often: [4]float32{calendar.Spring: 1, calendar.Autumn: 1, calendar.Winter: 1}},
	}}
	for season, want := range map[calendar.Season]int32{calendar.Summer: 0, calendar.Autumn: 1, calendar.Spring: 1} {
		r := weatherOf(t, cfg, season)
		r.tick(time.Millisecond)
		if w := r.now(); w.State != want {
			t.Errorf("begun in %v the weather is %d, want %d", season, w.State, want)
		}
	}
	autumn := weatherOf(t, cfg, calendar.Autumn)
	autumn.tick(time.Millisecond)
	if w := autumn.now(); w.Clouds != 0.8 || w.Rain != 0.5 {
		t.Errorf("begun wet the weather is %+v, want its clouds and rain at once", w)
	}
}

func TestWeather_BegunInWinterIsColdAtOnce(t *testing.T) {
	r := weatherOf(t, twoStates(), calendar.Winter)
	r.tick(time.Millisecond)
	if w := r.now(); w.Temperature >= 0 {
		t.Errorf("begun in midwinter it is %v°, want the frost at once rather than coming down to it", w.Temperature)
	}
}

// The weather goes by the steps it is given, the simulation's: a longer step moves it further, and
// the behaviours hear the step.
func TestWeather_GoesByTheStepsOfTheSimulation(t *testing.T) {
	r := weatherOf(t, twoStates(), calendar.Summer)
	r.c.set.Add(control.Nobody, Set{Name: "cloudy"}) // a minute long
	r.tick(time.Millisecond)
	left := r.now().Left
	r.tick(5 * time.Second)
	if got := left - r.now().Left; got < 4.9 || got > 5.1 {
		t.Errorf("a step of five seconds moved the weather %vs on, want 5", got)
	}
}

func TestWeather_RainsLessInTheZonesDrySeason(t *testing.T) {
	r := weatherOf(t, Config{Zone: Tropical}, calendar.Summer)
	rain := weather.Default[2]
	if wet, dry := r.sys.likely(rain, calendar.Summer), r.sys.likely(rain, calendar.Winter); dry >= wet/3 {
		t.Errorf("in the tropics rain comes %v in summer and %v in winter, want the winter dry", wet, dry)
	}
	if clear := weather.Default[0]; r.sys.likely(clear, calendar.Winter) != 1 {
		t.Errorf("a dry weather comes %v in the dry season, want as likely as ever", r.sys.likely(clear, calendar.Winter))
	}
}
