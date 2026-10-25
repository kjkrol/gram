package site_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/bridges/site"
)

// valley is a square board 12 x 12 of cells 10 wide, the ground at 30 but for a valley at 0
// along the columns 4 to 6.
type valley struct {
	g       grid.Grid
	wet     map[cell.ID]bool
	bridged map[cell.ID]site.Part
	roads   map[cell.ID]cell.Links
	height  func(p geom.Vec) float32
}

func newValley() *valley {
	v := &valley{
		g:       grid.DefaultGrids{}.Square(12, 12, 10),
		wet:     map[cell.ID]bool{},
		bridged: map[cell.ID]site.Part{},
		roads:   map[cell.ID]cell.Links{},
	}
	v.height = func(p geom.Vec) float32 {
		if p.X > 40 && p.X < 70 {
			return 0
		}
		return 30
	}
	return v
}

func (v *valley) Grid() grid.Grid { return v.g }

func (v *valley) Heights(c cell.ID, dst []float32) []float32 {
	for _, p := range v.g.CellOutline(c, nil) {
		dst = append(dst, v.height(p))
	}
	return dst
}

func (v *valley) Takes(c cell.ID) bool                { return !v.wet[c] }
func (v *valley) Wet(c cell.ID) bool                  { return v.wet[c] }
func (v *valley) Road(c cell.ID) cell.Links           { return v.roads[c] }
func (v *valley) Bridged(c cell.ID) (site.Part, bool) { p, ok := v.bridged[c]; return p, ok }

var rules = site.Rules{Longest: 3, Under: 20, Clearance: 1, Reshape: 5, Heights: true}

func way(g grid.Grid, dx, dy int) int {
	i, _ := grid.Direction(g, dx, dy)
	return i
}

func TestPlan_ABridgeOverAValley(t *testing.T) {
	v := newValley()
	b, err := site.Plan(v, rules, v.g.At(3, 5), v.g.At(7, 5), 30)
	if err != nil {
		t.Fatal(err)
	}
	if b.Way != way(v.g, 1, 0) || len(b.Spans) != 3 || len(b.Piers) != 2 {
		t.Errorf("planned %+v, want three spans east and two piers", b)
	}
	if want := geom.NewVec(50, 55); b.Piers[0] != want {
		t.Errorf("the first pier stands at %v, want %v: between the first two spans", b.Piers[0],
			want)
	}
}

func TestPlan_RefusesWhatBreaksARule(t *testing.T) {
	g := newValley().g
	for name, tc := range map[string]struct {
		from, to cell.ID
		deck     float32
		setup    func(v *valley)
		why      string
	}{
		"too long":     {g.At(3, 5), g.At(8, 5), 30, nil, "spans 4 cells"},
		"off a line":   {g.At(3, 5), g.At(7, 6), 30, nil, "off every line"},
		"side by side": {g.At(3, 5), g.At(4, 5), 30, nil, "spans no cell"},
		"too shallow": {g.At(3, 5), g.At(7, 5), 30, func(v *valley) {
			v.height = func(p geom.Vec) float32 {
				if p.X > 40 && p.X < 70 {
					return 15
				}
				return 30
			}
		}, "crosses no water"},
		"over water however shallow": {g.At(3, 5), g.At(7, 5), 30, func(v *valley) {
			v.height = func(p geom.Vec) float32 {
				if p.X > 30 && p.X < 70 {
					return 25
				}
				return 30
			}
			v.wet[g.At(5, 5)] = true
		}, ""},
		"into the ground": {g.At(3, 5), g.At(7, 5), 10, func(v *valley) {
			v.height = func(p geom.Vec) float32 {
				if p.X > 40 && p.X < 70 {
					return 12
				}
				return 10
			}
		}, "the ground rises"},
		"one span over nothing": {g.At(3, 5), g.At(5, 5), 30, func(v *valley) {
			v.height = func(p geom.Vec) float32 {
				if p.X > 30 && p.X < 50 {
					return 0
				}
				return 30
			}
		}, "the ground rises"},
		"a head too far off": {g.At(3, 5), g.At(7, 5), 24, nil, "a head's ground lies"},
		"beside a span": {g.At(3, 5), g.At(7, 5), 30, func(v *valley) {
			v.bridged[g.At(5, 6)] = site.Part{Way: way(g, 1, 0)}
		}, "beside another bridge"},
		"beside a head": {g.At(3, 5), g.At(7, 5), 30, func(v *valley) {
			v.bridged[g.At(3, 6)] = site.Part{Way: way(g, 1, 0), Head: true}
		}, ""},
		"over a road along it": {g.At(3, 5), g.At(7, 5), 30, func(v *valley) {
			v.roads[g.At(5, 5)] = 1<<way(g, 1, 0) | 1<<way(g, -1, 0)
		}, "not crossing it square"},
		"over a road across it": {g.At(3, 5), g.At(7, 5), 30, func(v *valley) {
			v.roads[g.At(5, 5)] = 1<<way(g, 0, 1) | 1<<way(g, 0, -1)
		}, ""},
		"breaking a road": {g.At(3, 5), g.At(7, 5), 27, func(v *valley) {
			v.roads[g.At(2, 5)] = 1<<way(g, 1, 0) | 1<<way(g, -1, 0)
		}, "would break the road"},
	} {
		t.Run(name, func(t *testing.T) {
			v := newValley()
			if tc.setup != nil {
				tc.setup(v)
			}
			_, err := site.Plan(v, rules, tc.from, tc.to, tc.deck)
			if tc.why == "" {
				if err != nil {
					t.Fatalf("refused: %v", err)
				}
				return
			}
			var r *site.Refusal
			if !errors.As(err, &r) || !strings.Contains(r.Why(), tc.why) {
				t.Fatalf("got %v, want a refusal saying %q", err, tc.why)
			}
		})
	}
}

