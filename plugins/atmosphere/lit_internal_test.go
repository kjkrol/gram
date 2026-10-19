package atmosphere

import (
	"github.com/kjkrol/ceremony/command"
	"strings"
	"testing"

	"github.com/kjkrol/ceremony/host"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/atmosphere/aloft"
	"github.com/kjkrol/gram/plugins/atmosphere/backdrop"
	"github.com/kjkrol/gram/plugins/atmosphere/precipitation"
	"github.com/kjkrol/gram/plugins/atmosphere/sky"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

type litSheet struct{}

func (litSheet) Atlas() *render.Image                            { return nil }
func (litSheet) UV(render.SpriteID) (sx0, sy0, sx1, sy1 float32) { return 0, 0, 8, 8 }
func (litSheet) White() (u, v float32)                           { return 9, 9 }

type drawsCtx struct {
	plugin.Installer
	drawn []render.Source
}

func (*drawsCtx) UseModule(goke.Module)      {}
func (*drawsCtx) Hosts(...host.Host)         {}
func (*drawsCtx) ECS() *goke.ECS             { return goke.New() }
func (c *drawsCtx) Draws(s ...render.Source) { c.drawn = append(c.drawn, s...) }
func (*drawsCtx) Looks(func())               {}

func TestPlugin_DrawsTheSkyTheCloudsAndWhatFalls(t *testing.T) {
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 128, Height: 128},
		Entities: world.EntitiesCfg{MaxCount: 4, MinSize: 1, MaxSize: 20},
		Heights:  true,
	})
	a := NewPlugin(w, nil, Config{})
	ctx := &drawsCtx{}
	if err := a.Install(ctx); err != nil {
		t.Fatal(err)
	}
	if len(ctx.drawn) != 3 {
		t.Fatalf("the atmosphere draws %v, want the sky, the low clouds seen from above and what "+
			"falls", ctx.drawn)
	}
	if _, ok := ctx.drawn[0].(*backdrop.Renderer); !ok {
		t.Errorf("the atmosphere draws %T first, want the sky", ctx.drawn[0])
	}
	if _, ok := ctx.drawn[1].(*aloft.Renderer); !ok {
		t.Errorf("the atmosphere draws %T second, want the low clouds seen from above",
			ctx.drawn[1])
	}
	if _, ok := ctx.drawn[2].(*precipitation.Drops); !ok {
		t.Errorf("the atmosphere draws %T last, want what falls drop by drop", ctx.drawn[2])
	}
	look := func() (CloudLook, bool, [2]bool) {
		return a.ShownClouds(), a.aloft.Shown(), a.skyClouds().Bodies
	}
	for _, want := range []struct {
		look   CloudLook
		above  bool
		bodies [2]bool
	}{
		{AllBodies, true, [2]bool{true, true}},
		{ShadowsAlone, false, [2]bool{}},
		{FlatClouds, true, [2]bool{}},
		{LowBodies, true, [2]bool{true, false}},
		{AllBodies, true, [2]bool{true, true}},
	} {
		if l, above, bodies := look(); l != want.look || above != want.above ||
			bodies != want.bodies || a.skyClouds().Hidden != (l == ShadowsAlone) {
			t.Errorf("the clouds look %v, seen from above %v, heaped on the sky %v; want %+v", l,
				above, bodies, want)
		}
		a.clouds.Put(command.Nobody, Clouds{Look: (a.ShownClouds() + 1) % (ShadowsAlone + 1)})
		a.carryOut()
	}
}

func TestPlugin_TheSkyHandsTheFrameTheSunAndTheWeatherOfItsDaylight(t *testing.T) {
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 128, Height: 128},
		Entities: world.EntitiesCfg{MaxCount: 4, MinSize: 1, MaxSize: 20},
		Heights:  true,
	})
	red := render.Light{1, 0, 0}
	a := NewPlugin(w, nil, Config{Sky: sky.Config{
		Daylight: []sky.Daylight{{Height: 0, Sky: red, Sun: red, Ambient: 0.5}},
	}})
	ctx := &drawsCtx{}
	if err := a.Install(ctx); err != nil {
		t.Fatal(err)
	}
	c := render.NewComposer(ctx.drawn...)
	c.DrawWorld(nil, cameratest.NewFromSpace(128, 128, 0))
	got := c.Uniforms()
	for _, name := range []string{"LightWay", "LightStrength", "LightColor", "AmbientColor", "Ambience", "Wind",
		"Drift", "Cover", "Billow", "Fog", "SkyOverhead"} {
		if _, ok := got[name]; !ok {
			t.Errorf("the atmosphere alone hands the frame no %s", name)
		}
	}
	if sky, fog := got["AmbientColor"], got["Fog"]; sky[0] != 1 || sky[1] != 0 || fog[0] <= fog[2] {
		t.Errorf("under a red daylight the frame's sky is %v and its horizon %v, want red", sky,
			fog)
	}
}

func TestPlugin_ReportsTheMoonOnItsOwnHowTheCloudsLookAndWhoseShadowsFall(t *testing.T) {
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 128, Height: 128},
		Entities: world.EntitiesCfg{MaxCount: 4, MinSize: 1, MaxSize: 20},
		Heights:  true,
	})
	a := NewPlugin(w, nil, Config{})
	a.ShowClouds(LowBodies)
	got := map[string]string{}
	reporters{a.calendar.Reporter(), a.Reporter().(reporters)[3]}.Report(
		func(label, value string) { got[label] = value })
	if got["Moon"] == "" || strings.Count(got["Season"], ",") != 1 {
		t.Errorf("the season is %q and the moon %q, want the moon on a line of its own",
			got["Season"], got["Moon"])
	}
	if got["Clouds"] != "the low as bodies" ||
		got["Shadows"] != "both layers' on the ground and the figures" {
		t.Errorf("the clouds look %q, their shadows %q; want the low as bodies, both layers' "+
			"shadows as the game starts", got["Clouds"], got["Shadows"])
	}
	a.shades.Put(command.Nobody, HighShade{On: !a.shadows.High})
	a.carryOut()
	a.Reporter().(reporters)[3].Report(func(label, value string) { got[label] = value })
	if a.Air().HighShade || got["Shadows"] != "the low clouds' on the ground and the figures" {
		t.Errorf("after HighShade the weather's HighShade is %v, the shadows %q; want the low "+
			"clouds' alone", a.Air().HighShade, got["Shadows"])
	}
}
