package site_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/underground/site"
)

// a ridge 30 high over the columns 2 to 5 of a board 8 by 8, its sides climbing over a cell
type ridge struct {
	g     grid.Grid
	rock  map[cell.ID]bool
	built map[cell.ID]cell.Built
	flat  bool
	dug   geom.Vec // a corner of the cells dug 100 down, but for the board's own
}

func newRidge() *ridge {
	return &ridge{
		g:     grid.DefaultGrids{}.Square(8, 8, 32),
		rock:  map[cell.ID]bool{},
		built: map[cell.ID]cell.Built{},
	}
}

func (r *ridge) Grid() grid.Grid { return r.g }

func (r *ridge) at(p geom.Vec) float32 {
	switch {
	case r.dug != geom.Vec{} && p == r.dug:
		return -100
	case r.flat, p.X <= 64, p.X >= 192:
		return 0
	case p.X < 96:
		return (p.X - 64) / 32 * 30
	case p.X > 160:
		return (192 - p.X) / 32 * 30
	}
	return 30
}

func (r *ridge) Heights(c cell.ID, dst []float32) []float32 {
	for _, p := range r.g.CellOutline(c, nil) {
		dst = append(dst, r.at(p))
	}
	return dst
}

func (r *ridge) Takes(c cell.ID) bool { return !r.rock[c] }

func (r *ridge) Built(c cell.ID) cell.Built { return r.built[c] }

func TestPlan_RunsFromHeadToHeadUnderTheRidge(t *testing.T) {
	r := newRidge()
	g := r.g
	tn, err := site.Plan(r, site.Rules{Cover: 2}, 4, g.At(1, 3), g.At(6, 3))
	if err != nil {
		t.Fatal(err)
	}
	want := []cell.ID{g.At(2, 3), g.At(3, 3), g.At(4, 3), g.At(5, 3)}
	if len(tn.Cells) != len(want) {
		t.Fatalf("runs under %v, want %v", tn.Cells, want)
	}
	for k, c := range want {
		if tn.Cells[k] != c || tn.Floors[k] != 0 {
			t.Errorf("cell %d: %v at %v, want %v at the heads' level", k, tn.Cells[k],
				tn.Floors[k], c)
		}
	}
	if tn.Storey != 1 || tn.Ends != [2]float32{} {
		t.Errorf("storey %d, ends %v; want the second storey from ground to ground", tn.Storey,
			tn.Ends)
	}
}

func TestPlan_TurnsAtTheCellsItIsLedThrough(t *testing.T) {
	r := newRidge()
	g := r.g
	tn, err := site.Plan(r, site.Rules{}, 4, g.At(1, 1), g.At(6, 5), g.At(3, 1), g.At(3, 5))
	if err != nil {
		t.Fatal(err)
	}
	want := []cell.ID{g.At(2, 1), g.At(3, 1), g.At(3, 2), g.At(3, 3), g.At(3, 4), g.At(3, 5),
		g.At(4, 5), g.At(5, 5)}
	if len(tn.Cells) != len(want) {
		t.Fatalf("runs under %v, want %v", tn.Cells, want)
	}
	for k, c := range want {
		if tn.Cells[k] != c {
			t.Errorf("cell %d: %v, want %v", k, tn.Cells[k], c)
		}
	}
}

