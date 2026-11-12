package main

import (
	"bytes"
	"container/heap"
	"fmt"
	"math"
	"slices"
)

// coastCfg is where a map's shore comes from: OpenStreetMap's coastline, breakwaters and
// piers in place of the heights' zero, which a town's harbour is too fine for; the land by
// the water no lower than Quay metres, a pier's cells of the kind Pier (a breakwater's are
// rock)
type coastCfg struct {
	Source string  `yaml:"source"`
	Quay   float64 `yaml:"quay"`
	Pier   string  `yaml:"pier"`
	// the kind of bridge a pier drawn as a line is written as, none where empty
	Pontoon string `yaml:"pontoon"`
}

func (c coastCfg) validate() error {
	if c.Source != "" && c.Source != "osm" {
		return fmt.Errorf("coast: the source is %q, want osm or none", c.Source)
	}
	return nil
}

// osmCoast is a shore as OpenStreetMap has it, longitude and latitude: the coastline's ways,
// the land on the left of each as it runs; what is built into the sea, outlined; and the
// piers drawn as lines, too narrow for an outline
type osmCoast struct {
	shore [][][2]float64
	moles []mole
	piers [][][2]float64
}

type mole struct {
	breakwater bool
	ring       [][2]float64
}

// coast is the shore within the box south, west, north, east
func (o *overpass) coast(box [4]float64) (*osmCoast, error) {
	q := fmt.Sprintf("[out:json][timeout:180][bbox:%s];(way[natural=coastline];"+
		"way[man_made~\"^(breakwater|pier|groyne|quay)$\"];);out tags geom;", boxOf(box))
	var answer struct {
		Elements []osmElement `json:"elements"`
	}
	if err := o.answer(q, &answer); err != nil {
		return nil, err
	}
	slices.SortFunc(answer.Elements, func(a, b osmElement) int { return int(a.ID - b.ID) })
	c := &osmCoast{}
	for _, e := range answer.Elements {
		line := e.line()
		if len(line) < 2 {
			continue
		}
		closed := len(line) >= 4 && line[0] == line[len(line)-1]
		switch kind := e.Tags["man_made"]; {
		case e.Tags["natural"] == "coastline":
			c.shore = append(c.shore, line)
		case closed:
			c.moles = append(c.moles, mole{
				breakwater: kind == "breakwater" || kind == "groyne",
				ring:       line,
			})
		case kind == "pier":
			c.piers = append(c.piers, line)
		}
	}
	return c, nil
}

// how many squares a cell's side is cut in to tell the land from the sea, and the share of a
// cell a mole covers that makes the cell its own
const (
	shoreFine = 5
	moleShare = 0.25
)

// shore has the chart's land lie where the coast c has it: the sea is what the water reaches
// from the right of the coastline's ways without crossing one, a mole is land, and no water
// is left cut off from the rest, a way a cell wide opened to it where the cells closed one
// narrower. False where c has no coastline in the chart, the land left as the heights have it.
func (w *chart) shore(c *osmCoast) bool {
	w.coast = c
	fw, fh := w.cols*shoreFine, w.rows*shoreFine
	wall, sea := make([]bool, fw*fh), make([]bool, fw*fh)
	built := make([]uint8, fw*fh)
	at := func(p [2]float64) (float64, float64) {
		x, y := w.place(p[0], p[1])
		return x * shoreFine, y * shoreFine
	}
	inside := func(x, y int) bool { return x >= 0 && y >= 0 && x < fw && y < fh }
	for _, way := range c.shore {
		for i := 1; i < len(way); i++ {
			ax, ay := at(way[i-1])
			bx, by := at(way[i])
			n := int(math.Ceil(math.Hypot(bx-ax, by-ay)*2)) + 1
			for k := 0; k <= n; k++ {
				t := float64(k) / float64(n)
				x, y := int(math.Floor(ax+(bx-ax)*t)), int(math.Floor(ay+(by-ay)*t))
				if inside(x, y) {
					wall[y*fw+x] = true
				}
			}
		}
	}
	// the coastline cuts the chart in parts: a part is the sea's where more of the ways along
	// it have it on their right, east of one running north, than on their left; one way's
	// side alone may fall across a spur narrower than the squares
	part := make([]int32, fw*fh)
	parts := int32(0)
	for i := range part {
		if wall[i] || part[i] != 0 {
			continue
		}
		parts++
		part[i] = parts
		stack := []int{i}
		for len(stack) > 0 {
			k := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			x, y := k%fw, k/fw
			for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
				nx, ny := x+d[0], y+d[1]
				if inside(nx, ny) && !wall[ny*fw+nx] && part[ny*fw+nx] == 0 {
					part[ny*fw+nx] = parts
					stack = append(stack, ny*fw+nx)
				}
			}
		}
	}
	votes := make([]float64, parts+1)
	for _, way := range c.shore {
		for i := 1; i < len(way); i++ {
			ax, ay := at(way[i-1])
			bx, by := at(way[i])
			l := math.Hypot(bx-ax, by-ay)
			if l == 0 {
				continue
			}
			for _, side := range [2]float64{1, -1} {
				x := int(math.Floor((ax+bx)/2 - side*(by-ay)/l*1.5))
				y := int(math.Floor((ay+by)/2 + side*(bx-ax)/l*1.5))
				if inside(x, y) && !wall[y*fw+x] {
					votes[part[y*fw+x]] += side * l
				}
			}
		}
	}
	any := false
	for i := range sea {
		if !wall[i] && votes[part[i]] > 0 {
			sea[i], any = true, true
		}
	}
	if !any {
		return false
	}
	for _, m := range c.moles {
		kind := uint8(1)
		if m.breakwater {
			kind = 2
		}
		ring := make([][2]float64, len(m.ring))
		for i, p := range m.ring {
			ring[i][0], ring[i][1] = at(p)
		}
		for y := range fh {
			var xs []float64
			level := float64(y) + 0.5
			for i := 1; i < len(ring); i++ {
				a, b := ring[i-1], ring[i]
				if (a[1] <= level) != (b[1] <= level) {
					xs = append(xs, a[0]+(b[0]-a[0])*(level-a[1])/(b[1]-a[1]))
				}
			}
			slices.Sort(xs)
			for i := 1; i < len(xs); i += 2 {
				from, to := int(math.Ceil(xs[i-1]-0.5)), int(math.Floor(xs[i]-0.5))
				for x := max(from, 0); x <= min(to, fw-1); x++ {
					built[y*fw+x] = kind
				}
			}
		}
	}
	// a cell is the sea's where the water has most of it, a mole's where the mole has enough
	// of it to stand unbroken
	share := make([]float64, w.cols*w.rows)
	w.moles = map[int]bool{}
	const squares = shoreFine * shoreFine
	for cy := range w.rows {
		for cx := range w.cols {
			wet, piers, breakwaters := 0, 0, 0
			for dy := range shoreFine {
				for dx := range shoreFine {
					i := (cy*shoreFine+dy)*fw + cx*shoreFine + dx
					switch {
					case built[i] == 1:
						piers++
					case built[i] == 2:
						breakwaters++
					case sea[i]:
						wet++
					}
				}
			}
			i := cy*w.cols + cx
			share[i] = float64(wet) / squares
			w.land[i] = wet*2 <= squares
			if float64(piers+breakwaters) >= moleShare*squares {
				w.land[i] = true
				w.moles[i] = breakwaters > piers
			}
		}
	}
	w.rejoin(share)
	return true
}