func TestPlan_Slantwise(t *testing.T) {
	v := newValley()
	v.height = func(p geom.Vec) float32 {
		if p.X+p.Y > 80 && p.X+p.Y < 140 {
			return 0
		}
		return 30
	}
	b, err := site.Plan(v, rules, v.g.At(3, 3), v.g.At(7, 7), 30)
	if err != nil {
		t.Fatal(err)
	}
	if len(b.Spans) != 3 || b.Piers[0] != geom.NewVec(50, 50) {
		t.Errorf("planned %+v, want three spans and the first pier on the corner (50, 50)", b)
	}
}

func TestPlan_AFlatWorldCrossesWater(t *testing.T) {
	v := newValley()
	flat := site.Rules{Longest: 3}
	if _, err := site.Plan(v, flat, v.g.At(3, 5), v.g.At(5, 5), 0); err == nil {
		t.Error("a bridge over dry ground was planned in a flat world")
	}
	v.wet[v.g.At(4, 5)] = true
	b, err := site.Plan(v, flat, v.g.At(3, 5), v.g.At(5, 5), 0)
	if err != nil || len(b.Piers) != 0 {
		t.Errorf("planned %+v, %v; want a bridge over the water with no pier", b, err)
	}
}

func TestPlan_Hex(t *testing.T) {
	g := grid.DefaultGrids{}.Hex(12, 12, 10)
	v := &hexes{g: g, wet: map[cell.ID]bool{}}
	from := g.At(2, 5)
	at := from
	for range 3 {
		at, _ = g.Toward(at, 0)
		v.wet[at] = true
	}
	to, _ := g.Toward(at, 0)
	b, err := site.Plan(v, site.Rules{Longest: 3, Under: 5, Heights: true}, from, to, 10)
	if err != nil || len(b.Spans) != 3 {
		t.Errorf("planned %+v, %v; want three spans", b, err)
	}
}

type hexes struct {
	g   grid.Grid
	wet map[cell.ID]bool
}

func (h *hexes) Grid() grid.Grid { return h.g }
func (h *hexes) Heights(c cell.ID, dst []float32) []float32 {
	if h.wet[c] {
		return append(dst, 0)
	}
	return append(dst, 10)
}
func (h *hexes) Takes(c cell.ID) bool              { return !h.wet[c] }
func (h *hexes) Wet(c cell.ID) bool                { return h.wet[c] }
func (h *hexes) Road(cell.ID) cell.Links           { return 0 }
func (h *hexes) Bridged(cell.ID) (site.Part, bool) { return site.Part{}, false }