func TestPlan_RefusesWhatBreaksARule(t *testing.T) {
	g := grid.DefaultGrids{}.Square(8, 8, 32)
	for name, tc := range map[string]struct {
		prepare  func(*ridge)
		rules    site.Rules
		clear    float32
		from, to cell.ID
		via      []cell.ID
		why      string
	}{
		"side by side": {from: g.At(1, 3), to: g.At(2, 3), clear: 4, why: "side by side"},
		"too long": {from: g.At(1, 3), to: g.At(6, 3), clear: 4,
			rules: site.Rules{Longest: 3}, why: "3 at most"},
		"off a line": {from: g.At(1, 3), to: g.At(6, 4), clear: 4, why: "off every line"},
		"a head on rock": {from: g.At(1, 3), to: g.At(6, 3), clear: 4,
			prepare: func(r *ridge) { r.rock[g.At(6, 3)] = true }, why: "may not take"},
		"a head on a slope": {from: g.At(2, 3), to: g.At(6, 3), clear: 4, why: "off level"},
		"no ground over it": {from: g.At(1, 3), to: g.At(6, 3), clear: 4,
			prepare: func(r *ridge) { r.flat = true }, why: "the ground lies"},
		"too little rock": {from: g.At(1, 3), to: g.At(6, 3), clear: 4,
			rules: site.Rules{Cover: 27}, why: "of rock"},
		"every storey taken": {from: g.At(1, 3), to: g.At(6, 3), clear: 4,
			prepare: func(r *ridge) {
				var b cell.Built
				b.More[0] = cell.Floor{Allows: cell.Land, Height: -20, Clear: 3, Storey: 1}
				b.More[1] = cell.Floor{Allows: cell.Land, Height: -10, Clear: 3, Storey: 2}
				r.built[g.At(4, 3)] = b
			}, why: "every storey"},
	} {
		r := newRidge()
		if tc.prepare != nil {
			tc.prepare(r)
		}
		_, err := site.Plan(r, tc.rules, tc.clear, tc.from, tc.to, tc.via...)
		var refusal *site.Refusal
		if !errors.As(err, &refusal) || !strings.Contains(refusal.Why(), tc.why) {
			t.Errorf("%s: %v, want a refusal saying %q", name, err, tc.why)
		}
	}
}

func TestPlan_TakesTheStoreyLeftFreeWhereItCrossesAnother(t *testing.T) {
	r := newRidge()
	g := r.g
	var b cell.Built
	b.More[0] = cell.Floor{Allows: cell.Land, Height: -20, Clear: 3, Storey: 1}
	r.built[g.At(4, 3)] = b
	tn, err := site.Plan(r, site.Rules{}, 4, g.At(1, 3), g.At(6, 3))
	if err != nil || tn.Storey != 2 {
		t.Errorf("storey %d, %v; want the third, the second taken on a cell", tn.Storey, err)
	}
}

func TestPlan_ClimbsFromOneHeadsGroundToTheOthers(t *testing.T) {
	r := newRidge()
	g := r.g
	// the far side's ground 6 up: a ridge between a valley and a terrace
	hill := &terraced{ridge: r}
	tn, err := site.Plan(hill, site.Rules{}, 4, g.At(1, 3), g.At(6, 3))
	if err != nil {
		t.Fatal(err)
	}
	if tn.Ends != [2]float32{0, 6} {
		t.Fatalf("ends at %v, want from 0 to 6", tn.Ends)
	}
	for k, f := range tn.Floors {
		if want := 6 * float32(k+1) / 5; f != want {
			t.Errorf("under cell %d the floor at %v, want %v", k, f, want)
		}
	}
	if _, err := site.Plan(hill, site.Rules{Grade: 0.01}, 4, g.At(1, 3), g.At(6, 3)); err == nil {
		t.Error("a floor climbing 6 in 160 passed a grade of 0.01")
	}
}

type terraced struct{ *ridge }

func (t *terraced) Heights(c cell.ID, dst []float32) []float32 {
	for _, p := range t.g.CellOutline(c, nil) {
		h := t.at(p)
		if p.X >= 192 {
			h = 6
		}
		dst = append(dst, max(h, 0))
	}
	return dst
}

func TestRoom_LiesUnderTheLowestGroundOverIt(t *testing.T) {
	r := newRidge()
	g := r.g
	ch, err := site.Room(r, site.Rules{Cover: 2}, 4, 10, g.At(3, 2), g.At(4, 4))
	if err != nil {
		t.Fatal(err)
	}
	if len(ch.Cells) != 6 || ch.Floor != 20 || ch.Storey != 1 {
		t.Errorf("%d cells, floor %v, storey %d; want six cells 10 under a ground at 30",
			len(ch.Cells), ch.Floor, ch.Storey)
	}
	if _, err := site.Room(r, site.Rules{Cover: 2}, 4, 5, g.At(3, 2), g.At(4, 4)); err == nil {
		t.Error("a chamber 5 under the ground, its roof 4 up, passed 2 of rock over it")
	}
}

