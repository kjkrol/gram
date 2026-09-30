//go:build ignore

// stars_gen writes stars.bin from the Yale Bright Star Catalogue, 5th revised edition (Hoffleit &
// Warren 1991, CDS catalogue V/50), read from stdin as its catalog file: every star of V ≤ 6.0,
// brightest first, eight bytes each, little-endian — the right ascension (J2000) over 24 hours in
// a uint16, the declination over ±90° in an int16, V and B−V in hundredths of a magnitude in two
// int16s. Run: curl -s https://cdsarc.cds.unistra.fr/ftp/V/50/catalog.gz | gunzip | go run stars_gen.go
package main

import (
	"bufio"
	"encoding/binary"
	"log"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
)

type star struct {
	ra, dec, v, bv float64
}

func main() {
	var stars []star
	in := bufio.NewScanner(os.Stdin)
	for in.Scan() {
		l := in.Text()
		if len(l) < 107 || strings.TrimSpace(l[75:77]) == "" || strings.TrimSpace(l[102:107]) == "" {
			continue // a star gone, or none measured
		}
		f := func(a, b int) float64 {
			v, err := strconv.ParseFloat(strings.TrimSpace(l[a:b]), 64)
			if err != nil {
				log.Fatalf("%q: %v", l[a:b], err)
			}
			return v
		}
		s := star{ra: f(75, 77) + f(77, 79)/60 + f(79, 83)/3600, dec: f(84, 86) + f(86, 88)/60 + f(88, 90)/3600, v: f(102, 107), bv: 0.6}
		if l[83] == '-' {
			s.dec = -s.dec
		}
		if len(l) >= 114 && strings.TrimSpace(l[109:114]) != "" {
			s.bv = f(109, 114)
		}
		if s.v <= 6.0 {
			stars = append(stars, s)
		}
	}
	sort.SliceStable(stars, func(i, j int) bool { return stars[i].v < stars[j].v })
	out, err := os.Create("stars.bin")
	if err != nil {
		log.Fatal(err)
	}
	defer out.Close()
	for _, s := range stars {
		rec := [4]uint16{
			uint16(uint32(math.Round(s.ra/24*65536)) & 0xffff),
			uint16(int16(math.Round(s.dec / 90 * 32767))),
			uint16(int16(math.Round(s.v * 100))),
			uint16(int16(math.Round(s.bv * 100))),
		}
		if err := binary.Write(out, binary.LittleEndian, rec); err != nil {
			log.Fatal(err)
		}
	}
	log.Printf("%d stars", len(stars))
}
