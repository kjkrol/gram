package rule_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/uid"
)

// roles and ranks are the tag families these tests name.
type (
	roles struct{}
	ranks struct{}
)

const (
	hunter tag.Tag[roles] = iota
	hunted
	scout
)

const (
	veteran tag.Tag[ranks] = iota
	rookie
)

// armour is a component a Having rule asks for.
type armour struct{ N int }

// poke is a made-up host's moment of one entity.
type poke struct{ self uid.UID64 }

func (p poke) Who() uid.UID64 { return p.self }

// glance is a made-up host's moment of a pair: one entity glancing at another, its Subject.
type glance struct{ self, other uid.UID64 }

func (g glance) Who() uid.UID64             { return g.self }
func (g glance) Whom(each func(uid.UID64))  { each(g.other) }
func (g glance) Subject() (uid.UID64, bool) { return g.other, true }

// look is a made-up host's moment of one entity against all it sees.
type look struct {
	self uid.UID64
	seen []uid.UID64
}

func (l look) Who() uid.UID64 { return l.self }
func (l look) Whom(each func(uid.UID64)) {
	for _, s := range l.seen {
		each(s)
	}
}

// dawn is a made-up moment of the world as a whole, about its own entity, as a clock.Moment is.
type dawn struct{ world uid.UID64 }

func (d dawn) Who() uid.UID64 { return d.world }

// noted is a command a rule's entity gives itself: whom the rule fired for.
type noted struct{}

// toward is a command aimed at a glance's other: which pair a rule fired for.
type toward struct{ Of uid.UID64 }

func (c *toward) Aim(who uid.UID64) { c.Of = who }

// told carries the commands the rules give to queues of their own.
type told struct {
	carrier control.Carrier
	notes   control.Queue[noted]
	aims    control.Queue[toward]
}

func newTold(t *testing.T) *told {
	t.Helper()
	k := &told{}
	if err := k.carrier.Carry(&k.notes, &k.aims); err != nil {
		t.Fatal(err)
	}
	return k
}

func (k *told) tick() plugin.Tick { return plugin.Tick{Commands: &k.carrier} }

// who is whom the rules noted since the last call, by place in ids, sorted.
func (k *told) who(ids []uid.UID64) []int {
	var got []int
	k.notes.Drain(func(i control.Issued[noted]) { got = append(got, slices.Index(ids, i.Entity)) })
	slices.Sort(got)
	return got
}

// firing is a pair a rule fired for, by the places of its entities.
type firing struct{ self, other int }

// pairs is the pairs the rules fired for since the last call, in the order they did.
func (k *told) pairs(ids []uid.UID64) []firing {
	var got []firing
	k.aims.Drain(func(i control.Issued[toward]) {
		got = append(got, firing{slices.Index(ids, i.Entity), slices.Index(ids, i.Command.Of)})
	})
	return got
}

// kit is what one entity of a test carries: its tags of each family (nil: not the component) and
// armour.
type kit struct {
	roles  *tag.Tags[roles]
	ranks  *tag.Tags[ranks]
	armour bool
}

func as(ts ...tag.Tag[roles]) *tag.Tags[roles] {
	s := tag.Tags[roles](0).With(ts...)
	return &s
}

func ranked(ts ...tag.Tag[ranks]) *tag.Tags[ranks] {
	s := tag.Tags[ranks](0).With(ts...)
	return &s
}

// spawn makes an entity of each kit, a token and all, and hands back their ids.
func spawn(si *goke.SysInit, kits []kit) []uid.UID64 {
	ids := make([]uid.UID64, len(kits))
	for k, kt := range kits {
		var tokens goke.Comp[token]
		var rs goke.Comp[tag.Tags[roles]]
		var rk goke.Comp[tag.Tags[ranks]]
		var ar goke.Comp[armour]
		comps := []goke.Addable{&tokens}
		if kt.roles != nil {
			comps = append(comps, &rs)
		}
		if kt.ranks != nil {
			comps = append(comps, &rk)
		}
		if kt.armour {
			comps = append(comps, &ar)
		}
		f := si.NewFactory(comps...)
		f.Create(1)
		for f.Next() {
			ids[k] = f.IDs[0]
			if kt.roles != nil {
				rs.Slice(&f.Cursor)[0] = *kt.roles
			}
			if kt.ranks != nil {
				rk.Slice(&f.Cursor)[0] = *kt.ranks
			}
		}
	}
	return ids
}

