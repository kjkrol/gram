package atmosphere

import (
	"math"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/game/stage"
	"github.com/kjkrol/gram/internal/engine"
	"github.com/kjkrol/gram/plugins/atmosphere/calendar"
	"github.com/kjkrol/gram/plugins/atmosphere/climate"
	"github.com/kjkrol/gram/plugins/atmosphere/climate/weather"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/render/gpu"
	"github.com/kjkrol/gram/ui"
)

var hudScreen = geom.NewAABB(geom.NewVec(0, 0), geom.NewVec(1600, 900))

// an atmosphere of cfg over a world, its Stage's one scene screen's, a tick a sixtieth of a second
func hudStage(t *testing.T, cfg Config,
	screen func(a *Plugin) *ui.Element) (a *Plugin, scene *ui.Scene, tick func(n int)) {
	t.Helper()
	var w *world.Plugin
	var ecs *goke.ECS
	st := stage.New("hud").
		World(world.Config{
			Space:    world.SpaceCfg{Width: 128, Height: 128},
			Entities: world.EntitiesCfg{MaxCount: 4, MinSize: 1, MaxSize: 20},
			Heights:  true,
		}).
		Plugins(func(made *world.Plugin) { w, a = made, NewPlugin(made, nil, cfg) }).
		Effects(func(ctx game.Initializer) { ecs = ctx.ECS() }).
		Scenes(func() []game.Scene {
			scene = ui.NewScene("hud", screen(a))
			return []game.Scene{scene}
		}).
		Update(func(ctx goke.RunCtx, d time.Duration) {
			w.RunPlan(ctx, d)
			a.RunPlan(ctx, d)
		})
	if err := engine.NewEngine(single{stage: st}).Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	tick = func(n int) {
		for range n {
			ecs.Tick(time.Second / 60)
		}
	}
	return a, scene, tick
}

func width(e *ui.Element) float64 { return e.Box().BottomRight.X - e.Box().TopLeft.X }

func TestTimeOfDay_FollowsTheCalendarsClock(t *testing.T) {
	// a game minute a second
	cfg := Config{Calendar: calendar.Config{DayRealTime: 24 * time.Minute}}
	a, _, tick := hudStage(t, cfg, func(a *Plugin) *ui.Element { return a.TimeOfDay() })
	words := &hourWords{p: a}
	if got, _ := words.Text(0, false); got != "08:00" {
		t.Errorf("as the game begins the hour is %q, want 08:00", got)
	}
	tick(90)
	if got, _ := words.Text(0, false); got != "08:01" {
		t.Errorf("a game minute and a half on the hour is %q, want 08:01", got)
	}
}

func TestDate_NamesTheSeasonAndTheDate(t *testing.T) {
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 128, Height: 128},
		Entities: world.EntitiesCfg{MaxCount: 4, MinSize: 1, MaxSize: 20},
		Heights:  true,
	})
	a := NewPlugin(w, nil, Config{Calendar: calendar.Config{
		Year:   calendar.EarthYear,
		Season: calendar.Summer,
	}})
	if got, _ := (&dateWords{p: a, icon: ui.Blank()}).Text(0, false); got != "summer, 3 August" {
		t.Errorf("in the middle of an Earth's summer the date says %q, want summer, 3 August",
			got)
	}
}

func TestDate_ShowsTheMoonBesideTheWordsWhileTheMoonRuns(t *testing.T) {
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 128, Height: 128},
		Entities: world.EntitiesCfg{MaxCount: 4, MinSize: 1, MaxSize: 20},
		Heights:  true,
	})
	a := NewPlugin(w, nil, Config{})
	date := a.Date()
	scene := ui.NewScene("hud", ui.TopLeft(date))
	scene.Lay(hudScreen)
	with := width(date)
	r := a.Running()
	r.Moon = false
	a.SetRunning(r)
	scene.Lay(hudScreen)
	line := float64(render.DefaultFont().LineHeight())
	if without := width(date); with-without != line+6 {
		t.Errorf("the date is %v wide with the moon running and %v without, want the icon's %v "+
			"between", with, without, line+6)
	}
}

func TestWeather_NamesTheWeatherSetAndIsLeftOutWithoutAClimate(t *testing.T) {
	words := &weatherWords{}
	cfg := Config{Climate: climate.Config{Start: weather.Clear}}
	a, _, tick := hudStage(t, cfg, func(a *Plugin) *ui.Element {
		words.p = a
		return ui.Drawer(words)
	})
	tick(1)
	if got, ok := words.Text(0, false); !ok || !strings.HasPrefix(got, "clear · ") ||
		!strings.Contains(got, "°C · wind ") {
		t.Errorf("the weather begun clear says %q", got)
	}
	a.Climate().Set(weather.Rain)
	tick(1)
	if got, _ := words.Text(0, false); !strings.HasPrefix(got, "rain") {
		t.Errorf("the weather set to rain says %q", got)
	}
	a.SetRunning(Running{Day: true, Stars: true, Moon: true})
	if got, ok := words.Text(0, false); ok {
		t.Errorf("with none of the climate's workings running the weather says %q, want nothing",
			got)
	}
}

