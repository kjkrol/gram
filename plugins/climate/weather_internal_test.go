package climate

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugin/host"
	"github.com/kjkrol/gram/plugins/climate/weather"
	"github.com/kjkrol/gram/plugins/sky"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

func near(a, b, tolerance float32) bool { return math.Abs(float64(a-b)) <= float64(tolerance) }

type rig struct {
	w      *world.Plugin
	sys    *weatherSystem
	change control.Queue[Change]
	set    control.Queue[Set]
	heard  []Weathering
}

// weatherOf runs a weather system of cfg over a new world, under a sky on date of 8 days when
// date is not negative.
func weatherOf(t *testing.T, cfg Config, date int32) *rig {
	t.Helper()
	r := &rig{w: world.NewPlugin(world.Config{Space: world.SpaceCfg{Width: 640, Height: 480}, Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 8}})}
	behaviours := &host.EachHost[Weathering]{}
	if err := behaviours.Add(Every(func(_ plugin.Tick, w Weathering) { r.heard = append(r.heard, w) })); err != nil {
		t.Fatal(err)
	}
	r.sys = newWeatherSystem(cfg.withDefaults(), r.w, &r.change, &r.set, behaviours)
	ecs := goke.New()
	ecs.Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		if date >= 0 {
			var day goke.Comp[sky.Day]
			f := si.NewFactory(&day)
			f.Create(1)
			for f.Next() {
				day.Slice(&f.Cursor)[0] = sky.Day{Date: date, Time: 0.5, Pace: 1, Length: 4 * time.Minute}
			}
		}
		r.sys.Init(si)
	}})
	return r
}

// tick moves the sky's day on by d at its pace, as the sky does, then the weather.
func (r *rig) tick(d time.Duration) {
	for r.sys.days.All(); r.sys.days.Next(); {
		day := &r.sys.day.Slice(r.sys.days.Cursor())[0]
		if !day.Stopped {
			r.move(day, float32(d.Seconds()/day.Length.Seconds())*day.Pace)
		}
	}
	r.sys.Update(nil, d)
}

// move moves day on by part of a day.
func (r *rig) move(day *sky.Day, by float32) {
	day.Time += by
	for day.Time >= 1 {
		day.Time--
		day.Date = (day.Date + 1) % day.Calendar.Days()
	}
}

