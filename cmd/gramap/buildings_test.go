package main

import (
	"math"
	"strings"
	"testing"
)

func TestFramed_IsTheOutlinesAreaLyingAsItsNarrowestFrame(t *testing.T) {
	// a rectangle 40 by 10 turned 30 degrees
	sin, cos := math.Sincos(30 * math.Pi / 180)
	var ring [][2]float64
	for _, c := range [][2]float64{{-20, -5}, {20, -5}, {20, 5}, {-20, 5}} {
		ring = append(ring, [2]float64{100 + c[0]*cos - c[1]*sin, 50 + c[0]*sin + c[1]*cos})
	}
	mid, size, turn, ok := framed(ring)
	if !ok {
		t.Fatal("no frame round a rectangle")
	}
	for name, d := range map[string]float64{
		"middle x": mid[0] - 100,
		"middle y": mid[1] - 50,
		"length":   size[0] - 40,
		"width":    size[1] - 10,
		"turn":     turn - 30,
	} {
		if math.Abs(d) > 1e-6 {
			t.Errorf("the %s is %g off", name, d)
		}
	}
	// an L of three squares of 10: its frame is 20 by 20, its area 300
	_, size, _, ok = framed([][2]float64{{0, 0}, {20, 0}, {20, 10}, {10, 10}, {10, 20}, {0, 20}})
	if !ok || math.Abs(size[0]*size[1]-300) > 1e-6 {
		t.Errorf("an L of 300 stands as %g by %g, want its area kept", size[0], size[1])
	}
	if _, _, _, ok := framed([][2]float64{{0, 0}, {10, 0}, {20, 0}}); ok {
		t.Error("a line stands as a building")
	}
}

func TestBuildings_AreOfTheFirstKindWhoseWordsTheyAnswer(t *testing.T) {
	cfg := buildingsCfg{Kinds: []buildingCfg{
		{Kind: "church", Tags: map[string]cellKinds{"amenity": {"place_of_worship"}}},
		{Kind: "tower", Floors: 8},
		{Kind: "shed", Under: 25},
		{Kind: "hall", Over: 1500},
		{Kind: "house"},
	}}
	for _, c := range []struct {
		tags   map[string]string
		floors int
		area   float64
		want   string
	}{
		{map[string]string{"amenity": "place_of_worship"}, 9, 400, "church"},
		{map[string]string{"building": "yes"}, 9, 400, "tower"},
		{nil, 0, 20, "shed"},
		{nil, 2, 2000, "hall"},
		{nil, 2, 120, "house"},
	} {
		if got, _ := cfg.kindOf(c.tags, c.floors, c.area); got != c.want {
			t.Errorf("%v of %d storeys and %g m² is a %s, want a %s", c.tags, c.floors, c.area,
				got, c.want)
		}
	}
	if n := storeys(map[string]string{"building:levels": "4"}, 3); n != 4 {
		t.Errorf("4 levels are %d storeys", n)
	}
	if n := storeys(map[string]string{"height": "10 m"}, 3); n != 3 {
		t.Errorf("10 m are %d storeys of 3 m", n)
	}
	if n := storeys(nil, 3); n != 0 {
		t.Errorf("no word is %d storeys", n)
	}
}

func TestBuild_StandsOpenStreetMapsBuildingsEachItsOwn(t *testing.T) {
	r, err := parseRecipe([]byte(`
area: {centre: [10, 40], cells: [40, 30], cell: 10, units: 32, figures: 1}
ground: {rock: {slope: 2}}
rivers: {brook: 1000, stream: 2000, river: 3000}
buildings:
  cells: building
  storey: 3
  longest: 40
  kinds:
    - {kind: block, floors: 4}
    - {kind: house}
fields:
  - {cells: building, kind: settlement, grain: 16, least: 1}
`), "town.yaml")
	if err != nil {
		t.Fatal(err)
	}
	height := func(lon, lat float64) (float64, error) { return 20, nil }
	at := func(x, y float64) [2]float64 {
		lon, lat := lonLatOf(r.Area.Centre, x, y)
		return [2]float64{lon, lat}
	}
	osm := func([4]float64, []string, bool) (*osmData, error) {
		return &osmData{
			footprints: []osmBuilding{
				{id: 1, ring: [][2]float64{at(-60, 20), at(-40, 20), at(-40, 30), at(-60, 30)}},
				{
					id:   2,
					tags: map[string]string{"building:levels": "5"},
					ring: [][2]float64{at(20, 20), at(110, 20), at(110, 40), at(20, 40)},
				},
			},
		}, nil
	}
	c, err := build(r, height, osm, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]int{}
	for _, b := range c.buildings {
		kinds[b.kind]++
		if b.size[0]*c.perUnit > 40+1e-6 {
			t.Errorf("a %s of %g m stands whole, want parts of 40 m at most", b.kind,
				b.size[0]*c.perUnit)
		}
	}
	// the block of 90 m is written in three parts
	if kinds["house"] != 1 || kinds["block"] != 3 {
		t.Fatalf("%v stand, want a house and a block in three parts", kinds)
	}
	house := c.buildings[0]
	if math.Abs(house.size[0]*c.perUnit-20) > 0.1 || math.Abs(house.size[1]*c.perUnit-10) > 0.1 {
		t.Errorf("the house of 20 m by 10 stands %g by %g m", house.size[0]*c.perUnit,
			house.size[1]*c.perUnit)
	}
	fx, fy := c.place(lonLatOf(r.Area.Centre, -50, 25))
	if got := c.kindAt(int(fx), int(fy)); got != "building" {
		t.Errorf("the cell under the house is %q", got)
	}
	data := c.text("town.yaml", "")
	if !strings.Contains(string(data), "kind: block, mid: ") ||
		!strings.Contains(string(data), "floors: 5}") ||
		!strings.Contains(string(data), "kind: settlement") {
		t.Error("the map lists no block of five storeys or no settlement over it")
	}
	if err := c.check(data); err != nil {
		t.Errorf("a board refuses the map: %v", err)
	}
}
