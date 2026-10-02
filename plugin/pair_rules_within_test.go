package plugin_test

import (
	"fmt"
	"slices"
	"testing"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/plugin"
)

// leader is a third tag of roles, for a within of the sides' own family.
const leader = hunted + 1

// squads is a second family of tags; alpha and bravo are two of its bits.
type squads struct{}

const (
	alpha tag.Tag[squads] = iota
	bravo
)

// More families, past roles and squads, for the limit of eight.
type (
	fam3 struct{}
	fam4 struct{}
	fam5 struct{}
	fam6 struct{}
	fam7 struct{}
	fam8 struct{}
	fam9 struct{}
)

// carrying is what one made-up entity carries of roles and squads.
type carrying struct {
	roles  tag.Tags[roles]
	squads tag.Tags[squads]
}

func as(r ...tag.Tag[roles]) carrying { return carrying{roles: tag.Tags[roles](0).With(r...)} }

func (c carrying) in(s ...tag.Tag[squads]) carrying {
	c.squads = c.squads.With(s...)
	return c
}

// pairWithin is pairRule whose first side must carry within too, as rule.Within narrows a rule.
func pairWithin[FA, FB, P any](a tag.Tag[FA], b tag.Tag[FB], within []plugin.Side, react func(plugin.Tick, P)) any {
	return &pairWithinOf[P]{pairOf: pairOf[P]{a: plugin.SideOf(a), b: plugin.SideOf(b), react: react}, within: within}
}

type pairWithinOf[P any] struct {
	pairOf[P]
	within []plugin.Side
}

func (p *pairWithinOf[P]) PairWithin() []plugin.Side { return p.within }

// hostWithin adds rules to a host of P, makes an entity for each of whom and reads what each
// carries by walking the host's query, as a host does.
func hostWithin[P any](t *testing.T, rules []any, whom ...carrying) (*plugin.PairRules[P], []plugin.Marks) {
	t.Helper()
	h := &plugin.PairRules[P]{}
	for _, r := range rules {
		if err := h.Add(r); err != nil {
			t.Fatalf("Add: %v", err)
		}
	}
	marks := make([]plugin.Marks, len(whom))
	read := 0
	goke.New().Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		var bodies goke.Comp[hostBody]
		var rs goke.Comp[tag.Tags[roles]]
		var ss goke.Comp[tag.Tags[squads]]
		f := si.NewFactory(&bodies, &rs, &ss)
		f.Create(len(whom))
		for made := 0; f.Next(); made += len(f.IDs) {
			for i := range f.IDs {
				bodies.Slice(&f.Cursor)[i].N = made + i
				rs.Slice(&f.Cursor)[i] = whom[made+i].roles
				ss.Slice(&f.Cursor)[i] = whom[made+i].squads
			}
		}

		var walked goke.Comp[hostBody]
		walk := si.NewQueryBuilder(&walked)
		h.Bind(walk)
		walking := walk.Build()
		for walking.All(); walking.Next(); {
			cursor := walking.Cursor()
			for i := range cursor.IDs {
				marks[walked.Slice(cursor)[i].N] = h.InChunk(0, cursor, i)
				read++
			}
		}
	}})
	if read != len(whom) {
		t.Fatalf("read %d entities, want %d", read, len(whom))
	}
	return h, marks
}

// recordingWithin is a rule of hunter and hunted whose hunter carries within too, noting the
// moments it is handed.
func recordingWithin(into *[]int, within ...plugin.Side) any {
	return pairWithin(hunter, hunted, within, func(_ plugin.Tick, n int) { *into = append(*into, n) })
}

// The first side carries the within tag besides its own; the other side carrying it stands in
// for nobody.
func TestPairRules_Within_Dispatch_NeedsTheTagOnTheFirstSide(t *testing.T) {
	var got []int
	h, m := hostWithin[int](t, []any{recordingWithin(&got, plugin.SideOf(alpha))},
		as(hunter).in(alpha), as(hunter), as(hunted), as(hunted).in(alpha))

	h.Dispatch(plugin.Tick{}, m[0], m[2], 1)
	h.Dispatch(plugin.Tick{}, m[1], m[2], 2)
	h.Dispatch(plugin.Tick{}, m[1], m[3], 3)
	h.Dispatch(plugin.Tick{}, m[3], m[0], 4)

	if !slices.Equal(got, []int{1}) {
		t.Errorf("the rule ran for %v, want just 1: the hunter in alpha looking at a hunted", got)
	}
}