func TestPlan_AHeadOpensIntoAChamber(t *testing.T) {
	r := newRidge()
	g := r.g
	var b cell.Built
	b.More[0] = cell.Floor{Allows: cell.Land, Height: -30, Clear: 4, Storey: 1}
	r.built[g.At(4, 3)] = b
	tn, err := site.Plan(r, site.Rules{Cover: 2}, 4, g.At(1, 3), g.At(4, 3))
	if err != nil {
		t.Fatal(err)
	}
	if tn.Ends != [2]float32{0, 0} || len(tn.Cells) != 2 {
		t.Errorf("ends %v under %v; want from the ground to the chamber's floor under two cells",
			tn.Ends, tn.Cells)
	}
}

func TestPlan_RefusesAMouthAcrossItsHeadsCorner(t *testing.T) {
	r := newRidge()
	g := r.g
	_, err := site.Plan(r, site.Rules{}, 4, g.At(1, 1), g.At(6, 6))
	var refused *site.Refusal
	if !errors.As(err, &refused) || !strings.Contains(refused.Why(), "corner") {
		t.Fatalf("a tunnel out across its head's corner: %v, want it refused", err)
	}
	if _, err := site.Plan(r, site.Rules{}, 4, g.At(1, 2), g.At(6, 3), g.At(3, 2),
		g.At(4, 3)); err != nil {
		t.Errorf("the tunnel led out of its heads across their edges: %v", err)
	}
	_, err = site.Plan(r, site.Rules{}, 4, g.At(1, 2), g.At(6, 3), g.At(2, 2), g.At(3, 3))
	if !errors.As(err, &refused) || !strings.Contains(refused.Why(), "turns under") {
		t.Errorf("a tunnel turning in its mouth's cell: %v, want it refused", err)
	}
}

func TestSquare_LeadsASlantingTunnelOutOfItsHeadsAcrossTheirEdges(t *testing.T) {
	g := grid.DefaultGrids{}.Square(8, 8, 32)
	for name, c := range map[string]struct {
		from, to cell.ID
		want     []cell.ID
	}{
		"along a row":       {g.At(1, 3), g.At(6, 3), nil},
		"slanting":          {g.At(1, 1), g.At(5, 5), []cell.ID{g.At(3, 1), g.At(5, 3)}},
		"slanting back":     {g.At(5, 1), g.At(1, 5), []cell.ID{g.At(3, 1), g.At(1, 3)}},
		"slanting two":      {g.At(1, 1), g.At(3, 3), []cell.ID{g.At(3, 1)}},
		"across one corner": {g.At(3, 3), g.At(4, 2), []cell.ID{g.At(4, 3)}},
		"along no way":      {g.At(1, 1), g.At(5, 3), nil},
	} {
		if got := site.Square(g, c.from, c.to); !slices.Equal(got, c.want) {
			t.Errorf("%s: turns at %v, want %v", name, got, c.want)
		}
	}
}

func TestPlan_RefusesACornerCrossedUnderGroundLowerThanTheRoofBesideIt(t *testing.T) {
	r := newRidge()
	g := r.g
	rules := site.Rules{Width: 40}
	plan := func() error {
		_, err := site.Plan(r, rules, 12, g.At(1, 3), g.At(6, 4), g.At(3, 3), g.At(4, 4))
		return err
	}
	if err := plan(); err != nil {
		t.Fatalf("under the ridge whole: %v", err)
	}
	// the cell beside the corner the tunnel crosses from (3, 3) to (4, 4), dug down at its own
	r.dug = geom.NewVec(96, 160)
	var refused *site.Refusal
	if err := plan(); !errors.As(err, &refused) || !strings.Contains(refused.Why(), "beside") {
		t.Fatalf("the ground dug beside the corner: %v, want the tunnel refused", err)
	}
}