// pace sets the sky's day going by at pace, or standing.
func (r *rig) pace(pace float32, stopped bool) {
	for r.sys.days.All(); r.sys.days.Next(); {
		d := &r.sys.day.Slice(r.sys.days.Cursor())[0]
		d.Pace, d.Stopped = pace, stopped
	}
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
	r := weatherOf(t, twoStates(), 2)
	if w := r.now(); w.State != unbegun {
		t.Fatalf("a fresh weather is %+v, want it not begun before its first tick", w)
	}
	r.tick(time.Millisecond)
	if w := r.now(); w.State != 0 || w.Blow != 10 || w.Clouds != 0 {
		t.Fatalf("a begun weather is %+v, want its Start at its own wind and clouds", w)
	}
	r.tick(time.Second / 2)
	if air := r.w.Weather(); !near(float32(math.Hypot(float64(air.Wind[0]), float64(air.Wind[1]))), 10, 0.01) {
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
	r := weatherOf(t, twoStates(), 2)
	r.change.Add(control.Nobody, Change{})
	r.tick(time.Millisecond)
	if r.now().State != 1 {
		t.Errorf("after Change the state is %d, want the next", r.now().State)
	}
	r.set.Add(control.Nobody, Set{Name: "clear"})
	r.set.Add(control.Nobody, Set{Name: "fog"})
	r.tick(time.Millisecond)
	if r.now().State != 0 {
		t.Errorf("after Set clear (and an unknown fog) the state is %d, want clear", r.now().State)
	}
}

func TestWeather_TheWindCarriesTheClouds(t *testing.T) {
	r := weatherOf(t, twoStates(), 2)
	r.set.Add(control.Nobody, Set{Name: "cloudy"})
	r.run(5 * time.Second) // blown up to 20
	before := r.now().Drift
	r.run(10 * time.Second)
	after := r.now().Drift
	if d := float32(math.Hypot(float64(after[0]-before[0]), float64(after[1]-before[1]))); !near(d, 200, 10) {
		t.Errorf("in 10 seconds of a wind of 20 the clouds drifted %v, want about 200", d)
	}
}

func TestWeather_IsColdInWinterWarmInSummerAndSnowsWhenCold(t *testing.T) {
	winter := weatherOf(t, twoStates(), 7)
	winter.set.Add(control.Nobody, Set{Name: "cloudy"})
	winter.run(20 * time.Second)
	if w := winter.now(); w.Temperature >= 0 || w.Rain > 0.01 || w.Snow < 0.4 {
		t.Errorf("in midwinter it is %v°, rain %v, snow %v; want below freezing and snowing", w.Temperature, w.Rain, w.Snow)
	}
	summer := weatherOf(t, twoStates(), 3)
	summer.set.Add(control.Nobody, Set{Name: "cloudy"})
	summer.run(20 * time.Second)
	if w := summer.now(); w.Temperature < 10 || w.Snow > 0.01 || w.Rain < 0.4 {
		t.Errorf("in midsummer it is %v°, rain %v, snow %v; want warm and raining", w.Temperature, w.Rain, w.Snow)
	}
	if air := summer.w.Weather(); air.Temperature != summer.now().Temperature {
		t.Errorf("the world's temperature is %v, want the weather's %v", air.Temperature, summer.now().Temperature)
	}
}

func TestWeather_TellsItsBehavioursEveryTick(t *testing.T) {
	r := weatherOf(t, twoStates(), 7)
	r.tick(time.Millisecond)
	r.tick(time.Millisecond)
	if len(r.heard) != 2 || r.heard[1].Season != sky.Winter || r.heard[1].Weather != r.w.Weather() {
		t.Errorf("the behaviour heard %+v, want the world's weather and the winter, every tick", r.heard)
	}
}

func TestWeather_AStateComesOnlyInItsSeasonsAndTheSameSeedGoesTheSameWay(t *testing.T) {
	cfg := twoStates()
	cfg.Weathers = append(cfg.Weathers, weather.State{Name: "blizzard", Lasts: [2]time.Duration{time.Second, time.Second}, Often: [4]float32{sky.Winter: 1}})
	cfg.Weathers[0].Next = nil // after the clear sky, any other alike
	seen := map[int32]bool{}
	r := weatherOf(t, cfg, 2)
	for range 40 {
		r.set.Add(control.Nobody, Set{Name: "clear"})
		r.tick(time.Millisecond)
		r.change.Add(control.Nobody, Change{})
		r.tick(time.Millisecond)
		seen[r.now().State] = true
	}
	if seen[2] || !seen[1] {
		t.Errorf("in summer the weather went to %v, want the clouds and never the winter's blizzard", seen)
	}
	a, b := weatherOf(t, cfg, 7), weatherOf(t, cfg, 7)
	for range 20 {
		a.change.Add(control.Nobody, Change{})
		b.change.Add(control.Nobody, Change{})
		a.tick(time.Millisecond)
		b.tick(time.Millisecond)
		if a.now().State != b.now().State {
			t.Fatal("two weathers of one seed went apart")
		}
	}
}

func TestPrecipitation_FallsAsMuchAsTheWeatherSaysAndNotAtAllWhenDry(t *testing.T) {
	w := world.NewPlugin(world.Config{Space: world.SpaceCfg{Width: 640, Height: 480}, Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 8}})
	p := &precipitation{world: w}
	count := func() (n int, tier render.Tier) {
		var f render.Frame
		f.Reset(w.Camera())
		p.Compose(&f, w.Camera())
		f.Each(func(t render.Tier, _ float32, _ []ebiten.Vertex) { n, tier = n+1, t })
		return
	}
	if n, _ := count(); n != 0 {
		t.Errorf("a dry sky drew %d drops", n)
	}
	w.SetWeather(world.Weather{Rain: 0.5})
	half, tier := count()
	w.SetWeather(world.Weather{Rain: 1})
	full, _ := count()
	if half == 0 || tier != render.Air || full < 2*half-1 || full > 2*half+1 {
		t.Errorf("half a rain drew %d drops on tier %v, a full one %d; want some in the air, twice as many", half, tier, full)
	}
	w.SetWeather(world.Weather{Snow: 1})
	if n, _ := count(); n == 0 {
		t.Error("a snowfall drew no flakes")
	}
}

func TestReport_SaysTheWeatherTheWindAndTheSnow(t *testing.T) {
	r := weatherOf(t, twoStates(), 7)
	rep := &report{names: []string{"clear", "cloudy"}, query: r.sys.query, now: r.sys.now}
	r.set.Add(control.Nobody, Set{Name: "cloudy"})
	r.run(20 * time.Second)
	var got string
	rep.Report(func(label, value string) { got = label + ": " + value })
	if !strings.HasPrefix(got, "Weather: cloudy (snow), -") || !strings.Contains(got, "°C, wind 20") {
		t.Errorf("the report says %q, want the clouds snowing, the frost and the wind of 20", got)
	}
}

