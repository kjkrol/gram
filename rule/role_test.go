package rule_test

import (
	"fmt"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/uid"
)

var (
	mortalTag = rule.Role("mortal").Tag()
	hastyTag  = rule.Role("hasty").Tag()
)

type standing struct{ who uid.UID64 }

func (s standing) Who() uid.UID64 { return s.who }

type meeting struct{ who, whom uid.UID64 }

func (m meeting) Who() uid.UID64             { return m.who }
func (m meeting) Whom(each func(uid.UID64))  { each(m.whom) }
func (m meeting) Subject() (uid.UID64, bool) { return m.whom, true }

type heard struct {
	Rule string
	At   uid.UID64
}

func (heard) Command() {}

func (c *heard) Aim(who uid.UID64) { c.At = who }

type body struct{ N int }

type being struct {
	name  string
	bare  bool // no tag.Tags[rule.Roles] at all
	plays []*rule.Part
}

func (b being) roles() uint64 {
	var bits uint64
	for _, p := range b.plays {
		bits |= 1 << p.Tag()
	}
	return bits
}

func spawnBeing(si *goke.SysInit, b being) uid.UID64 {
	var bodies goke.Comp[body]
	columns := []goke.Addable{&bodies}
	var plays comp.Spawner
	if !b.bare {
		plays = rule.Plays(b.plays...).Spawner()
		columns = append(columns, plays.Columns()...)
	}
	f := si.NewFactory(columns...)
	f.Create(1)
	f.Next()
	id := f.Cursor.IDs[0]
	if plays != nil {
		plays.Write(&f.Cursor, 0, nil, id)
	}
	return id
}

func tellOf[P any](name string, when any) rule.Rule {
	return rule.Branch[P]{Name: name, When: when, On: rule.Self, Do: rule.Apply(heard{Rule: name})}
}

func listen(t *testing.T) (*control.Carrier, *control.Queue[heard]) {
	t.Helper()
	var carrier control.Carrier
	var q control.Queue[heard]
	if err := carrier.Carry(&q); err != nil {
		t.Fatal(err)
	}
	return &carrier, &q
}

func TestRole_Obeys_NarrowsEachRuleToItsPlayers(t *testing.T) {
	hasty := rule.Role("hasty")
	mortal := rule.Role("mortal").Obeys(
		tellOf[standing]("fall in", nil),
		tellOf[standing]("rush", rule.Playing(hasty)),
	)
	carrier, q := listen(t)
	h := &plugin.Rules[standing]{}
	for _, r := range mortal.Rules() {
		if err := h.Add(r); err != nil {
			t.Fatalf("Add %v: %v", r, err)
		}
	}
	beings := []being{
		{name: "bare", bare: true},
		{name: "playing none"},
		{name: "hasty", plays: []*rule.Part{hasty}},
		{name: "mortal", plays: []*rule.Part{mortal}},
		{name: "mortal and hasty", plays: []*rule.Part{mortal, hasty}},
	}
	names := map[uid.UID64]string{}
	roles := map[uid.UID64]uint64{}
	goke.New().Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		for _, b := range beings {
			id := spawnBeing(si, b)
			names[id], roles[id] = b.name, b.roles()
		}
		var bodies goke.Comp[body]
		qb := si.NewQueryBuilder(&bodies)
		h.Bind(qb)
		walk := qb.Build()
		tick := plugin.Tick{Dt: time.Millisecond, Commands: carrier,
			Roles: func(id uid.UID64) uint64 { return roles[id] }}
		for walk.All(); walk.Next(); {
			cur := walk.Cursor()
			h.Run(tick, cur, func(i int) standing { return standing{who: cur.IDs[i]} })
		}
	}})
	got := map[string][]string{}
	q.Drain(func(i control.Issued[heard]) {
		got[i.Command.Rule] = append(got[i.Command.Rule], names[i.Entity])
	})
	want := map[string][]string{
		"fall in": {"mortal", "mortal and hasty"},
		"rush":    {"mortal and hasty"},
	}
	for name, who := range want {
		slices.Sort(got[name])
		if !slices.Equal(got[name], who) {
			t.Errorf("%q fired for %v; want %v", name, got[name], who)
		}
	}
	if len(got) != len(want) {
		t.Errorf("rules fired: %v; want only %v", got, want)
	}
}