// firedFor runs r through a plugin.Rules of pokes over an entity of each kit, as a host's pass
// over its entities does, and hands back the places of the kits it fired for.
func firedFor(t *testing.T, r rule.Rule, kits ...kit) []int {
	t.Helper()
	k := newTold(t)
	h := &plugin.Rules[poke]{}
	if err := h.Add(r); err != nil {
		t.Fatalf("Add(%v): %v", r, err)
	}
	var ids []uid.UID64
	goke.New().Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		ids = spawn(si, kits)
		var tokens goke.Comp[token]
		qb := si.NewQueryBuilder(&tokens)
		h.Bind(qb)
		q := qb.Build()
		for q.All(); q.Next(); {
			cur := q.Cursor()
			h.Run(k.tick(), cur, func(i int) poke { return poke{self: cur.IDs[i]} })
		}
	}})
	return k.who(ids)
}

// marksOf binds h, spawns an entity of each kit and reads what each carries off the chunks, as a
// host's pass walking its entities does.
func marksOf[P any](t *testing.T, h *plugin.PairRules[P], kits ...kit) ([]plugin.Marks, []uid.UID64) {
	t.Helper()
	marks := make([]plugin.Marks, len(kits))
	var ids []uid.UID64
	goke.New().Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		ids = spawn(si, kits)
		var tokens goke.Comp[token]
		qb := si.NewQueryBuilder(&tokens)
		h.Bind(qb)
		q := qb.Build()
		for q.All(); q.Next(); {
			cur := q.Cursor()
			for i, id := range cur.IDs {
				marks[slices.Index(ids, id)] = h.InChunk(0, cur, i)
			}
		}
	}})
	return marks, ids
}

// firedPairs dispatches every ordered pair of an entity of each kit to a plugin.PairRules holding
// r, as a host of directed pairs does, and hands back the pairs r fired for.
func firedPairs(t *testing.T, r rule.Rule, kits ...kit) []firing {
	t.Helper()
	k := newTold(t)
	h := &plugin.PairRules[glance]{}
	if err := h.Add(r); err != nil {
		t.Fatalf("Add(%v): %v", r, err)
	}
	marks, ids := marksOf(t, h, kits...)
	for i := range ids {
		for j := range ids {
			if i != j {
				h.Dispatch(k.tick(), marks[i], marks[j], glance{self: ids[i], other: ids[j]})
			}
		}
	}
	return k.pairs(ids)
}

// everyPair is every ordered pair of n entities whose first is among selves and whose other among
// others (nil: anybody), in the order firedPairs dispatches them.
func everyPair(n int, selves, others []int) []firing {
	var all []firing
	for i := range n {
		for j := range n {
			if i != j && slices.Contains(selves, i) && (others == nil || slices.Contains(others, j)) {
				all = append(all, firing{i, j})
			}
		}
	}
	return all
}

func noteOf[P any](name string, filter rule.Filter) rule.Rule {
	return rule.Then[P](name, filter, rule.Order(noted{}))
}

func aimOf[P any](name string, filter rule.Filter) rule.Rule {
	return rule.Then[P](name, filter, rule.Order(toward{}))
}

// A rule over every entity, narrowed to hunters, fires for those carrying the tag alone: not for
// one without the family, one carrying another bit of it, or one of another family.
func TestWithin_AllFiresForTheCarriersAlone(t *testing.T) {
	kits := []kit{
		{},                         // 0: no tags at all
		{roles: as(hunter)},        // 1
		{roles: as(hunted)},        // 2: another bit of the family
		{roles: as(hunter, scout)}, // 3
		{ranks: ranked(veteran)},   // 4: another family alone
		{roles: as()},              // 5: the family, no bit
	}
	plain := noteOf[poke]("note", rule.All)
	narrowed := rule.Within(hunter, plain)

	if got, want := firedFor(t, narrowed, kits...), []int{1, 3}; !slices.Equal(got, want) {
		t.Errorf("narrowed to hunters, the rule fired for %v; want %v", got, want)
	}
	if got, want := firedFor(t, plain, kits...), []int{0, 1, 2, 3, 4, 5}; !slices.Equal(got, want) {
		t.Errorf("the rule itself fired for %v after Within; want every entity %v", got, want)
	}
}

