package collision_test

import (
	"errors"
	"testing"
	"time"

	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/act"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/ceremony/cond"
	"github.com/kjkrol/ceremony/host"
	"github.com/kjkrol/ceremony/tag"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

var (
	bullet = ceremony.NewPart("bullet", 0)
	target = ceremony.NewPart("target", 1)
)

type tagged struct {
	x              float32
	bullet, target bool
	id             uid.UID64
}

type registrar interface {
	Hook(rules ...act.Rule) error
}

// Other: given by the one met
type heard struct {
	Rule  string
	Other bool
}

func (heard) Command() {}

type heards struct{ command.Queue[heard] }

func (h *heards) Queues() []command.CommandQueue     { return []command.CommandQueue{&h.Queue} }
func (h *heards) DefaultBindings() []control.Binding { return nil }

func (h *heards) given() []command.Issued[heard] {
	var got []command.Issued[heard]
	h.Drain(func(i command.Issued[heard]) { got = append(got, i) })
	return got
}

type met struct {
	rule        string
	self, other uid.UID64
}

func meet(t *testing.T, rules []act.Rule, boxes ...*tagged) []met {
	t.Helper()
	return meetWith(t, func(engine registrar) {
		if err := engine.Hook(rules...); err != nil {
			t.Fatalf("Hook: %v", err)
		}
	}, boxes...)
}

func meetWith(t *testing.T, register func(engine registrar), boxes ...*tagged) []met {
	t.Helper()
	var commands command.Carrier
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
		var tags goke.Comp[tag.Tags[cond.Roles]]
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
				var marks tag.Tags[cond.Roles]
				if box.bullet {
					marks = marks.With(bullet.Tag())
				}
				if box.target {
					marks = marks.With(target.Tag())
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

// b nil: anybody
func heardOf(name string, a, b *ceremony.Part) act.Rule {
	when := cond.Self.If(a)
	if b != nil {
		when = when.And(cond.Other.If(b))
	}
	return act.Branch[collision.Meeting]{
		Name: name,
		When: when,
		Then: act.Steps(
			act.Self.Do(heard{Rule: name}),
			act.Branch[any]{When: cond.Self.If(cond.Always), Then: act.Other.Do(heard{Rule: name, Other: true})},
		),
	}
}

func bulletsAgainstTargets() []act.Rule { return []act.Rule{heardOf("bullets", bullet, target)} }

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

func TestBetween_SameTagOnBothSides_RunsOncePerContact(t *testing.T) {
	got := meet(t, []act.Rule{heardOf("bullets", bullet, bullet)}, &tagged{x: 100, bullet: true}, &tagged{x: 105, bullet: true})

	if len(got) != 1 {
		t.Errorf("the rule fired %d times, want once for one contact", len(got))
	}
}

func TestBetween_Anything_MatchesWhateverIsThere(t *testing.T) {
	shot, wall := &tagged{x: 100, bullet: true}, &tagged{x: 105}

	got := meet(t, []act.Rule{heardOf("anything", bullet, nil)}, shot, wall)

	if len(got) != 1 || got[0].self != shot.id || got[0].other != wall.id {
		t.Errorf("fired %+v, want for the bullet %v about the untagged %v once", got, shot.id, wall.id)
	}
}

func TestBetween_RulesSharingATag_BothRun(t *testing.T) {
	got := meet(t, []act.Rule{heardOf("first", bullet, target), heardOf("second", bullet, nil)},
		&tagged{x: 100, bullet: true}, &tagged{x: 105, target: true})

	if len(got) != 2 || got[0].rule != "first" || got[1].rule != "second" {
		t.Errorf("fired %+v, want the first rule, then the second, once each", got)
	}
}

type elsewhere struct{}

func (elsewhere) Who() uid.UID64       { return 0 }
func (elsewhere) Whom(func(uid.UID64)) {}

func fromElsewhere(paired bool) act.Rule {
	when := cond.Self.If(cond.Always)
	if paired {
		when = cond.Self.If(bullet).And(cond.Other.If(target))
	}
	return act.Branch[elsewhere]{
		Name: "elsewhere",
		When: when,
		Then: act.Self.Do(heard{}),
	}
}

func TestHook_RefusesWhatItCannotHost(t *testing.T) {
	engine := collision.New(testSpace(t), goke.New())

	for name, b := range map[string]act.Rule{
		"a pair made for another host":   fromElsewhere(true),
		"an entity made for another one": fromElsewhere(false),
	} {
		if err := engine.Hook(b); !errors.Is(err, host.ErrUnhosted) {
			t.Errorf("%s: Hook = %v, want ErrUnhosted", name, err)
		}
	}
}

func TestHook_RefusesOneThatComesTooLate(t *testing.T) {
	ecs := goke.New()
	engine := collision.New(testSpace(t), ecs)
	ecs.Setup()
	engine.RegSystems(ecs)

	if err := engine.Hook(bulletsAgainstTargets()...); !errors.Is(err, host.ErrHostBuilt) {
		t.Errorf("Hook after the systems were built = %v, want ErrHostBuilt", err)
	}
}

func TestHook_StopsAtTheFirstItCannotHost(t *testing.T) {
	var refused error

	got := meetWith(t, func(engine registrar) {
		refused = engine.Hook(heardOf("before", bullet, target), fromElsewhere(true), heardOf("after", bullet, target))
	}, &tagged{x: 100, bullet: true}, &tagged{x: 105, target: true})

	if !errors.Is(refused, host.ErrUnhosted) {
		t.Errorf("Hook = %v, want ErrUnhosted", refused)
	}
	if len(got) != 1 || got[0].rule != "before" {
		t.Errorf("fired %+v, want the rule before the refused one alone", got)
	}
}
