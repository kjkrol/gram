package main

import (
	"math"
	"strings"
	"testing"
)

func TestStreets_LieWhereOpenStreetMapsWaysRunLevelAcross(t *testing.T) {
	r, err := parseRecipe([]byte(`
area: {centre: [10, 40], cells: [40, 30], cell: 10, units: 32, figures: 1}
kinds: {road: asphalt}
ground: {rock: {slope: 2}}
rivers: {brook: 1000, stream: 2000, river: 3000}
roads:
  source: osm
  lay: profile
  classes: [residential]
  slope: 5
  lane: 3
  verge: 0.5
`), "town.yaml")
	if err != nil {
		t.Fatal(err)
	}
	// the ground falls 15 m in every 100 m northwards
	height := func(lon, lat float64) (float64, error) {
		_, south := metresOf(r.Area.Centre, lon, lat)
		return 60 + 0.15*south, nil
	}
	at := func(x, y float64) [2]float64 {
		lon, lat := lonLatOf(r.Area.Centre, x, y)
		return [2]float64{lon, lat}
	}
	osm := func([4]float64, []string, bool) (*osmData, error) {
		return &osmData{ways: []osmWay{
			// along the slope, 3 m south of its cells' middles
			{id: 1, lanes: 2, line: [][2]float64{at(-150, 8), at(150, 8)}},
			// straight up it, driven one way
			{id: 2, lanes: 1, oneway: true, line: [][2]float64{at(52, 8), at(52, 120)}},
		}}, nil
	}
	c, err := build(r, height, osm, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	s := c.roads.streets
	if s == nil || len(s.legs) < 3 {
		t.Fatalf("the streets: %+v, want three stretches meeting at a junction", s)
	}
	if got, most := s.steepest, math.Tan(5*math.Pi/180); got > most+1e-6 || got < most/2 {
		t.Errorf("the steepest stretch climbs %.3f, want the recipe's %.3f on a slope of 0.15", got,
			most)
	}
	x, y := c.place(at(-100, 8)[0], at(-100, 8)[1])
	k := c.cornersOf(c.cell(int(x), int(y)))
	if d := math.Abs(c.corners[k[0]] - c.corners[k[2]]); d > 0.02 {
		t.Errorf("the street along the slope lies %.2f m lower on one side than on the other", d)
	}
	if d := math.Abs(c.corners[k[0]] - c.corners[k[1]]); d > 0.02 {
		t.Errorf("the street along the slope climbs %.2f m along a cell", d)
	}
	data := string(c.text("town.yaml", ""))
	for _, want := range []string{"asphalt:", "{lanes: 2, via: ", "oneway: true", "axes: ["} {
		if !strings.Contains(data, want) {
			t.Errorf("the map says no %q", want)
		}
	}
	if err := c.check([]byte(data)); err != nil {
		t.Errorf("a board refuses the map: %v", err)
	}
}

func TestStreets_ARingOfWaysIsARoundaboutWhereTheyMeet(t *testing.T) {
	r, err := parseRecipe([]byte(`
area: {centre: [10, 40], cells: [40, 30], cell: 10, units: 32, figures: 1}
kinds: {road: asphalt}
ground: {rock: {slope: 2}}
rivers: {brook: 1000, stream: 2000, river: 3000}
roads: {source: osm, lay: profile, classes: [residential], slope: 10, lane: 3, verge: 0.5}
`), "town.yaml")
	if err != nil {
		t.Fatal(err)
	}
	height := func(lon, lat float64) (float64, error) { return 20, nil }
	at := func(x, y float64) [2]float64 {
		lon, lat := lonLatOf(r.Area.Centre, x, y)
		return [2]float64{lon, lat}
	}
	var ring [][2]float64
	for i := 0; i <= 12; i++ {
		a := 2 * math.Pi * float64(i) / 12
		ring = append(ring, at(5+12*math.Cos(a), 5+12*math.Sin(a)))
	}
	ring[12] = ring[0]
	osm := func([4]float64, []string, bool) (*osmData, error) {
		return &osmData{ways: []osmWay{
			{id: 1, lanes: 1, oneway: true, roundabout: true, line: ring},
			{id: 2, lanes: 2, line: [][2]float64{at(-150, 5), at(-7, 5)}},
			{id: 3, lanes: 2, line: [][2]float64{at(17, 5), at(150, 5)}},
			{id: 4, lanes: 2, line: [][2]float64{at(5, 17), at(5, 120)}},
		}}, nil
	}
	c, err := build(r, height, osm, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	s := c.roads.streets
	if len(s.rounds) != 1 {
		t.Fatalf("%d roundabouts, want the one", len(s.rounds))
	}
	x, y := c.place(at(5, 5)[0], at(5, 5)[1])
	if got := s.rounds[c.cell(int(x), int(y))]; math.Abs(got-12) > 0.5 {
		t.Errorf("the roundabout at its ring's middle is %.1f m round, want 12", got)
	}
	data := string(c.text("town.yaml", ""))
	if !strings.Contains(data, "junctions: [{at: [") || !strings.Contains(data, "roundabout: ") {
		t.Error("the map writes no roundabout")
	}
	if err := c.check([]byte(data)); err != nil {
		t.Errorf("a board refuses the map: %v", err)
	}
}

func TestStreets_RunIntoATunnelAtItsHeadsAndLeaveTheGroundOverIt(t *testing.T) {
	r, err := parseRecipe([]byte(`
area: {centre: [10, 40], cells: [40, 30], cell: 10, units: 32, figures: 1}
kinds: {road: asphalt}
ground: {rock: {slope: 2}}
rivers: {brook: 1000, stream: 2000, river: 3000}
roads: {source: osm, lay: profile, classes: [residential], slope: 8, lane: 3, verge: 0.5}
tunnels: {clear: 4.5, width: 8, cover: 1, grade: 0.1, longest: 100, reshape: 12}
`), "town.yaml")
	if err != nil {
		t.Fatal(err)
	}
	// a ridge 30 m high across the street, 80 m either way
	height := func(lon, lat float64) (float64, error) {
		east, _ := metresOf(r.Area.Centre, lon, lat)
		return 60 + 30*math.Max(1-math.Abs(east)/80, 0), nil
	}
	at := func(x, y float64) [2]float64 {
		lon, lat := lonLatOf(r.Area.Centre, x, y)
		return [2]float64{lon, lat}
	}
	osm := func([4]float64, []string, bool) (*osmData, error) {
		return &osmData{ways: []osmWay{
			{id: 1, lanes: 2, line: [][2]float64{at(-180, 5), at(-75, 5)}},
			{id: 2, lanes: 2, tunnel: true, line: [][2]float64{at(-75, 5), at(75, 5)}},
			{id: 3, lanes: 2, line: [][2]float64{at(75, 5), at(180, 5)}},
		}}, nil
	}
	c, err := build(r, height, osm, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if n := len(c.roads.tunnels); n != 1 {
		t.Fatalf("%d tunnels under the ridge, want one: %s", n, c.roads.summary())
	}
	tn := c.roads.tunnels[0]
	s := c.roads.streets
	if s.lots[tn.from] == nil || s.lots[tn.to] == nil || !s.lots[tn.from].pin {
		t.Fatalf("no street runs to the tunnel's heads: %s", c.roads.summary())
	}
	for _, h := range []struct {
		lot   *lot
		level float64
	}{{s.lots[tn.from], c.level(tn.from)}, {s.lots[tn.to], c.level(tn.to)}} {
		if math.Abs(h.lot.h-h.level) > 1e-6 {
			t.Errorf("the street meets a head lying at %.2f at %.2f", h.level, h.lot.h)
		}
	}
	if got, most := s.steepest, math.Tan(8*math.Pi/180); got > most+1e-6 {
		t.Errorf("the steepest stretch climbs %.3f, the recipe's %.3f at most", got, most)
	}
	x, y := c.place(at(0, 5)[0], at(0, 5)[1])
	top := c.level(c.cell(int(x), int(y)))
	if top < 85 {
		t.Errorf("the ridge over the tunnel lies at %.1f, cut from its 90", top)
	}
	data := string(c.text("town.yaml", ""))
	for _, want := range []string{"kind: tunnel", "{onto: "} {
		if !strings.Contains(data, want) {
			t.Errorf("the map says no %q", want)
		}
	}
	if err := c.check([]byte(data)); err != nil {
		t.Errorf("a board refuses the map: %v", err)
	}
}

func TestStraighten_LeadsASlantingStretchInStepsWithinFollowOfIt(t *testing.T) {
	var run [][2]int
	for x := 0; x <= 40; x++ {
		run = append(run, [2]int{x, x * 9 / 40})
	}
	if one := straighten(run, 1.5, 0); len(one) != 1 {
		t.Fatalf("led through %v with no following, want one turn", one)
	}
	via := straighten(run, 1.5, 1.5)
	if len(via) < 3 {
		t.Fatalf("led through %v, want steps", via)
	}
	turns := append(append([][2]int{run[0]}, via...), run[len(run)-1])
	for i := 1; i < len(turns); i++ {
		dx, dy := turns[i][0]-turns[i-1][0], turns[i][1]-turns[i-1][1]
		if dx != 0 && dy != 0 && abs(dx) != abs(dy) {
			t.Errorf("the stretch from %v to %v runs along none of the grid's ways", turns[i-1],
				turns[i])
		}
		if d := off(run, turns[i]); d > 2.5 {
			t.Errorf("the turn at %v lies %.1f cells off the line", turns[i], d)
		}
	}
}

// the bridge's way ends on the ravine's floor, where heights coarser than the town put its
// abutment: the road still lies on one line from tunnel to tunnel
func TestStreets_ATunnelABridgeAndATunnelLieOnOneGradeLine(t *testing.T) {
	r, err := parseRecipe([]byte(`
area: {centre: [10, 40], cells: [40, 30], cell: 10, units: 32, figures: 1}
kinds: {road: asphalt, bridge: beam}
ground: {rock: {slope: 2}}
rivers: {brook: 1000, stream: 2000, river: 3000}
roads: {source: osm, lay: profile, classes: [residential], slope: 8, lane: 3, verge: 0.5, thickness: 1}
tunnels: {clear: 4.5, width: 8, cover: 1, grade: 0.1, longest: 100, reshape: 12, follow: 1.5}
`), "town.yaml")
	if err != nil {
		t.Fatal(err)
	}
	// a plain at 100 m, a hill 20 m high either side and a ravine 40 m deep between them
	height := func(lon, lat float64) (float64, error) {
		east, _ := metresOf(r.Area.Centre, lon, lat)
		hill := 20 * math.Max(1-math.Abs(math.Abs(east)-105)/45, 0)
		ravine := 40 * math.Max(1-math.Abs(east)/30, 0)
		return 100 + hill - ravine, nil
	}
	at := func(x, y float64) [2]float64 {
		lon, lat := lonLatOf(r.Area.Centre, x, y)
		return [2]float64{lon, lat}
	}
	osm := func([4]float64, []string, bool) (*osmData, error) {
		return &osmData{ways: []osmWay{
			{id: 1, lanes: 2, line: [][2]float64{at(-190, 5), at(-150, 5)}},
			{id: 2, lanes: 2, tunnel: true, line: [][2]float64{at(-150, 5), at(-60, 5)}},
			{id: 3, lanes: 2, bridge: true, line: [][2]float64{at(-60, 5), at(20, 5)}},
			{id: 4, lanes: 2, tunnel: true, line: [][2]float64{at(20, 5), at(150, 5)}},
			{id: 5, lanes: 2, line: [][2]float64{at(150, 5), at(190, 5)}},
		}}, nil
	}
	c, err := build(r, height, osm, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	s := c.roads.streets
	if len(c.roads.tunnels) != 2 || len(s.spans) != 1 {
		t.Fatalf("%d tunnels and %d bridges, want a tunnel either side of one bridge: %s",
			len(c.roads.tunnels), len(s.spans), c.roads.summary())
	}
	for _, tn := range c.roads.tunnels {
		for _, h := range [2]float64{c.level(tn.from), c.level(tn.to)} {
			if math.Abs(h-100) > 6 {
				t.Errorf("a tunnel's head lies at %.1f, off the line at 100", h)
			}
		}
	}
	v := s.spans[0]
	for _, h := range []float64{s.lot(v.cells[0]).h, s.lot(v.cells[len(v.cells)-1]).h} {
		if math.Abs(h-100) > 6 {
			t.Errorf("the bridge's head lies at %.1f, off the line at 100", h)
		}
	}
	for _, h := range [2]int{0, len(v.cells) - 1} {
		if at := s.lot(v.cells[h]).at; v.at[h] != at {
			t.Errorf("the deck begins at %v in its head, its street's axis passes %v", v.at[h], at)
		}
	}
	// from wall to wall: the ravine is 60 m wide where it lies 3 m under the line
	if n := len(v.cells); n < 6 {
		t.Errorf("the bridge spans %d cells, short of the ravine's far wall", n)
	}
	for _, l := range s.lots {
		if l.set && l.h < 90 {
			t.Errorf("a street lies at %.1f, down in the ravine", l.h)
		}
	}
	data := string(c.text("town.yaml", ""))
	if err := c.check([]byte(data)); err != nil {
		t.Errorf("a board refuses the map: %v", err)
	}
}

// a rise of 4 m over a tunnel's way closes over no roof: the road dips under it
func TestStreets_ATunnelsWayUnderShallowGroundIsSunkToATunnel(t *testing.T) {
	r, err := parseRecipe([]byte(`
area: {centre: [10, 40], cells: [40, 30], cell: 10, units: 32, figures: 1}
kinds: {road: asphalt, bridge: beam}
ground: {rock: {slope: 2}}
rivers: {brook: 1000, stream: 2000, river: 3000}
roads: {source: osm, lay: profile, classes: [residential], slope: 8, lane: 3, verge: 0.5, thickness: 1}
tunnels: {clear: 4.5, width: 8, cover: 1, grade: 0.1, longest: 100, reshape: 12, follow: 1.5}
`), "town.yaml")
	if err != nil {
		t.Fatal(err)
	}
	height := func(lon, lat float64) (float64, error) {
		east, _ := metresOf(r.Area.Centre, lon, lat)
		return 100 + 4*math.Max(1-math.Abs(east)/120, 0), nil
	}
	at := func(x, y float64) [2]float64 {
		lon, lat := lonLatOf(r.Area.Centre, x, y)
		return [2]float64{lon, lat}
	}
	osm := func([4]float64, []string, bool) (*osmData, error) {
		return &osmData{ways: []osmWay{
			{id: 1, lanes: 2, line: [][2]float64{at(-190, 5), at(-120, 5)}},
			{id: 2, lanes: 2, tunnel: true, line: [][2]float64{at(-120, 5), at(120, 5)}},
			{id: 3, lanes: 2, line: [][2]float64{at(120, 5), at(190, 5)}},
		}}, nil
	}
	c, err := build(r, height, osm, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if n := len(c.roads.tunnels); n != 1 {
		t.Fatalf("%d tunnels under the rise, want one: %s", n, c.roads.summary())
	}
	tn := c.roads.tunnels[0]
	if n := len(tn.plan.Cells); n < sagLeast {
		t.Errorf("the tunnel runs under %d cells, %d at least", n, sagLeast)
	}
	for _, h := range [2]float64{c.level(tn.from), c.level(tn.to)} {
		if h > 99.5 || h < 100-sagMost-0.1 {
			t.Errorf("a head lies at %.1f: sunk under the plain at 100, %.1f at most", h, sagMost)
		}
	}
	if err := c.check(c.text("town.yaml", "")); err != nil {
		t.Errorf("a board refuses the map: %v", err)
	}
}