// Narrowed, a Self rule needs both its own tag and the one it is narrowed to, of one family or two.
func TestWithin_SelfNeedsBothTags(t *testing.T) {
	spot := noteOf[poke]("spot", rule.Self(hunter))

	sameFamily := []kit{
		{roles: as(hunter)},        // 0
		{roles: as(scout)},         // 1
		{roles: as(hunter, scout)}, // 2
		{roles: as(hunted, scout)}, // 3
		{},                         // 4
	}
	if got, want := firedFor(t, rule.Within(scout, spot), sameFamily...), []int{2}; !slices.Equal(got, want) {
		t.Errorf("Self(hunter) within scouts fired for %v; want %v", got, want)
	}

	twoFamilies := []kit{
		{roles: as(hunter)},                         // 0
		{ranks: ranked(veteran)},                    // 1
		{roles: as(hunter), ranks: ranked(veteran)}, // 2
		{roles: as(hunter), ranks: ranked(rookie)},  // 3
		{roles: as(hunted), ranks: ranked(veteran)}, // 4
	}
	if got, want := firedFor(t, rule.Within(veteran, spot), twoFamilies...), []int{2}; !slices.Equal(got, want) {
		t.Errorf("Self(hunter) within veterans fired for %v; want %v", got, want)
	}
	if got, want := firedFor(t, spot, twoFamilies...), []int{0, 2, 3}; !slices.Equal(got, want) {
		t.Errorf("Self(hunter) itself fired for %v after Within; want %v", got, want)
	}
}

// Narrowed, a Having rule needs both the component and the tag.
func TestWithin_HavingNeedsTheComponentAndTheTag(t *testing.T) {
	kits := []kit{
		{armour: true},                    // 0
		{roles: as(hunter)},               // 1
		{roles: as(hunter), armour: true}, // 2
		{roles: as(hunted), armour: true}, // 3
		{roles: as(), armour: true},       // 4
	}
	armoured := noteOf[poke]("armoured", rule.Having[armour]())

	if got, want := firedFor(t, rule.Within(hunter, armoured), kits...), []int{2}; !slices.Equal(got, want) {
		t.Errorf("Having[armour] within hunters fired for %v; want %v", got, want)
	}
	if got, want := firedFor(t, armoured, kits...), []int{0, 2, 3, 4}; !slices.Equal(got, want) {
		t.Errorf("Having[armour] itself fired for %v after Within; want %v", got, want)
	}
}

// Narrowed twice, a rule needs both tags, of one family or two, and the rule narrowed once is
// left as it was.
func TestWithin_TwiceNeedsBothTags(t *testing.T) {
	kits := []kit{
		{roles: as(hunter)},                                // 0
		{ranks: ranked(veteran)},                           // 1
		{roles: as(hunter), ranks: ranked(veteran)},        // 2
		{roles: as(hunted), ranks: ranked(veteran)},        // 3
		{roles: as(hunter), ranks: ranked(rookie)},         // 4
		{roles: as(hunter, scout), ranks: ranked(veteran)}, // 5
		{roles: as(hunter, scout)},                         // 6
	}
	hunters := rule.Within(hunter, noteOf[poke]("note", rule.All))

	if got, want := firedFor(t, rule.Within(veteran, hunters), kits...), []int{2, 5}; !slices.Equal(got, want) {
		t.Errorf("within hunters and veterans the rule fired for %v; want %v", got, want)
	}
	if got, want := firedFor(t, rule.Within(scout, hunters), kits...), []int{5, 6}; !slices.Equal(got, want) {
		t.Errorf("within hunters and scouts the rule fired for %v; want %v", got, want)
	}
	if got, want := firedFor(t, hunters, kits...), []int{0, 2, 4, 5, 6}; !slices.Equal(got, want) {
		t.Errorf("within hunters alone the rule fired for %v after narrowing it again; want %v", got, want)
	}

	armoured := []kit{
		{roles: as(hunter), armour: true},                         // 0
		{roles: as(hunter), ranks: ranked(veteran), armour: true}, // 1
		{roles: as(hunter), ranks: ranked(veteran)},               // 2
		{ranks: ranked(veteran), armour: true},                    // 3
	}
	thrice := rule.Within(veteran, rule.Within(hunter, noteOf[poke]("armoured", rule.Having[armour]())))
	if got, want := firedFor(t, thrice, armoured...), []int{1}; !slices.Equal(got, want) {
		t.Errorf("Having[armour] within hunters and veterans fired for %v; want %v", got, want)
	}
}

