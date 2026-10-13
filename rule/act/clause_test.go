package act_test

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/rule/act"
	"github.com/kjkrol/gram/rule/cond"
	"github.com/kjkrol/uid"
)

// the first entity plays a, the second b
func rolesAB(ids *[]uid.UID64) func(uid.UID64) uint64 {
	a, b := Role("a"), Role("b")
	return func(id uid.UID64) uint64 {
		if len(*ids) == 2 && id == (*ids)[1] {
			return 1 << b.Tag()
		}
		return 1 << a.Tag()
	}
}

func dodgers(got []control.Issued[dodge]) []uid.UID64 {
	var who []uid.UID64
	for _, d := range got {
		who = append(who, d.Entity)
	}
	slices.Sort(who)
	return who
}

func TestCond_OrHoldsWhereEitherDoesAndAndWhereBoth(t *testing.T) {
	a, b := Role("a"), Role("b")
	for name, c := range map[string]struct {
		when cond.Cond
		want int
	}{
		"or":  {cond.Self.If(a).Or(cond.Self.If(b)), 2},
		"and": {cond.Self.If(cond.Not(a)).And(cond.Self.If(b)), 1},
	} {
		t.Run(name, func(t *testing.T) {
			var ids []uid.UID64
			tick := plugin.Tick{Dt: time.Millisecond, Roles: rolesAB(&ids)}
			got, dodged, _ := nudged(t, act.Branch[nudge]{
				Name: name,
				When: c.when,
				Then: act.Self.Do(dodge{}),
			}, false, tick, &ids)
			if len(dodged) != c.want {
				t.Errorf("%v dodged of %v; want %d", dodgers(dodged), got, c.want)
			}
		})
	}
}

func TestCond_NotTurnsAWholeConditionRound(t *testing.T) {
	a, b := Role("a"), Role("b")
	var ids []uid.UID64
	tick := plugin.Tick{Dt: time.Millisecond, Roles: rolesAB(&ids)}
	_, dodged, _ := nudged(t, act.Branch[nudge]{
		Name: "neither",
		When: cond.Not(cond.Self.If(a).Or(cond.Self.If(b))),
		Then: act.Self.Do(dodge{}),
	}, false, tick, &ids)
	if len(dodged) != 0 {
		t.Errorf("%v dodged; want none: both play a or b", dodgers(dodged))
	}
}

func TestCond_OtherReadsTheSubjectAndHoldsNotWithNone(t *testing.T) {
	a := Role("a")
	var ids []uid.UID64
	tick := plugin.Tick{Dt: time.Millisecond, Roles: rolesAB(&ids)}
	got, dodged, _ := nudged(t, act.Branch[nudge]{
		Name: "nudged by one not playing a",
		When: cond.Other.If(cond.Not(a)),
		Then: act.Self.Do(dodge{}),
	}, false, tick, &ids)
	if !slices.Equal(dodgers(dodged), got[:1]) {
		t.Errorf("%v dodged; want %v alone: nudged by the one playing b", dodgers(dodged), got[0])
	}
	_, dodged, _ = nudged(t, act.Branch[nudge]{
		Name: "nudged by anybody",
		When: cond.Other.If(cond.Always),
		Then: act.Self.Do(dodge{}),
	}, true, tick, &ids)
	if !slices.Equal(dodgers(dodged), got[1:]) {
		t.Errorf("%v dodged; want %v alone: the first was nudged by nobody",
			dodgers(dodged), got[1])
	}
}

func TestCond_ChanceDrawsForWhomItIsAsked(t *testing.T) {
	var ids []uid.UID64
	tick := plugin.Tick{Dt: time.Millisecond, Time: 7 * time.Millisecond, Seed: 3}
	var ofSelf, ofOther [][]uid.UID64
	for k := range 40 {
		tick.Time = time.Duration(k) * time.Millisecond
		_, mine, _ := nudged(t, act.Branch[nudge]{
			Name: "draw",
			When: cond.Self.If(cond.Chance(0.5)),
			Then: act.Self.Do(dodge{}),
		}, false, tick, &ids)
		_, theirs, _ := nudged(t, act.Branch[nudge]{
			Name: "draw",
			When: cond.Other.If(cond.Chance(0.5)),
			Then: act.Self.Do(dodge{}),
		}, false, tick, &ids)
		ofSelf, ofOther = append(ofSelf, dodgers(mine)), append(ofOther, dodgers(theirs))
	}
	swapped := func(who []uid.UID64) []uid.UID64 {
		var out []uid.UID64
		for _, id := range who {
			out = append(out, ids[1-slices.Index(ids, id)])
		}
		slices.Sort(out)
		return out
	}
	for k := range ofSelf {
		if !slices.Equal(swapped(ofSelf[k]), ofOther[k]) {
			t.Fatalf("step %d: Self drew %v, Other %v; want the other's draw to be its own",
				k, ofSelf[k], ofOther[k])
		}
	}
}

