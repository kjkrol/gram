package site_test

import (
	"errors"
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/ground"
	"github.com/kjkrol/gram/plugins/roads/site"
)

// planGround is a board whose ground lies at at(p) at each corner p, level at 0 without it; a cell
// takes a road unless refused holds it.
type planGround struct {
	g       grid.Grid
	at      func(p geom.Vec) float32
	refused map[cell.ID]bool
	wet     map[cell.ID]bool
	roads   map[cell.ID]cell.Links
	decks   map[cell.ID]cell.Links
	fords   map[cell.ID]bool
	fixed   map[cell.ID]bool
	held    map[cell.ID]bool
	outline []geom.Vec
}

var _ site.Ground = (*planGround)(nil)

func newPlanGround(g grid.Grid) *planGround {
	return &planGround{
		g:       g,
		refused: map[cell.ID]bool{},
		wet:     map[cell.ID]bool{},
		roads:   map[cell.ID]cell.Links{},
		decks:   map[cell.ID]cell.Links{},
		fords:   map[cell.ID]bool{},
		fixed:   map[cell.ID]bool{},
		held:    map[cell.ID]bool{},
	}
}

func (f *planGround) Grid() grid.Grid { return f.g }

func (f *planGround) Heights(c cell.ID, dst []float32) []float32 {
	f.outline = f.g.CellOutline(c, f.outline[:0])
	for _, p := range f.outline {
		h := float32(0)
		if f.at != nil {
			h = f.at(p)
		}
		dst = append(dst, h)
	}
	return dst
}

func (f *planGround) Takes(c cell.ID) bool      { return !f.refused[c] }
func (f *planGround) Wet(c cell.ID) bool        { return f.wet[c] }
func (f *planGround) Road(c cell.ID) cell.Links { return f.roads[c] }
func (f *planGround) Ford(c cell.ID) bool       { return f.fords[c] }
func (f *planGround) Fixed(c cell.ID) bool      { return f.fixed[c] }
func (f *planGround) Held(c cell.ID) bool       { return f.held[c] }
func (f *planGround) Price(cell.ID) float32     { return 1 }

func (f *planGround) Decked(c cell.ID) (cell.Links, bool) {
	l, ok := f.decks[c]
	return l, ok
}

// a square board 10 x 10 of cells 10 wide
func planSquare() grid.Grid { return grid.DefaultGrids{}.Square(10, 10, 10) }

// a hex board 12 x 12 of cells of size 10
func planHex() grid.Grid { return grid.DefaultGrids{}.Hex(12, 12, 10) }

// planRules allow every road the tests lay, as deep as they level
func planRules() site.Rules {
	return site.Rules{
		Cut:       100,
		Fill:      100,
		Earth:     1e9,
		Spread:    10,
		Slope:     ground.RoadSlope,
		Crossings: true,
		Decks:     true,
		Fords:     true,
		Ends:      true,
	}
}

// planRoad is the cells at the coordinates given in pairs
func planRoad(g grid.Grid, xy ...uint32) []cell.ID {
	var out []cell.ID
	for i := 0; i+1 < len(xy); i += 2 {
		out = append(out, g.At(xy[i], xy[i+1]))
	}
	return out
}

func planWay(t *testing.T, g grid.Grid, dx, dy int) int {
	t.Helper()
	i, ok := grid.Direction(g, dx, dy)
	if !ok {
		t.Fatalf("no direction %d, %d", dx, dy)
	}
	return i
}

func planRefused(t *testing.T, err error, at cell.ID, why string) {
	t.Helper()
	var r *site.Refusal
	if !errors.As(err, &r) {
		t.Fatalf("planned (%v), want a refusal that %s", err, why)
	}
	if r.At != at || !strings.Contains(r.Why(), why) {
		t.Errorf("refused: %v at %d, want %q at %d", err, r.At, why, at)
	}
}

// planFits fails where a way of e lies on ground no road may once e is laid
func planFits(t *testing.T, f *planGround, e site.Estimate) {
	t.Helper()
	for _, w := range e.Ways {
		if hs := planLevelled(f, e, w.Cell); !ground.RoadFits(f.g, w.Cell, hs, w.Links) {
			x, y, _ := f.g.Coords(w.Cell)
			t.Errorf("the road at [%d, %d] lies on %v, as no road may", x, y, hs)
		}
	}
}

