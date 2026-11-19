package underground

// LampEvery is how far apart a structure's lamps hang under its roof, in its room's heights.
const LampEvery = lampEvery

// Stretches are the stretches the tube lays of every sunk floor, as the shader takes them.
func (p *Plugin) Stretches() [][]float32 {
	p.tubes.Publish()
	b := p.tubes.bufs[1-p.tubes.front]
	var out [][]float32
	for k := 0; k+tubeFloats <= len(b); k += tubeFloats {
		out = append(out, b[k:k+tubeFloats])
	}
	return out
}
