package lod_test

import (
	"math/bits"
	"testing"

	"github.com/kjkrol/gram/lod"
	"github.com/kjkrol/uid"
)

// a change to Seed moves every part of every aggregate a game has saved
func TestSeed_IsTheSameNumberForAPlaceOfAWholeEveryTime(t *testing.T) {
	for _, c := range []struct {
		whole uid.UID64
		cell  uint64
		want  uint64
	}{
		{7, 3, 0x8ca3cbb6ca63129b},
		{0, 0, 0x48218226ff3cd4bf},
		{1<<40 | 5, 1 << 20, 0xe53c0d7dcb2f73c4},
	} {
		if got := lod.Seed(c.whole, c.cell); got != c.want || lod.Seed(c.whole, c.cell) != got {
			t.Errorf("Seed(%d, %d) = %#x, want %#x every time", c.whole, c.cell, got, c.want)
		}
	}
}

func TestSeed_SpreadsOverThePlacesOfAWholeAndOverTheWholes(t *testing.T) {
	const n = 4096
	for name, of := range map[string]func(i uint64) uint64{
		"the places of a whole": func(i uint64) uint64 { return lod.Seed(1, i) },
		"the wholes at a place": func(i uint64) uint64 { return lod.Seed(uid.UID64(i), 0) },
	} {
		seen := make(map[uint64]bool, n)
		var set [64]int
		for i := range uint64(n) {
			s := of(i)
			seen[s] = true
			for b := range 64 {
				set[b] += int(s >> b & 1)
			}
			if d := bits.OnesCount64(s ^ of(i+1)); d < 8 {
				t.Errorf("%s: %d and the next differ in %d bits, want many", name, i, d)
			}
		}
		if len(seen) != n {
			t.Errorf("%s: %d numbers of %d differ, want all", name, len(seen), n)
		}
		for b, k := range set {
			if k < n*45/100 || k > n*55/100 {
				t.Errorf("%s: bit %d set in %d of %d, want about half", name, b, k, n)
			}
		}
	}
}