// Every within tag must be carried, of the sides' own family and of another alike.
func TestPairRules_Within_Dispatch_NeedsEveryTag(t *testing.T) {
	var got []int
	h, m := hostWithin[int](t, []any{recordingWithin(&got, plugin.SideOf(alpha), plugin.SideOf(leader))},
		as(hunter).in(alpha), as(hunter, leader), as(hunter, leader).in(alpha), as(hunted))

	h.Dispatch(plugin.Tick{}, m[0], m[3], 1)
	h.Dispatch(plugin.Tick{}, m[1], m[3], 2)
	h.Dispatch(plugin.Tick{}, m[2], m[3], 3)

	if !slices.Equal(got, []int{3}) {
		t.Errorf("the rule ran for %v, want just 3: the hunter leading in alpha", got)
	}
}

// A rule without a within beside one with it is not narrowed by it.
func TestPairRules_Within_LeavesOtherRulesAlone(t *testing.T) {
	var narrowed, plain []int
	h, m := hostWithin[int](t, []any{
		recordingWithin(&narrowed, plugin.SideOf(alpha)),
		pairRule(hunter, hunted, func(_ plugin.Tick, n int) { plain = append(plain, n) }),
	}, as(hunter).in(alpha), as(hunter), as(hunted))

	h.Dispatch(plugin.Tick{}, m[0], m[2], 1)
	h.Dispatch(plugin.Tick{}, m[1], m[2], 2)

	if !slices.Equal(narrowed, []int{1}) || !slices.Equal(plain, []int{1, 2}) {
		t.Errorf("the narrowed rule ran for %v, the plain one for %v; want [1] and [1 2]", narrowed, plain)
	}
}

// A family named by a within and by another rule's sides is read once, at one place.
func TestPairRules_Within_SharesItsFamilyWithTheSides(t *testing.T) {
	var narrowed, squad []int
	h, m := hostWithin[int](t, []any{
		recordingWithin(&narrowed, plugin.SideOf(alpha)),
		pairRule(alpha, bravo, func(_ plugin.Tick, n int) { squad = append(squad, n) }),
	}, as(hunter).in(alpha), as(hunted).in(bravo))

	h.Dispatch(plugin.Tick{}, m[0], m[1], 1)
	h.Dispatch(plugin.Tick{}, m[1], m[0], 2)

	if !slices.Equal(narrowed, []int{1}) || !slices.Equal(squad, []int{1}) {
		t.Errorf("the narrowed rule ran for %v, the squads' for %v; want both [1]", narrowed, squad)
	}
}

// A self without the within tag is no group at all, not even an empty one; the others need not
// carry it.
func TestPairRules_Within_DispatchGrouped_NeedsTheTagOnSelf(t *testing.T) {
	var got [][]int
	rule := pairWithin(hunter, hunted, []plugin.Side{plugin.SideOf(alpha)}, func(_ plugin.Tick, g group) {
		got = append(got, append([]int{}, g.others...))
	})
	h, m := hostWithin[group](t, []any{rule}, as(hunter).in(alpha), as(hunter), as(hunted), as(hunted).in(alpha))
	build := func(matched []int) group { return group{others: matched} }

	h.DispatchGrouped(plugin.Tick{}, m[0], []plugin.Marks{m[2], m[3], m[1]}, build)
	h.DispatchGrouped(plugin.Tick{}, m[1], []plugin.Marks{m[2], m[3]}, build)
	h.DispatchGrouped(plugin.Tick{}, m[0], nil, build)
	h.DispatchGrouped(plugin.Tick{}, m[3], []plugin.Marks{m[2]}, build)

	if len(got) != 2 {
		t.Fatalf("the rule ran %d times (%v), want twice: the hunter in alpha with prey and without", len(got), got)
	}
	if !slices.Equal(got[0], []int{0, 1}) || len(got[1]) != 0 {
		t.Errorf("groups = %v, want [[0 1] []]: both hunted, in alpha or not, then none", got)
	}
}

// A pair with no direction runs the way round whose first entity carries the within tag.
func TestPairRules_Within_DispatchEitherWay_FindsTheCarrier(t *testing.T) {
	var got []int
	h, m := hostWithin[int](t, []any{recordingWithin(&got, plugin.SideOf(alpha))},
		as(hunter).in(alpha), as(hunter), as(hunted).in(alpha))

	h.DispatchEitherWay(plugin.Tick{}, m[0], m[2], 1, 2)
	h.DispatchEitherWay(plugin.Tick{}, m[2], m[0], 3, 4)
	h.DispatchEitherWay(plugin.Tick{}, m[1], m[2], 5, 6)
	h.DispatchEitherWay(plugin.Tick{}, m[2], m[1], 7, 8)

	if !slices.Equal(got, []int{1, 4}) {
		t.Errorf("the rule ran for %v, want [1 4]: forward, then backward, the hunter in alpha as Self", got)
	}
}

