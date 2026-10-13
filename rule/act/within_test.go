package act_test

import (
	"errors"
	"fmt"
	"slices"
	"testing"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/rule/act"
	"github.com/kjkrol/gram/rule/cond"
	"github.com/kjkrol/uid"
)

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

type armour struct{ N int }

type poke struct{ self uid.UID64 }

func (p poke) Who() uid.UID64 { return p.self }

type glance struct{ self, other uid.UID64 }

func (g glance) Who() uid.UID64             { return g.self }
func (g glance) Whom(each func(uid.UID64))  { each(g.other) }
func (g glance) Subject() (uid.UID64, bool) { return g.other, true }

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

type dawn struct{ world uid.UID64 }

func (d dawn) Who() uid.UID64 { return d.world }

type noted struct{}

func (noted) Command() {}

type toward struct{ Of uid.UID64 }

func (toward) Command() {}

func (c *toward) Aim(who uid.UID64) { c.Of = who }

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

func (k *told) who(ids []uid.UID64) []int {
	var got []int
	k.notes.Drain(func(i control.Issued[noted]) { got = append(got, slices.Index(ids, i.Entity)) })
	slices.Sort(got)
	return got
}

type firing struct{ self, other int }

func (k *told) pairs(ids []uid.UID64) []firing {
	var got []firing
	k.aims.Drain(func(i control.Issued[toward]) {
		got = append(got, firing{slices.Index(ids, i.Entity), slices.Index(ids, i.Command.Of)})
	})
	return got
}

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

func firedFor(t *testing.T, r act.Rule, kits ...kit) []int {
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

func firedPairs(t *testing.T, r act.Rule, kits ...kit) []firing {
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

func noteOf[P any](name string) act.Rule {
	return act.Branch[P]{Name: name, When: cond.Self.If(cond.Always), Then: act.Self.Do(noted{})}
}

func aimOf[P any](name string) act.Rule {
	return act.Branch[P]{Name: name, When: cond.Self.If(cond.Always), Then: act.Self.Do(toward{})}
}

// a rule of P for the carriers of self whose other carries other
func between[P any](name string, self tag.Tag[roles], other tag.Tag[roles]) act.Rule {
	return act.Within(self, act.WithOther(other, aimOf[P](name)))
}

func TestWithin_AllFiresForTheCarriersAlone(t *testing.T) {
	kits := []kit{
		{},                         // 0: no tags at all
		{roles: as(hunter)},        // 1
		{roles: as(hunted)},        // 2: another bit of the family
		{roles: as(hunter, scout)}, // 3
		{ranks: ranked(veteran)},   // 4: another family alone
		{roles: as()},              // 5: the family, no bit
	}
	plain := noteOf[poke]("note")
	narrowed := act.Within(hunter, plain)

	if got, want := firedFor(t, narrowed, kits...), []int{1, 3}; !slices.Equal(got, want) {
		t.Errorf("narrowed to hunters, the rule fired for %v; want %v", got, want)
	}
	if got, want := firedFor(t, plain, kits...), []int{0, 1, 2, 3, 4, 5}; !slices.Equal(got, want) {
		t.Errorf("the rule itself fired for %v after Within; want every entity %v", got, want)
	}
}

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
	hunters := act.Within(hunter, noteOf[poke]("note"))

	if got, want := firedFor(t, act.Within(veteran, hunters), kits...), []int{2, 5}; !slices.Equal(got, want) {
		t.Errorf("within hunters and veterans the rule fired for %v; want %v", got, want)
	}
	if got, want := firedFor(t, act.Within(scout, hunters), kits...), []int{5, 6}; !slices.Equal(got, want) {
		t.Errorf("within hunters and scouts the rule fired for %v; want %v", got, want)
	}
	if got, want := firedFor(t, hunters, kits...), []int{0, 2, 4, 5, 6}; !slices.Equal(got, want) {
		t.Errorf("within hunters alone the rule fired for %v after narrowing it again; want %v", got, want)
	}

}

func TestWithin_AnyIsTheRuleItself(t *testing.T) {
	for _, r := range []act.Rule{
		noteOf[poke]("all"),
		act.Within(hunter, noteOf[poke]("narrowed")),
		between[glance]("between", hunter, hunted),
		aimOf[glance]("pairs"),
	} {
		if got := act.Within(tag.Any, r); fmt.Sprint(got) != fmt.Sprint(r) {
			t.Errorf("act.Within(tag.Any, %v) = %v, a rule of its own; want the rule itself", r, got)
		}
	}

	kits := []kit{{}, {roles: as(hunter)}, {roles: as(hunted)}, {ranks: ranked(veteran)}}
	all := noteOf[poke]("all")
	if got, want := firedFor(t, act.Within(tag.Any, all), kits...), firedFor(t, all, kits...); !slices.Equal(got, want) {
		t.Errorf("within anybody the rule fired for %v; the rule itself for %v", got, want)
	}
	between := between[glance]("between", hunter, hunted)
	if got, want := firedPairs(t, act.Within(tag.Any, between), kits...), firedPairs(t, between, kits...); !slices.Equal(got, want) {
		t.Errorf("within anybody the pair rule fired for %v; the rule itself for %v", got, want)
	}
}

func TestWithin_StepRulesRefuseANarrowedRule(t *testing.T) {
	wake := noteOf[dawn]("wake")
	var h plugin.StepRules[dawn]

	if err := h.Add(act.Within(hunter, wake)); !errors.Is(err, plugin.ErrUnhosted) {
		t.Errorf("Add(act.Within(hunter, wake)) = %v, want plugin.ErrUnhosted", err)
	}
	if !h.Empty() {
		t.Error("the narrowed rule was taken")
	}
	if err := h.Add(act.Within(tag.Any, wake)); err != nil {
		t.Fatalf("Add(act.Within(tag.Any, wake)) = %v; want it taken, the rule itself", err)
	}
	k := newTold(t)
	h.Run(k.tick(), dawn{world: 7})
	if got := k.who([]uid.UID64{7}); !slices.Equal(got, []int{0}) {
		t.Errorf("the rule fired for %v; want the world's own entity once", got)
	}
}

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
	between := between[glance]("glance", hunter, hunted)
	for _, c := range []struct {
		name string
		r    act.Rule
		want []firing
	}{
		{"between hunter and hunted", between, everyPair(n, []int{0, 1, 4, 6}, []int{2, 3})},
		{"within scouts", act.Within(scout, between), everyPair(n, []int{1, 6}, []int{2, 3})},
		{"within veterans, another family", act.Within(veteran, between), everyPair(n, []int{4, 6}, []int{2, 3})},
		{"within scouts and veterans", act.Within(veteran, act.Within(scout, between)),
			everyPair(n, []int{6}, []int{2, 3})},
		{"every pair within scouts", act.Within(scout, aimOf[glance]("all")),
			everyPair(n, []int{1, 3, 6}, nil)},
		{"hunters within scouts",
			act.Within(scout, act.Within(hunter, aimOf[glance]("self"))),
			everyPair(n, []int{1, 6}, nil)},
		{"between anybody and hunted within veterans",
			act.Within(veteran, act.WithOther(hunted, aimOf[glance]("any"))),
			everyPair(n, []int{4, 6}, []int{2, 3})},
		{"between hunter and hunted, after Within", between, everyPair(n, []int{0, 1, 4, 6}, []int{2, 3})},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := firedPairs(t, c.r, kits...); !slices.Equal(got, c.want) {
				t.Errorf("fired for %v; want %v", got, c.want)
			}
		})
	}
}

