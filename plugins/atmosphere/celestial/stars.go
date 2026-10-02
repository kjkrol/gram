package celestial

import (
	"bytes"
	_ "embed"
	"encoding/binary"
	"math"
	"sync"
)

// Star is a star of the night sky: the way towards it on the celestial sphere (an equatorial unit
// vector: x towards right ascension 0h, y 6h, z the north pole), how bright it is (Mag, the visual
// magnitude: smaller brighter) and its colour index (BV, B−V: bluer below 0.4, redder above).
type Star struct {
	Dir     [3]float32
	Mag, BV float32
}

// StarSky is which stars a sky shows at night: the real ones (RealStars, Stars), turning over the
// world's latitude as they do, or ScatteredStars, made up, for a world of its own.
type StarSky uint8

const (
	RealStars StarSky = iota
	ScatteredStars
)

// starsBin is the stars of the Yale Bright Star Catalogue, 5th revised edition (Hoffleit & Warren
// 1991, CDS catalogue V/50) as bright as V 6.0, the naked eye's under a dark sky; stars_gen.go
// writes it.
//
//go:embed stars.bin
var starsBin []byte

var (
	starsOnce sync.Once
	stars     []Star
)

// Stars is the real night sky's stars, brightest first: some five thousand, all the naked eye sees.
func Stars() []Star {
	starsOnce.Do(func() {
		n := len(starsBin) / 8
		rec := make([][4]uint16, n)
		if err := binary.Read(bytes.NewReader(starsBin), binary.LittleEndian, rec); err != nil {
			panic("celestial: the stars: " + err.Error())
		}
		stars = make([]Star, n)
		for i, r := range rec {
			ra := float64(r[0]) / 65536 * 2 * math.Pi
			dec := float64(int16(r[1])) / 32767 * math.Pi / 2
			sd, cd := math.Sincos(dec)
			sr, cr := math.Sincos(ra)
			stars[i] = Star{Dir: [3]float32{float32(cd * cr), float32(cd * sr), float32(sd)},
				Mag: float32(int16(r[2])) / 100, BV: float32(int16(r[3])) / 100}
		}
	})
	return stars
}
