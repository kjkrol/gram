package climate

import (
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/act"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/ceremony/cond"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/clock"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugins/atmosphere/air"
	"github.com/kjkrol/gram/plugins/atmosphere/calendar"
	"github.com/kjkrol/gram/plugins/atmosphere/climate/weather"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

func near(a, b, tolerance float32) bool { return math.Abs(float64(a-b)) <= float64(tolerance) }

type rig struct {
	w      *world.Plugin
	clk    *clock.Clock
	c      *Climate
	sys    *weatherSystem
	ecs    *goke.ECS
	winter heards // the world's, told by a rule of the weather in a winter
}

type winterNow struct{}

func (winterNow) Command() {}

type heards struct{ command.Queue[winterNow] }

func (h *heards) Queues() []command.CommandQueue     { return []command.CommandQueue{&h.Queue} }
func (h *heards) DefaultBindings() []control.Binding { return nil }

func weatherOf(t *testing.T, cfg Config, season calendar.Season) *rig {
	t.Helper()
	return loaded(t, cfg, season, "")
}

// a weather read back from the save at path, or a fresh one when path is empty; over the ground
// the first of ground gives, if any
func loaded(t *testing.T, cfg Config, season calendar.Season, path string,
	ground ...func() (low, peaks float32)) *rig {
	t.Helper()
	return sized(t, cfg, season, path, 640, 480, ground...)
}

// a weather over a world width by height
func sized(t *testing.T, cfg Config, season calendar.Season, path string, width, height uint32,
	ground ...func() (low, peaks float32)) *rig {
	t.Helper()
	r := &rig{w: world.NewPlugin(world.Config{Space: world.SpaceCfg{Width: width, Height: height},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 8}}),
		clk: clock.New(clock.Config{})}
	cal := calendar.New(r.clk, calendar.Config{DayRealTime: 4 * time.Minute, Season: season})
	r.c = New(r.w, cal, cfg)
	if len(ground) > 0 {
		r.c.Over(ground[0])
	}
	if err := r.w.Carry(&r.winter); err != nil {
		t.Fatal(err)
	}
	if err := r.c.Rules().Add(act.Branch[Weathering]{
		Name: "winter",
		When: cond.Self.If(cond.NewWord("winter", func(w Weathering) bool {
			return w.Season == calendar.Winter && w.Weather == r.c.Air()
		})),
		Then: act.Self.Do(winterNow{}),
	}); err != nil {
		t.Fatal(err)
	}
	r.sys = r.c.System().(*weatherSystem)
	r.ecs = goke.New()
	if path != "" {
		if err := r.ecs.Load(path, r.c.LoadComps()...); err != nil {
			t.Fatal(err)
		}
	}
	var run goke.Runnable
	r.ecs.Setup(goke.SystemFn{OnInit: func(*goke.SysInit) { run = r.ecs.RegSys(r.sys) }})
	r.ecs.SetPlan(func(ctx goke.RunCtx, d time.Duration) {
		ctx.Run(run, d)
		r.clk.Replay(ctx, d)
		ctx.Sync()
	})
	return r
}

func (r *rig) tick(d time.Duration) { r.ecs.Tick(d) }

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

func twoStates() Config {
	return Config{Weathers: []weather.State{
		{Name: "clear", Wind: [2]float32{10, 10}, Lasts: [2]time.Duration{time.Second, time.Second}, Next: map[string]float32{"cloudy": 1}},
		{Name: "cloudy", Wind: [2]float32{20, 20}, Clouds: [2]float32{0.8, 0.8}, Falls: 0.5, Lasts: [2]time.Duration{time.Minute, time.Minute}, Next: map[string]float32{"clear": 1}},
	}, Zone: Temperate, Start: "clear", Blend: time.Second}
}

