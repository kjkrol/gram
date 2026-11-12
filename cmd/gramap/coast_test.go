package main

import "testing"

func TestShore_AHarboursWaterIsTheSeasBehindItsBreakwaters(t *testing.T) {
	r, err := parseRecipe([]byte(`
area: {centre: [10, 40], cells: [40, 30], cell: 10, units: 32, figures: 1}
coast: {source: osm, quay: 1.5, pontoon: pontoon}
ground: {rock: {slope: 2}}
rivers: {brook: 1000, stream: 2000, river: 3000}
`), "port.yaml")
	if err != nil {
		t.Fatal(err)
	}
	// the heights know no harbour: land a hand over the sea everywhere
	height := func(lon, lat float64) (float64, error) { return 0.2, nil }
	at := func(x, y float64) [2]float64 {
		lon, lat := lonLatOf(r.Area.Centre, x, y)
		return [2]float64{lon, lat}
	}
	ring := func(x0, y0, x1, y1 float64) [][2]float64 {
		return [][2]float64{at(x0, y0), at(x1, y0), at(x1, y1), at(x0, y1), at(x0, y0)}
	}
	osm := func([4]float64, []string, bool) (*osmData, error) {
		return &osmData{coast: &osmCoast{
			// eastwards, 50 m south of the middle: the land on its left, to the north
			shore: [][][2]float64{{at(-250, 50), at(250, 50)}},
			// a basin 180 m by 50 m behind a breakwater a cell thick, its mouth 4 m wide
			moles: []mole{
				{breakwater: true, ring: ring(-100, 100, 100, 110)},
				{breakwater: true, ring: ring(-100, 50, -90, 100)},
				{breakwater: true, ring: ring(90, 50, 100, 96)},
			},
			// a pier from the quay 40 m out into the basin, and one too short to be worth it
			piers: [][][2]float64{
				{at(3, 45), at(3, 88)},
				{at(40, 52), at(40, 62)},
			},
		}}, nil
	}
	c, err := build(r, height, osm, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	cell := func(x, y float64) int {
		cx, cy := c.place(at(x, y)[0], at(x, y)[1])
		return int(cy)*c.cols + int(cx)
	}
	for name, at := range map[string]int{
		"north of the shore": cell(0, 20),
		"the breakwater":     cell(0, 105),
		"its western arm":    cell(-95, 75),
	} {
		if !c.land[at] {
			t.Errorf("%s is the sea's, want it land", name)
		}
	}
	if got := c.soils[cell(0, 105)]; got != r.Kinds.Rock {
		t.Errorf("the breakwater is %q, want rock", got)
	}
	for name, at := range map[string]int{
		"the basin":    cell(0, 75),
		"the open sea": cell(0, 140),
	} {
		if c.land[at] {
			t.Errorf("%s is land, want it the sea's", name)
		}
	}
	// the mouth is narrower than a cell: a cell is opened, the basin no puddle
	seen := map[int]bool{cell(0, 140): true}
	stack := []int{cell(0, 140)}
	for len(stack) > 0 {
		i := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		for _, to := range c.beside(i) {
			if !c.land[to] && !seen[to] {
				seen[to] = true
				stack = append(stack, to)
			}
		}
	}
	if !seen[cell(0, 75)] {
		t.Error("the basin's water is cut off from the sea, want a way a cell wide into it")
	}
	for i, land := range c.land {
		if !land && !seen[i] {
			t.Fatalf("the cell %d, %d is water cut off from the sea", i%c.cols, i/c.cols)
		}
	}
	// the quay: the land by the water lies level over it
	k := c.corner(c.grid.CellCenter(c.cell(20, 5)))
	if h := c.corners[k]; h != 1.5 {
		t.Errorf("the land lies at %v, want the quay's 1.5 over ground the heights have lower", h)
	}
	piers := c.pontoons()
	if len(piers) != 1 {
		t.Fatalf("%d piers, want the one 40 m long", len(piers))
	}
	p := piers[0]
	first, last := p.cells[0], p.cells[len(p.cells)-1]
	if !c.land[first[1]*c.cols+first[0]] || c.land[last[1]*c.cols+last[0]] || len(p.cells) != 5 {
		t.Errorf("the pier runs %v, want from a cell of the quay over four of the basin's", p.cells)
	}
}
