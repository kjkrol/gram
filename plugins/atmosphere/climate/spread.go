package climate

import (
	"math"
	"slices"

	"github.com/kjkrol/ceremony/effect"
	"github.com/kjkrol/gram/plugins/atmosphere/air"
	"github.com/kjkrol/gram/plugins/atmosphere/climate/weather"
)

// Spread is a knob on the atmosphere's entity: how much of the world the rain takes at most, a
// storm's included (Rain), how much a storm (Storm) and how much the fair breaks under the clouds
// (Fair), shares 0 to 1, so the weather comes in places and never covers the whole map.
type Spread struct{ Rain, Storm, Fair float32 }

// Knob: an effect may turn it (effect.Knob).
func (Spread) Knob() {}

var _ effect.Knob = Spread{}

// DefaultSpread is rain on a quarter of the world at most, a storm on an eighth in its middle and
// fair breaks on up to a quarter.
func DefaultSpread() Spread { return Spread{Rain: 0.25, Storm: 0.125, Fair: 0.25} }

// the places the weather comes in over a world width by height: a noise as wide as the world, the
// same again past its edges, drifting with the clouds, read as its rank among its values over
// the world, so a share of the ranks is that share of any stretch as wide as the world
type places struct {
	width, height float64
	sorted        []float64
}

// how many lattice cells the places' noise has across the world, and how many samples its ranks
// are worked out from each way
const (
	placesCells   = 4
	placesSamples = 96
)

func newPlaces(width, height float32) *places {
	p := &places{width: float64(width), height: float64(height)}
	for j := range placesSamples {
		for i := range placesSamples {
			p.sorted = append(p.sorted, p.noise((float64(i)+0.5)/placesSamples,
				(float64(j)+0.5)/placesSamples))
		}
	}
	slices.Sort(p.sorted)
	return p
}

// the rank, 0 to 1, of the places' noise at the world point (x, y) under clouds drifted by drift
func (p *places) rank(x, y float32, drift [2]float32) float32 {
	u := wrapUnit((float64(x) - float64(drift[0])) / p.width)
	v := wrapUnit((float64(y) - float64(drift[1])) / p.height)
	at, _ := slices.BinarySearch(p.sorted, p.noise(u, v))
	return float32(at) / float32(len(p.sorted))
}

// two octaves of value noise over the world taken as a unit square, the same again past it
func (p *places) noise(u, v float64) float64 {
	return valueNoise(u*placesCells, v*placesCells, placesCells) +
		0.5*valueNoise(u*2*placesCells+7.31, v*2*placesCells+3.17, 2*placesCells)
}

func wrapUnit(v float64) float64 { return v - math.Floor(v) }

func valueNoise(x, y, period float64) float64 {
	x0, y0 := math.Floor(x), math.Floor(y)
	fx, fy := x-x0, y-y0
	fx, fy = fx*fx*(3-2*fx), fy*fy*(3-2*fy)
	at := func(i, j float64) float64 {
		i, j = i-period*math.Floor(i/period), j-period*math.Floor(j/period)
		h := uint32(i)*374761393 + uint32(j)*668265263
		h = (h ^ h>>13) * 1274126177
		return float64(h^h>>16) / float64(math.MaxUint32)
	}
	top := at(x0, y0) + (at(x0+1, y0)-at(x0, y0))*fx
	bottom := at(x0, y0+1) + (at(x0+1, y0+1)-at(x0, y0+1))*fx
	return top + (bottom-top)*fy
}

// a part of the world a weather comes in: from what rank on, the mass there and its state
type part struct {
	from  float32
	mass  air.Mass
	state int16
}

// how far in rank one part grows out of the one before it
const placesSoft = 0.02

// the mass at the rank r among parts, ordered by from: the part r falls in, grown out of the one
// before it over the soft rim it begins with, so no part reaches past its share
func spreadAt(parts []part, r float32) (air.Mass, int16) {
	k := 0
	for k+1 < len(parts) && r >= parts[k+1].from {
		k++
	}
	m, st := parts[k].mass, parts[k].state
	if t := (r - parts[k].from) / placesSoft; k > 0 && t < 1 {
		was := parts[k-1].mass
		m = air.Mass{
			Cover:  was.Cover + (m.Cover-was.Cover)*t,
			Billow: was.Billow + (m.Billow-was.Billow)*t,
			Falls:  was.Falls + (m.Falls-was.Falls)*t,
			Damp:   was.Damp + (m.Damp-was.Damp)*t,
		}
		if t < 0.5 {
			st = parts[k-1].state
		}
	}
	return m, st
}

// the middle of a weather's ranges, for a part of the world it takes round another
func middling(st weather.State) air.Mass {
	return air.Mass{
		Cover:  (st.Clouds[0] + st.Clouds[1]) / 2,
		Billow: (st.Billow[0] + st.Billow[1]) / 2,
		Falls:  st.Falls,
		Damp:   st.Damp,
	}
}
