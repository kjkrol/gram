package rule_test

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/clock"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/uid"
)

// The roles' tags, given explicitly and away from the first bits.
var (
	mortalTag = rule.Role("mortal").Tag()
	hastyTag  = rule.Role("hasty").Tag()
)

// standing is a made-up host's moment of one entity.
type standing struct{ who uid.UID64 }

func (s standing) Who() uid.UID64 { return s.who }

// meeting is a made-up host's moment of one entity meeting another, its Subject.
type meeting struct{ who, whom uid.UID64 }

func (m meeting) Who() uid.UID64             { return m.who }
func (m meeting) Whom(each func(uid.UID64))  { each(m.whom) }
func (m meeting) Subject() (uid.UID64, bool) { return m.whom, true }

// heard is the command every rule here gives: which rule, aimed at the moment's subject if any.
type heard struct {
	Rule string
	At   uid.UID64
}

func (c *heard) Aim(who uid.UID64) { c.At = who }

// body is what every entity here carries; fuel is what some carry besides.
type (
	body struct{ N int }
	fuel struct{ N int }
)

// being is an entity to spawn: the roles it plays through rule.Plays — none given and bare, no
// tag.Tags[rule.Roles] at all — and whether it carries fuel.
type being struct {
	name  string
	bare  bool
	plays []*rule.Part
	fuel  bool
}

// spawn makes b in si, a body and what b says.
func spawnBeing(si *goke.SysInit, b being) uid.UID64 {
	var bodies goke.Comp[body]
	var fuels goke.Comp[fuel]
	columns := []goke.Addable{&bodies}
	var plays comp.Spawner
	if !b.bare {
		plays = rule.Plays(b.plays...).Spawner()
		columns = append(columns, plays.Columns()...)
	}
	if b.fuel {
		columns = append(columns, &fuels)
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

// tellOf is a rule of P named name, for whom filter lets through: its entity gives a heard of name.
func tellOf[P any](name string, filter rule.Filter) rule.Rule {
	return rule.Then[P](name, filter, rule.Order(heard{Rule: name}))
}

// listen is a carrier of heard commands and the queue they land in.
func listen(t *testing.T) (*control.Carrier, *control.Queue[heard]) {
	t.Helper()
	var carrier control.Carrier
	var q control.Queue[heard]
	if err := carrier.Carry(&q); err != nil {
		t.Fatal(err)
	}
	return &carrier, &q
}

// Obeys narrows each rule of one entity to the role's players, whatever its own filter: an entity
// without tag.Tags[rule.Roles], one playing none, one playing another role are left out.
func TestRole_Obeys_NarrowsEachRuleToItsPlayers(t *testing.T) {
	hasty := rule.Role("hasty")
	mortal := rule.Role("mortal").Obeys(
		tellOf[standing]("fall in", rule.All),
		tellOf[standing]("burn", rule.Having[fuel]()),
		tellOf[standing]("rush", rule.Self(hastyTag)),
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
		{name: "bare with fuel", bare: true, fuel: true},
		{name: "playing none"},
		{name: "hasty", plays: []*rule.Part{hasty}},
		{name: "hasty with fuel", plays: []*rule.Part{hasty}, fuel: true},
		{name: "mortal", plays: []*rule.Part{mortal}},
		{name: "mortal with fuel", plays: []*rule.Part{mortal}, fuel: true},
		{name: "mortal and hasty", plays: []*rule.Part{mortal, hasty}},
	}
	names := map[uid.UID64]string{}
	goke.New().Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		for _, b := range beings {
			names[spawnBeing(si, b)] = b.name
		}
		var bodies goke.Comp[body]
		qb := si.NewQueryBuilder(&bodies)
		h.Bind(qb)
		walk := qb.Build()
		for walk.All(); walk.Next(); {
			cur := walk.Cursor()
			h.Run(plugin.Tick{Dt: time.Millisecond, Commands: carrier}, cur, func(i int) standing {
				return standing{who: cur.IDs[i]}
			})
		}
	}})
	got := map[string][]string{}
	q.Drain(func(i control.Issued[heard]) {
		got[i.Command.Rule] = append(got[i.Command.Rule], names[i.Entity])
	})
	want := map[string][]string{
		"fall in": {"mortal", "mortal and hasty", "mortal with fuel"},
		"burn":    {"mortal with fuel"},
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

// Obeys narrows a rule of a Met moment to the pairs whose entity plays the role, keeping the
// rule's own sides. Dispatched either way a rule of the same side on both runs once a pair, as the
// plain rule does: of two players, once.
func TestRole_Obeys_NarrowsAPairRuleToItsPlayers(t *testing.T) {
	hasty := rule.Role("hasty")
	mortal := rule.Role("mortal").Obeys(
		tellOf[meeting]("flinch", rule.All),
		tellOf[meeting]("parry", rule.Between(tag.Any, hastyTag)),
		tellOf[meeting]("rush", rule.Between(hastyTag, tag.Any)),
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
			var ids []uid.UID64
			marks := map[uid.UID64]plugin.Marks{}
			goke.New().Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
				for _, b := range beings {
					id := spawnBeing(si, b)
					names[id] = b.name
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
			tick := plugin.Tick{Dt: time.Millisecond, Commands: carrier}
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

// A rule of the world as a whole walks no entities: obeyed by a role, the step host refuses it.
func TestRole_Obeys_ARuleOfAStepIsRefusedByItsHost(t *testing.T) {
	toll := tellOf[clock.Moment]("toll", rule.All)
	var h plugin.StepRules[clock.Moment]
	if err := h.Add(toll); err != nil {
		t.Fatalf("the rule itself: Add = %v, want it taken", err)
	}
	mortal := rule.Role("mortal").Obeys(toll)
	if len(mortal.Rules()) != 1 {
		t.Fatalf("the role obeys %d rules, want 1", len(mortal.Rules()))
	}
	if err := h.Add(mortal.Rules()[0]); !errors.Is(err, plugin.ErrUnhosted) {
		t.Errorf("Add of the role's rule = %v, want plugin.ErrUnhosted", err)
	}
}

// A role's Rules are its own, narrowed to it, in the order it obeyed them; a role of no rules has
// none, and changing what Rules hands back changes nothing of the role.
func TestRole_Rules_AreItsOwnNarrowedInOrder(t *testing.T) {
	a, b := tellOf[standing]("a", rule.All), tellOf[standing]("b", rule.Having[fuel]())
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

// Plays is one tag.Tags[rule.Roles] carrying every role's bit and no other, spawned as one
// component; of no role it is the component empty.
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

// Playing runs its step while the entity plays the role, as the world's lookup tells: not for
// one playing another, nor for one the lookup knows nothing of.
func TestMoment_Playing_RunsForTheRolesPlayersAlone(t *testing.T) {
	lever, trapdoor := rule.Role("playing lever"), rule.Role("playing trapdoor")
	carrier, q := listen(t)
	h := &plugin.StepRules[standing]{}
	if err := h.Add(rule.Then[standing]("pull", rule.All, rule.Playing(lever, rule.Order(heard{Rule: "pull"})))); err != nil {
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