// Within tag.Any is the rule itself, of one entity or of pairs.
func TestWithin_AnyIsTheRuleItself(t *testing.T) {
	for _, r := range []rule.Rule{
		noteOf[poke]("all", rule.All),
		noteOf[poke]("self", rule.Self(hunter)),
		noteOf[poke]("having", rule.Having[armour]()),
		rule.Within(hunter, noteOf[poke]("narrowed", rule.All)),
		aimOf[glance]("between", rule.Between(hunter, hunted)),
		aimOf[glance]("pairs", rule.All),
	} {
		if got := rule.Within(tag.Any, r); got != r {
			t.Errorf("Within(tag.Any, %v) = %v, a rule of its own; want the rule itself", r, got)
		}
	}

	kits := []kit{{}, {roles: as(hunter)}, {roles: as(hunted)}, {ranks: ranked(veteran)}}
	all := noteOf[poke]("all", rule.All)
	if got, want := firedFor(t, rule.Within(tag.Any, all), kits...), firedFor(t, all, kits...); !slices.Equal(got, want) {
		t.Errorf("within anybody the rule fired for %v; the rule itself for %v", got, want)
	}
	between := aimOf[glance]("between", rule.Between(hunter, hunted))
	if got, want := firedPairs(t, rule.Within(tag.Any, between), kits...), firedPairs(t, between, kits...); !slices.Equal(got, want) {
		t.Errorf("within anybody the pair rule fired for %v; the rule itself for %v", got, want)
	}
}

// A rule of a moment of the world as a whole walks no entities: its host refuses it narrowed and
// still takes it as it is.
func TestWithin_StepRulesRefuseANarrowedRule(t *testing.T) {
	wake := noteOf[dawn]("wake", rule.All)
	var h plugin.StepRules[dawn]

	if err := h.Add(rule.Within(hunter, wake)); !errors.Is(err, plugin.ErrUnhosted) {
		t.Errorf("Add(Within(hunter, wake)) = %v, want plugin.ErrUnhosted", err)
	}
	if !h.Empty() {
		t.Error("the narrowed rule was taken")
	}
	if err := h.Add(rule.Within(tag.Any, wake)); err != nil {
		t.Fatalf("Add(Within(tag.Any, wake)) = %v; want it taken, the rule itself", err)
	}
	k := newTold(t)
	h.Run(k.tick(), dawn{world: 7})
	if got := k.who([]uid.UID64{7}); !slices.Equal(got, []int{0}) {
		t.Errorf("the rule fired for %v; want the world's own entity once", got)
	}
}

// Narrowed, a pair rule is dispatched for the pairs whose first entity carries the tag besides
// its own side's, whatever the other carries; the rule narrowed is left as it was.
func TestWithin_PairRulesDispatchForAFirstSideCarryingTheTag(t *testing.T) {
	kits := []kit{
		{roles: as(hunter)},                         // 0
		{roles: as(hunter, scout)},                  // 1
		{roles: as(hunted)},                         // 2
		{roles: as(hunted, scout)},                  // 3
		{roles: as(hunter), ranks: ranked(veteran)}, // 4
		{}, // 5
		{roles: as(hunter, scout), ranks: ranked(veteran)}, // 6
	}
	n := len(kits)
	between := aimOf[glance]("glance", rule.Between(hunter, hunted))
	for _, c := range []struct {
		name string
		r    rule.Rule
		want []firing
	}{
		{"between hunter and hunted", between, everyPair(n, []int{0, 1, 4, 6}, []int{2, 3})},
		{"within scouts", rule.Within(scout, between), everyPair(n, []int{1, 6}, []int{2, 3})},
		{"within veterans, another family", rule.Within(veteran, between), everyPair(n, []int{4, 6}, []int{2, 3})},
		{"within scouts and veterans", rule.Within(veteran, rule.Within(scout, between)), everyPair(n, []int{6}, []int{2, 3})},
		{"every pair within scouts", rule.Within(scout, aimOf[glance]("all", rule.All)), everyPair(n, []int{1, 3, 6}, nil)},
		{"Self(hunter) within scouts", rule.Within(scout, aimOf[glance]("self", rule.Self(hunter))), everyPair(n, []int{1, 6}, nil)},
		{"between anybody and hunted within veterans", rule.Within(veteran, aimOf[glance]("any", rule.Between(tag.Any, hunted))), everyPair(n, []int{4, 6}, []int{2, 3})},
		{"between hunter and hunted, after Within", between, everyPair(n, []int{0, 1, 4, 6}, []int{2, 3})},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := firedPairs(t, c.r, kits...); !slices.Equal(got, c.want) {
				t.Errorf("fired for %v; want %v", got, c.want)
			}
		})
	}
}

