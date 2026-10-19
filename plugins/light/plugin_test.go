package light_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/ceremony/effect"
	"github.com/kjkrol/ceremony/host"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/atmosphere/sky"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	"github.com/kjkrol/gram/plugins/light"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

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

type spot struct{ x, y float64 }

// a level world 1000 a side with heights, lit by the plugin, tick a tick of both; after sets up
// after them
func litWorld(t *testing.T, cfg light.Config, define func(w *world.Plugin, p *light.Plugin),
	after ...goke.System) (w *world.Plugin, p *light.Plugin, tick func()) {
	t.Helper()
	w = world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 1000, Height: 1000},
		Entities: world.EntitiesCfg{MaxCount: 16, MinSize: 4, MaxSize: 4},
		Heights:  true,
	})
	p = light.NewPlugin(w, cfg)
	if err := w.Carry(p); err != nil {
		t.Fatal(err)
	}
	if define != nil {
		define(w, p)
	}
	if err := w.Populate(); err != nil {
		t.Fatal(err)
	}
	ctx := &installCtx{ecs: goke.New()}
	for _, pl := range []plugin.Plugin{w, p} {
		if err := pl.Install(ctx); err != nil {
			t.Fatal(err)
		}
	}
	var systems []goke.System
	for _, produce := range ctx.pending {
		systems = append(systems, produce()...)
	}
	ctx.ecs.Setup(append(systems, after...)...)
	ctx.ecs.SetPlan(func(rc goke.RunCtx, d time.Duration) {
		w.RunPlan(rc, d)
		p.RunPlan(rc, d)
		rc.Sync()
		w.Clock().Replay(rc, d)
	})
	return w, p, func() { ctx.ecs.Tick(time.Second / 60) }
}

func lampKind(w *world.Plugin, name string, src light.Source) kind.Of[spot] {
	kind.Define[spot](w.Kinds(), name, kind.Spec{
		comp.Load(func(s spot) world.Position {
			return world.Position{AABB: plane.NewAABB(geom.NewVec(s.x-2, s.y-2), 4, 4)}
		}),
		comp.Const(world.Velocity{}),
		comp.Const(src),
	})
	return kind.Named[spot](w.Kinds(), name)
}

func TestPlugin_WithoutASunLightsEverythingWhiteAndCastsNoShadow(t *testing.T) {
	_, p, tick := litWorld(t, light.Config{}, nil)
	tick()
	lit := p.At(500, 500, 0)
	if lit.Light != (render.Light{1, 1, 1}) || lit.Takes != render.TakesAll {
		t.Errorf("the ground is lit %+v, want white, taking all", lit)
	}
	if _, ok := p.Distant().ShadowOf(geom.NewAABB(geom.NewVec(0, 0), geom.NewVec(4, 4)), world.Z{},
		nil); ok {
		t.Error("a world without a sun casts a shadow")
	}
}

func TestPlugin_ShinesTheScenesSunAndAnEffectOnItTurnsItsSource(t *testing.T) {
	var eclipse effect.Effect
	w, p, tick := litWorld(t, light.Config{}, func(w *world.Plugin, p *light.Plugin) {
		w.Effects().Define("eclipse", effect.Spec{effect.Lasts(time.Hour),
			effect.Alter(func(s *light.Source) { s.Strength = 0.1 })})
		eclipse = w.Effects().Named("eclipse")
	})
	p.WithDistant(light.Distant{Dir: [3]float32{0, 0, 1}, Strength: 0.6, Ambient: 0.3})
	tick()
	if got := p.At(10, 10, 0).Light[0]; !near(got, 0.9) {
		t.Errorf("under the scene's sun the ground is lit %v, want 0.9", got)
	}
	if !w.Carrier().Put(command.Nobody, ceremony.Put(eclipse).On(p)) {
		t.Fatal("the world carries no command for the light plugin")
	}
	tick()
	tick()
	if got := p.Distant().Strength; !near(got, 0.06) {
		t.Errorf("under the eclipse the sun is %v strong, want a tenth of 0.6", got)
	}
	w.Carrier().Put(command.Nobody, ceremony.Take(eclipse).On(p))
	tick()
	tick()
	if got := p.Distant().Strength; !near(got, 0.6) {
		t.Errorf("the eclipse over, the sun is %v strong, want 0.6 again", got)
	}
}

