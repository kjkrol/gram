package minimap

import (
	"image/color"
	"math"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/ceremony/host"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/internal/parallel"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/sector"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/render/gputest"
)

func near(a, b float32) bool { return math.Abs(float64(a-b)) < 1e-3 }

func TestLens_LaysItsMiddleInThePicturesAndAheadUpIt(t *testing.T) {
	l := newLens(geom.NewVec(1000, 1000), false, false, 100)
	l.SetViewport(200, 200)
	for _, c := range []struct {
		ax, ay   float32
		x, y     float32
		sx, sy   float32
		ahead    string
		thatWays string
	}{
		{0, -1, 500, 450, 100, 50, "north", "50 north"},
		{1, 0, 550, 500, 100, 50, "east", "50 east"},
		{1, 0, 500, 550, 150, 100, "east", "50 south"},
	} {
		l.aim(500, 500, c.ax, c.ay)
		if sx, sy := l.Project(500, 500, 0); !near(sx, 100) || !near(sy, 100) {
			t.Errorf("ahead %s: the middle lies at (%v, %v), want (100, 100)", c.ahead, sx, sy)
		}
		sx, sy := l.Project(c.x, c.y, 0)
		if !near(sx, c.sx) || !near(sy, c.sy) {
			t.Errorf("ahead %s: %s lies at (%v, %v), want (%v, %v)", c.ahead, c.thatWays, sx, sy,
				c.sx, c.sy)
		}
		if x, y := l.Unproject(sx, sy, 0); !near(x, c.x) || !near(y, c.y) {
			t.Errorf("ahead %s: (%v, %v) back to the world is (%v, %v)", c.ahead, sx, sy, x, y)
		}
	}
}

func TestLens_TakesTheShorterWayRoundATorus(t *testing.T) {
	l := newLens(geom.NewVec(1000, 1000), true, true, 100)
	l.SetViewport(200, 200)
	l.aim(990, 500, 0, -1)
	if sx, _ := l.Project(10, 500, 0); !near(sx, 120) {
		t.Errorf("20 east across the seam lies at x %v, want 120", sx)
	}
	if !l.Visible(geom.NewAABB(geom.NewVec(0, 490), geom.NewVec(20, 510))) {
		t.Error("a box across the seam is not visible")
	}
}

// a camera turned to look ahead
type turned struct {
	camera.Camera
	ax, ay float32
}

func (c turned) Ahead() (float32, float32) { return c.ax, c.ay }

func TestMinimap_FollowsItsCameraTurningWithItOnlyWhenAsked(t *testing.T) {
	w := flatWorld(nil)
	p := carried(t, w, NewPlugin(w, nil, Config{Reach: 200}))
	cam := cameratest.NewFromSpace(1000, 1000, 0)
	cam.SetViewport(200, 200)
	cam.CenterOn(300, 700, 0)
	p.For(turned{cam, 1, 0}, command.Nobody)
	m := p.maps[0]
	m.lens.SetViewport(100, 100)
	p.RunPlan(nil, 0)
	if at := m.shown(); at != (aim{300, 700, 0, -1}) {
		t.Errorf("north up the minimap looks %v, want over (300, 700) north", at)
	}
	if m.lens.scale != 0.25 {
		t.Errorf("a picture 100 across reaching 200 is %v a unit, want 0.25", m.lens.scale)
	}
	w.Write().Carrier().Put(command.Nobody, Turn{On: true})
	p.RunPlan(nil, 0)
	if at := m.shown(); !p.read.Turning() || at.ax != 1 || at.ay != 0 {
		t.Errorf("turning, the minimap looks %v, want ahead east as the camera", at)
	}
}

func TestMinimap_IsDrawnLookingWhereItWasPublished(t *testing.T) {
	w := flatWorld(nil)
	p := carried(t, w, NewPlugin(w, nil, Config{Reach: 200}))
	cam := cameratest.NewFromSpace(1000, 1000, 0)
	cam.SetViewport(200, 200)
	cam.CenterOn(300, 700, 0)
	p.For(cam, command.Nobody)
	m := p.maps[0]
	m.lens.SetViewport(100, 100)
	m.marks.publish(time.Now(), true)
	m.marks.swap()
	cam.CenterOn(500, 500, 0)
	p.RunPlan(nil, 0)
	if at := m.shown(); at.cx != 300 || at.cy != 700 {
		t.Errorf("moved after the minimap was published, it is drawn over (%v, %v), want "+
			"(300, 700) till the next", at.cx, at.cy)
	}
	m.marks.publish(time.Now().Add(redraw), true)
	m.marks.swap()
	if at := m.shown(); at.cx != 500 || at.cy != 500 {
		t.Errorf("published again, it is drawn over (%v, %v), want (500, 500)", at.cx, at.cy)
	}
}