// rejoin opens a way a cell wide from every water the cells cut off to the widest water, by
// the cells the sea has most of: a harbour's mouth narrower than a cell stays a mouth
func (w *chart) rejoin(share []float64) {
	of := make([]int, len(w.land))
	var sizes []int
	for i := range of {
		of[i] = -1
	}
	for i := range w.land {
		if w.land[i] || of[i] >= 0 {
			continue
		}
		id, stack := len(sizes), []int{i}
		of[i] = id
		n := 0
		for len(stack) > 0 {
			c := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			n++
			for _, to := range w.beside(c) {
				if !w.land[to] && of[to] < 0 {
					of[to] = id
					stack = append(stack, to)
				}
			}
		}
		sizes = append(sizes, n)
	}
	if len(sizes) < 2 {
		return
	}
	main := 0
	for id, n := range sizes {
		if n > sizes[main] {
			main = id
		}
	}
	for id := range sizes {
		if id == main {
			continue
		}
		cost := make([]float64, len(w.land))
		from := make([]int, len(w.land))
		for i := range cost {
			cost[i], from[i] = math.Inf(1), -1
		}
		open := &reach{}
		for i := range w.land {
			if of[i] == id {
				cost[i] = 0
				heap.Push(open, reached{0, i})
			}
		}
		for open.Len() > 0 {
			r := heap.Pop(open).(reached)
			if r.cost > cost[r.at] {
				continue
			}
			if of[r.at] == main {
				for c := r.at; c >= 0 && of[c] != id; c = from[c] {
					w.land[c], of[c] = false, main
					delete(w.moles, c)
				}
				break
			}
			for _, to := range w.beside(r.at) {
				step := 1.0
				if w.land[to] {
					step += 20 * (1 - share[to])
				}
				if c := r.cost + step; c < cost[to] {
					cost[to], from[to] = c, r.at
					heap.Push(open, reached{c, to})
				}
			}
		}
		for i := range of {
			if of[i] == id {
				of[i] = main
			}
		}
	}
}

// beside are the cells sharing a side with the cell i
func (w *chart) beside(i int) []int {
	x, y := i%w.cols, i/w.cols
	out := make([]int, 0, 4)
	if x > 0 {
		out = append(out, i-1)
	}
	if x < w.cols-1 {
		out = append(out, i+1)
	}
	if y > 0 {
		out = append(out, i-w.cols)
	}
	if y < w.rows-1 {
		out = append(out, i+w.cols)
	}
	return out
}

type reached struct {
	cost float64
	at   int
}

type reach []reached