func TestRole_Obeys_NarrowsAPairRuleToItsPlayers(t *testing.T) {
	hasty := rule.Role("hasty")
	mortal := rule.Role("mortal").Obeys(
		tellOf[meeting]("flinch", nil),
		tellOf[meeting]("parry", rule.Paired(hasty)),
		tellOf[meeting]("rush", rule.Playing(hasty)),
	)
	beings := []being{
		{name: "bare", bare: true},
		{name: "hasty", plays: []*rule.Part{hasty}},
		{name: "mortal", plays: []*rule.Part{mortal}},
		{name: "both", plays: []*rule.Part{mortal, hasty}},
	}
	want := map[string][]string{
		"flinch": {"both→bare", "both→hasty", "both→mortal", "mortal→bare", "mortal→both", "mortal→hasty"},
		"parry":  {"both→hasty", "mortal→both", "mortal→hasty"},
		"rush":   {"both→bare", "both→hasty", "both→mortal"},
	}
	for _, eitherWay := range []bool{false, true} {
		t.Run(fmt.Sprintf("either way %v", eitherWay), func(t *testing.T) {
			carrier, q := listen(t)
			h := &plugin.PairRules[meeting]{}
			for _, r := range mortal.Rules() {
				if err := h.Add(r); err != nil {
					t.Fatalf("Add %v: %v", r, err)
				}
			}
			names := map[uid.UID64]string{}
			roles := map[uid.UID64]uint64{}
			var ids []uid.UID64
			marks := map[uid.UID64]plugin.Marks{}
			goke.New().Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
				for _, b := range beings {
					id := spawnBeing(si, b)
					names[id], roles[id] = b.name, b.roles()
					ids = append(ids, id)
				}
				var bodies goke.Comp[body]
				qb := si.NewQueryBuilder(&bodies)
				h.Bind(qb)
				walk := qb.Build()
				for walk.All(); walk.Next(); {
					for i, id := range walk.Cursor().IDs {
						marks[id] = h.InChunk(0, walk.Cursor(), i)
					}
				}
			}})
			tick := plugin.Tick{Dt: time.Millisecond, Commands: carrier,
				Roles: func(id uid.UID64) uint64 { return roles[id] }}
			for i, a := range ids {
				for j, b := range ids {
					switch {
					case eitherWay && i < j:
						h.DispatchEitherWay(tick, marks[a], marks[b], meeting{a, b}, meeting{b, a})
					case !eitherWay && i != j:
						h.Dispatch(tick, marks[a], marks[b], meeting{a, b})
					}
				}
			}
			got := map[string][]string{}
			q.Drain(func(i control.Issued[heard]) {
				got[i.Command.Rule] = append(got[i.Command.Rule], names[i.Entity]+"→"+names[i.Command.At])
			})
			for name, pairs := range want {
				if eitherWay && name == "flinch" { // mortal and both play it: once, mortal first
					pairs = slices.DeleteFunc(slices.Clone(pairs), func(p string) bool { return p == "both→mortal" })
				}
				slices.Sort(got[name])
				if !slices.Equal(got[name], pairs) {
					t.Errorf("%q fired for %v; want %v", name, got[name], pairs)
				}
			}
			if len(got) != len(want) {
				t.Errorf("rules fired: %v; want only %v", got, want)
			}
		})
	}
}

func TestRole_Obeys_ARuleOfAStepFiresWhileTheMomentsEntityPlaysTheRole(t *testing.T) {
	early, late := rule.Role("step early"), rule.Role("step late")
	toll := tellOf[standing]("toll", nil)
	carrier, q := listen(t)
	h := &plugin.StepRules[standing]{}
	both := late.Obeys(early.Obeys(toll).Rules()[0]).Rules()[0]
	for _, r := range []rule.Rule{early.Rules()[0], both} {
		if err := h.Add(r); err != nil {
			t.Fatalf("Add of %v = %v, want it taken", r, err)
		}
	}
	roles := map[uid.UID64]uint64{1: 1 << early.Tag(), 2: 1 << late.Tag(), 3: 1<<early.Tag() | 1<<late.Tag()}
	tick := plugin.Tick{Dt: time.Millisecond, Commands: carrier, Roles: func(id uid.UID64) uint64 { return roles[id] }}
	for _, id := range []uid.UID64{1, 2, 3, 4} {
		h.Run(tick, standing{who: id})
	}
	var got []uid.UID64
	q.Drain(func(i control.Issued[heard]) { got = append(got, i.Entity) })
	if !slices.Equal(got, []uid.UID64{1, 3, 3}) {
		t.Errorf("the roles' rules fired for %v; want [1 3 3]: early's for 1 and 3, the rule of both for 3 alone", got)
	}
	h.Run(plugin.Tick{Dt: time.Millisecond, Commands: carrier}, standing{who: 1})
	if !q.Empty() {
		t.Error("with nobody telling the roles the rules fired; want none")
	}
}

