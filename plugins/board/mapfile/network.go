package mapfile

import (
	"math"
	"sort"

	"github.com/kjkrol/aabbworld/geom"
)

// Road is a road's middle line from one meeting or end to the next, of one highway.
type Road struct {
	Highway string
	Path    []geom.Vec
}

// Meeting is where three or more roads end: the ways they leave it, in the order that turns
// left as the world lies.
type Meeting struct {
	At   geom.Vec
	Ways []geom.Vec
}

// a point of the lines to a centimetre: lines sharing one meet there
type node [2]int32

func nodeOf(v geom.Vec) node {
	return node{int32(math.Round(float64(v.X) * 100)), int32(math.Round(float64(v.Y) * 100))}
}

// Network is the level's lines tagged highway as roads and the meetings they end at: a line
// cut where another shares a point within it, two of one highway joined where they alone end.
func (l *File) Network() ([]Road, []Meeting) {
	type piece struct {
		highway string
		path    []geom.Vec
	}
	uses := map[node]int{}
	var lines []piece
	for _, f := range l.Features {
		if f.Shape != Line || f.Tags["highway"] == "" {
			continue
		}
		lines = append(lines, piece{f.Tags["highway"], f.Path})
		for _, v := range f.Path {
			uses[nodeOf(v)]++
		}
	}
	var cut []piece
	for _, line := range lines {
		from := 0
		for k := 1; k < len(line.path); k++ {
			if k == len(line.path)-1 || uses[nodeOf(line.path[k])] > 1 {
				cut = append(cut, piece{line.highway, line.path[from : k+1]})
				from = k
			}
		}
	}
	// the pieces ending at a point, twice one that closes on itself
	ends := map[node][]int{}
	for i, p := range cut {
		for _, v := range []geom.Vec{p.path[0], p.path[len(p.path)-1]} {
			ends[nodeOf(v)] = append(ends[nodeOf(v)], i)
		}
	}
	at := make([]node, 0, len(ends))
	for n := range ends {
		at = append(at, n)
	}
	sort.Slice(at, func(i, j int) bool {
		return at[i][0] < at[j][0] || at[i][0] == at[j][0] && at[i][1] < at[j][1]
	})
	gone := make([]bool, len(cut))
	for _, n := range at {
		two := ends[n]
		if len(two) != 2 || two[0] == two[1] || cut[two[0]].highway != cut[two[1]].highway {
			continue
		}
		a, b := cut[two[0]].path, cut[two[1]].path
		if nodeOf(a[len(a)-1]) != n {
			a = turned(a)
		}
		if nodeOf(b[0]) != n {
			b = turned(b)
		}
		joined := append(append([]geom.Vec{}, a...), b[1:]...)
		cut[two[0]].path, gone[two[1]] = joined, true
		for _, far := range []geom.Vec{joined[0], joined[len(joined)-1]} {
			for k, i := range ends[nodeOf(far)] {
				if i == two[1] {
					ends[nodeOf(far)][k] = two[0]
				}
			}
		}
	}
	var roads []Road
	for i, p := range cut {
		if !gone[i] {
			roads = append(roads, Road{Highway: p.highway, Path: p.path})
		}
	}
	type arm struct {
		way   geom.Vec
		angle float64
	}
	arms := map[node][]arm{}
	place := map[node]geom.Vec{}
	for _, r := range roads {
		n := len(r.Path)
		for _, e := range [][2]geom.Vec{{r.Path[0], r.Path[1]}, {r.Path[n-1], r.Path[n-2]}} {
			k := nodeOf(e[0])
			if len(ends[k]) < 3 {
				continue
			}
			dx, dy := float64(e[1].X-e[0].X), float64(e[1].Y-e[0].Y)
			d := math.Hypot(dx, dy)
			way := geom.NewVec(float32(dx/d), float32(dy/d))
			arms[k] = append(arms[k], arm{way, math.Atan2(dy, dx)})
			place[k] = e[0]
		}
	}
	var meetings []Meeting
	for _, n := range at {
		all, ok := arms[n]
		if !ok {
			continue
		}
		sort.SliceStable(all, func(i, j int) bool { return all[i].angle < all[j].angle })
		m := Meeting{At: place[n]}
		for _, a := range all {
			m.Ways = append(m.Ways, a.way)
		}
		meetings = append(meetings, m)
	}
	return roads, meetings
}

func turned(pts []geom.Vec) []geom.Vec {
	out := make([]geom.Vec, len(pts))
	for i, p := range pts {
		out[len(pts)-1-i] = p
	}
	return out
}