func TestWithin_PairRulesDispatchGroupedForAFirstSideCarryingTheTag(t *testing.T) {
	kits := []kit{
		{roles: as(hunter)},        // 0
		{roles: as(hunter, scout)}, // 1
		{roles: as(hunted)},        // 2
		{roles: as(hunted, scout)}, // 3
	}
	plain := act.Within(hunter, act.WithOther(hunted, noteOf[look]("look")))
	for _, c := range []struct {
		name string
		r    act.Rule
		want []int // once each, with others in view; not with none: the rule names the other
	}{
		{"between hunter and hunted", plain, []int{0, 1}},
		{"within scouts", act.Within(scout, plain), []int{1}},
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
				if want := []int{0, 2}; !slices.Equal(m, want) {
					t.Errorf("run %d matched %v; want %v", i, m, want)
				}
			}
		})
	}
}

func TestWithin_PairRulesDispatchEitherWayHandTheCarrierAsTheFirst(t *testing.T) {
	kits := []kit{
		{roles: as(hunter)},        // 0
		{roles: as(hunter, scout)}, // 1
		{roles: as(hunted)},        // 2
	}
	k := newTold(t)
	h := &plugin.PairRules[glance]{}
	if err := h.Add(act.Within(scout, between[glance]("glance", hunter, hunted))); err != nil {
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

func TestWithin_PairRulesDispatchEitherWayOfOneTagOnBothSides(t *testing.T) {
	kits := []kit{
		{roles: as(hunter)},        // 0
		{roles: as(hunter, scout)}, // 1
		{roles: as(hunter, scout)}, // 2
		{roles: as(hunter)},        // 3
	}
	meet := between[glance]("meet", hunter, hunter)
	for _, c := range []struct {
		name  string
		r     act.Rule
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
		{"within scouts", act.Within(scout, meet), []struct {
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

func TestRule_StringNamesItAndItsMoment(t *testing.T) {
	for _, c := range []struct {
		r    act.Rule
		want string
	}{
		{noteOf[poke]("drown"), `"drown" of act_test.poke`},
		{act.WithOther(hunted, aimOf[glance]("glance")), `"glance" of act_test.glance`},
		{noteOf[dawn]("wake"), `"wake" of act_test.dawn`},
		{act.Within(scout, noteOf[poke]("drown")), `"drown" of act_test.poke, within`},
		{between[glance]("glance", hunter, hunted), `"glance" of act_test.glance, within`},
		{act.Within(scout, between[glance]("glance", hunter, hunted)),
			`"glance" of act_test.glance, within, within`},
		{Role("string mortal").Obeys(noteOf[poke]("drown")).Rules()[0],
			`"drown" of act_test.poke, for the role string mortal`},
		{Role("string mortal"), `the role string mortal`},
	} {
		if got := c.r.String(); got != c.want {
			t.Errorf("String() = %s; want %s", got, c.want)
		}
	}
}

func TestWithin_RunWhereKeepsToWhatKeepLetsThrough(t *testing.T) {
	kits := []kit{{roles: as(hunter)}, {roles: as(hunter)}, {roles: as(hunted)}}
	for _, c := range []struct {
		name string
		r    act.Rule
	}{
		{"within", act.Within(hunter, noteOf[poke]("drown"))},
		{"within twice", act.Within(hunter, act.Within(hunter, noteOf[poke]("drown")))},
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