func TestMinimap_SwitchShowsAndHidesItsPicture(t *testing.T) {
	w := flatWorld(nil)
	p := carried(t, w, NewPlugin(w, nil, Config{Reach: 200}))
	e := p.For(cameratest.NewFromSpace(1000, 1000, 0), command.Nobody).Size(100, 100)
	for _, want := range []bool{false, true} {
		w.Write().Carrier().Put(command.Nobody, Switch{Shown: want})
		p.RunPlan(nil, 0)
		if p.read.Shown() != want {
			t.Errorf("switched, shown %v, want %v", p.read.Shown(), want)
		}
	}
	p.write.Show(false)
	if e.Hits(e.Box().TopLeft) {
		t.Error("hidden, the minimap's picture is hit")
	}
}

// p with its commands carried by w's carrier, as the players have them
func carried(t *testing.T, w *world.Plugin, p *Plugin) *Plugin {
	t.Helper()
	if err := w.Carry(p); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestMinimap_RefusesAReachPastTheLastBand(t *testing.T) {
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 4096, Height: 512},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 5, MaxSize: 20},
		Sectors:  world.SectorsCfg{Bands: []sector.Band{{Reach: 600}}},
	})
	NewPlugin(w, nil, Config{Reach: 600})
	defer func() {
		if recover() == nil {
			t.Error("a reach of 700 past the band of 600 was taken")
		}
	}()
	NewPlugin(w, nil, Config{Reach: 700})
}

// a handler taking the cameras' CenterAt alone
type centring struct {
	q command.Queue[cameras.CenterAt]
}

func (c *centring) Queues() []command.CommandQueue     { return []command.CommandQueue{&c.q} }
func (c *centring) DefaultBindings() []control.Binding { return nil }

func TestMinimap_AClickMovesItsCameraToTheWorldPointUnderIt(t *testing.T) {
	w := flatWorld(nil)
	got := &centring{}
	if err := w.Carry(got); err != nil {
		t.Fatal(err)
	}
	p := NewPlugin(w, nil, Config{Reach: 200})
	cam := cameratest.NewFromSpace(1000, 1000, 0)
	cam.SetViewport(200, 200)
	cam.CenterOn(500, 500, 0)
	p.For(cam, command.Nobody)
	p.maps[0].lens.SetViewport(200, 200)
	p.RunPlan(nil, 0)
	// 50 right and 50 up of the middle, two world units a pixel
	(&clicks{m: p.maps[0]}).Click(geom.NewVec(150, 50))
	var at []cameras.CenterAt
	got.q.Drain(func(i command.Issued[cameras.CenterAt]) { at = append(at, i.Command) })
	if len(at) != 1 || at[0].Camera != cam || at[0].X != 600 || at[0].Y != 400 {
		t.Errorf("the click gave %+v, want the camera to (600, 400)", at)
	}
}

func TestOutline_IsWhatTheCameraShowsOfTheGround(t *testing.T) {
	w := flatWorld(nil)
	p := NewPlugin(w, nil, Config{Reach: 200})
	cam := cameratest.NewFromSpace(1000, 1000, 0)
	cam.SetViewport(200, 100)
	cam.CenterOn(500, 500, 0)
	p.For(cam, command.Nobody)
	m := p.maps[0]
	m.lens.SetViewport(200, 200)
	p.RunPlan(nil, 0)
	var f render.Frame
	f.Reset(m.lens)
	(&outlineView{m: m}).Compose(&f, m.lens)
	// the camera shows 400 to 600 across, 450 to 550 down: on the minimap half a pixel a unit
	var xs, ys []float32
	f.Each(func(_ render.At, v []render.Vertex) {
		for _, p := range v {
			xs = append(xs, p.DstX)
			ys = append(ys, p.DstY)
		}
	})
	// a line's quad runs a pixel and its width past its ends, to smooth its edges
	within := func(got, want float32) bool { return math.Abs(float64(got-want)) <= 2 }
	if len(xs) == 0 || !within(minOf(xs), 50) || !within(maxOf(xs), 150) ||
		!within(minOf(ys), 75) || !within(maxOf(ys), 125) {
		t.Errorf("the outline spans x %v..%v, y %v..%v; want 50..150 and 75..125", minOf(xs),
			maxOf(xs), minOf(ys), maxOf(ys))
	}
}