func TestPlugin_RefusesTwoDistantLights(t *testing.T) {
	_, p, tick := litWorld(t, light.Config{}, func(w *world.Plugin, p *light.Plugin) {
		kind.Define[spot](w.Kinds(), "star", kind.Spec{
			comp.Load(func(s spot) world.Position {
				return world.Position{AABB: plane.NewAABB(geom.NewVec(s.x, s.y), 4, 4)}
			}),
			comp.Const(world.Velocity{}),
			comp.Const(light.Source{Strength: 1, Ambient: 1}),
			comp.Const(light.Shining{Light: sky.DefaultSun}),
		})
		w.Seed(kind.Named[spot](w.Kinds(), "star").Entry(spot{100, 100}))
	})
	p.WithDistant(sky.DefaultSun)
	tick()
	defer func() {
		if msg, _ := recover().(string); !strings.Contains(msg, "two distant lights") {
			t.Errorf("two distant lights shining panic %q, want them refused", msg)
		}
	}()
	p.Distant()
}

func TestPlugin_LampsLightWhatLiesInTheirReachFadingWithTheDistance(t *testing.T) {
	_, p, tick := litWorld(t, light.Config{}, func(w *world.Plugin, p *light.Plugin) {
		torch := lampKind(w, "torch", light.Source{Color: render.Light{1, 0.5, 0}, Strength: 2,
			Reach: 100})
		w.Seed(torch.Entry(spot{500, 500}))
	})
	p.WithDistant(light.Distant{Dir: [3]float32{0, 0, 1}, Ambient: 0.5})
	tick()
	p.Begin(&render.Frame{}, cameratest.NewFromSpace(1000, 1000, 0))
	if lit := p.At(500, 500, 0); !near(lit.Lamps[0], 2) || !near(lit.Lamps[1], 1) ||
		!near(lit.Light[0], 2.5) {
		t.Errorf("at the torch the light is %+v, want the sky's 0.5 and the torch's 2, 1, 0", lit)
	}
	if lit := p.At(550, 500, 0); !near(lit.Lamps[0], 0.5) {
		t.Errorf("halfway to the torch's reach its light is %v, want a quarter of 2", lit.Lamps)
	}
	if lit := p.At(650, 500, 0); lit.Lamps != (render.Light{}) {
		t.Errorf("past the torch's reach its light is %v, want none", lit.Lamps)
	}
	if n := len(p.Lamps()); n != 1 {
		t.Errorf("%d lamps, want the torch", n)
	}
}

func TestLamps_UniformNamesAreTheDeclaredOnes(t *testing.T) {
	declared := map[string]int{}
	for _, u := range light.LampUniforms {
		declared[u.Name] = u.Size
	}
	for k, names := range light.LampNames {
		at, colour := fmt.Sprintf("Lamp%dAt", k), fmt.Sprintf("Lamp%dColor", k)
		if names != [2]string{at, colour} || declared[at] != 4 || declared[colour] != 3 {
			t.Errorf("lamp %d goes to the frame as %v, declared as %d and %d floats; "+
				"want %s of 4 and %s of 3", k, names, declared[at], declared[colour], at, colour)
		}
	}
}

func TestPlugin_BeginPicksTheLampsNearestTheCameraAndHandsTheFrameThem(t *testing.T) {
	_, p, tick := litWorld(t, light.Config{LampShadows: 2}, func(w *world.Plugin, p *light.Plugin) {
		torch := lampKind(w, "torch", light.Source{Strength: 1, Reach: 50})
		var entries []kind.Entry
		for i := range 10 {
			entries = append(entries, torch.Entry(spot{float64(50 + 90*i), 500}))
		}
		w.Seed(entries...)
	})
	tick()
	cam := cameratest.NewFromSpace(1000, 1000, 0)
	cam.SetViewport(200, 200)
	cam.CenterOn(900, 500, 0)
	c := render.NewComposer(source(func(f *render.Frame) { p.Begin(f, cam) }))
	c.DrawWorld(nil, cam)
	got := c.Uniforms()
	if got["LampCount"][0] != light.MaxLamps || got["Lamp0At"][0] != 860 ||
		got["Lamp1At"][0] != 770 || got["Lamp7At"][0] != 230 {
		t.Errorf("the frame is handed %v lamps, the first at %v, the second %v, the last %v; "+
			"want 8 nearest 900: 860, 770 .. 230", got["LampCount"], got["Lamp0At"],
			got["Lamp1At"], got["Lamp7At"])
	}
	if cast := p.Casting(); len(cast) != 2 || cast[0].X != 860 {
		t.Errorf("casting %v, want the two nearest", cast)
	}
}