func TestWeather_BeginsInItsStartLastsAndGoesOnAsItsWeightsSay(t *testing.T) {
	r := weatherOf(t, twoStates(), calendar.Summer)
	if w := r.now(); w.Lanes[0].State != unbegun {
		t.Fatalf("a fresh weather is %+v, want it not begun before its first step", w)
	}
	r.tick(time.Millisecond)
	if w := r.now(); w.Lanes[0].State != 0 || w.Over != 0 || w.Blow != 10 ||
		w.Lanes[0].Clouds != 0 {
		t.Fatalf("a begun weather is %+v, want its Start at its own wind and clouds", w)
	}
	r.tick(time.Second / 2)
	if air := r.c.Air(); !near(float32(math.Hypot(float64(air.Wind[0]), float64(air.Wind[1]))), 10, 0.01) {
		t.Errorf("the world's wind is %v, want the weather's 10", air.Wind)
	}
	r.run(time.Second)
	if w := r.now(); w.Lanes[0].State != 0 {
		t.Errorf("after its second the weather is in state %d, want it clear till the wind of 10 "+
			"has carried it across the 640 of the world", w.Lanes[0].State)
	}
	r.run(64 * time.Second)
	if w := r.now(); w.Lanes[0].State != 1 {
		t.Errorf("once carried across the world the weather is in state %d, want the clouds",
			w.Lanes[0].State)
	}
	r.run(3 * time.Second)
	w := r.now()
	if l := w.Lanes[0]; !near(l.Clouds, 0.8, 0.05) || !near(w.Blow, 20, 1) ||
		!near(l.Falls, 0.5, 0.05) {
		t.Errorf("three blends on the horizon brings %+v, want the clouds' 0.8, wind 20 and "+
			"rain 0.5", w)
	}
	if w.Over != 0 || w.Rain != 0 {
		t.Errorf("over the world the weather is %d raining %v, want it clear till the clouds come",
			w.Over, w.Rain)
	}
}

