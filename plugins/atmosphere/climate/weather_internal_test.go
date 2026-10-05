package climate

import (
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/clock"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/atmosphere/calendar"
	"github.com/kjkrol/gram/plugins/atmosphere/climate/weather"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/uid"
)

func near(a, b, tolerance float32) bool { return math.Abs(float64(a-b)) <= float64(tolerance) }

type rig struct {
	w      *world.Plugin
	clk    *clock.Clock
	c      *Climate
	sys    *weatherSystem
	winter heards // the world's, told by a rule of the weather in a winter
}

// winterNow is the command the rig's rule gives in a winter, as the world's weather says.
type winterNow struct{}

// heards is where the winterNow commands land, for the world to carry.
type heards struct{ control.Queue[winterNow] }

func (h *heards) Queues() []control.CommandQueue     { return []control.CommandQueue{&h.Queue} }
func (h *heards) DefaultBindings() []control.Binding { return nil }

// weatherOf runs a weather system of cfg over a new world, on a calendar of 4-minute days begun in
// the middle of season.
func weatherOf(t *testing.T, cfg Config, season calendar.Season) *rig {
	t.Helper()
	r := &rig{w: world.NewPlugin(world.Config{Space: world.SpaceCfg{Width: 640, Height: 480}, Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 8}}), clk: clock.New(clock.Config{})}
	cal := calendar.New(r.clk, calendar.Config{Day: 4 * time.Minute, Season: season})
	r.c = New(r.w, cal, cfg)
	if err := r.w.Carry(&r.winter); err != nil {
		t.Fatal(err)
	}
	if err := r.c.Rules().Add(rule.Then[Weathering]("winter", rule.All, rule.If(func(w Weathering) bool { return w.Season == calendar.Winter && w.Weather == r.c.Air() }, rule.Order(winterNow{})))); err != nil {
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
		{Name: "cloudy", Wind: [2]float32{20, 20}, Clouds: [2]float32{0.8, 0.8}, Falls: 0.5, Lasts: [2]time.Duration{time.Minute, time.Minute}, Next: map[string]float32{"clear": 1}},
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
	r.c.Change()
	r.tick(time.Millisecond)
	if r.now().State != 1 {
		t.Errorf("after Change the state is %d, want the next", r.now().State)
	}
	r.c.Set("clear")
	r.c.Set("fog")
	r.tick(time.Millisecond)
	if r.now().State != 0 {
		t.Errorf("after Set clear (and an unknown fog) the state is %d, want clear", r.now().State)
	}
}

func TestWeather_TheWindCarriesTheClouds(t *testing.T) {
	r := weatherOf(t, twoStates(), calendar.Summer)
	r.c.Set("cloudy")
	r.run(5 * time.Second) // blown up to 20
	before := r.now().Drift
	r.run(10 * time.Second)
	after := r.now().Drift
	if d := float32(math.Hypot(float64(after[0]-before[0]), float64(after[1]-before[1]))); !near(d, 200, 10) {
		t.Errorf("in 10 seconds of a wind of 20 the clouds drifted %v, want about 200", d)
	}
}

// With the weather's workings stopped the clear sky stays past its second, the clouds stand
// still and the air has no wind, clouds or rain, though the weather goes on underneath; set going
// again, all of it comes back.
func TestClimate_RunningLeavesOutWhatIsStopped(t *testing.T) {
	r := weatherOf(t, twoStates(), calendar.Summer)
	r.c.SetRunning(Running{})
	r.run(5 * time.Second)
	if w := r.now(); w.State != 0 {
		t.Fatalf("the weather changes stopped, it went on to state %d", w.State)
	}
	r.c.Set("cloudy")
	r.run(10 * time.Second)
	w, a := r.now(), r.c.Air()
	if w.State != 1 || w.Clouds < 0.5 || w.Rain < 0.3 {
		t.Fatalf("set by hand the weather is %+v, want the clouds and the rain come underneath", w)
	}
	if w.Drift != ([2]float32{}) || a.Wind != ([2]float32{}) || a.Clouds != 0 || a.Rain != 0 || a.Snow != 0 {
		t.Errorf("stopped, the clouds drifted %v and the air is %+v, want still, clear and dry", w.Drift, a)
	}
	r.c.SetRunning(AllRunning())
	r.run(time.Second)
	if a := r.c.Air(); a.Wind == ([2]float32{}) || a.Clouds < 0.5 || a.Rain < 0.3 || r.now().Drift == ([2]float32{}) {
		t.Errorf("going again the air is %+v, want the wind, the clouds and the rain back", a)
	}
}