// The same tag on both sides runs once a pair, with a within as without: the carrier as Self when
// one carries it, forward when both do — a within never adds runs.
func TestPairRules_Within_DispatchEitherWay_SameSidesRunOnce(t *testing.T) {
	var got []int
	rule := pairWithin(hunter, hunter, []plugin.Side{plugin.SideOf(alpha)}, func(_ plugin.Tick, n int) { got = append(got, n) })
	h, m := hostWithin[int](t, []any{rule}, as(hunter).in(alpha), as(hunter), as(hunter).in(alpha))

	h.DispatchEitherWay(plugin.Tick{}, m[0], m[1], 1, 2)
	h.DispatchEitherWay(plugin.Tick{}, m[1], m[0], 3, 4)
	h.DispatchEitherWay(plugin.Tick{}, m[0], m[2], 5, 6)

	if !slices.Equal(got, []int{1, 4, 5}) {
		t.Errorf("the rule ran for %v, want [1 4 5]: the one carrier as Self, then once for two carriers", got)
	}
}

// A within of tag.Any is anybody: it narrows nothing.
func TestPairRules_Within_AnyMatchesEveryone(t *testing.T) {
	var got, alongside []int
	h, m := hostWithin[int](t, []any{
		recordingWithin(&got, plugin.SideOf(tag.Any)),
		recordingWithin(&alongside, plugin.SideOf(tag.Any), plugin.SideOf(alpha)),
	}, as(hunter), as(hunter).in(alpha), as(hunted))

	h.Dispatch(plugin.Tick{}, m[0], m[2], 1)
	h.Dispatch(plugin.Tick{}, m[1], m[2], 2)
	h.Dispatch(plugin.Tick{}, plugin.Marks{}, m[2], 3)

	if !slices.Equal(got, []int{1, 2}) || !slices.Equal(alongside, []int{2}) {
		t.Errorf("Any alone ran for %v, Any beside alpha for %v; want [1 2] and [2]", got, alongside)
	}
}

// A within's families count towards the eight a host reads, each once and tag.Any not at all.
func TestPairRules_Within_FamiliesCountTowardsTheLimit(t *testing.T) {
	var h plugin.PairRules[int]
	eight := []plugin.Side{
		plugin.SideOf(alpha), plugin.SideOf(bravo), plugin.SideOf(leader), plugin.SideOf(tag.Any),
		plugin.SideOf(tag.Tag[fam3](0)), plugin.SideOf(tag.Tag[fam4](0)), plugin.SideOf(tag.Tag[fam5](0)),
		plugin.SideOf(tag.Tag[fam6](0)), plugin.SideOf(tag.Tag[fam7](0)), plugin.SideOf(tag.Tag[fam8](0)),
	}
	if msg := panicOf(func() { _ = h.Add(pairWithin(hunter, hunted, eight, func(plugin.Tick, int) {})) }); msg != nil {
		t.Fatalf("Add of a rule naming eight families panicked: %v", msg)
	}
	if msg := panicOf(func() { _ = h.Add(pairWithin(hunter, alpha, eight, func(plugin.Tick, int) {})) }); msg != nil {
		t.Fatalf("Add of a second rule over the same eight panicked: %v", msg)
	}

	ninth := []plugin.Side{plugin.SideOf(tag.Tag[fam9](0))}
	msg := panicOf(func() { _ = h.Add(pairWithin(hunter, hunted, ninth, func(plugin.Tick, int) {})) })
	if want := "plugin: pair rules name more than 8 tag families"; fmt.Sprint(msg) != want {
		t.Errorf("Add of a ninth family's within panicked with %v, want %q", msg, want)
	}
}

// A single rule whose within names a ninth family is refused the same way.
func TestPairRules_Within_OneRuleOverTheLimitPanics(t *testing.T) {
	var h plugin.PairRules[int]
	nine := []plugin.Side{
		plugin.SideOf(alpha), plugin.SideOf(tag.Tag[fam3](0)), plugin.SideOf(tag.Tag[fam4](0)),
		plugin.SideOf(tag.Tag[fam5](0)), plugin.SideOf(tag.Tag[fam6](0)), plugin.SideOf(tag.Tag[fam7](0)),
		plugin.SideOf(tag.Tag[fam8](0)), plugin.SideOf(tag.Tag[fam9](0)),
	}
	msg := panicOf(func() { _ = h.Add(pairWithin(hunter, hunted, nine, func(plugin.Tick, int) {})) })
	if want := "plugin: pair rules name more than 8 tag families"; fmt.Sprint(msg) != want {
		t.Errorf("Add panicked with %v, want %q", msg, want)
	}
}

// panicOf is what f panics with, nil if it returns.
func panicOf(f func()) (msg any) {
	defer func() { msg = recover() }()
	f()
	return nil
}