func TestPlugin_OfTakesWhatTheEntitysShadowSays(t *testing.T) {
	var lantern, plain kind.Of[spot]
	var base goke.Comp[world.Base]
	var q *goke.Query
	var p *light.Plugin
	joined := goke.SystemFn{OnInit: func(si *goke.SysInit) {
		qb := si.NewQueryBuilder(&base)
		p.Join(qb)
		q = qb.Build()
	}}
	var tick func()
	_, _, tick = litWorld(t, light.Config{}, func(w *world.Plugin, pl *light.Plugin) {
		p = pl
		lantern = lampKind(w, "lantern", light.Source{Strength: 1, Reach: 100})
		kind.Define[spot](w.Kinds(), "plain", kind.Spec{
			comp.Load(func(s spot) world.Position {
				return world.Position{AABB: plane.NewAABB(geom.NewVec(s.x, s.y), 4, 4)}
			}),
			comp.Const(world.Velocity{}),
			comp.Const(light.Shadow{Cast: light.NoShadow, Take: render.Shines}),
		})
		plain = kind.Named[spot](w.Kinds(), "plain")
		w.Seed(lantern.Entry(spot{500, 500}), plain.Entry(spot{500, 520}))
	}, joined)
	p.WithDistant(light.Distant{Dir: [3]float32{0, 0, 1}, Ambient: 0.5})
	tick()
	p.Begin(&render.Frame{}, cameratest.NewFromSpace(1000, 1000, 0))
	seen := 0
	for q.All(); q.Next(); {
		cur := q.Cursor()
		for i := range cur.IDs {
			box := base.Slice(cur)[i].Pos.AABB
			lit := p.Of(cur, i, box, world.Z{}, render.Appearance{})
			if box.TopLeft.Y == 520 { // the plain one
				if lit.Takes != 0 || lit.Lamps != (render.Light{}) || lit.Light != (render.Light{1, 1, 1}) {
					t.Errorf("what shines is lit %+v, want white, taking nothing", lit)
				}
			} else if lit.Takes != render.TakesAll || !near(lit.Lamps[0], 1) {
				t.Errorf("the lantern is lit %+v, want it taking all, the lamp's light on it", lit)
			}
			seen++
		}
	}
	if seen != 2 {
		t.Errorf("%d entities lit, want both", seen)
	}
}

func TestPlugin_CommandsSwitchTheShadowsCoarseAndTheLampsOff(t *testing.T) {
	w, p, tick := litWorld(t, light.Config{LampShadows: 3}, nil)
	keys := map[control.Key]command.Command{}
	for _, b := range p.DefaultBindings() {
		press := b.Trigger.(control.KeyPress)
		keys[press.Key], _ = b.Build(control.Context{})
		if press.Key == control.KeyL && !press.Mods.Shift {
			t.Error("the lamps' shadows switch without Shift")
		}
	}
	if keys[control.KeyH] != (light.CoarseShadows{On: true}) ||
		keys[control.KeyL] != (light.LampShadows{Count: 0}) {
		t.Errorf("the keys build %v, want H CoarseShadows on and Shift+L LampShadows off", keys)
	}
	w.Carrier().Put(command.Nobody, keys[control.KeyH])
	w.Carrier().Put(command.Nobody, keys[control.KeyL])
	tick()
	if !p.ShadowsCoarse() || p.LampShadows() != 0 {
		t.Errorf("switched: coarse %v, lamps casting %d; want coarse, none", p.ShadowsCoarse(),
			p.LampShadows())
	}
	lamps := p.Settings()[1].(control.Switch)
	if lamps.Get() {
		t.Error("the lamps' setting says on with none casting")
	}
	w.Carrier().Put(command.Nobody, lamps.Set(true))
	tick()
	if p.LampShadows() != 3 || !lamps.Get() {
		t.Errorf("switched again, %d lamps cast, want the Config's 3", p.LampShadows())
	}
	saved := p.Serializable().Persisted()
	_, q, _ := litWorld(t, light.Config{LampShadows: 3}, nil)
	loading := q.Serializable().Persisted()
	*loading[0].(*[]byte) = append([]byte(nil), *saved[0].(*[]byte)...)
	q.Restore()
	if !q.ShadowsCoarse() || q.LampShadows() != 3 {
		t.Errorf("loaded: coarse %v, lamps %d; want what was saved", q.ShadowsCoarse(),
			q.LampShadows())
	}
	var lines []string
	p.Reporter().Report(func(label, value string) { lines = append(lines, label+": "+value) })
	if len(lines) != 1 || !strings.HasPrefix(lines[0], "Light: sun 0.00 · lamps 0, shadows of 0") {
		t.Errorf("the panel says %v", lines)
	}
}

func TestPlugin_AsksEveryKindToCastAPatchUnlessItSays(t *testing.T) {
	w, _, _ := litWorld(t, light.Config{}, nil)
	spec := w.Roster().Spec(world.EntityTemplate, comp.Const(world.Position{}))
	var got []light.Shadow
	for _, c := range spec {
		if s, ok := c.(comp.Template[light.Shadow]); ok {
			got = append(got, s.Resolve(nil))
		}
	}
	if len(got) != 1 || got[0] != (light.Shadow{Cast: light.Patch}) {
		t.Errorf("an entity's spec carries the shadows %v, want a Patch taking all", got)
	}
}