func TestWeather_IsColdInWinterWarmInSummerAndSnowsWhenCold(t *testing.T) {
	winter := weatherOf(t, twoStates(), calendar.Winter)
	winter.c.Set("cloudy")
	winter.run(20 * time.Second)
	if w := winter.now(); w.Temperature >= 0 || w.Rain > 0.01 || w.Snow < 0.4 {
		t.Errorf("in midwinter it is %v°, rain %v, snow %v; want below freezing and snowing", w.Temperature, w.Rain, w.Snow)
	}
	summer := weatherOf(t, twoStates(), calendar.Summer)
	summer.c.Set("cloudy")
	summer.run(20 * time.Second)
	if w := summer.now(); w.Temperature < 10 || w.Snow > 0.01 || w.Rain < 0.4 {
		t.Errorf("in midsummer it is %v°, rain %v, snow %v; want warm and raining", w.Temperature, w.Rain, w.Snow)
	}
	if air := summer.c.Air(); air.Temperature != summer.now().Temperature {
		t.Errorf("the world's temperature is %v, want the weather's %v", air.Temperature, summer.now().Temperature)
	}
}

func TestWeather_TellsItsRulesEveryStep(t *testing.T) {
	r := weatherOf(t, twoStates(), calendar.Winter)
	r.tick(time.Millisecond)
	r.tick(time.Millisecond)
	var told []uid.UID64
	r.winter.Drain(func(i control.Issued[winterNow]) { told = append(told, i.Entity) })
	if len(told) != 2 || told[0] != r.w.Clock().Entity() {
		t.Errorf("the rule told %v, want the world's entity twice, of its weather in the winter", told)
	}
}

func TestWeather_AStateComesOnlyInItsSeasonsAndTheSameSeedGoesTheSameWay(t *testing.T) {
	cfg := twoStates()
	cfg.Weathers = append(cfg.Weathers, weather.State{Name: "blizzard", Lasts: [2]time.Duration{time.Second, time.Second}, Often: [4]float32{calendar.Winter: 1}})
	cfg.Weathers[0].Next = nil // after the clear sky, any other alike
	seen := map[int32]bool{}
	r := weatherOf(t, cfg, calendar.Summer)
	for range 40 {
		r.c.Set("clear")
		r.tick(time.Millisecond)
		r.c.Change()
		r.tick(time.Millisecond)
		seen[r.now().State] = true
	}
	if seen[2] || !seen[1] {
		t.Errorf("in summer the weather went to %v, want the clouds and never the winter's blizzard", seen)
	}
	a, b := weatherOf(t, cfg, calendar.Winter), weatherOf(t, cfg, calendar.Winter)
	for range 20 {
		a.c.Change()
		b.c.Change()
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
	r.c.Set("cloudy")
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
		{Name: "wet", Clouds: [2]float32{0.8, 0.8}, Falls: 0.5, Lasts: [2]time.Duration{time.Minute, time.Minute}, Often: [4]float32{calendar.Spring: 1, calendar.Autumn: 1, calendar.Winter: 1}},
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
// the rules hear the step.
func TestWeather_GoesByTheStepsOfTheSimulation(t *testing.T) {
	r := weatherOf(t, twoStates(), calendar.Summer)
	r.c.Set("cloudy") // a minute long
	r.tick(time.Millisecond)
	left := r.now().Left
	r.tick(5 * time.Second)
	if got := left - r.now().Left; got < 4.9 || got > 5.1 {
		t.Errorf("a step of five seconds moved the weather %vs on, want 5", got)
	}
}

func TestWeather_RainsLessInTheZonesDrySeason(t *testing.T) {
	r := weatherOf(t, Config{Zone: Tropical}, calendar.Summer)
	rain := weather.Default[Config{Weathers: weather.Default}.index("rain")]
	if wet, dry := r.sys.likely(rain, calendar.Summer), r.sys.likely(rain, calendar.Winter); dry >= wet/3 {
		t.Errorf("in the tropics rain comes %v in summer and %v in winter, want the winter dry", wet, dry)
	}
	if clear := weather.Default[Config{Weathers: weather.Default}.index("clear")]; r.sys.likely(clear, calendar.Winter) != 1 {
		t.Errorf("a dry weather comes %v in the dry season, want as likely as ever", r.sys.likely(clear, calendar.Winter))
	}
}

// The climate takes rules of every weather, a role's too, and refuses a filtered one: a moment of
// the world as a whole has no entity's components to read.
func TestClimate_TakesARolesRuleAndRefusesAFilteredOne(t *testing.T) {
	w := world.NewPlugin(world.Config{Space: world.SpaceCfg{Width: 640, Height: 480}, Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 8}})
	c := New(w, calendar.New(clock.New(clock.Config{}), calendar.Config{Day: time.Minute}), Config{})
	every := rule.Then[Weathering]("any weather", rule.All, rule.Order(winterNow{}))
	having := rule.Then[Weathering]("having", rule.Having[Weather](), rule.Order(winterNow{}))
	if err := c.Rules().Add(having); !errors.Is(err, plugin.ErrUnhosted) {
		t.Errorf("Add of a filtered rule = %v; want plugin.ErrUnhosted", err)
	}
	for name, r := range map[string]rule.Rule{
		"every":  every,
		"a role": rule.NewPart("climate sheltered", 0).Obeys(every).Rules()[0],
	} {
		if err := c.Rules().Add(r); err != nil {
			t.Errorf("%s: Add = %v; want it taken", name, err)
		}
	}
}