func (r reach) Len() int { return len(r) }
func (r reach) Less(i, j int) bool {
	if r[i].cost != r[j].cost {
		return r[i].cost < r[j].cost
	}
	return r[i].at < r[j].at
}
func (r reach) Swap(i, j int) { r[i], r[j] = r[j], r[i] }
func (r *reach) Push(x any)   { *r = append(*r, x.(reached)) }
func (r *reach) Pop() any {
	old := *r
	x := old[len(old)-1]
	*r = old[:len(old)-1]
	return x
}

// metres: the shortest pier drawn as a line that is written, and how wide one is
const (
	pontoonLeast = 25
	pontoonWide  = 3
)

// the cells round a cell, those sharing a side first
var round = [8][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}, {1, 1}, {-1, 1}, {1, -1}, {-1, -1}}

// pontoon is a pier too narrow for cells of its own: the cells it passes from the land out
// over the water, and where its line passes each, in cells
type pontoon struct {
	cells [][2]int
	at    [][2]float64
}

// pontoons are the piers OpenStreetMap draws as lines, each from a cell of land out over the
// water, one cell a pier: those shorter than a pier is worth, reaching no land, or crossing
// one laid already are left out
func (w *chart) pontoons() []pontoon {
	if w.coast == nil || w.r.Coast.Pontoon == "" {
		return nil
	}
	taken := map[[2]int]bool{}
	if s := w.roads.streets; s != nil {
		for _, v := range s.spans {
			for _, c := range v.cells {
				x, y := w.xy(c)
				taken[[2]int{x, y}] = true
			}
		}
	}
	land := func(c [2]int) bool {
		if c[0] < 0 || c[1] < 0 || c[0] >= w.cols || c[1] >= w.rows {
			return false
		}
		i := c[1]*w.cols + c[0]
		return w.land[i] && w.soils[i] != w.r.Buildings.Cells && !w.wet(w.cell(c[0], c[1]))
	}
	var out []pontoon
	for _, line := range w.coast.piers {
		var p pontoon
		long := 0.0
		for i := range line {
			x, y := w.place(line[i][0], line[i][1])
			if i == 0 {
				p.cells = [][2]int{{int(math.Floor(x)), int(math.Floor(y))}}
				p.at = [][2]float64{{x, y}}
				continue
			}
			px, py := w.place(line[i-1][0], line[i-1][1])
			l := math.Hypot(x-px, y-py)
			long += l * w.r.Area.Cell
			n := int(math.Ceil(l*8)) + 1
			for k := 1; k <= n; k++ {
				t := float64(k) / float64(n)
				qx, qy := px+(x-px)*t, py+(y-py)*t
				c := [2]int{int(math.Floor(qx)), int(math.Floor(qy))}
				if c != p.cells[len(p.cells)-1] {
					p.cells, p.at = append(p.cells, c), append(p.at, [2]float64{qx, qy})
				}
			}
		}
		if long < pontoonLeast || len(p.cells) < 2 {
			continue
		}
		// from the land: its landward end first, the cell of land beside it where it ends
		// short of the quay
		if !land(p.cells[0]) && land(p.cells[len(p.cells)-1]) {
			slices.Reverse(p.cells)
			slices.Reverse(p.at)
		}
		if !land(p.cells[0]) {
			for _, d := range round {
				c := [2]int{p.cells[0][0] + d[0], p.cells[0][1] + d[1]}
				if land(c) && (len(p.cells) < 2 || c != p.cells[1]) {
					p.cells = append([][2]int{c}, p.cells...)
					p.at = append([][2]float64{{float64(c[0]) + 0.5, float64(c[1]) + 0.5}}, p.at...)
					break
				}
			}
		}
		if !land(p.cells[0]) {
			continue
		}
		// out to where it comes to land again, or ends
		end := len(p.cells)
		for i := 1; i < len(p.cells); i++ {
			if w.land[p.cells[i][1]*w.cols+p.cells[i][0]] {
				end = i + 1
				if i == 1 {
					end = 0
				}
				break
			}
		}
		p.cells, p.at = p.cells[:end], p.at[:end]
		crosses := slices.ContainsFunc(p.cells, func(c [2]int) bool { return taken[c] })
		if len(p.cells) < 3 || crosses {
			continue
		}
		seen := map[[2]int]bool{}
		twice := false
		for _, c := range p.cells {
			twice = twice || seen[c]
			seen[c] = true
		}
		if twice {
			continue
		}
		for _, c := range p.cells {
			taken[c] = true
		}
		out = append(out, p)
	}
	return out
}

func (w *chart) writePontoons(b *bytes.Buffer, ps []pontoon) {
	for _, p := range ps {
		var path []string
		for i, c := range p.cells {
			path = append(path, fmt.Sprintf("[%d, %d, %s, %s]", c[0], c[1],
				number(math.Round((p.at[i][0]-float64(c[0])-0.5)*w.size*10)/10),
				number(math.Round((p.at[i][1]-float64(c[1])-0.5)*w.size*10)/10)))
		}
		writeList(b, fmt.Sprintf("    - {kind: %s, lanes: 0, width: %s, path: ", w.r.Coast.Pontoon,
			number(pontoonWide/w.perUnit)), path)
		b.Truncate(b.Len() - 1)
		b.WriteString("}\n")
	}
}
