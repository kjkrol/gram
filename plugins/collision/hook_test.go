package collision_test

import (
	"errors"
	"testing"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/uid"
)

// roles is the tag family of these tests: bullets and targets.
type roles struct{}

const (
	bullet tag.Tag[roles] = iota
	target
)

// tagged is one box of a rule fixture: where it is and which tags it carries.
type tagged struct {
	x              float64
	bullet, target bool
	id             uid.UID64
}

// registrar is the part of the collision engine a fixture hooks rules on.
type registrar interface {
	Hook(rules ...rule.Rule) error
}

// heard is the command the tests' rules give: which rule, given by the one met when Other.
type heard struct {
	Rule  string
	Other bool
}

// heards is where the heard commands land, for a world to carry.
type heards struct{ control.Queue[heard] }

func (h *heards) Queues() []control.CommandQueue     { return []control.CommandQueue{&h.Queue} }
func (h *heards) DefaultBindings() []control.Binding { return nil }

// given is every heard given since the last time.
func (h *heards) given() []control.Issued[heard] {
	var got []control.Issued[heard]
	h.Drain(func(i control.Issued[heard]) { got = append(got, i) })
	return got
}

// met is one Meeting a rule fired for: which rule, Self and Other.
type met struct {
	rule        string
	self, other uid.UID64
}

// meet runs the real collision engine for one tick over boxes, rules hooked, and returns every
// Meeting the rules fired for.
func meet(t *testing.T, rules []rule.Rule, boxes ...*tagged) []met {
	t.Helper()
	return meetWith(t, func(engine registrar) {
		if err := engine.Hook(rules...); err != nil {
			t.Fatalf("Hook: %v", err)
		}
	}, boxes...)
}

// meetWith is meet with the registering left to the caller.
func meetWith(t *testing.T, register func(engine registrar), boxes ...*tagged) []met {
	t.Helper()
	var commands control.Carrier
	var orders heards
	if err := commands.Carry(&orders); err != nil {
		t.Fatal(err)
	}
	space := testSpace(t)
	ecs := goke.New()
	engine := collision.New(space, ecs)
	engine.Carry(&commands)
	register(engine)

	ecs.Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		var base goke.Comp[world.Base]
		var coll goke.Comp[collision.Collider]
		var tags goke.Comp[tag.Tags[roles]]
		for _, box := range boxes {
			comps := []goke.Addable{&base, &coll}
			if box.bullet || box.target {
				comps = append(comps, &tags)
			}
			f := si.NewFactory(comps...)
			f.Create(1)
			f.Next()
			box.id = f.IDs[0]
			placed := posAt(box.x, 100, 10, 10)
			base.Slice(&f.Cursor)[0].Pos = placed
			if box.bullet || box.target {
				var marks tag.Tags[roles]
				if box.bullet {
					marks = marks.With(bullet)
				}
				if box.target {
					marks = marks.With(target)
				}
				tags.Slice(&f.Cursor)[0] = marks
			}
		}
	}})
	engine.RegSystems(ecs)
	ecs.SetPlan(engine.RunPlan)
	ecs.Tick(time.Millisecond)
	var got []met
	for _, h := range orders.given() {
		if h.Command.Other {
			got[len(got)-1].other = h.Entity
			continue
		}
		got = append(got, met{rule: h.Command.Rule, self: h.Entity})
	}
	return got
}

// heardOf is a rule named name of a Meeting between a and b: Self gives a heard of it, then
// Other one more.
func heardOf[F any](name string, a tag.Tag[roles], b tag.Tag[F]) rule.Rule {
	return rule.On(name, rule.Between(a, b), func(m *rule.Moment[collision.Meeting]) rule.Step {
		return m.Steps(m.Order(heard{Rule: name}), m.ForOther(m.Order(heard{Rule: name, Other: true})))
	})
}

func bulletsAgainstTargets() []rule.Rule { return []rule.Rule{heardOf("bullets", bullet, target)} }

func TestBetween_HandsOverThePairWithSelfOnTheFirstTag(t *testing.T) {
	for name, order := range map[string][2]bool{"bullet spawned first": {true, false}, "target spawned first": {false, true}} {
		t.Run(name, func(t *testing.T) {
			first := &tagged{x: 100, bullet: order[0], target: !order[0]}
			second := &tagged{x: 105, bullet: order[1], target: !order[1]}

			got := meet(t, bulletsAgainstTargets(), first, second)

			shot, struck := first, second
			if !first.bullet {
				shot, struck = second, first
			}
			if len(got) != 1 {
				t.Fatalf("the rule fired %d times, want once", len(got))
			}
			if got[0].self != shot.id || got[0].other != struck.id {
				t.Errorf("fired for %v about %v, want for the bullet %v about the target %v", got[0].self, got[0].other, shot.id, struck.id)
			}
		})
	}
}

