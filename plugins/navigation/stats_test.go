package navigation

import (
	"fmt"
	"testing"

	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/navigation/internal/pathfind"
)

// a party's routes off its field and a search of the traffic's own: the counts are the routers'
// and the traffic's together
func TestNavigation_CountsAreTheRoutersSum(t *testing.T) {
	r := newGroupRig(t, nil, westOf(8))
	read := r.nw.nav.Read(0)
	if got := read.Counts(); got != (Counts{}) {
		t.Fatalf("counts %+v before any route, want none", got)
	}
	g := r.nw.grid
	r.send(1, g.At(26, 16), every(8)...)
	r.update()
	traffic := r.nw.nav.finder
	if _, ok := traffic.FindPath(r.nw.ids[0], cell.Land, pathfind.Body{}, g.At(2, 6),
		g.At(2, 28), EitherLevel); !ok {
		t.Fatal("the traffic's finder found no route")
	}
	c := r.coord
	want := Counts{
		Searches:   c.Searches() + traffic.Searches(),
		Walks:      c.Walks() + traffic.Walks(),
		Fills:      c.Fills() + traffic.Fills(),
		Climbs:     c.Climbs() + traffic.Climbs(),
		Builds:     c.Builds() + traffic.Builds(),
		Expansions: c.Expansions() + traffic.Expansions(),
	}
	got := read.Counts()
	if got != want {
		t.Errorf("counts %+v, want the routers' and the traffic's together, %+v", got, want)
	}
	if got.Fills != 1 || got.Walks == 0 || got.Searches <= c.Searches() ||
		got.Expansions < got.Searches {
		t.Errorf("counts %+v, want a field filled and walked, the traffic's search among the "+
			"searches, every search expanding a node at least", got)
	}
}

func TestNavigation_TheReporterLinesTheCounts(t *testing.T) {
	r := newGroupRig(t, nil, westOf(8))
	r.send(1, r.nw.grid.At(26, 16), every(8)...)
	r.update()
	read := r.nw.nav.Read(0)
	c := read.Counts()
	var lines []string
	read.Reporter().Report(func(label, value string) { lines = append(lines, label+": "+value) })
	want := fmt.Sprintf("Routes: searches %d, climbs %d, walks %d, fills %d, builds %d, "+
		"expansions %d", c.Searches, c.Climbs, c.Walks, c.Fills, c.Builds, c.Expansions)
	if len(lines) != 1 || lines[0] != want {
		t.Errorf("the reporter says %q, want %q", lines, want)
	}
}