func TestWeather_ChangeAndSetGoOnAtOnceOverTheWorld(t *testing.T) {
	r := weatherOf(t, twoStates(), calendar.Summer)
	r.c.Change()
	r.tick(time.Millisecond)
	if w := r.now(); w.Lanes[0].State != 1 || w.Over != 1 {
		t.Errorf("after Change the horizon brings %d and the world has %d, want the next on both",
			w.Lanes[0].State, w.Over)
	}
	r.c.Set("clear")
	r.c.Set("fog")
	r.tick(time.Millisecond)
	if w := r.now(); w.Lanes[0].State != 0 || w.Over != 0 {
		t.Errorf("after Set clear (and an unknown fog) the state is %d over the world, want clear",
			w.Over)
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

func TestClimate_RunningLeavesOutWhatIsStopped(t *testing.T) {
	r := weatherOf(t, twoStates(), calendar.Summer)
	r.c.SetRunning(Running{})
	r.run(5 * time.Second)
	if w := r.now(); w.Lanes[0].State != 0 {
		t.Fatalf("the weather changes stopped, it went on to state %d", w.Lanes[0].State)
	}
	r.c.Set("cloudy")
	r.run(10 * time.Second)
	w, a := r.now(), r.c.Air()
	if w.Over != 1 || w.Lanes[0].Clouds < 0.5 || w.Rain < 0.3 {
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
	r.winter.Drain(func(i command.Issued[winterNow]) { told = append(told, i.Entity) })
	if len(told) != 2 || told[0] != r.w.Read(0).Clock().Entity() {
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
		seen[r.now().Lanes[0].State] = true
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
		if a.now().Lanes[0].State != b.now().Lanes[0].State {
			t.Fatal("two weathers of one seed went apart")
		}
	}
}

// a line each, as the panel lays a value's lines under its label
func TestReport_SaysTheWeatherTheWindAndTheSnow(t *testing.T) {
	r := weatherOf(t, twoStates(), calendar.Winter)
	rep := &report{names: []string{"clear", "cloudy"}, query: r.sys.query, now: r.sys.now}
	r.c.Set("cloudy")
	r.run(20 * time.Second)
	var got string
	rep.Report(func(label, value string) { got = label + ": " + value })
	lines := strings.Split(got, "\n")
	if len(lines) != 3 || lines[0] != "Weather: cloudy (snow)" ||
		!strings.HasPrefix(lines[1], "-") || !strings.HasSuffix(lines[1], "°C") ||
		!strings.HasPrefix(lines[2], "wind 20") {
		t.Errorf("the report says %q, want the clouds snowing, the frost and the wind of 20, a "+
			"line each", got)
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
		if w := r.now(); w.Lanes[0].State != want || w.Over != want {
			t.Errorf("begun in %v the weather is %d, want %d", season, w.Over, want)
		}
	}
	autumn := weatherOf(t, cfg, calendar.Autumn)
	autumn.tick(time.Millisecond)
	if w := autumn.now(); w.Lanes[0].Clouds != 0.8 || w.Rain != 0.5 ||
		autumn.c.Air().At(30, 400).Cover != 0.8 {
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

func TestWeather_GoesByTheStepsOfTheSimulation(t *testing.T) {
	r := weatherOf(t, twoStates(), calendar.Summer)
	r.c.Set("cloudy") // a minute long
	r.tick(time.Millisecond)
	left := r.now().Lanes[0].Left
	r.tick(5 * time.Second)
	if got := left - r.now().Lanes[0].Left; got < 4.9 || got > 5.1 {
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

// a moment of the world as a whole has no entity's components to read
func TestClimate_TakesARolesRule(t *testing.T) {
	w := world.NewPlugin(world.Config{Space: world.SpaceCfg{Width: 640, Height: 480}, Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 8}})
	cfg := calendar.Config{DayRealTime: time.Minute}
	c := New(w, calendar.New(clock.New(clock.Config{}), cfg), Config{})
	every := act.Branch[Weathering]{
		Name: "any weather",
		When: cond.Self.If(cond.Always),
		Then: act.Self.Do(winterNow{}),
	}
	for name, r := range map[string]act.Rule{
		"every":  every,
		"a role": ceremony.NewPart("climate sheltered", 0).Obeys(every).Rules()[0],
	} {
		if err := c.Rules().Add(r); err != nil {
			t.Errorf("%s: Add = %v; want it taken", name, err)
		}
	}
}

// the clouds come for ten minutes, long enough to cross the dome
func longClouds() Config {
	cfg := twoStates()
	cfg.Weathers[1].Lasts = [2]time.Duration{10 * time.Minute, 10 * time.Minute}
	cfg.Horizon = 2000
	return cfg
}

func TestWeather_ANewWeatherIsBornAtTheHorizonAndComesOverTheWorldWithTheWind(t *testing.T) {
	r := weatherOf(t, longClouds(), calendar.Summer)
	r.tick(time.Millisecond)
	for r.now().Lanes[0].State == 0 {
		r.tick(100 * time.Millisecond)
	}
	edge := r.sys.dome.Reach()[0]
	r.run(30 * time.Second)
	a := r.c.Air()
	// the wind blows along x, from the dome's west edge
	horizon, over := a.At(edge+200, 240).Cover, a.At(320, 240).Cover
	if horizon < 0.6 || over > 0.05 {
		t.Fatalf("30 s into the clouds they cover %v by the horizon and %v over the world, want "+
			"them come in where the wind blows from and the clear air over the world still",
			horizon, over)
	}
	came := 30 * time.Second
	for ; r.c.Air().At(30, 240).Cover < 0.6 && came < 20*time.Minute; came += time.Second {
		r.run(time.Second)
	}
	// the wind of 20 from the horizon, give or take its heading wandering
	if want := (30 - edge) / 20; came.Seconds() < float64(want)*0.8 ||
		came.Seconds() > float64(want)*1.25 {
		t.Errorf("the clouds came over the world's west edge %v after the horizon, want about %vs",
			came, want)
	}
	if w, down := r.now(), r.c.Air().At(600, 240).Cover; w.Over != 0 || down > 0.3 {
		t.Errorf("as the clouds come over the west edge the east is %v, the middle's weather %d; "+
			"want the clear air not gone yet", down, w.Over)
	}
	r.run(40 * time.Second)
	if w, down := r.now(), r.c.Air().At(600, 240).Cover; w.Over != 1 || down < 0.6 {
		t.Errorf("40 s on the east is %v, the middle's weather %d, want the clouds over all of it",
			down, w.Over)
	}
}

func TestWeather_ASetWeatherSettlesOverTheWorldAndUpwindTheOldSailsOff(t *testing.T) {
	r := weatherOf(t, longClouds(), calendar.Summer)
	r.tick(time.Millisecond)
	r.c.Set("cloudy")
	r.tick(time.Millisecond)
	if l := r.now().Lanes[0]; l.Clouds != l.CloudsTo || l.Falls != 0.5 {
		t.Errorf("set cloudy, the horizon brings %+v, want the cloudy weather at once", l)
	}
	r.run(2 * time.Second)
	if c := r.c.Air().At(320, 240).Cover; c < 0.35 {
		t.Errorf("2 s after cloudy was set the world's middle is covered %v, want it well on "+
			"the way to 0.8", c)
	}
	r.run(8 * time.Second)
	a := r.c.Air()
	for _, x := range []float32{-1500, 30, 320, 600} {
		if c := a.At(x, 240).Cover; !near(c, 0.8, 0.05) {
			t.Errorf("10 s after cloudy was set the cover at %v is %v, want 0.8 over the world "+
				"and upwind of it", x, c)
		}
	}
	if c := a.At(1500, 240).Cover; c > 0.05 {
		t.Errorf("downwind past the world the cover is %v, want the clear air sailing off", c)
	}
	r.run(60 * time.Second)
	if w, c := r.now(), r.c.Air().At(320, 240).Cover; w.Over != 1 || !near(c, 0.8, 0.05) {
		t.Errorf("a minute on the world has %d covered %v, want the set weather kept, the horizon "+
			"bringing it in", w.Over, c)
	}
}

func TestWeather_WithoutWindANewWeatherSettlesWhereItIs(t *testing.T) {
	r := weatherOf(t, twoStates(), calendar.Summer)
	r.c.SetRunning(Running{Changes: true, Clouds: true, Falls: true})
	r.tick(time.Millisecond)
	r.run(70 * time.Second) // the clear sky's minute, as long as the wind of 10 crosses 640
	for _, x := range []float32{-5000, 60, 600, 5000} {
		if c := r.c.Air().At(x, 240).Cover; !near(c, 0.8, 0.05) {
			t.Errorf("without wind the clouds that came cover %v at %v, want 0.8 everywhere", c, x)
		}
	}
	if w := r.now(); w.Drift != ([2]float32{}) || w.Over != 1 {
		t.Errorf("without wind the clouds drifted %v and the world has %d, want them still and "+
			"cloudy", w.Drift, w.Over)
	}
}

func TestWeather_APatchyClimateBringsWeathersLaneByLane(t *testing.T) {
	cfg := Config{Weathers: []weather.State{
		{Name: "clear", Wind: [2]float32{30, 30},
			Lasts: [2]time.Duration{time.Minute, time.Minute}},
		{Name: "cloudy", Wind: [2]float32{30, 30}, Clouds: [2]float32{0.9, 0.9},
			Lasts: [2]time.Duration{time.Minute, time.Minute}},
	}, Zone: Temperate, Blend: time.Second, Seed: 7}
	for _, patchy := range []bool{false, true} {
		cfg.Patchy = patchy
		r := weatherOf(t, cfg, calendar.Summer)
		r.tick(time.Millisecond)
		states := map[int32]bool{}
		for range 30 {
			r.run(time.Minute)
			w := r.now()
			r.sys.each(func(k int) { states[w.Lanes[lane(k)].State] = true })
		}
		lanes := 2*r.sys.lanes + 1
		if patchy && (lanes < 5 || len(states) != 2) {
			t.Errorf("patchy, the horizon has %d lanes bringing %v, want several and both weathers",
				lanes, states)
		}
		covers := map[bool]bool{}
		edge := r.sys.dome.Reach()
		for y := edge[1]; y < edge[3]; y += (edge[3] - edge[1]) / 40 {
			covers[r.c.Air().At(edge[0], y).Cover > 0.45] = true
		}
		if patchy != (len(covers) == 2) {
			t.Errorf("patchy %v, along the horizon the sky is cloudy %v", patchy, covers)
		}
	}
}

func TestWeather_TheMassesOutlastASave(t *testing.T) {
	path := t.TempDir() + "/weather.save"
	r := weatherOf(t, longClouds(), calendar.Summer)
	r.tick(time.Millisecond)
	for r.now().Lanes[0].State == 0 {
		r.tick(100 * time.Millisecond)
	}
	r.run(2 * time.Minute)
	r.ecs.Pause()
	if err := r.ecs.Save(path); err != nil {
		t.Fatal(err)
	}
	r.ecs.Resume()
	back := loaded(t, longClouds(), calendar.Summer, path)
	if !back.sys.kept || len(back.sys.stale) != 0 {
		t.Fatalf("read back, the dome kept its patches %v, stale %d", back.sys.kept,
			len(back.sys.stale))
	}
	r.tick(time.Millisecond)
	back.tick(time.Millisecond)
	edge := r.sys.dome.Reach()
	for x := edge[0]; x < edge[2]; x += 150 {
		if a, b := r.c.Air().At(x, 240), back.c.Air().At(x, 240); !near(a.Cover, b.Cover, 1e-3) {
			t.Errorf("at %v the cover is %v, read back from a save %v", x, a.Cover, b.Cover)
		}
	}
	// a sky reaching less far keeps the levels of the save it has, the others gone; one reaching
	// further is laid anew
	for _, horizon := range []float32{100, 60000} {
		other := longClouds()
		other.Horizon = horizon
		r := loaded(t, other, calendar.Summer, path)
		if less := horizon < 2000; r.sys.kept != less || less != (len(r.sys.stale) > 0) {
			t.Errorf("read back into a sky reaching %v the dome kept its patches %v, stale %d",
				horizon, r.sys.kept, len(r.sys.stale))
		}
		r.tick(time.Millisecond)
		r.tick(time.Millisecond)
		n := 0
		for r.sys.patches.All(); r.sys.patches.Next(); {
			n += len(r.sys.patch.Slice(r.sys.patches.Cursor()))
		}
		if n != len(r.sys.dome.Slots()) {
			t.Errorf("the sky reaching %v has %d patches, want its dome's %d", horizon, n,
				len(r.sys.dome.Slots()))
		}
	}
}

func TestWeather_IsAComponentOfFixedSizeWithoutPointers(t *testing.T) {
	var check func(t reflect.Type, path string)
	check = func(ty reflect.Type, path string) {
		switch ty.Kind() {
		case reflect.Struct:
			for i := range ty.NumField() {
				check(ty.Field(i).Type, path+"."+ty.Field(i).Name)
			}
		case reflect.Array:
			check(ty.Elem(), path+"[]")
		case reflect.Pointer, reflect.Slice, reflect.Map, reflect.Interface, reflect.String,
			reflect.Chan, reflect.Func:
			t.Errorf("%s is a %v", path, ty.Kind())
		}
	}
	check(reflect.TypeFor[Weather](), "Weather")
}

func TestWeather_HangsTheLowCloudsOverTheGround(t *testing.T) {
	over := loaded(t, twoStates(), calendar.Summer, "", func() (low, peaks float32) {
		return 20, 1880
	})
	over.tick(time.Millisecond)
	relief := air.ReliefOf(world.Scale{}, 20, 1880)
	if got := over.c.Air(); got.Relief != relief ||
		got.Bases != air.BasesOver(world.Scale{}, relief) {
		t.Errorf("over ground 20 up to peaks 1880 up the low clouds hang over %v, the layers' "+
			"bases %v; want %v and %v", got.Relief, got.Bases, relief,
			air.BasesOver(world.Scale{}, relief))
	}
	none := weatherOf(t, twoStates(), calendar.Summer)
	none.tick(time.Millisecond)
	if got := none.c.Air().Relief; got != [2]float32{0, air.LeastRelief} {
		t.Errorf("over no ground in particular the low clouds hang over %v, want [0 %v]", got,
			air.LeastRelief)
	}
}

func TestWeather_TheLowCloudsBandsComeToTheWeathersShares(t *testing.T) {
	cfg := twoStates()
	cfg.Weathers[0].Ceilings = [3]float32{2, 0, 0}
	cfg.Weathers[1].Ceilings = [3]float32{0, 1, 3}
	r := weatherOf(t, cfg, calendar.Summer)
	r.tick(time.Millisecond)
	if got := r.c.Air().Ceilings; got != [3]float32{1, 0, 0} {
		t.Fatalf("begun clear, the low clouds' bands share %v, want the clear's [1 0 0]", got)
	}
	r.c.Set("cloudy")
	r.run(time.Second)
	half := r.c.Air().Ceilings
	r.run(9 * time.Second)
	got := r.c.Air().Ceilings
	if half[0] < 0.1 || half[0] > 0.9 || !near(got[0], 0, 0.02) || !near(got[1], 0.25, 0.02) ||
		!near(got[2], 0.75, 0.02) {
		t.Errorf("set cloudy, the bands share %v after a second and %v after ten, want them "+
			"going over to the cloudy's [0 0.25 0.75]", half, got)
	}
}

func TestWeather_TheMassesCarryTheirWeathersDamp(t *testing.T) {
	cfg := twoStates()
	cfg.Weathers[0].Damp = 0.25
	cfg.Weathers[1].Damp = 0.75
	r := weatherOf(t, cfg, calendar.Summer)
	r.tick(time.Millisecond)
	if got := r.c.Air().At(320, 240).Damp; !near(got, 0.25, 1e-3) {
		t.Fatalf("begun clear, the air over the world is %v damp, want the clear's 0.25", got)
	}
	r.c.Set("cloudy")
	r.run(10 * time.Second)
	if got := r.c.Air().At(320, 240).Damp; !near(got, 0.75, 0.02) {
		t.Errorf("set cloudy, the air over the world is %v damp after 10 s, want the cloudy's "+
			"0.75", got)
	}
}

// the shares of the world 6400 by 3200 under rain (falling at least half the rain's), under a
// storm (falling more than the rain does), and fair (a thin cover, nothing falling)
func (r *rig) shares() (rain, storm, fair float32) {
	n := float32(0)
	for x := float32(25); x < 6400; x += 50 {
		for y := float32(25); y < 3200; y += 50 {
			m := r.c.Air().At(x, y)
			n++
			switch {
			case m.Falls > 0.8:
				storm++
				rain++
			case m.Falls > 0.3:
				rain++
			case m.Cover < 0.3:
				fair++
			}
		}
	}
	return rain / n, storm / n, fair / n
}

func TestWeather_RainAndStormsComeInPlacesNeverOverTheWholeWorld(t *testing.T) {
	for _, name := range []string{weather.Rain, weather.Storm} {
		// as wide as a map of fifty of the air's cells, which the places' patches span many of
		r := sized(t, Config{Start: weather.Fair, Seed: 3}, calendar.Summer, "", 6400, 3200)
		r.tick(time.Millisecond)
		r.c.Set(name)
		r.run(12 * time.Second)
		for range 3 {
			rain, storm, fair := r.shares()
			if rain > 0.25+0.01 || storm > 0.125+0.01 || fair > 0.25+0.01 || rain < 0.2 {
				t.Errorf("%s: rain on %.2f of the world, a storm on %.2f, fair on %.2f; want "+
					"rain on a quarter at most, a storm on an eighth, fair on a quarter", name,
					rain, storm, fair)
			}
			if name == weather.Rain && storm > 0 {
				t.Errorf("rain alone: a storm on %.2f of the world, want none", storm)
			}
			r.run(5 * time.Second) // the wind carries the places on
		}
	}
}

func TestWeather_CloudsLeaveFairBreaksAndNoRain(t *testing.T) {
	r := sized(t, Config{Start: weather.Fair, Seed: 5}, calendar.Summer, "", 6400, 3200)
	r.tick(time.Millisecond)
	r.c.Set(weather.Cloudy)
	r.run(12 * time.Second)
	if rain, _, fair := r.shares(); rain > 0 || fair > 0.25+0.01 {
		t.Errorf("cloudy: rain on %.2f of the world, fair on %.2f; want no rain and fair on a "+
			"quarter at most", rain, fair)
	}
}