func planLevelled(f *planGround, e site.Estimate, c cell.ID) []float32 {
	return e.Levelled(f.g, c, f.Heights(c, nil))
}

// planMoved is the move of the corner at p, false where e moves none there
func planMoved(e site.Estimate, p geom.Vec) (site.CornerMove, bool) {
	for _, m := range e.Corners {
		if planNear(float64(m.At.X), float64(p.X), 0.01) &&
			planNear(float64(m.At.Y), float64(p.Y), 0.01) {
			return m, true
		}
	}
	return site.CornerMove{}, false
}

func planNear(a, b, tol float64) bool { return math.Abs(a-b) <= tol }

// three tenths of a world unit up for each one east
func planSlope(p geom.Vec) float32 { return p.X * 3 / 10 }

func TestPlan_LinksAlongTheRoad(t *testing.T) {
	g := planSquare()
	f := newPlanGround(g)
	n, s := planWay(t, g, 0, -1), planWay(t, g, 0, 1)
	w, e := planWay(t, g, -1, 0), planWay(t, g, 1, 0)
	cells := planRoad(g, 2, 5, 3, 5, 4, 5, 4, 6)
	got, err := site.Plan(f, planRules(), cells, "road", 4, true, true, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []cell.Links{1 << e, 1<<w | 1<<e, 1<<w | 1<<s, 1 << n}
	if !slices.Equal(got.Cells, cells) || !slices.Equal(got.Links, want) {
		t.Errorf("cells %v linked %v, want %v linked %v", got.Cells, got.Links, cells, want)
	}
	bend := cell.WayEntry{Kind: "road", Cell: cells[2], Width: 4, Links: want[2]}
	if len(got.Ways) != 4 || got.Ways[2] != bend {
		t.Errorf("ways %+v, want one a cell, the bend's %+v", got.Ways, bend)
	}
	if got.Length != 30 {
		t.Errorf("length %v, want 30", got.Length)
	}
	if len(got.Corners) != 0 || len(got.Graded) != 0 || got.Cut != 0 || got.Fill != 0 {
		t.Errorf("level ground levelled: %+v", got)
	}
}

func TestPlan_LinksAlongAHexRow(t *testing.T) {
	g := planHex()
	f := newPlanGround(g)
	w, e := planWay(t, g, -1, 0), planWay(t, g, 1, 0)
	cells := planRoad(g, 2, 5, 3, 5, 4, 5)
	got, err := site.Plan(f, planRules(), cells, "road", 4, true, true, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if want := []cell.Links{1 << e, 1<<w | 1<<e, 1 << w}; !slices.Equal(got.Links, want) {
		t.Errorf("linked %v, want %v", got.Links, want)
	}
	if want := 2 * math.Sqrt(3) * 10; !planNear(float64(got.Length), want, 1e-3) {
		t.Errorf("length %v, want %v", got.Length, want)
	}
	if len(got.Corners) != 0 {
		t.Errorf("level ground levelled: %+v", got.Corners)
	}
}

func TestPlan_CrossesARoadStanding(t *testing.T) {
	g := planSquare()
	n, s := planWay(t, g, 0, -1), planWay(t, g, 0, 1)
	w, e := planWay(t, g, -1, 0), planWay(t, g, 1, 0)
	f := newPlanGround(g)
	for y := uint32(4); y <= 6; y++ {
		f.roads[g.At(3, y)] = 1<<n | 1<<s
	}
	cells := planRoad(g, 2, 5, 3, 5, 4, 5)
	got, err := site.Plan(f, planRules(), cells, "road", 4, true, true, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if want := cell.Links(1<<n | 1<<s | 1<<w | 1<<e); got.Links[1] != want ||
		got.Ways[1].Links != want {
		t.Errorf("the crossing linked %v, laid %v, want %v", got.Links[1], got.Ways[1].Links, want)
	}
	if !slices.Equal(got.Crossings, cells[1:2]) {
		t.Errorf("crossings %v, want %v", got.Crossings, cells[1:2])
	}
	r := planRules()
	r.Crossings = false
	_, err = site.Plan(f, r, cells, "road", 4, true, true, 0, nil)
	planRefused(t, err, cells[1], "crosses a road")
}

func TestPlan_JoinsARoadRunningTowardItsStart(t *testing.T) {
	g := planSquare()
	w, e := planWay(t, g, -1, 0), planWay(t, g, 1, 0)
	f := newPlanGround(g)
	f.roads[g.At(1, 5)] = 1 << e
	r := planRules()
	r.Ends = false
	cells := planRoad(g, 2, 5, 3, 5)
	got, err := site.Plan(f, r, cells, "road", 4, true, true, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if want := cell.Links(1<<w | 1<<e); got.Links[0] != want {
		t.Errorf("the start linked %v, want %v", got.Links[0], want)
	}
	if !slices.Equal(got.Crossings, cells[:1]) {
		t.Errorf("crossings %v, want the start", got.Crossings)
	}
	r.Crossings = false
	_, err = site.Plan(f, r, cells, "road", 4, true, true, 0, nil)
	planRefused(t, err, cells[0], "begins loose")
	r.Crossings = true
	_, err = site.Plan(newPlanGround(g), r, cells, "road", 4, true, true, 0, nil)
	planRefused(t, err, cells[0], "begins loose")
}

// A lone piece at [2, 5] joins the road running toward it from the west and runs out east as its
// extra says: on ground rising east it lies as a ramp, where without its way out it would be an
// end, levelled.
func TestPlan_LaysALonePieceAsExtraSays(t *testing.T) {
	g := planSquare()
	w, e := planWay(t, g, -1, 0), planWay(t, g, 1, 0)
	f := newPlanGround(g)
	f.at = func(p geom.Vec) float32 { return p.X / 10 }
	f.roads[g.At(1, 5)] = 1 << e
	r := planRules()
	r.Ends = false
	piece := planRoad(g, 2, 5)
	got, err := site.Plan(f, r, piece, "road", 4, true, true, 0, []cell.Links{1 << e})
	if err != nil {
		t.Fatal(err)
	}
	want := cell.Links(1<<w | 1<<e)
	if len(got.Ways) != 1 || got.Links[0] != want || got.Ways[0].Links != want {
		t.Errorf("linked %v laying %+v, want %v", got.Links, got.Ways, want)
	}
	if len(got.Corners) != 0 {
		t.Errorf("a piece lying as a ramp levelled: %+v", got.Corners)
	}
	got, err = site.Plan(f, r, piece, "road", 4, true, true, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.Links[0] != 1<<w || len(got.Corners) != 4 {
		t.Errorf("an end linked %v moved %d corners, want %v levelled, its 4", got.Links[0],
			len(got.Corners), cell.Links(1<<w))
	}
	planFits(t, f, got)
	_, err = site.Plan(f, r, planRoad(g, 9, 5), "road", 4, true, true, 0, []cell.Links{1 << e})
	planRefused(t, err, g.At(9, 5), "runs off the board")
}

// The ends of the road hold their corners level, at the means 4.5 and 10.5; the ramp between
// them rises 6, past its most, and the two classes move half of the excess each, sized alike: as
// gramap's grade moved them.
func TestPlan_GradesAsGramapGraded(t *testing.T) {
	g := planSquare()
	f := newPlanGround(g)
	f.at = planSlope
	cells := planRoad(g, 1, 5, 2, 5, 3, 5)
	got, err := site.Plan(f, planRules(), cells, "road", 4, true, true, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	rise := 0.999 * 10 * math.Tan(ground.RoadSlope)
	over := 6 - rise
	low, high := 4.5+over/2, 10.5-over/2
	for c, want := range map[cell.ID]float64{cells[0]: low, cells[2]: high} {
		for _, h := range planLevelled(f, got, c) {
			if !planNear(float64(h), want, 1e-4) {
				t.Errorf("an end's corner at %v, want %v", h, want)
			}
		}
	}
	// top left, top right, bottom right, bottom left: west low, east high
	ramp := planLevelled(f, got, cells[1])
	if !planNear(float64(ramp[1]-ramp[0]), rise, 1e-4) ||
		!planNear(float64(ramp[2]-ramp[3]), rise, 1e-4) {
		t.Errorf("the ramp lies at %v, want rising %v east", ramp, rise)
	}
	if len(got.Corners) != 8 {
		t.Errorf("moved %d corners, want the road's 8", len(got.Corners))
	}
	if !planNear(float64(got.Lift), low-3, 1e-4) || !planNear(float64(got.Drop), 12-high, 1e-4) {
		t.Errorf("lifted %v, dropped %v; want %v and %v", got.Lift, got.Drop, low-3, 12-high)
	}
	// every corner held by four cells, a quarter of each's 100
	fill := 2 * 100 * ((low - 3) + (low - 6))
	cut := 2 * 100 * ((9 - high) + (12 - high))
	if !planNear(float64(got.Fill), fill, 0.01) || !planNear(float64(got.Cut), cut, 0.01) {
		t.Errorf("filled %v, cut %v; want %v and %v", got.Fill, got.Cut, fill, cut)
	}
	if len(got.Graded) != 15 || !slices.Equal(got.Graded[:3], cells) {
		t.Errorf("graded %v, want the road and the 12 cells round it", got.Graded)
	}
	planFits(t, f, got)
}

func TestPlan_RefusesEarthworksPastTheRules(t *testing.T) {
	g := planSquare()
	cells := planRoad(g, 1, 5, 2, 5, 3, 5)
	for name, tc := range map[string]struct {
		rule func(r *site.Rules)
		at   cell.ID
		why  string
	}{
		// the west end may stand 6 high at most, the east end 9 at least: the ramp climbs 3
		"no profile within cut and fill": {func(r *site.Rules) { r.Cut, r.Fill = 3, 3 },
			cells[1], "climbs"},
		"too much earth":  {func(r *site.Rules) { r.Earth = 1000 }, cells[0], "of earth"},
		"no earth at all": {func(r *site.Rules) { r.Earth = 0 }, cells[0], "of earth"},
	} {
		t.Run(name, func(t *testing.T) {
			f := newPlanGround(g)
			f.at = planSlope
			r := planRules()
			tc.rule(&r)
			_, err := site.Plan(f, r, cells, "road", 4, true, true, 0, nil)
			planRefused(t, err, tc.at, tc.why)
		})
	}
}

// The east end may sink 2.5 at most, to 9.5, the west end rise 5, to 8: the ramp between climbs
// all it may from the east end's 9.5 down, every corner within the cut and fill.
func TestPlan_LevelsWithinTheCutAndFill(t *testing.T) {
	g := planSquare()
	f := newPlanGround(g)
	f.at = planSlope
	cells := planRoad(g, 1, 5, 2, 5, 3, 5)
	r := planRules()
	r.Cut, r.Fill = 2.5, 5
	got, err := site.Plan(f, r, cells, "road", 4, true, true, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range got.Corners {
		if dh := float64(m.To - m.From); dh < -2.5-1e-4 || dh > 5+1e-4 {
			t.Errorf("a corner moved %v, past the cut and fill: %+v", dh, m)
		}
	}
	rise := 0.999 * 10 * math.Tan(ground.RoadSlope)
	for c, want := range map[cell.ID]float64{cells[0]: 9.5 - rise, cells[2]: 9.5} {
		for _, h := range planLevelled(f, got, c) {
			if !planNear(float64(h), want, 1e-4) {
				t.Errorf("an end's corner at %v, want %v", h, want)
			}
		}
	}
	planFits(t, f, got)
}

// Three hundred up over ten cells 333 wide, as a road on Madeira climbs: the ends lie level, the
// ramps between as the ground does.
func TestPlan_LevelsALongUniformClimb(t *testing.T) {
	g := grid.DefaultGrids{}.Square(14, 6, 333)
	f := newPlanGround(g)
	f.at = func(p geom.Vec) float32 { return p.X * 300 / 3330 }
	var cells []cell.ID
	for x := uint32(1); x <= 10; x++ {
		cells = append(cells, g.At(x, 2))
	}
	r := planRules()
	r.Cut, r.Fill = 200, 200
	got, err := site.Plan(f, r, cells, "road", 4, true, true, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range got.Corners {
		if dh := m.To - m.From; dh < -200 || dh > 200 {
			t.Errorf("a corner moved %v, past the cut and fill: %+v", dh, m)
		}
	}
	planFits(t, f, got)
}

// A road off the ground onto a bridge's head, over its two spans and off the other head: ways on
// the heads as on ground, none on the spans.
func TestPlan_RunsOntoAHeadOverTheSpansAndOffTheOther(t *testing.T) {
	g := planSquare()
	w, e := planWay(t, g, -1, 0), planWay(t, g, 1, 0)
	f := newPlanGround(g)
	cells := planRoad(g, 1, 5, 2, 5, 3, 5, 4, 5, 5, 5, 6, 5)
	f.decks[cells[1]] = 1 << e
	f.decks[cells[2]] = 1<<w | 1<<e
	f.decks[cells[3]] = 1<<w | 1<<e
	f.decks[cells[4]] = 1 << w
	got, err := site.Plan(f, planRules(), cells, "road", 4, true, true, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	var ways []cell.ID
	for _, way := range got.Ways {
		ways = append(ways, way.Cell)
	}
	if want := []cell.ID{cells[0], cells[1], cells[4], cells[5]}; !slices.Equal(ways, want) {
		t.Errorf("ways laid on %v, want %v", ways, want)
	}
	if !slices.Equal(got.Decks, cells[2:4]) {
		t.Errorf("over the decks %v, want the spans %v", got.Decks, cells[2:4])
	}
	if want := cell.Links(1<<w | 1<<e); got.Links[1] != want || got.Links[4] != want {
		t.Errorf("the heads linked %v and %v, want %v", got.Links[1], got.Links[4], want)
	}
	r := planRules()
	r.Decks = false
	_, err = site.Plan(f, r, cells, "road", 4, true, true, 0, nil)
	planRefused(t, err, cells[1], "a bridge's deck")
}

func TestPlan_ARoadEndingOnAHeadLeadsOntoTheDeck(t *testing.T) {
	g := planSquare()
	w, e := planWay(t, g, -1, 0), planWay(t, g, 1, 0)
	f := newPlanGround(g)
	cells := planRoad(g, 1, 5, 2, 5, 3, 5)
	f.decks[cells[2]] = 1 << e
	got, err := site.Plan(f, planRules(), cells, "road", 4, true, true, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Ways) != 3 || got.Ways[2].Cell != cells[2] {
		t.Errorf("ways laid on %v, want every cell, the head too", got.Ways)
	}
	if want := cell.Links(1<<w | 1<<e); got.Links[2] != want {
		t.Errorf("the head linked %08b, want %08b, onto the span too", got.Links[2], want)
	}
	if got.Laid != 3 || got.Length != 20 {
		t.Errorf("laid %d cells over %v, want 3 over 20", got.Laid, got.Length)
	}
}

func TestPlan_CountsOnlyTheRoadLaidAnew(t *testing.T) {
	g := planSquare()
	w, e := planWay(t, g, -1, 0), planWay(t, g, 1, 0)
	f := newPlanGround(g)
	cells := planRoad(g, 1, 5, 2, 5, 3, 5, 4, 5)
	f.roads[cells[0]] = 1 << e
	f.roads[cells[1]] = 1<<w | 1<<e
	f.roads[cells[2]] = 1 << w
	got, err := site.Plan(f, planRules(), cells, "road", 4, true, true, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.Laid != 2 || got.Length != 10 {
		t.Errorf("laid %d cells over %v, want the end extended and the new cell over 10",
			got.Laid, got.Length)
	}
	f.roads[cells[2]] = 1<<w | 1<<e
	f.roads[cells[3]] = 1 << w
	got, err = site.Plan(f, planRules(), cells, "road", 4, true, true, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.Laid != 0 || got.Length != 0 || len(got.Ways) != 4 {
		t.Errorf("along a road standing: laid %d over %v, %d ways; want nothing anew, the 4 ways "+
			"as they are", got.Laid, got.Length, len(got.Ways))
	}
}

func TestPlan_LevelsNoSteeperThanTheRulesSlope(t *testing.T) {
	g := planSquare()
	f := newPlanGround(g)
	f.at = planSlope
	cells := planRoad(g, 1, 5, 2, 5, 3, 5)
	r := planRules()
	r.Slope = 5 * math.Pi / 180
	got, err := site.Plan(f, r, cells, "road", 4, true, true, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	ramp := planLevelled(f, got, cells[1])
	if want := 0.999 * 10 * math.Tan(r.Slope); !planNear(float64(ramp[1]-ramp[0]), want, 1e-4) {
		t.Errorf("the ramp lies at %v, want rising %v", ramp, want)
	}
	planFits(t, f, got)
}

func TestPlan_KeepsTheCornersThatMayNotMove(t *testing.T) {
	g := planSquare()
	f := newPlanGround(g)
	f.at = planSlope
	f.fixed[g.At(0, 5)] = true
	cells := planRoad(g, 1, 5, 2, 5, 3, 5)
	got, err := site.Plan(f, planRules(), cells, "road", 4, true, true, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range got.Corners {
		if m.At.X == 10 {
			t.Errorf("a corner the fixed cell holds moved: %+v", m)
		}
	}
	// the west end held at the fixed corners' 3, the ramp climbing from it all it may
	high := 3 + 0.999*10*math.Tan(ground.RoadSlope)
	for c, want := range map[cell.ID]float64{cells[0]: 3, cells[2]: high} {
		for _, h := range planLevelled(f, got, c) {
			if !planNear(float64(h), want, 1e-4) {
				t.Errorf("an end's corner at %v, want %v", h, want)
			}
		}
	}
	if slices.Contains(got.Graded, g.At(0, 5)) {
		t.Errorf("the fixed cell graded: %v", got.Graded)
	}
	planFits(t, f, got)
}

func TestPlan_RefusesGroundThatMayNotMoveAndDoesNotFit(t *testing.T) {
	g := planSquare()
	cells := planRoad(g, 1, 5, 2, 5, 3, 5)
	for name, tc := range map[string]struct {
		fixed cell.ID
		why   string
	}{
		"a ramp too steep, fixed":  {cells[1], "climbs"},
		"an end on a slope, fixed": {cells[0], "may not move"},
	} {
		t.Run(name, func(t *testing.T) {
			f := newPlanGround(g)
			f.at = planSlope
			f.fixed[tc.fixed] = true
			_, err := site.Plan(f, planRules(), cells, "road", 4, true, true, 0, nil)
			planRefused(t, err, tc.fixed, tc.why)
		})
	}
}

// The road's six corners level at their mean, 0.1, the bump at [3, 5]'s corner cut down to it;
// with a step of 0.05 the ground round them follows to 0.05, a cell off the road.
func TestPlan_RipplesNoFurtherThanSpread(t *testing.T) {
	g := planSquare()
	f := newPlanGround(g)
	f.at = func(p geom.Vec) float32 {
		if p.X == 30 && p.Y == 50 {
			return 0.6
		}
		return 0
	}
	f.held[g.At(1, 5)] = true
	f.held[g.At(8, 8)] = true
	cells := planRoad(g, 3, 5, 4, 5)
	r := planRules()
	r.Spread = 1
	got, err := site.Plan(f, r, cells, "road", 4, true, true, 0.05, nil)
	if err != nil {
		t.Fatal(err)
	}
	if m, ok := planMoved(got, geom.NewVec(20, 50)); !ok || !planNear(float64(m.To), 0.05, 1e-6) {
		t.Errorf("the corner beside the road moved %+v (%v), want to 0.05", m, ok)
	}
	if len(got.Corners) != 16 {
		t.Errorf("moved %d corners, want the road's 6 and the 10 beside them", len(got.Corners))
	}
	if !slices.Contains(got.Graded, g.At(1, 5)) || !slices.Contains(got.Graded, g.At(2, 5)) {
		t.Errorf("graded %v, want the cells the ripple reaches", got.Graded)
	}
	if want := []cell.ID{g.At(1, 5)}; !slices.Equal(got.Blocked, want) {
		t.Errorf("blocked %v, want %v", got.Blocked, want)
	}
	planFits(t, f, got)

	still, err := site.Plan(f, r, cells, "road", 4, true, true, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(still.Graded, g.At(1, 5)) || !slices.Contains(still.Graded, g.At(2, 5)) {
		t.Errorf("without a step graded %v, want the road's cells and those sharing a corner",
			still.Graded)
	}
	if _, ok := planMoved(still, geom.NewVec(20, 50)); ok || len(still.Blocked) != 0 {
		t.Errorf("without a step the ground round moved: %+v", still)
	}

	r.Spread = 0
	_, err = site.Plan(f, r, cells, "road", 4, true, true, 0.05, nil)
	planRefused(t, err, cells[0], "cells off")
}

func TestPlan_RefusesBreakingARoadStanding(t *testing.T) {
	g := planSquare()
	w, e := planWay(t, g, -1, 0), planWay(t, g, 1, 0)
	f := newPlanGround(g)
	f.at = func(p geom.Vec) float32 {
		if p.X == 50 && p.Y == 60 {
			return 0.6
		}
		return 0
	}
	// level now, it would tilt across its way as the road's corners rise to 0.1
	f.roads[g.At(3, 4)] = 1<<w | 1<<e
	_, err := site.Plan(f, planRules(), planRoad(g, 3, 5, 4, 5), "road", 4, true, true, 0, nil)
	planRefused(t, err, g.At(3, 4), "would be broken")
}

func TestPlan_WithoutLevellingTheGroundMustFit(t *testing.T) {
	g := planSquare()
	f := newPlanGround(g)
	f.at = planSlope
	cells := planRoad(g, 1, 5, 2, 5, 3, 5)
	_, err := site.Plan(f, planRules(), cells, "road", 4, true, false, 0, nil)
	planRefused(t, err, cells[0], "does not fit")

	// level west of x 20, level a unit up east of x 30, a ramp between
	f.at = func(p geom.Vec) float32 { return min(max((p.X-20)/10, 0), 1) }
	f.held[cells[1]] = true
	got, err := site.Plan(f, planRules(), cells, "road", 4, true, false, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Corners) != 0 || len(got.Graded) != 0 || len(got.Blocked) != 0 {
		t.Errorf("nothing levelled, yet %+v", got)
	}
	r := planRules()
	r.Slope = 5 * math.Pi / 180
	_, err = site.Plan(f, r, cells, "road", 4, true, false, 0, nil)
	planRefused(t, err, cells[1], "climbs")
}

func TestPlan_ATrackClimbsNoSteeperThanTheRulesSlope(t *testing.T) {
	g := planSquare()
	f := newPlanGround(g)
	f.at = func(p geom.Vec) float32 { return p.X / 10 }
	cells := planRoad(g, 1, 5, 2, 5, 3, 5)
	got, err := site.Plan(f, planRules(), cells, "track", 2, false, true, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Corners) != 0 || len(got.Ways) != 3 {
		t.Errorf("a track levelled %v, laid %d ways", got.Corners, len(got.Ways))
	}
	r := planRules()
	r.Slope = 5 * math.Pi / 180
	_, err = site.Plan(f, r, cells, "track", 2, false, true, 0, nil)
	planRefused(t, err, cells[1], "climbs")
}

func TestPlan_LongestCapsAPlan(t *testing.T) {
	g := planSquare()
	f := newPlanGround(g)
	cells := planRoad(g, 2, 5, 3, 5, 4, 5)
	r := planRules()
	r.Longest = 2
	_, err := site.Plan(f, r, cells, "road", 4, true, true, 0, nil)
	planRefused(t, err, cells[2], "runs 3 cells, 2 at most")
	for _, longest := range []int{3, 0} {
		r.Longest = longest
		if _, err := site.Plan(f, r, cells, "road", 4, true, true, 0, nil); err != nil {
			t.Errorf("longest %d: %v", longest, err)
		}
	}
}

func TestPlan_RefusesWhatTheRoadMayNotTake(t *testing.T) {
	g := planSquare()
	n, s := planWay(t, g, 0, -1), planWay(t, g, 0, 1)
	w, e := planWay(t, g, -1, 0), planWay(t, g, 1, 0)
	road := planRoad(g, 2, 5, 3, 5, 4, 5)
	mid, off := road[1], cell.ID(1<<40)
	for name, tc := range map[string]struct {
		cells []cell.ID
		setup func(f *planGround, r *site.Rules)
		at    cell.ID
		why   string
	}{
		"into water": {road, func(f *planGround, _ *site.Rules) { f.wet[mid] = true }, mid,
			"runs into water"},
		"over ground that takes none": {road,
			func(f *planGround, _ *site.Rules) { f.refused[mid] = true }, mid, "takes no road"},
		"over a deck it may not": {road, func(f *planGround, r *site.Rules) {
			f.decks[mid] = 1<<w | 1<<e
			r.Decks = false
		}, mid, "a bridge's deck"},
		"off a deck sideways": {road,
			func(f *planGround, _ *site.Rules) { f.decks[mid] = 1<<n | 1<<s }, mid, "sideways"},
		"through a ford it may not": {road, func(f *planGround, r *site.Rules) {
			f.fords[mid] = true
			r.Fords = false
		}, mid, "through a ford"},
		"off the board": {[]cell.ID{road[0], off}, nil, off, "off the board"},
	} {
		t.Run(name, func(t *testing.T) {
			f, r := newPlanGround(g), planRules()
			if tc.setup != nil {
				tc.setup(f, &r)
			}
			_, err := site.Plan(f, r, tc.cells, "road", 4, true, true, 0, nil)
			planRefused(t, err, tc.at, tc.why)
		})
	}
}

func TestPlan_RunsOverDecksAndThroughFordsLayingNoWayThere(t *testing.T) {
	g := planSquare()
	w, e := planWay(t, g, -1, 0), planWay(t, g, 1, 0)
	road := planRoad(g, 2, 5, 3, 5, 4, 5)
	mid := road[1]
	over := newPlanGround(g)
	over.decks[mid] = 1<<w | 1<<e
	through := newPlanGround(g)
	through.fords[mid] = true
	for name, f := range map[string]*planGround{"a deck": over, "a ford": through} {
		got, err := site.Plan(f, planRules(), road, "road", 4, true, true, 0, nil)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		ran := got.Decks
		if f == through {
			ran = got.Fords
		}
		if !slices.Equal(ran, []cell.ID{mid}) || len(got.Ways) != 2 || got.Ways[1].Cell != road[2] {
			t.Errorf("%s: over %v laying %+v, want over the middle and a way either side", name,
				ran, got.Ways)
		}
	}
}

// A network laid in one plan, two legs apart from one bridge's head to the other's.
func TestPlan_LinksNoTwoCellsInARowThatAreNoNeighbours(t *testing.T) {
	g := planSquare()
	w, e := planWay(t, g, -1, 0), planWay(t, g, 1, 0)
	cells := planRoad(g, 2, 5, 3, 5, 6, 5, 7, 5)
	got, err := site.Plan(newPlanGround(g), planRules(), cells, "road", 4, true, true, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []cell.Links{1 << e, 1 << w, 1 << e, 1 << w}
	if !slices.Equal(got.Links, want) || got.Length != 20 {
		t.Errorf("linked %v over %v, want %v over 20", got.Links, got.Length, want)
	}
}

// A ramp across [3, 5] from 0 to 2, its side corners halfway: it lies as a road may; a side corner
// raised to 1.8 is brought back onto the ramp's plane alone, its edges where they lay.
func TestPlan_LaysAHexRampsSideCornersOnItsPlane(t *testing.T) {
	g := planHex()
	c2, c3, c4 := g.At(2, 5), g.At(3, 5), g.At(4, 5)
	mid := g.CellCenter(c3).X
	top := g.CellOutline(c3, nil)[0]
	steps := func(p geom.Vec) float32 {
		switch {
		case p.X < mid-1:
			return 0
		case p.X > mid+1:
			return 2
		}
		return 1
	}
	f := newPlanGround(g)
	f.at = steps
	cells := []cell.ID{c2, c3, c4}
	got, err := site.Plan(f, planRules(), cells, "road", 4, true, true, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Corners) != 0 {
		t.Errorf("a ramp lying as a road may levelled: %+v", got.Corners)
	}
	planFits(t, f, got)

	f.at = func(p geom.Vec) float32 {
		if planNear(float64(p.X), float64(top.X), 0.01) &&
			planNear(float64(p.Y), float64(top.Y), 0.01) {
			return 1.8
		}
		return steps(p)
	}
	got, err = site.Plan(f, planRules(), cells, "road", 4, true, true, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	m, ok := planMoved(got, top)
	if len(got.Corners) != 1 || !ok || m.From != 1.8 || !planNear(float64(m.To), 1, 1e-4) {
		t.Errorf("moved %+v, want the side corner alone from 1.8 to 1", got.Corners)
	}
	if want := []cell.ID{c3, g.At(3, 4), g.At(4, 4)}; !slices.Equal(got.Graded, want) {
		t.Errorf("graded %v, want the ramp's cell and the two sharing its top corner %v",
			got.Graded, want)
	}
	planFits(t, f, got)
}

func TestPlan_GradesAHexRoadUpASlope(t *testing.T) {
	g := planHex()
	f := newPlanGround(g)
	f.at = planSlope
	cells := planRoad(g, 2, 5, 3, 5, 4, 5, 5, 5, 6, 5)
	got, err := site.Plan(f, planRules(), cells, "road", 4, true, true, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Corners) == 0 {
		t.Fatal("a road too steep for the ground left as it lies")
	}
	planFits(t, f, got)
	if first, last := planLevelled(f, got, cells[0]), planLevelled(f, got, cells[4]); last[0] <=
		first[0] {
		t.Errorf("the road climbs from %v to %v, want up east", first[0], last[0])
	}
	// clockwise from the top: a ramp east has its top corner halfway from its west edge to its east
	for _, c := range cells[1:4] {
		hs := planLevelled(f, got, c)
		if !planNear(float64(hs[0]), float64(hs[5]+hs[1])/2, 1e-3) {
			t.Errorf("a ramp's side corner at %v, want halfway in %v", hs[0], hs)
		}
	}
}