// Narrowed, a pair rule of one against many runs only for a first side carrying the tag — with the
// others matched, or none — and never for one without it.
func TestWithin_PairRulesDispatchGroupedForAFirstSideCarryingTheTag(t *testing.T) {
	kits := []kit{
		{roles: as(hunter)},        // 0
		{roles: as(hunter, scout)}, // 1
		{roles: as(hunted)},        // 2
		{roles: as(hunted, scout)}, // 3
	}
	plain := noteOf[look]("look", rule.Between(hunter, hunted))
	for _, c := range []struct {
		name string
		r    rule.Rule
		want []int // who looked: twice each, with others in view and with none
	}{
		{"between hunter and hunted", plain, []int{0, 0, 1, 1}},
		{"within scouts", rule.Within(scout, plain), []int{1, 1}},
	} {
		t.Run(c.name, func(t *testing.T) {
			k := newTold(t)
			h := &plugin.PairRules[look]{}
			if err := h.Add(c.r); err != nil {
				t.Fatalf("Add: %v", err)
			}
			marks, ids := marksOf(t, h, kits...)
			others := []plugin.Marks{marks[2], marks[0], marks[3]}
			var matched [][]int
			for i := range kits {
				for _, in := range [][]plugin.Marks{others, nil} {
					h.DispatchGrouped(k.tick(), marks[i], in, func(m []int) look {
						matched = append(matched, slices.Clone(m))
						return look{self: ids[i]}
					})
				}
			}
			if got := k.who(ids); !slices.Equal(got, c.want) {
				t.Errorf("the rule ran for %v; want %v", got, c.want)
			}
			if len(matched) != len(c.want) {
				t.Fatalf("the moment was built %d times; want %d", len(matched), len(c.want))
			}
			for i, m := range matched {
				want := []int{0, 2}
				if i%2 == 1 {
					want = nil
				}
				if !slices.Equal(m, want) {
					t.Errorf("run %d matched %v; want %v", i, m, want)
				}
			}
		})
	}
}

// Narrowed, a pair rule with no direction is handed the pair with the side carrying the tag as
// its first, whichever way round the pair comes.
func TestWithin_PairRulesDispatchEitherWayHandTheCarrierAsTheFirst(t *testing.T) {
	kits := []kit{
		{roles: as(hunter)},        // 0
		{roles: as(hunter, scout)}, // 1
		{roles: as(hunted)},        // 2
	}
	k := newTold(t)
	h := &plugin.PairRules[glance]{}
	if err := h.Add(rule.Within(scout, aimOf[glance]("glance", rule.Between(hunter, hunted)))); err != nil {
		t.Fatalf("Add: %v", err)
	}
	marks, ids := marksOf(t, h, kits...)
	either := func(a, b int) []firing {
		h.DispatchEitherWay(k.tick(), marks[a], marks[b], glance{ids[a], ids[b]}, glance{ids[b], ids[a]})
		return k.pairs(ids)
	}

	for _, c := range []struct {
		a, b int
		want []firing
	}{
		{2, 1, []firing{{1, 2}}},
		{1, 2, []firing{{1, 2}}},
		{2, 0, nil},
		{0, 2, nil},
	} {
		if got := either(c.a, c.b); !slices.Equal(got, c.want) {
			t.Errorf("DispatchEitherWay(%d, %d) fired for %v; want %v", c.a, c.b, got, c.want)
		}
	}
}