func TestRole_Rules_AreItsOwnNarrowedInOrder(t *testing.T) {
	a, b := tellOf[standing]("a", nil), tellOf[standing]("b", rule.Playing(rule.Role("hasty")))
	mortal := rule.Role("mortal").Obeys(a).Obeys(b)
	got := mortal.Rules()
	want := []string{`"a" of rule_test.standing, for the role mortal`, `"b" of rule_test.standing, for the role mortal`}
	if len(got) != 2 || got[0].String() != want[0] || got[1].String() != want[1] {
		t.Fatalf("mortal.Rules() = %v; want %v", got, want)
	}
	if got[0] == a || got[1] == b {
		t.Error("Rules hands back the rules handed to Obeys, not ones narrowed to the role")
	}
	got[0] = b
	if mortal.Rules()[0].String() != want[0] {
		t.Error("changing what Rules handed back changed the role")
	}
	if len(rule.Role("idle").Rules()) != 0 {
		t.Error("a role of no rules has some")
	}
}

func TestPlays_IsOneComponentCarryingEveryRole(t *testing.T) {
	mortal, hasty := rule.Role("mortal"), rule.Role("hasty")
	plays := rule.Plays(mortal, hasty)
	if got, want := comp.TypeOf(plays), reflect.TypeFor[tag.Tags[rule.Roles]](); got != want {
		t.Fatalf("Plays is a %v; want a %v", got, want)
	}
	want := tag.Tags[rule.Roles](0).With(mortalTag, hastyTag)
	if got := plays.Resolve(nil); got != want {
		t.Errorf("Plays(mortal, hasty) = %b; want %b", got, want)
	}
	if got := rule.Plays().Resolve(nil); got != 0 {
		t.Errorf("Plays() = %b; want none", got)
	}

	var spawned tag.Tags[rule.Roles]
	goke.New().Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		id := spawnBeing(si, being{plays: []*rule.Part{mortal, hasty}})
		var tags goke.Comp[tag.Tags[rule.Roles]]
		seek := si.NewQueryBuilder(&tags).Build()
		if !seek.Seek(id) {
			t.Fatal("the entity spawned with Plays carries no tag.Tags[rule.Roles]")
		}
		spawned = *tags.At(seek.Cursor())
	}})
	if spawned != want {
		t.Errorf("the entity spawned carries %b; want %b", spawned, want)
	}
}

func TestMoment_Playing_RunsForTheRolesPlayersAlone(t *testing.T) {
	lever, trapdoor := rule.Role("playing lever"), rule.Role("playing trapdoor")
	carrier, q := listen(t)
	h := &plugin.StepRules[standing]{}
	if err := h.Add(rule.Branch[standing]{
		Name: "pull",
		When: rule.Playing(lever),
		On:   rule.Self,
		Do:   rule.Apply(heard{Rule: "pull"}),
	}); err != nil {
		t.Fatal(err)
	}
	roles := map[uid.UID64]uint64{1: 1 << lever.Tag(), 2: 1 << trapdoor.Tag()}
	tick := plugin.Tick{Dt: time.Millisecond, Commands: carrier, Roles: func(id uid.UID64) uint64 { return roles[id] }}
	for _, id := range []uid.UID64{1, 2, 3} {
		h.Run(tick, standing{who: id})
	}
	var got []uid.UID64
	q.Drain(func(i control.Issued[heard]) { got = append(got, i.Entity) })
	if !slices.Equal(got, []uid.UID64{1}) {
		t.Errorf("Playing ran for %v; want [1], the lever alone", got)
	}
}