type spot struct {
	self uid.UID64
}

func (s spot) Who() uid.UID64 { return s.self }
func (spot) Placed()          {}

// a rule of a spot over three places round the entity, the middle one playing lake
func placed(t *testing.T, r act.Rule) (places []uid.UID64, dodged, braced []uid.UID64) {
	t.Helper()
	lake := Role("lake")
	places = []uid.UID64{uid.UID64(10), uid.UID64(11), uid.UID64(12)}
	var carrier control.Carrier
	var dodges control.Queue[dodge]
	var braces control.Queue[brace]
	if err := carrier.Carry(&dodges, &braces); err != nil {
		t.Fatal(err)
	}
	h := &plugin.Rules[spot]{}
	if err := h.Add(r); err != nil {
		t.Fatal(err)
	}
	tick := plugin.Tick{Dt: time.Millisecond, Commands: &carrier,
		Around: func(_ any, rings int, each func(uid.UID64)) {
			for _, p := range places[:rings+1] {
				each(p)
			}
		},
		Roles: func(id uid.UID64) uint64 {
			if id == places[1] {
				return 1 << lake.Tag()
			}
			return 0
		}}
	runRule(t, h, tick, func(id uid.UID64) spot { return spot{self: id} })
	dodges.Drain(func(i control.Issued[dodge]) { dodged = append(dodged, i.Entity) })
	braces.Drain(func(i control.Issued[brace]) { braced = append(braced, i.Entity) })
	slices.Sort(dodged)
	slices.Sort(braced)
	return places, dodged, braced
}

// the rule runs for two entities, so each place comes twice
func twice(places ...uid.UID64) []uid.UID64 {
	var out []uid.UID64
	for _, p := range places {
		out = append(out, p, p)
	}
	return out
}

func TestAround_RunsThePartHoldingItsClausesOnceAPlace(t *testing.T) {
	lake := Role("lake")
	places, dodged, braced := placed(t, act.Branch[spot]{
		Name: "freeze",
		When: cond.Self.If(cond.Always),
		Then: act.OneOf(
			act.Branch[any]{
				When: cond.Around(2).If(lake),
				Then: act.Around(2).Do(dodge{}),
			},
			act.Around(2).Do(brace{}),
		),
	})
	if !slices.Equal(dodged, twice(places[1])) ||
		!slices.Equal(braced, twice(places[0], places[2])) {
		t.Errorf("dodged %v, braced %v; want the lake dodging and the other two bracing",
			dodged, braced)
	}
}

func TestAround_InTheWhenTurnsTheWholeBranchToEachPlace(t *testing.T) {
	lake := Role("lake")
	places, dodged, _ := placed(t, act.Branch[spot]{
		Name: "pull",
		When: cond.Self.If(cond.Always).And(cond.Around(2).If(lake)),
		Then: act.Around(2).Do(dodge{}),
	})
	if !slices.Equal(dodged, twice(places[1])) {
		t.Errorf("dodged %v; want the lake alone", dodged)
	}
}

func TestBuild_RefusesWhatTheGrammarDoesNotSay(t *testing.T) {
	a := Role("a")
	seen := cond.NewWord("seen", func(spot) bool { return true }).OfOther()
	for name, c := range map[string]struct {
		r    act.Rule
		want string
	}{
		"two rings": {act.Branch[spot]{
			Name: "rings",
			When: cond.Around(1).If(a),
			Then: act.Around(2).Do(dodge{}),
		}, "one ring"},
		"a word of the other in Self": {act.Branch[spot]{
			Name: "side",
			When: cond.Self.If(seen),
			Then: act.Self.Do(dodge{}),
		}, "of the other"},
		"a word in Around": {act.Branch[spot]{
			Name: "place",
			When: cond.Around(1).If(seen),
			Then: act.Self.Do(dodge{}),
		}, "of Self or of Other"},
		"Not of a state alone": {act.Branch[spot]{
			Name: "nobody",
			When: cond.Not(a),
			Then: act.Self.Do(dodge{}),
		}, "says of nobody"},
		"On of no plugin": {act.Branch[spot]{
			Name: "plugin",
			When: cond.On(42).If(a),
			Then: act.Self.Do(dodge{}),
		}, "no plugin"},
		"a word of another moment": {act.Branch[spot]{
			Name: "moment",
			When: cond.Self.If(cond.NewWord("nudged", func(nudge) bool { return true })),
			Then: act.Self.Do(dodge{}),
		}, "a word of"},
	} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				msg, _ := recover().(string)
				if !strings.Contains(msg, c.want) {
					t.Errorf("panic %q; want one saying %q", msg, c.want)
				}
			}()
			act.Build(c.r)
		})
	}
}