func TestBetween_IgnoresPairsThatDoNotCarryBothTags(t *testing.T) {
	got := meet(t, bulletsAgainstTargets(),
		&tagged{x: 100, bullet: true}, &tagged{x: 105, bullet: true},
		&tagged{x: 300}, &tagged{x: 305, target: true},
	)

	if len(got) != 0 {
		t.Errorf("the rule fired %+v, want it left alone — no bullet met a target", got)
	}
}

// A pair of the same tag would match either way round, and is still one contact.
func TestBetween_SameTagOnBothSides_RunsOncePerContact(t *testing.T) {
	got := meet(t, []rule.Rule{heardOf("bullets", bullet, bullet)}, &tagged{x: 100, bullet: true}, &tagged{x: 105, bullet: true})

	if len(got) != 1 {
		t.Errorf("the rule fired %d times, want once for one contact", len(got))
	}
}

// Anything stands for whatever is on the other side — tagged or not.
func TestBetween_Anything_MatchesWhateverIsThere(t *testing.T) {
	shot, wall := &tagged{x: 100, bullet: true}, &tagged{x: 105}

	got := meet(t, []rule.Rule{heardOf("anything", bullet, tag.Any)}, shot, wall)

	if len(got) != 1 || got[0].self != shot.id || got[0].other != wall.id {
		t.Errorf("fired %+v, want for the bullet %v about the untagged %v once", got, shot.id, wall.id)
	}
}

func TestBetween_RulesSharingATag_BothRun(t *testing.T) {
	got := meet(t, []rule.Rule{heardOf("first", bullet, target), heardOf("second", bullet, tag.Any)},
		&tagged{x: 100, bullet: true}, &tagged{x: 105, target: true})

	if len(got) != 2 || got[0].rule != "first" || got[1].rule != "second" {
		t.Errorf("fired %+v, want the first rule, then the second, once each", got)
	}
}

// elsewhere is a moment of a host other than collision's.
type elsewhere struct{}

func (elsewhere) Who() uid.UID64       { return 0 }
func (elsewhere) Whom(func(uid.UID64)) {}

// fromElsewhere is a rule of elsewhere: of a pair when paired, else of anyone.
func fromElsewhere(paired bool) rule.Rule {
	filter := rule.All
	if paired {
		filter = rule.Between(bullet, target)
	}
	return rule.On("elsewhere", filter, func(m *rule.Moment[elsewhere]) rule.Step { return m.Order(heard{}) })
}

func TestHook_RefusesWhatItCannotHost(t *testing.T) {
	engine := collision.New(testSpace(t), goke.New())

	for name, b := range map[string]rule.Rule{
		"a pair made for another host":   fromElsewhere(true),
		"an entity made for another one": fromElsewhere(false),
	} {
		if err := engine.Hook(b); !errors.Is(err, plugin.ErrUnhosted) {
			t.Errorf("%s: Hook = %v, want ErrUnhosted", name, err)
		}
	}
}

func TestHook_RefusesOneThatComesTooLate(t *testing.T) {
	ecs := goke.New()
	engine := collision.New(testSpace(t), ecs)
	ecs.Setup()
	engine.RegSystems(ecs)

	if err := engine.Hook(bulletsAgainstTargets()...); !errors.Is(err, plugin.ErrHostBuilt) {
		t.Errorf("Hook after the systems were built = %v, want ErrHostBuilt", err)
	}
}

func TestHook_StopsAtTheFirstItCannotHost(t *testing.T) {
	var refused error

	got := meetWith(t, func(engine registrar) {
		refused = engine.Hook(heardOf("before", bullet, target), fromElsewhere(true), heardOf("after", bullet, target))
	}, &tagged{x: 100, bullet: true}, &tagged{x: 105, target: true})

	if !errors.Is(refused, plugin.ErrUnhosted) {
		t.Errorf("Hook = %v, want ErrUnhosted", refused)
	}
	if len(got) != 1 || got[0].rule != "before" {
		t.Errorf("fired %+v, want the rule before the refused one alone", got)
	}
}