func minOf(v []float32) float32 {
	m := float32(math.Inf(1))
	for _, x := range v {
		m = min(m, x)
	}
	return m
}

func maxOf(v []float32) float32 {
	m := float32(math.Inf(-1))
	for _, x := range v {
		m = max(m, x)
	}
	return m
}

type spot struct{ x, y float64 }

var (
	markRed  = color.RGBA{R: 255, A: 255}
	plainSky = color.RGBA{B: 255, A: 255}
)

// a flat world 1000 a side with a marked kind and an unmarked one, define given the world
func flatWorld(define func(w *world.Plugin)) *world.Plugin {
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 1000, Height: 1000},
		Entities: world.EntitiesCfg{MaxCount: 8, MinSize: 4, MaxSize: 4},
	})
	if define != nil {
		define(w)
	}
	return w
}

func TestMarks_ADotForEveryEntityRoundWhoseKindHasAMark(t *testing.T) {
	w := flatWorld(nil)
	for _, name := range []string{"marked", "plain"} {
		kind.Define[spot](w.Kinds(), name, kind.Spec{
			comp.Load(func(s spot) world.Position {
				return world.Position{AABB: plane.NewAABB(geom.NewVec(s.x-2, s.y-2), 4, 4)}
			}),
			comp.Const(world.Velocity{}),
		})
	}
	marked := kind.Named[spot](w.Kinds(), "marked")
	plain := kind.Named[spot](w.Kinds(), "plain")
	w.Write().Seed(marked.Entry(spot{450, 500}), marked.Entry(spot{550, 500}), plain.Entry(spot{500,
		500}),
		marked.Entry(spot{900, 900}))
	a := w.NewAtlas()
	a.Add(marked, 4, render.Solid(markRed)).Styled(Mark{Color: markRed, Size: 6})
	a.Add(plain, 4, render.Solid(plainSky))
	a.Close()
	if err := w.Populate(); err != nil {
		t.Fatal(err)
	}
	p := NewPlugin(w, nil, Config{Reach: 100})
	cam := cameratest.NewFromSpace(1000, 1000, 0)
	cam.SetViewport(200, 200)
	cam.CenterOn(500, 500, 0)
	p.For(cam, command.Nobody)
	m := p.maps[0]
	m.lens.SetViewport(200, 200)
	marks := &marksView{m: m}
	ecs := start(t, w, goke.SystemFn{OnInit: marks.Init})
	ecs.Tick(time.Second / 60)
	p.RunPlan(nil, 0)
	marks.publish(time.Now(), true)
	marks.swap()
	var f render.Frame
	f.Reset(m.lens)
	marks.Compose(&f, m.lens)
	var dots [][2]float32
	f.Each(func(_ render.At, v []render.Vertex) {
		var x, y float32
		for _, p := range v {
			x += p.DstX / float32(len(v))
			y += p.DstY / float32(len(v))
		}
		if v[0].ColorR < 0.99 || v[0].ColorB > 0.01 {
			t.Errorf("a dot coloured %v, want the mark's red", v[0])
		}
		dots = append(dots, [2]float32{x, y})
	})
	// a unit a pixel round (500, 500): the marked at 450 and 550, neither the plain nor the far
	if len(dots) != 2 || !(near(dots[0][0], 50) && near(dots[1][0], 150) ||
		near(dots[0][0], 150) && near(dots[1][0], 50)) || !near(dots[0][1], 100) {
		t.Errorf("dots at %v, want two, at (50, 100) and (150, 100)", dots)
	}
}

type installCtx struct {
	ecs     *goke.ECS
	pending []func() []goke.System
}

func (c *installCtx) UseModule(m goke.Module) {
	regSys := goke.SystemFn{OnInit: func(*goke.SysInit) { m.RegSystems(c.ecs) }}
	c.pending = append(c.pending, func() []goke.System { return append(m.SetupSystems(), regSys) })
}
func (c *installCtx) Setup(providers ...goke.SetupProvider) {
	for _, p := range providers {
		c.pending = append(c.pending, p.SetupSystems)
	}
}
func (c *installCtx) RegSys(factory func() goke.System) goke.Runnable {
	return c.ecs.RegSys(factory())
}
func (c *installCtx) ECS() *goke.ECS         { return c.ecs }
func (c *installCtx) Hosts(...host.Host)     {}
func (c *installCtx) Draws(...render.Source) {}
func (c *installCtx) Looks(func())           {}
func (*installCtx) Pool() *parallel.Pool     { return nil }

