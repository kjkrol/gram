package light

import (
	"encoding/binary"
	"math"
)

// Shelter is a roof over part of the world: its floor's height, what of the sky's light gets
// under it, Sky of the sky's and Bounce of the distant light's thrown round from where it falls
// in, and its own Lamp, burning as the sky's light goes or ByDay too. The distant light gets in
// by its Openings alone, MaxOpenings of them at the most.
type Shelter struct {
	Floor       float32
	Sky, Bounce float32
	Lamp        Light
	ByDay       bool
	Openings    []Opening
}

// Opening is where a shelter's wall lets the distant light in: the stretch of its line from A to
// B, from Bottom to Top over the shelter's floor.
type Opening struct {
	A, B        [2]float32
	Bottom, Top float32
}

// MaxOpenings is how many of a shelter's openings let the distant light in.
const MaxOpenings = 32

// ShelterTexels is how many texels a shelter's row of the frame's Shelters image holds: its
// floor, its shares, its lamp, by day, how many openings, then six an opening.
const ShelterTexels = 8 + 6*MaxOpenings

// ShelterRows appends to dst the shelters' rows of the frame's Shelters image, a float32 a texel
// in its four bytes (light.wgsl's shelterFloat).
func ShelterRows(dst []byte, shelters []Shelter) []byte {
	for _, s := range shelters {
		row := len(dst)
		dst = append(dst, make([]byte, 4*ShelterTexels)...)
		put := func(texel int, v float32) {
			binary.LittleEndian.PutUint32(dst[row+4*texel:], math.Float32bits(v))
		}
		byDay := float32(0)
		if s.ByDay {
			byDay = 1
		}
		n := min(len(s.Openings), MaxOpenings)
		for k, v := range [8]float32{s.Floor, s.Sky, s.Bounce, s.Lamp[0], s.Lamp[1], s.Lamp[2],
			byDay, float32(n)} {
			put(k, v)
		}
		for k, o := range s.Openings[:n] {
			for j, v := range [6]float32{o.A[0], o.A[1], o.B[0], o.B[1], o.Bottom, o.Top} {
				put(8+6*k+j, v)
			}
		}
	}
	return dst
}
