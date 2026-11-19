package light

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/effect"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/uid"
)

// Shelter is a roof over part of the world, a knob of the entity standing for it, its floor as
// high as the entity's world.Z: what of the sky's light gets under it, Sky of the sky's and Bounce
// of the distant light's thrown round from where it falls in, its own Lamp's light, burning as
// the sky's goes or ByDay too, and its first Open Openings, by which the distant light gets in.
type Shelter struct {
	Openings    [MaxOpenings]Opening
	Open        int
	Sky, Bounce float32
	Lamp        [3]float32
	ByDay       bool
}

// MaxOpenings is how many openings a shelter keeps: its windows and its doors.
const MaxOpenings = render.MaxOpenings

// Opens has s open by o as well, where it keeps fewer than MaxOpenings.
func (s *Shelter) Opens(o Opening) {
	if s.Open < MaxOpenings {
		s.Openings[s.Open] = o
		s.Open++
	}
}

// Knob: an effect may turn it (effect.Knob).
func (Shelter) Knob() {}

var _ effect.Knob = Shelter{}

// Opening is where a shelter's wall lets the distant light in, a window or a door: the stretch of
// its line from A to B, from Bottom to Top over the shelter's floor.
type Opening struct {
	A, B        geom.Vec
	Bottom, Top float32
}

// Sheltered is the entity whose Shelter an entity stands under, as whoever keeps who stands where
// writes it: it is drawn in the light that gets in there, pixel by pixel, casting no shadow.
type Sheltered struct{ In uid.UID64 }

// shelters keeps the shelters' places in render.Lights, an entity's for as long as it stands, and
// the rows as last published, raising their version as they change
type shelters struct {
	at      map[uid.UID64]uint16
	free    []uint16
	seen    map[uid.UID64]bool
	rows    []render.Shelter
	version uint64
}

// place is the place of the shelter the entity id stands for, given it the first time it is asked
func (s *shelters) place(id uid.UID64) uint16 {
	if k, ok := s.at[id]; ok {
		return k
	}
	if s.at == nil {
		s.at = map[uid.UID64]uint16{}
	}
	var k uint16
	if n := len(s.free); n > 0 {
		k, s.free = s.free[n-1], s.free[:n-1]
	} else {
		k = uint16(len(s.at))
	}
	s.at[id] = k
	return k
}

// shelter lays into b the shelters standing, each at its place, the places of those gone freed,
// raising the version where any row changed
func (p *Plugin) shelter(b *render.Lights) {
	s := &p.sheltering
	if p.shelters == nil {
		b.Shelters, b.SheltersVersion = b.Shelters[:0], s.version
		return
	}
	if s.seen == nil {
		s.seen = map[uid.UID64]bool{}
	}
	clear(s.seen)
	changed := false
	for p.shelters.All(); p.shelters.Next(); {
		cur := p.shelters.Cursor()
		zs := p.shelterZ.Slice(cur)
		for i, sh := range p.shelterOf.Slice(cur) {
			id := cur.IDs[i]
			s.seen[id] = true
			k := int(s.place(id))
			for len(s.rows) <= k {
				s.rows = append(s.rows, render.Shelter{})
				changed = true
			}
			if s.lay(&s.rows[k], sh, zs[i].Altitude) {
				changed = true
			}
		}
	}
	for id, k := range s.at {
		if !s.seen[id] {
			delete(s.at, id)
			s.free = append(s.free, k)
			s.rows[k] = render.Shelter{Openings: s.rows[k].Openings[:0]}
			changed = true
		}
	}
	if changed {
		s.version++
	}
	if b.SheltersVersion == s.version && len(b.Shelters) == len(s.rows) {
		return
	}
	for len(b.Shelters) < len(s.rows) {
		b.Shelters = append(b.Shelters, render.Shelter{})
	}
	b.Shelters = b.Shelters[:len(s.rows)]
	for k, r := range s.rows {
		openings := append(b.Shelters[k].Openings[:0], r.Openings...)
		b.Shelters[k] = r
		b.Shelters[k].Openings = openings
	}
	b.SheltersVersion = s.version
}

// lay has row stand for sh, its floor at floor, reporting whether it changed
func (s *shelters) lay(row *render.Shelter, sh Shelter, floor float32) bool {
	n := min(sh.Open, MaxOpenings)
	same := row.Floor == floor && row.Sky == sh.Sky && row.Bounce == sh.Bounce &&
		row.Lamp == render.Light(sh.Lamp) && row.ByDay == sh.ByDay && len(row.Openings) == n
	row.Floor, row.Sky, row.Bounce = floor, sh.Sky, sh.Bounce
	row.Lamp, row.ByDay = render.Light(sh.Lamp), sh.ByDay
	if len(row.Openings) != n {
		row.Openings = append(row.Openings[:0], make([]render.Opening, n)...)
	}
	for k, o := range sh.Openings[:n] {
		ro := render.Opening{A: [2]float32{o.A.X, o.A.Y}, B: [2]float32{o.B.X, o.B.Y},
			Bottom: o.Bottom, Top: o.Top}
		same = same && row.Openings[k] == ro
		row.Openings[k] = ro
	}
	return !same
}
