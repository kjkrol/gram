package backdrop

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/kjkrol/gram/plugins/atmosphere/celestial"
	"github.com/kjkrol/gram/render"
)

// Part is a part of the sky, drawn by the sky's shader: WGSL defining the part's entry
// (shaders/sky.wgsl) and whatever it calls, the uniforms of its own it reads (U.Name) and the Feed
// setting them each frame the sky is drawn. A Part with no Source draws nothing.
type Part struct {
	Source []byte
	Reads  []render.Uniform
	Feed   func(v *Values)
}

// Sun is the sun's disc and its glow, Size times the Earth's (0 as large).
type Sun struct {
	Part
	Size float32
}

// Moon is the moon's disc and its glow, Size times the Earth's (0 as large): its Image, north up
// (image1, made the first time the moon is drawn), its Color under the atmosphere's moon's tint
// (zero white) and how much of it shows in full day, ByDay.
type Moon struct {
	Part
	Size  float32
	Image func() *render.Image
	Color render.Light
	ByDay float32
}

// Stars are the stars at night: a Catalogue drawn behind the sky where each stands, and Dots the
// shader adds to the vault (SkyStars); either, both or none.
type Stars struct {
	Catalogue []celestial.Star
	Dots      Part
}

// Dome is the sky's four parts, which a game's sky (atmosphere.Sky) looks: the vault, the sun, the
// moon and the stars.
type Dome struct {
	Vault Part
	Sun   Sun
	Moon  Moon
	Stars Stars
}

// Earth is the Earth's sky.
func Earth() Dome {
	return Dome{Vault: EarthVault(), Sun: EarthSun(), Moon: EarthMoon(), Stars: EarthStars()}
}

// EarthVault is the air's own colour, paler at the horizon and deeper overhead.
func EarthVault() Part { return Part{Source: render.Files(shaders, "shaders/vault.wgsl")} }

// EarthSun is a white disc reddening low, a halo and a glare round it in its colour.
func EarthSun() Sun { return Sun{Part: Part{Source: render.Files(shaders, "shaders/sun.wgsl")}} }

// EarthMoon is the moon photographed (celestial.MoonFace), lit by its phase, a little of it showing
// by day.
func EarthMoon() Moon {
	return Moon{
		Part:  Part{Source: render.Files(shaders, "shaders/moon.wgsl")},
		Image: celestial.MoonFace,
		Color: render.Light{0.92, 0.93, 0.96},
		ByDay: 0.3,
	}
}

// EarthStars are the real stars, celestial.Stars.
func EarthStars() Stars { return Stars{Catalogue: celestial.Stars()} }

// ScatteredStars are made-up stars, Dots twinkling a pixel or two wide.
func ScatteredStars() Part { return Part{Source: render.Files(shaders, "shaders/scattered.wgsl")} }

// Values are a part's own uniforms, which its Feed sets.
type Values struct {
	b     *Renderer
	part  string
	reads []render.Uniform
}

// Set sets the part's uniform name to v; a name the part does not read panics.
func (v *Values) Set(name string, x ...float32) {
	if !slices.ContainsFunc(v.reads, func(u render.Uniform) bool { return u.Name == name }) {
		panic(fmt.Sprintf("backdrop: the %s sets %s, which it does not read", v.part, name))
	}
	v.b.set(name, x...)
}

// the parts in the order the shader is assembled, each with its entry and, after the entry's
// name, a stub for none
type piece struct {
	name, entry, stub string
	part              *Part
}

const (
	plainStub = "(s: SkyAt) -> vec3<f32> {\n    return vec3<f32>(0.0);\n}\n"
	discStub  = "(s: SkyAt, d: SkyDisc) -> SkyShine {\n    return SkyShine();\n}\n"
)

func (d *Dome) pieces() [4]piece {
	return [4]piece{
		{"vault", "SkyVault", plainStub, &d.Vault},
		{"stars' dots", "SkyStars", plainStub, &d.Stars.Dots},
		{"moon", "SkyMoon", discStub, &d.Moon.Part},
		{"sun", "SkySun", discStub, &d.Sun.Part},
	}
}

// what a module's scope declares
var declared = regexp.MustCompile(`(?m)^(?:fn|const|struct|var|alias|override)\s+([A-Za-z_]\w*)`)

// assemble is the sky's shader's source and uniforms for d, its parts checked: a uniform or a name
// two of them declare, or one of the library's, panics naming them
func assemble(d *Dome) (string, []render.Uniform) {
	var src strings.Builder
	owners := map[string]string{}
	declare := func(owner, source string) {
		for _, m := range declared.FindAllStringSubmatch(source, -1) {
			if other, ok := owners[m[1]]; ok {
				panic(fmt.Sprintf("backdrop: %s is defined by both the %s and the %s", m[1], other,
					owner))
			}
			owners[m[1]] = owner
		}
		src.WriteString(source)
		src.WriteString("\n")
	}
	declare("sky", string(render.Files(shaders, "shaders/sky.wgsl")))
	declare("sky", string(render.Files(shaders, "shaders/backdrop.wgsl")))
	uniforms := append([]render.Uniform(nil), own...)
	readers := map[string]string{}
	for _, u := range render.SharedUniforms() {
		readers[u.Name] = "library's"
	}
	for _, u := range own {
		readers[u.Name] = "sky's own"
	}
	for _, p := range d.pieces() {
		if len(p.part.Source) == 0 {
			declare(p.name, "fn "+p.entry+p.stub)
			continue
		}
		source := string(p.part.Source)
		if !regexp.MustCompile(`(?m)^fn\s+` + p.entry + `\s*\(`).MatchString(source) {
			panic(fmt.Sprintf("backdrop: the %s defines no fn %s", p.name, p.entry))
		}
		declare(p.name, source)
		for _, u := range p.part.Reads {
			if other, ok := readers[u.Name]; ok {
				panic(fmt.Sprintf("backdrop: the %s reads %s, which is the %s", p.name, u.Name,
					other))
			}
			readers[u.Name] = p.name + "'s"
			uniforms = append(uniforms, u)
		}
	}
	return src.String(), uniforms
}
