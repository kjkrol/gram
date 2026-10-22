package light

import "github.com/kjkrol/gram/plugins/world"

// Lighting is the light as the world lights its entities by it.
func Lighting(p *Plugin) world.Lighting { return lighting{p} }

// Casts is every live shadow gathered for the picture begun, Umbra by Umbra.
func Casts(p *Plugin) []Umbra {
	var out []Umbra
	for k := 0; k+12 <= len(p.casts); k += 12 {
		c := p.casts[k : k+12]
		out = append(out, Umbra{X: c[0], Y: c[1], UX: c[2], UY: c[3], Along: c[4], Wide: c[5],
			Fade: c[6], Veil: c[7]})
	}
	return out
}

// Shaped is whether each cast gathered has a sprite's shape.
func Shaped(p *Plugin) []bool {
	var out []bool
	for k := 0; k+12 <= len(p.casts); k += 12 {
		c := p.casts[k+8 : k+12]
		out = append(out, [4]float32(c) != [4]float32{})
	}
	return out
}

// LampNames are the names each lamp's place and colour go to the frame by, LampUniforms the
// uniforms declared.
var LampNames, LampUniforms = lampNames, lampUniforms