func TestBar_IsAsWideAsTheWidgetsItHoldsSideBySide(t *testing.T) {
	var bar, hour, date, weather *ui.Element
	a, scene, tick := hudStage(t, Config{}, func(a *Plugin) *ui.Element {
		bar, hour, date, weather = a.Bar(), a.TimeOfDay(), a.Date(), a.Weather()
		return ui.Layers(ui.TopLeft(bar), ui.TopLeft(hour), ui.TopLeft(date), ui.TopLeft(weather))
	})
	tick(1)
	scene.Lay(hudScreen)
	if three := width(hour) + width(date) + width(weather); width(weather) == 0 ||
		width(bar) != three {
		t.Errorf("the bar is %v wide, the hour, the date and the weather %v, %v and %v",
			width(bar), width(hour), width(date), width(weather))
	}
	a.SetRunning(Running{Day: true, Stars: true, Moon: true})
	scene.Lay(hudScreen)
	if two := width(hour) + width(date); width(weather) != 0 || width(bar) != two {
		t.Errorf("without a climate the bar is %v wide, the hour and the date %v and %v, the "+
			"weather %v; want it left out", width(bar), width(hour), width(date), width(weather))
	}
}

func needGPU(t *testing.T) {
	t.Helper()
	if err := gpu.Headless(os.Getenv("GRAM_GPU") == "software"); err != nil {
		t.Skipf("no GPU: %v", err)
	}
}

// the mean red of the left and the right of the moon drawn in a square box, the rim and the
// terminator at the quarters left out
func halves(t *testing.T, moon float32) (left, right float64) {
	t.Helper()
	icon := &moonIcon{moon: func() float32 { return moon }}
	icon.Resize(32, 32)
	img := icon.Draw()
	b := img.Bounds()
	pix := make([]byte, 4*b.Dx()*b.Dy())
	img.ReadPixels(pix)
	mid := float64(b.Dx()) / 2
	var sum [2]float64
	var n [2]int
	for y := range b.Dy() {
		for x := range b.Dx() {
			dx, dy := float64(x)+0.5-mid, float64(y)+0.5-mid
			if math.Hypot(dx, dy) > mid-4 || math.Abs(dx) < 3 {
				continue
			}
			side := 0
			if dx > 0 {
				side = 1
			}
			sum[side] += float64(pix[4*(y*b.Dx()+x)])
			n[side]++
		}
	}
	return sum[0] / float64(n[0]), sum[1] / float64(n[1])
}

func TestMoonIcon_LightsTheSideOfItsPhaseAsTheNorthSeesIt(t *testing.T) {
	needGPU(t)
	darkLeft, darkRight := halves(t, 0)
	for _, c := range []struct {
		moon        float32
		left, right bool
	}{
		{moon: 0.25, right: true},
		{moon: 0.5, left: true, right: true},
		{moon: 0.75, left: true},
	} {
		left, right := halves(t, c.moon)
		if lit := left > 2*darkLeft; lit != c.left {
			t.Errorf("at %v the left is %.0f against %.0f new: lit %v, want %v", c.moon, left,
				darkLeft, lit, c.left)
		}
		if lit := right > 2*darkRight; lit != c.right {
			t.Errorf("at %v the right is %.0f against %.0f new: lit %v, want %v", c.moon, right,
				darkRight, lit, c.right)
		}
	}
}

func TestMoonIcon_StandsAtTheRightOfABoxWiderThanTall(t *testing.T) {
	needGPU(t)
	icon := &moonIcon{moon: func() float32 { return 0.5 }}
	icon.Resize(40, 32)
	img := icon.Draw()
	b := img.Bounds()
	pix := make([]byte, 4*b.Dx()*b.Dy())
	img.ReadPixels(pix)
	alpha := func(x, y int) byte { return pix[4*(y*b.Dx()+x)+3] }
	if a := alpha(b.Dx()-b.Dy()/2, b.Dy()/2); a != 255 {
		t.Errorf("the middle of the disc a half of the height off the right is %v opaque", a)
	}
	for x := range b.Dx() - b.Dy() {
		if a := alpha(x, b.Dy()/2); a != 0 {
			t.Fatalf("at %v, left of the disc, the icon is %v opaque, want bare", x, a)
		}
	}
}