// the world installed on an ECS of its own, after set up after it, stepped as a game steps it
func start(t *testing.T, w *world.Plugin, after ...goke.System) *goke.ECS {
	t.Helper()
	ctx := &installCtx{ecs: goke.New()}
	if err := w.Install(ctx); err != nil {
		t.Fatal(err)
	}
	var systems []goke.System
	for _, produce := range ctx.pending {
		systems = append(systems, produce()...)
	}
	ctx.ecs.Setup(append(systems, after...)...)
	ctx.ecs.SetPlan(func(rc goke.RunCtx, d time.Duration) {
		w.RunPlan(rc, d)
		rc.Sync()
		w.Write().Clock().Replay(rc, d)
	})
	return ctx.ecs
}

// the world 1000 a side drawn two pixels a side, red to the west and green to the east
type halves struct{ img *render.Image }

func (h halves) Miniature() (*render.Image, float32) { return h.img, 0.002 }

func TestGround_LiesTurnedAsTheMinimapIs(t *testing.T) {
	gputest.Need(t)
	img := render.NewImage(2, 2)
	img.WritePixels([]byte{
		255, 0, 0, 255, 0, 255, 0, 255,
		255, 0, 0, 255, 0, 255, 0, 255,
	})
	w := flatWorld(nil)
	p := NewPlugin(w, halves{img}, Config{Reach: 500})
	p.For(cameratest.NewFromSpace(1000, 1000, 0), command.Nobody)
	m := p.maps[0]
	m.lens.SetViewport(100, 100)
	screen := render.NewImage(100, 100)
	pix := make([]byte, 4*100*100)
	at := func(x, y int) [2]byte { return [2]byte{pix[4*(y*100+x)], pix[4*(y*100+x)+1]} }
	g := &groundView{m: m}
	for _, c := range []struct {
		ax, ay                 float32
		x0, y0, x1, y1         int
		red, green, upAndRound string
	}{
		{0, -1, 20, 50, 80, 50, "left", "right", "north up"},
		{1, 0, 50, 80, 50, 20, "bottom", "top", "east up"},
	} {
		m.lens.aim(500, 500, c.ax, c.ay)
		screen.Fill(color.RGBA{A: 255})
		g.Draw(render.Target{Screen: screen}, m.lens, render.Uniforms{})
		screen.ReadPixels(pix)
		if r, g := at(c.x0, c.y0), at(c.x1, c.y1); r[0] < 200 || g[1] < 200 {
			t.Errorf("%s: the red half (%v at (%d, %d)) is not at the %s or the green (%v at "+
				"(%d, %d)) at the %s", c.upAndRound, r, c.x0, c.y0, c.red, g, c.x1, c.y1, c.green)
		}
	}
}

func TestCompass_HangsOnTheCirclesEdgePointingNorthTurningWithTheMinimap(t *testing.T) {
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 1000, Height: 1000},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10},
	})
	p := NewPlugin(w, nil, Config{Reach: 100, Turning: true})
	cam := cameratest.NewFromSpace(1000, 1000, 0)
	cam.SetViewport(400, 300)
	p.For(cam, command.Issuer(0))
	if e := p.Compass(cam); e == nil {
		t.Fatal("no compass for the camera's minimap")
	}
	surface := &compass{m: p.maps[0]}
	side := 220 + 2*Overhang
	surface.Resize(side, side)
	out := 110 + float64(northTall)*(northOut-0.5)
	mid := float64(side) / 2
	x, y := surface.at()
	if dx, dy := float64(x)-mid, float64(y)-mid; math.Abs(dx) > 0.5 || math.Abs(dy+out) > 0.5 {
		t.Errorf("north up, the triangle's middle is (%.1f, %.1f) from the box's middle, want "+
			"(0, %.1f): straight up, a quarter of it in", dx, dy, -out)
	}
	p.maps[0].of = turned{cam, 1, 0}
	p.maps[0].follow(true)
	x, y = surface.at()
	if dx, dy := float64(x)-mid, float64(y)-mid; math.Abs(dx+out) > 0.5 || math.Abs(dy) > 0.5 {
		t.Errorf("headed east, the triangle's middle is (%.1f, %.1f) from the box's middle, want "+
			"(%.1f, 0): on the left", dx, dy, -out)
	}
	if pts := surface.points(); pts[0][0] >= float32(mid-out) {
		t.Errorf("headed east, the tip at %v points %v, want left, past the base", pts[0], pts[1:])
	}
	if img := surface.Draw(); img == nil || img.Bounds().Dx() != side {
		t.Errorf("the compass drew %v, want a picture %d wide", img, side)
	}
}

