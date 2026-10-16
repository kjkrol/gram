package backdrop

import (
	"math"
	"strings"
	"testing"

	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/atmosphere/air"
	"github.com/kjkrol/gram/plugins/atmosphere/celestial"
	"github.com/kjkrol/gram/plugins/atmosphere/sky"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

func panics(t *testing.T, want string, f func()) {
	t.Helper()
	defer func() {
		t.Helper()
		r := recover()
		if r == nil {
			t.Errorf("no panic, want one saying %q", want)
			return
		}
		if msg, _ := r.(string); !strings.Contains(msg, want) {
			t.Errorf("the panic says %v, want %q", r, want)
		}
	}()
	f()
}

func vault(src string, reads ...string) Part {
	p := Part{Source: []byte(src)}
	for _, name := range reads {
		p.Reads = append(p.Reads, render.Uniform{Name: name, Size: 1})
	}
	return p
}

const red = "fn SkyVault(s: SkyAt) -> vec3<f32> {\n    return vec3<f32>(1.0, 0.0, 0.0);\n}\n"

func TestAssemble_RefusesPartsThatClash(t *testing.T) {
	sun := func(src string, reads ...string) Sun {
		return Sun{Part: vault("fn SkySun(s: SkyAt, d: SkyDisc) -> SkyShine {\n"+
			"    return SkyShine();\n}\n"+src, reads...)}
	}
	for _, c := range []struct {
		name, want string
		dome       Dome
	}{
		{"a uniform two parts read", "the sun reads Glow, which is the vault's",
			Dome{Vault: vault(red, "Glow"), Sun: sun("", "Glow")}},
		{"a uniform of the library", "the vault reads Fog, which is the library's",
			Dome{Vault: vault(red, "Fog")}},
		{"a uniform of the sky's own", "the vault reads SunAt, which is the sky's own",
			Dome{Vault: vault(red, "SunAt")}},
		{"a name two parts define", "glow is defined by both the vault and the sun",
			Dome{Vault: vault(red + "fn glow() -> f32 {\n    return 1.0;\n}\n"),
				Sun: sun("const glow: f32 = 2.0;\n")}},
		{"a name of the sky's shader", "skyPiece is defined by both the sky and the vault",
			Dome{Vault: vault(red + "fn skyPiece() {\n}\n")}},
		{"a part without its entry", "the vault defines no fn SkyVault",
			Dome{Vault: vault("fn Vault(s: SkyAt) -> vec3<f32> {\n    return s.air;\n}\n")}},
	} {
		t.Run(c.name, func(t *testing.T) {
			panics(t, "backdrop: "+c.want, func() { assemble(&c.dome) })
		})
	}
}

func TestAssemble_StandsAStubForEachPartWithoutWGSL(t *testing.T) {
	src, uniforms := assemble(&Dome{})
	for _, entry := range []string{"SkyVault", "SkyStars", "SkyMoon", "SkySun"} {
		if n := strings.Count(src, "\nfn "+entry+"("); n != 1 {
			t.Errorf("a sky of no parts defines %s %d times, want once", entry, n)
		}
	}
	if len(uniforms) != len(own) {
		t.Errorf("a sky of no parts has %d uniforms, want its own %d", len(uniforms), len(own))
	}
}

func TestValues_SetOnlyWhatThePartReads(t *testing.T) {
	b := New(world.SpaceCfg{Width: 100, Height: 100}, world.Scale{}, nil, nil)
	b.Dress(Dome{Vault: vault(red, "Glow")})
	b.values[0].Set("Glow", 0.5)
	if got := b.uniforms["Glow"]; len(got) != 1 || got[0] != 0.5 {
		t.Errorf("the vault set Glow to 0.5, the sky holds %v", got)
	}
	panics(t, "backdrop: the vault sets Shine, which it does not read", func() {
		b.values[0].Set("Shine", 1)
	})
}

func TestDome_ADiscWithoutWGSLShowsNoneAndSizeScalesIt(t *testing.T) {
	night := celestial.Heavens{Sun: norm3([3]float32{0, 1, -0.4}), Moon: eyedF, Full: 1,
		Pole: [3]float32{0, 0, 1}}
	radii := func(d Dome) (sun, moon float32) {
		b := New(world.SpaceCfg{Width: 100, Height: 100}, world.Scale{},
			func() sky.Sun { return sky.Sun{Dir: eyedF, Strength: 0.7} },
			func() air.Weather { return air.Weather{} })
		b.Dress(d)
		_, sun, _, _ = b.sunOn(eyed{}, 100, 100, eyedF, render.Light{1, 1, 1}, 0)
		var p skyPlan
		b.nightOn(&p, eyed{}, camera.RayField{Dir: eyedF, DDX: eyedR,
			DDY: [3]float32{-eyedU[0], -eyedU[1], -eyedU[2]}}, 100, 100, night, 0)
		return sun, p.MoonRadius
	}
	earthSun, earthMoon := radii(Earth())
	if earthSun == 0 || earthMoon == 0 {
		t.Fatalf("the Earth's sun ahead is %v wide and its moon %v, want both shown", earthSun,
			earthMoon)
	}
	if s, m := radii(Dome{}); s != 0 || m != 0 {
		t.Errorf("a sky without discs shows a sun %v and a moon %v wide, want none", s, m)
	}
	twice := Earth()
	twice.Sun.Size = 2
	twice.Moon.Size = 2
	if s, m := radii(twice); math.Abs(float64(s-2*earthSun)) > 1e-4 ||
		math.Abs(float64(m-2*earthMoon)) > 1e-4 {
		t.Errorf("of Size 2 the sun is %v and the moon %v wide, want twice %v and %v", s, m,
			earthSun, earthMoon)
	}
}

func TestDome_CompilesTheEarthsAndAGamesSkies(t *testing.T) {
	needGPU(t)
	dots := Earth()
	dots.Stars.Dots = ScatteredStars()
	own := Dome{Vault: Part{
		Source: []byte("fn SkyVault(s: SkyAt) -> vec3<f32> {\n" +
			"    return mix(s.air, U.VaultNight, s.stars * band(s.starWay));\n}\n" +
			"fn band(w: vec3<f32>) -> f32 {\n    return exp(-w.z * w.z * 40.0);\n}\n"),
		Reads: []render.Uniform{{Name: "VaultNight", Size: 3}},
	}}
	for name, d := range map[string]Dome{"Earth": Earth(), "Earth with dots": dots,
		"none": {}, "a game's vault": own} {
		b := New(world.SpaceCfg{Width: 100, Height: 100}, world.Scale{}, nil, nil)
		b.Dress(d)
		if err := b.shader.Compile(); err != nil {
			t.Errorf("the sky of %s does not compile: %v", name, err)
		}
	}
}

func TestDome_DrawsOnlyThePartsItHasOnTheGPU(t *testing.T) {
	needGPU(t)
	w := world.NewPlugin(world.Config{Space: world.SpaceCfg{Width: 200, Height: 200},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 8}})
	top := cameras.NewPlugin(w).New(cameras.TopDown(), camera.Config{})
	sun := sky.Sun{Dir: eyedF, Strength: 0.7, Sky: render.Light{0.5, 0.7, 1}}
	b := New(w.Res.Config.Space, world.Scale{}, func() sky.Sun { return sun },
		func() air.Weather { return air.Weather{} })
	screen := render.NewImage(100, 100)
	pix := make([]byte, 4*100*100)
	at := func(x, y int) [4]byte { i := 4 * (y*100 + x); return [4]byte(pix[i : i+4]) }
	draw := func() {
		screen.Clear()
		b.Draw(render.Target{Screen: screen}, raying{height: 30}, framed(b, top))
		screen.ReadPixels(pix)
	}
	b.Dress(Dome{})
	draw()
	for i := 0; i < len(pix); i += 4 {
		if pix[i] > 1 || pix[i+1] > 1 || pix[i+2] > 1 {
			t.Fatalf("a sky of no parts under the sun is %v at pixel %d, want black", pix[i:i+4],
				i/4)
		}
	}
	fed := EarthSun()
	b.Dress(Dome{
		Vault: Part{
			Source: []byte("fn SkyVault(s: SkyAt) -> vec3<f32> {\n    return U.Shade;\n}\n"),
			Reads:  []render.Uniform{{Name: "Shade", Size: 3}},
			Feed:   func(v *Values) { v.Set("Shade", 1, 0, 0) },
		},
		Sun: fed,
	})
	draw()
	if c := at(2, 2); c[0] < 250 || c[1] > 8 || c[2] > 8 {
		t.Errorf("under a vault its Feed reddens, a corner is %v, want red", c)
	}
	if c := at(50, 50); c[0] < 250 || c[1] < 250 || c[2] < 250 {
		t.Errorf("the sun ahead on a red vault is %v, want white", c)
	}
}