// One tag on both sides runs once either way round; narrowed, the rule is no longer the same
// either way: it runs for each side carrying the tag, as its first.
func TestWithin_PairRulesDispatchEitherWayOfOneTagOnBothSides(t *testing.T) {
	kits := []kit{
		{roles: as(hunter)},        // 0
		{roles: as(hunter, scout)}, // 1
		{roles: as(hunter, scout)}, // 2
		{roles: as(hunter)},        // 3
	}
	meet := aimOf[glance]("meet", rule.Between(hunter, hunter))
	for _, c := range []struct {
		name  string
		r     rule.Rule
		cases []struct {
			a, b int
			want []firing
		}
	}{
		{"between hunters", meet, []struct {
			a, b int
			want []firing
		}{
			{0, 3, []firing{{0, 3}}},
			{1, 2, []firing{{1, 2}}},
			{0, 1, []firing{{0, 1}}},
		}},
		{"within scouts", rule.Within(scout, meet), []struct {
			a, b int
			want []firing
		}{
			{0, 3, nil},
			{0, 1, []firing{{1, 0}}},
			{1, 0, []firing{{1, 0}}},
			{1, 2, []firing{{1, 2}}}, // both scouts: once, as the plain rule, never more
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			k := newTold(t)
			h := &plugin.PairRules[glance]{}
			if err := h.Add(c.r); err != nil {
				t.Fatalf("Add: %v", err)
			}
			marks, ids := marksOf(t, h, kits...)
			for _, e := range c.cases {
				h.DispatchEitherWay(k.tick(), marks[e.a], marks[e.b], glance{ids[e.a], ids[e.b]}, glance{ids[e.b], ids[e.a]})
				if got := k.pairs(ids); !slices.Equal(got, e.want) {
					t.Errorf("DispatchEitherWay(%d, %d) fired for %v; want %v", e.a, e.b, got, e.want)
				}
			}
		})
	}
}

// A rule's String is its name and its moment, and how it is narrowed; a role's names the role.
func TestRule_StringNamesItAndItsMoment(t *testing.T) {
	for _, c := range []struct {
		r    rule.Rule
		want string
	}{
		{noteOf[poke]("drown", rule.All), `"drown" of rule_test.poke`},
		{noteOf[poke]("spot", rule.Self(hunter)), `"spot" of rule_test.poke`},
		{noteOf[poke]("armoured", rule.Having[armour]()), `"armoured" of rule_test.poke`},
		{aimOf[glance]("glance", rule.Between(hunter, hunted)), `"glance" of rule_test.glance`},
		{noteOf[dawn]("wake", rule.All), `"wake" of rule_test.dawn`},
		{rule.Within(scout, noteOf[poke]("drown", rule.All)), `"drown" of rule_test.poke, within`},
		{rule.Within(veteran, noteOf[poke]("spot", rule.Self(hunter))), `"spot" of rule_test.poke, within`},
		{rule.Within(scout, aimOf[glance]("glance", rule.Between(hunter, hunted))), `"glance" of rule_test.glance, within`},
		{rule.Role("string mortal").Obeys(noteOf[poke]("drown", rule.All)).Rules()[0], `"drown" of rule_test.poke, for the role string mortal`},
		{rule.Role("string mortal"), `the role string mortal`},
	} {
		if got := c.r.String(); got != c.want {
			t.Errorf("String() = %s; want %s", got, c.want)
		}
	}
}

// A rule over one entity — narrowed, Self or Having — fires through RunWhere for the entities its
// keep lets through alone: a host's moment only some have (collision's Struck).
func TestWithin_RunWhereKeepsToWhatKeepLetsThrough(t *testing.T) {
	kits := []kit{{roles: as(hunter), armour: true}, {roles: as(hunter), armour: true}, {roles: as(hunted)}}
	for _, c := range []struct {
		name string
		r    rule.Rule
	}{
		{"within", rule.Within(hunter, noteOf[poke]("drown", rule.All))},
		{"self", noteOf[poke]("spot", rule.Self(hunter))},
		{"having", noteOf[poke]("armoured", rule.Having[armour]())},
	} {
		t.Run(c.name, func(t *testing.T) {
			k := newTold(t)
			h := &plugin.Rules[poke]{}
			if err := h.Add(c.r); err != nil {
				t.Fatal(err)
			}
			var ids []uid.UID64
			goke.New().Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
				ids = spawn(si, kits)
				var tokens goke.Comp[token]
				qb := si.NewQueryBuilder(&tokens)
				h.Bind(qb)
				q := qb.Build()
				for q.All(); q.Next(); {
					cur := q.Cursor()
					keep := func(i int) bool { return cur.IDs[i] != ids[0] } // the first hunter struck nothing
					h.RunWhere(k.tick(), cur, keep, func(i int) poke { return poke{self: cur.IDs[i]} })
				}
			}})
			if got := k.who(ids); !slices.Equal(got, []int{1}) {
				t.Errorf("fired for %v; want [1], the hunter keep lets through", got)
			}
		})
	}
}