func TestMarks_ArePublishedOnceARedrawAtMost(t *testing.T) {
	v := &marksView{m: &minimap{p: &Plugin{w: flatWorld(nil)}}}
	now := time.Now()
	for _, step := range []struct {
		after time.Duration
		want  uint64
	}{{0, 1}, {redraw / 2, 1}, {redraw, 2}, {redraw + redraw/2, 2}, {2 * redraw, 3}} {
		v.publish(now.Add(step.after), true)
		v.swap()
		if got := v.drawn.Front().version; got != step.want {
			t.Errorf("%v on, the marks drawn are the %dth published, want the %dth", step.after,
				got, step.want)
		}
	}
}

// the minimap of a camera 200 by 100 over (500, 500), reaching 200, laid out 200 by 200
func outlined() (*minimap, camera.Camera) {
	w := flatWorld(nil)
	p := NewPlugin(w, nil, Config{Reach: 200})
	cam := cameratest.NewFromSpace(1000, 1000, 0)
	cam.SetViewport(200, 100)
	cam.CenterOn(500, 500, 0)
	p.For(cam, command.Nobody)
	m := p.maps[0]
	m.lens.SetViewport(200, 200)
	p.RunPlan(nil, 0)
	return m, cam
}

// spans is where m's outline lies on the minimap
func spans(m *minimap) (x0, x1, y0, y1 float32) {
	var f render.Frame
	f.Reset(m.lens)
	(&outlineView{m: m}).Compose(&f, m.lens)
	var xs, ys []float32
	f.Each(func(_ render.At, v []render.Vertex) {
		for _, p := range v {
			xs = append(xs, p.DstX)
			ys = append(ys, p.DstY)
		}
	})
	return minOf(xs), maxOf(xs), minOf(ys), maxOf(ys)
}

func TestOutline_IsDrawnAsPublished(t *testing.T) {
	m, cam := outlined()
	m.marks.publish(time.Now(), true)
	m.marks.swap()
	before0, before1, _, _ := spans(m)
	cam.CenterOn(300, 300, 0)
	if x0, x1, _, _ := spans(m); x0 != before0 || x1 != before1 {
		t.Errorf("the camera moved after the publish, the outline spans x %v..%v, want %v..%v "+
			"as published", x0, x1, before0, before1)
	}
	m.marks.publish(time.Now().Add(redraw), true)
	m.marks.swap()
	if x0, _, _, _ := spans(m); x0 == before0 {
		t.Error("published again, the outline stays where the camera was")
	}
}

func TestMinimap_CoversWhereTheTicksHaveItLookNotWhereItIsDrawn(t *testing.T) {
	m, _ := outlined()
	want := m.covered()
	m.lens.aim(100, 100, 1, 0)
	if got := m.covered(); got != want {
		t.Errorf("the lens drawn aimed elsewhere, the minimap covers %v, want %v: where the "+
			"ticks have it look", got, want)
	}
}

func TestCompass_PointsAsPublishedWhileTheMinimapsAreHidden(t *testing.T) {
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 1000, Height: 1000},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10},
	})
	p := NewPlugin(w, nil, Config{Reach: 100, Turning: true})
	cam := cameratest.NewFromSpace(1000, 1000, 0)
	cam.SetViewport(400, 300)
	p.For(cam, command.Issuer(0))
	p.Res.Settings.Hidden = true
	m := p.maps[0]
	m.of = turned{cam, 1, 0}
	m.follow(true)
	allMarks{p}.Publish()
	allMarks{p}.Swap()
	m.of = turned{cam, 0, 1}
	m.follow(true)
	if dx, dy := (&compass{m: m}).north(); math.Abs(dx+1) > 1e-9 || math.Abs(dy) > 1e-9 {
		t.Errorf("hidden, headed east as published and south since, north runs (%.2f, %.2f), "+
			"want (-1, 0): as published", dx, dy)
	}
}

// run with -race
func TestMinimap_AimsTheLensDrawnWithoutReadingTheTicksOncePublished(t *testing.T) {
	m, _ := outlined()
	m.marks.publish(time.Now(), true)
	m.marks.swap()
	ticked := make(chan struct{})
	go func() {
		m.follow(false)
		close(ticked)
	}()
	m.aimLens()
	<-ticked
}
