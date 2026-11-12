package main

import (
	"strings"
	"testing"
)

func TestGreens_StandOpenStreetMapsTreesRowsAndParks(t *testing.T) {
	r, err := parseRecipe([]byte(`
area: {centre: [10, 40], cells: [40, 30], cell: 10, units: 32, figures: 1}
ground: {rock: {slope: 2}}
rivers: {brook: 1000, stream: 2000, river: 3000}
buildings:
  cells: building
  storey: 3
  longest: 40
  kinds: [{kind: house}]
greens:
  cells: park
  row: 8
  density: 100
  trees: [{kind: plane, share: 3}, {kind: palm, share: 1}]
fields:
  - {cells: [building, park], kind: settlement, grain: 16, least: 1, trees: true}
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
				{id: 1, ring: [][2]float64{at(-190, -140), at(-170, -140), at(-170, -120),
					at(-190, -120)}},
			},
			greens: &osmGreens{
				// one in the open, one on the house
				trees: [][2]float64{at(150, -100), at(-180, -130)},
				rows:  [][][2]float64{{at(-100, 100), at(-76, 100)}},
				parks: [][][2]float64{{at(0, 0), at(100, 0), at(100, 100), at(0, 100)}},
			},
		}, nil
	}
	c, err := build(r, height, osm, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]int{}
	park := 0
	for _, tr := range c.trees {
		kinds[tr.kind]++
		x, y := tr.at[0]*c.perUnit, tr.at[1]*c.perUnit
		if x >= 200 && x <= 300 && y >= 150 && y <= 250 {
			park++
		}
	}
	// a hectare of park at 100 to the hectare, a row of 24 m at 8 m and the tree in the open
	if park < 70 || park > 130 || len(c.trees) != park+4+1 {
		t.Errorf("%d trees, %d of them in the park; want about 100 there, four in the row and "+
			"the one in the open", len(c.trees), park)
	}
	if kinds["plane"] < 2*kinds["palm"] || kinds["palm"] == 0 {
		t.Errorf("%v stand, want three planes to a palm", kinds)
	}
	fx, fy := c.place(at(50, 50)[0], at(50, 50)[1])
	if got := c.kindAt(int(fx), int(fy)); got != "park" {
		t.Errorf("the cell in the park is %q", got)
	}
	data := string(c.text("town.yaml", ""))
	if !strings.Contains(data, "- {kind: plane, at: [") {
		t.Error("the map lists no plane")
	}
	// the tree in the open stands in a block of no building and no park
	if n := strings.Count(data, "kind: settlement"); n < 3 {
		t.Errorf("%d settlements, want one over the house, the park and each tree apart", n)
	}
	if err := c.check([]byte(data)); err != nil {
		t.Errorf("a board refuses the map: %v", err)
	}
}
