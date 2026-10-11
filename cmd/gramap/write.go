package main

import (
	"bytes"
	"cmp"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/water"
)

// attribution is what the heights come from, at the head of every map.
const attribution = `# Made by cmd/gramap. Heights: Terrain Tiles on AWS (Mapzen's Tilezen
# terrarium) — SRTM, EU-DEM, GMTED2010, ETOPO1 and others; see
# https://github.com/tilezen/joerd/blob/master/docs/attribution.md
`

// text is the map as a level file a board lays, units its units: section as it was, if any.
func (w *chart) text(recipePath, units string) []byte {
	var b bytes.Buffer
	b.WriteString(attribution)
	fmt.Fprintf(&b, "# From %s", recipePath)
	if w.flat {
		b.WriteString(", flat")
	}
	b.WriteString(".\n")
	w.writeBoard(&b)
	if !w.flat {
		w.writeRelief(&b)
	}
	w.writeWays(&b)
	b.WriteString(units)
	return b.Bytes()
}

var legend = []struct {
	key  string
	kind func(k kinds) string
}{
	{"~", func(k kinds) string { return k.Sea }},
	{".", func(k kinds) string { return k.Earth }},
	{":", func(k kinds) string { return k.Sand }},
	{"^", func(k kinds) string { return k.Rock }},
	{"*", func(k kinds) string { return k.Forest }},
}

func (w *chart) writeBoard(b *bytes.Buffer) {
	k := w.r.Kinds
	fmt.Fprintf(b, "board:\n  default: %s\n  legend:\n", k.Sea)
	keys := map[string]string{}
	for _, l := range legend {
		if kind := l.kind(k); kind != "" && (kind == k.Sea || slices.Contains(w.soils, kind)) {
			fmt.Fprintf(b, "    %q: %s\n", l.key, kind)
			keys[kind] = l.key
		}
	}
	b.WriteString("  cells: |\n")
	for y := range w.rows {
		b.WriteString("    ")
		for x := range w.cols {
			kind := k.Sea
			if w.land[y*w.cols+x] {
				kind = w.soils[y*w.cols+x]
			}
			b.WriteString(keys[kind])
		}
		b.WriteString("\n")
	}
	stands := w.standing()
	if len(stands) == 0 {
		return
	}
	b.WriteString("  entities:\n")
	for _, s := range stands {
		x, y := w.xy(s.at)
		fmt.Fprintf(b, "    - {kind: %s, at: [%d, %d]", s.kind, x, y)
		if s.height > 0 {
			fmt.Fprintf(b, ", height: %s", number(s.height))
		}
		fmt.Fprintf(b, ", way: [%d, %d]}\n", steps[s.way][0], steps[s.way][1])
	}
}

type standing struct {
	kind   string
	at     cell.ID
	height float64 // metres over the cell's level
	way    int
}

// standing is the bridges and fords, the cells of the roads' crossings and every ford of the
// rivers besides, across the river.
func (w *chart) standing() []standing {
	var out []standing
	taken := map[cell.ID]bool{}
	for _, s := range w.roads.live() {
		for _, c := range s.cells {
			st := standing{kind: w.r.Kinds.Bridge, at: c, way: s.way}
			if s.ford {
				st.kind = w.r.Kinds.Ford
			} else if !w.flat {
				st.height = max(s.deck-w.level(c), 0)
			}
			out = append(out, st)
			taken[c] = true
		}
	}
	for c, course := range w.rivers.Courses {
		if course != water.Ford || taken[c] {
			continue
		}
		way := squareTo[w.roads.wayOf(c, w.rivers.Down[c])]
		out = append(out, standing{kind: w.r.Kinds.Ford, at: c, way: way})
	}
	slices.SortFunc(out, func(a, b standing) int { return cmp.Compare(a.at, b.at) })
	return out
}

var squareTo = [8]int{3, 3, 0, 0, 5, 4, 4, 5}

func (w *chart) writeRelief(b *bytes.Buffer) {
	b.WriteString("relief: # metres\n  corners: |\n")
	for y := range w.rows + 1 {
		b.WriteString("    ")
		for x := range w.cols + 1 {
			if x > 0 {
				b.WriteByte(' ')
			}
			b.WriteString(number(w.corners[y*(w.cols+1)+x]))
		}
		b.WriteString("\n")
	}
}