func TestWeather_BeginsAsTheSeasonHasIt(t *testing.T) {
	cfg := Config{Weathers: []weather.State{
		{Name: "sunny", Lasts: [2]time.Duration{time.Minute, time.Minute}, Often: [4]float32{sky.Summer: 1}},
		{Name: "wet", Clouds: 0.8, Falls: 0.5, Lasts: [2]time.Duration{time.Minute, time.Minute}, Often: [4]float32{sky.Spring: 1, sky.Autumn: 1, sky.Winter: 1}},
	}}
	for date, want := range map[int32]int32{3: 0, 5: 1, 1: 1} {
		r := weatherOf(t, cfg, date)
		r.tick(time.Millisecond)
		if w := r.now(); w.State != want {
			t.Errorf("begun on day %d of 8 the weather is %d, want %d", date, w.State, want)
		}
	}
	autumn := weatherOf(t, cfg, 5)
	autumn.tick(time.Millisecond)
	if w := autumn.now(); w.Clouds != 0.8 || w.Rain != 0.5 {
		t.Errorf("begun wet the weather is %+v, want its clouds and rain at once", w)
	}
}

func TestWeather_BegunInWinterIsColdAtOnce(t *testing.T) {
	r := weatherOf(t, twoStates(), 7)
	r.tick(time.Millisecond)
	if w := r.now(); w.Temperature >= 0 {
		t.Errorf("begun in midwinter it is %v°, want the frost at once rather than coming down to it", w.Temperature)
	}
}

func TestWeather_GoesByInTheSkysTime(t *testing.T) {
	quick, still := weatherOf(t, twoStates(), 3), weatherOf(t, twoStates(), 3)
	quick.pace(4, false)
	still.pace(1, true)
	quick.tick(time.Millisecond)
	still.tick(time.Millisecond)
	quick.run(time.Second / 2) // two seconds of the sky's time: the clear sky's one is over
	still.run(2 * time.Second)
	if quick.now().State != 1 {
		t.Errorf("half a second of a day at pace 4 left the weather in state %d, want it gone on to the clouds", quick.now().State)
	}
	if w := still.now(); w.State != 0 || w.Drift != [2]float32{} {
		t.Errorf("two seconds of a day standing moved the weather to %+v, want it held", w)
	}
	var told time.Duration
	quick.sys.host = &host.EachHost[Weathering]{}
	if err := quick.sys.host.Add(Every(func(tk plugin.Tick, _ Weathering) { told = tk.Dt })); err != nil {
		t.Fatal(err)
	}
	quick.tick(time.Second / 10)
	if told < 399*time.Millisecond || told > 401*time.Millisecond {
		t.Errorf("a tenth of a second at pace 4 tells the behaviours %v, want 400ms of the sky's time", told)
	}
}

func TestWeather_FollowsTheDayMovedOnWhileItStands(t *testing.T) {
	r := weatherOf(t, twoStates(), 3)
	r.set.Add(control.Nobody, Set{Name: "cloudy"}) // a minute long: the jump stays within it
	r.tick(time.Millisecond)
	r.pace(1, true)
	for r.sys.days.All(); r.sys.days.Next(); {
		r.move(&r.sys.day.Slice(r.sys.days.Cursor())[0], 1.0/48) // half an hour on: five seconds of a 4-minute day
	}
	left := r.now().Left
	r.tick(time.Millisecond)
	if got := left - r.now().Left; got < 4.9 || got > 5.1 {
		t.Errorf("the day moved half an hour on while it stood moved the weather %vs on, want the 5 it is", got)
	}
}

func TestWeather_RainsLessInTheZonesDrySeason(t *testing.T) {
	r := weatherOf(t, Config{Zone: Tropical}, 3)
	rain := weather.Default[2]
	if wet, dry := r.sys.likely(rain, sky.Summer), r.sys.likely(rain, sky.Winter); dry >= wet/3 {
		t.Errorf("in the tropics rain comes %v in summer and %v in winter, want the winter dry", wet, dry)
	}
	if clear := weather.Default[0]; r.sys.likely(clear, sky.Winter) != 1 {
		t.Errorf("a dry weather comes %v in the dry season, want as likely as ever", r.sys.likely(clear, sky.Winter))
	}
}
