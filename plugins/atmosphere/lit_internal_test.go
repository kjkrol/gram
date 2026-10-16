package atmosphere

import (
	"github.com/kjkrol/ceremony/command"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/ceremony/host"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	icamera "github.com/kjkrol/gram/internal/camera"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/atmosphere/aloft"
	"github.com/kjkrol/gram/plugins/atmosphere/backdrop"
	"github.com/kjkrol/gram/plugins/atmosphere/precipitation"
	"github.com/kjkrol/gram/plugins/atmosphere/sky"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/look"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

type litSheet struct{}

func (litSheet) Atlas() *render.Image                            { return nil }
func (litSheet) UV(render.SpriteID) (sx0, sy0, sx1, sy1 float32) { return 0, 0, 8, 8 }
func (litSheet) White() (u, v float32)                           { return 9, 9 }

func TestAtmosphere_LightsAFlatBoardAndItsSpritesByTheHour(t *testing.T) {
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 128, Height: 128},
		Entities: world.EntitiesCfg{MaxCount: 4, MinSize: 1, MaxSize: 20},
		Heights:  true,
	})
	cam := cameras.NewPlugin(w).New(cameras.TopDown(), camera.Config{})
	grid := grid.DefaultGrids{}.Square(4, 4, 32)
	b := board.NewPlugin(grid, &cell.MultipleOccupancy{}, w)
	b.Board().SetAll(cell.Kind{Cost: 1, Allows: cell.Land})
	b.SetAtlas(litSheet{})
	even := func() render.Light {
		l, ok := b.Map().Dressing().(look.EvenLit).EvenLight()
		if !ok {
			t.Fatal("a flat board's dressing does not light its tiles alike")
		}
		return l
	}
	tile := func() render.Light { return b.Map().Dressing().Light(nil)[0] }
	sprite := func() render.Vertex {
		var f render.Frame
		f.Reset(cam)
		w.Look().Sprite(&f, cam, plane.NewAABB(geom.NewVec(40, 40), 10, 10), world.Z{}, litSheet{}, render.Appearance{}, render.Light{1, 1, 1})
		var v render.Vertex
		f.Each(func(_ render.At, verts []render.Vertex) { v = verts[0] })
		return v
	}
	if l := tile(); l != (render.Light{1, 1, 1}) {
		t.Fatalf("without an atmosphere a tile is lit %v, want as it is", l)
	}
	if l := even(); l != (render.Light{1, 1, 1}) {
		t.Fatalf("without an atmosphere the tiles' even light is %v, want white", l)
	}
	a := NewPlugin(w, Config{Sky: sky.Config{Frozen: true, Hour: 29 * time.Minute}})
	b.WithAtmosphere(a)
	if sun := a.Heavens().Sun; sun[2] > 0 {
		t.Fatalf("the sun at half past midnight stands at %v, want under the horizon", sun)
	}
	if l := tile(); l[0] >= 0.5 || l[2] <= l[0] {
		t.Errorf("under the midnight sky a tile is lit %v, want it dark and blue", l)
	}
	if l := even(); l != tile() {
		t.Errorf("under the midnight sky the tiles' even light is %v, want the tile's %v", l, tile())
	}
	if v := sprite(); v.ColorR >= 0.5 || v.ColorB <= v.ColorR {
		t.Errorf("under the midnight sky a sprite is lit %v %v %v, want it dark and blue", v.ColorR, v.ColorG, v.ColorB)
	}
}

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
	a := NewPlugin(w, Config{})
	ctx := &drawsCtx{}
	if err := a.Install(ctx); err != nil {
		t.Fatal(err)
	}
	if len(ctx.drawn) != 3 {
		t.Fatalf("the atmosphere draws %v, want the sky, the clouds seen from above and what "+
			"falls", ctx.drawn)
	}
	if _, ok := ctx.drawn[0].(*backdrop.Renderer); !ok {
		t.Errorf("the atmosphere draws %T first, want the sky", ctx.drawn[0])
	}
	if _, ok := ctx.drawn[1].(*aloft.Renderer); !ok {
		t.Errorf("the atmosphere draws %T second, want the clouds seen from above", ctx.drawn[1])
	}
	if _, ok := ctx.drawn[2].(*precipitation.Renderer); !ok {
		t.Errorf("the atmosphere draws %T third, want what falls", ctx.drawn[2])
	}
	if a.CloudsShown() {
		t.Error("the clouds are drawn from above before a scene says so")
	}
	a.clouds.Put(command.Nobody, Clouds{})
	a.carryOut()
	if !a.CloudsShown() {
		t.Error("Clouds left the clouds seen from above hidden")
	}
}

func TestPlugin_TheSkyHandsTheFrameTheSunAndTheWeatherOfItsDaylight(t *testing.T) {
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 128, Height: 128},
		Entities: world.EntitiesCfg{MaxCount: 4, MinSize: 1, MaxSize: 20},
		Heights:  true,
	})
	red := render.Light{1, 0, 0}
	a := NewPlugin(w, Config{Sky: sky.Config{
		Daylight: []sky.Daylight{{Height: 0, Sky: red, Sun: red, Ambient: 0.5}},
	}})
	ctx := &drawsCtx{}
	if err := a.Install(ctx); err != nil {
		t.Fatal(err)
	}
	c := render.NewComposer(ctx.drawn...)
	c.DrawWorld(nil, icamera.NewFromSpace(128, 128, 0))
	got := c.Uniforms()
	for _, name := range []string{"Sun", "SunStrength", "SunColor", "SkyColor", "Ambience", "Wind",
		"Drift", "Cover", "Billow", "Fog", "SkyOverhead"} {
		if _, ok := got[name]; !ok {
			t.Errorf("the atmosphere alone hands the frame no %s", name)
		}
	}
	if sky, fog := got["SkyColor"], got["Fog"]; sky[0] != 1 || sky[1] != 0 || fog[0] <= fog[2] {
		t.Errorf("under a red daylight the frame's sky is %v and its horizon %v, want red", sky,
			fog)
	}
}