func (w *chart) writeWays(b *bytes.Buffer) {
	b.WriteString("ways:\n")
	if len(w.roads.laid) > 0 {
		fmt.Fprintf(b, "  roads:\n    %s:\n      width: %s\n      paths:\n", w.r.Kinds.Road,
			number(w.roads.cfg.Width))
		for _, rd := range w.roads.laid {
			for _, seg := range segments(rd) {
				var items []string
				for i, at := range w.turns(seg.cells) {
					xy := w.point(seg.cells[at])
					if i == 0 && seg.first || at == len(seg.cells)-1 && seg.last {
						xy = "{onto: " + xy + "}"
					}
					items = append(items, xy)
				}
				writeList(b, "        - ", items)
			}
		}
	}
	rivers := w.riverPaths()
	if len(rivers) == 0 {
		return
	}
	b.WriteString("  rivers:\n")
	k := w.r.Kinds
	kinds := map[water.Course]string{
		water.Brook: k.Brook, water.Stream: k.Stream, water.River: k.River, water.Ford: k.River,
	}
	for _, path := range rivers {
		var items []string
		kind := ""
		turns := w.turns(path)
		for _, at := range turns {
			c := path[at]
			if at == len(path)-1 {
				items = append(items, w.point(c))
				break
			}
			item := "{at: " + w.point(c)
			if kk := kinds[w.rivers.Courses[c]]; kk != kind {
				kind = kk
				item += ", kind: " + kind
			}
			item += ", width: " + number(w.rivers.Width(c, w.size)) + "}"
			items = append(items, item)
		}
		writeList(b, "    - ", items)
	}
}

// segment is a stretch of a road's path between crossings: its cells, and whether it leads onto
// a crossing at its first cell or its last.
type segment struct {
	cells       []cell.ID
	first, last bool
}

func segments(rd road) []segment {
	var out []segment
	start := 0
	for i := 1; i < len(rd.cells); i++ {
		if rd.over[i] && !rd.over[i-1] {
			out = append(out, segment{cells: rd.cells[start : i+1], first: start > 0, last: true})
		}
		if !rd.over[i] && rd.over[i-1] {
			start = i - 1
		}
	}
	return append(out, segment{cells: rd.cells[start:], first: start > 0})
}

// turns is where cells turn — the first, every cell the way on changes at, the last — and where
// a river changes its kind: the points a level file routes along to come back to every cell.
func (w *chart) turns(cells []cell.ID) []int {
	out := []int{0}
	way := func(i int) cell.Links { l, _ := grid.Link(w.grid, cells[i], cells[i+1]); return l }
	for i := 1; i < len(cells)-1; i++ {
		kind := w.rivers.Courses[cells[i]] != w.rivers.Courses[cells[i-1]] && w.wet(cells[i])
		if way(i) != way(i-1) || kind {
			out = append(out, i)
		}
	}
	return append(out, len(cells)-1)
}

func (w *chart) point(c cell.ID) string {
	x, y := w.xy(c)
	return fmt.Sprintf("[%d, %d]", x, y)
}

// riverPaths is the running water as paths from every source down to where it goes — the sea, or
// a river taken before — the longest first, so the main stems run whole.
func (w *chart) riverPaths() [][]cell.ID {
	n := w.rivers
	fed := map[cell.ID]bool{}
	for c, course := range n.Courses {
		if course != water.Dry {
			fed[n.Down[c]] = true
		}
	}
	length := func(c cell.ID) int {
		l := 0
		for ; n.Courses[c] != water.Dry; c = n.Down[c] {
			l++
		}
		return l
	}
	var sources []cell.ID
	for c, course := range n.Courses {
		if course != water.Dry && !fed[c] {
			sources = append(sources, c)
		}
	}
	slices.SortFunc(sources, func(a, b cell.ID) int {
		return cmp.Or(cmp.Compare(length(b), length(a)), cmp.Compare(a, b))
	})
	taken := map[cell.ID]bool{}
	var out [][]cell.ID
	for _, c := range sources {
		path := []cell.ID{c}
		for taken[c] = true; ; {
			c = n.Down[c]
			path = append(path, c)
			if n.Courses[c] == water.Dry || taken[c] {
				break
			}
			taken[c] = true
		}
		out = append(out, path)
	}
	return out
}

func writeList(b *bytes.Buffer, lead string, items []string) {
	indent := strings.Repeat(" ", len(lead)+1)
	line := lead + "["
	for i, it := range items {
		tail := ", "
		if i == len(items)-1 {
			tail = "]"
		}
		if len(line)+len(it)+len(strings.TrimSpace(tail)) > 100 && len(line) > len(indent) {
			b.WriteString(strings.TrimRight(line, " ") + "\n")
			line = indent
		}
		line += it + tail
	}
	b.WriteString(line + "\n")
}

// number is v in metres to the centimetre, as short as it goes.
func number(v float64) string {
	v = math.Round(v*100) / 100
	if v == 0 {
		return "0"
	}
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// unitsOf is the units: section of a level file, kept from one making of a map to the next.
func unitsOf(data []byte) string {
	lines := strings.SplitAfter(string(data), "\n")
	for i, l := range lines {
		if !strings.HasPrefix(l, "units:") {
			continue
		}
		end := i + 1
		for end < len(lines) && (lines[end] == "" || strings.HasPrefix(lines[end], " ") ||
			strings.HasPrefix(lines[end], "#") || lines[end] == "\n") {
			end++
		}
		return strings.Join(lines[i:end], "")
	}
	return ""
}
