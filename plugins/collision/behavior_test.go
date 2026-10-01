package collision_test

import (
	"errors"
	"testing"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugin/host"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/entity/tag"
	"github.com/kjkrol/uid"
)

// roles is the tag family of these tests: bullets and targets.
type roles struct{}

const (
	bullet tag.Tag[roles] = iota
	target
)

// tagged is one box of a behavior fixture: where it is and which tags it carries.
type tagged struct {
	x              float64
	bullet, target bool
	id             uid.UID64
}

// registrar is the part of the collision engine a fixture registers behaviors on.
type registrar interface {
	Hook(behaviors ...plugin.Rule) error
}

// meet runs the real collision engine for one tick over boxes and returns every Meeting handed out.
func meet(t *testing.T, behaviorsOf func(record func(collision.Meeting)) []plugin.Rule, boxes ...*tagged) []collision.Meeting {
	t.Helper()
	var met []collision.Meeting
	meetWith(t, func(engine registrar) {
		if err := engine.Hook(behaviorsOf(func(m collision.Meeting) { met = append(met, m) })...); err != nil {
			t.Fatalf("Hook: %v", err)
		}
	}, boxes...)
	return met
}

// meetWith is meet with the registering left to the caller.
func meetWith(t *testing.T, register func(engine registrar), boxes ...*tagged) {
	t.Helper()
	space := testSpace(t)
	ecs := goke.New()
	engine := collision.New(space, ecs)
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
}

func bulletsAgainstTargets(record func(collision.Meeting)) []plugin.Rule {
	return []plugin.Rule{host.Pair(bullet, target, func(_ plugin.Tick, m collision.Meeting) { record(m) })}
}

func TestBetween_HandsOverThePairWithSelfOnTheFirstTag(t *testing.T) {
	for name, order := range map[string][2]bool{"bullet spawned first": {true, false}, "target spawned first": {false, true}} {
		t.Run(name, func(t *testing.T) {
			first := &tagged{x: 100, bullet: order[0], target: !order[0]}
			second := &tagged{x: 105, bullet: order[1], target: !order[1]}

			met := meet(t, bulletsAgainstTargets, first, second)

			shot, struck := first, second
			if !first.bullet {
				shot, struck = second, first
			}
			if len(met) != 1 {
				t.Fatalf("behavior ran %d times, want once", len(met))
			}
			if met[0].Self != shot.id || met[0].Other != struck.id {
				t.Errorf("Meeting = (self %v, other %v), want (bullet %v, target %v)", met[0].Self, met[0].Other, shot.id, struck.id)
			}
		})
	}
}

func TestBetween_IgnoresPairsThatDoNotCarryBothTags(t *testing.T) {
	met := meet(t, bulletsAgainstTargets,
		&tagged{x: 100, bullet: true}, &tagged{x: 105, bullet: true},
		&tagged{x: 300}, &tagged{x: 305, target: true},
	)

	if len(met) != 0 {
		t.Errorf("behavior ran for %+v, want it left alone — no bullet met a target", met)
	}
}

// A pair of the same tag would match either way round, and is still one contact.
func TestBetween_SameTagOnBothSides_RunsOncePerContact(t *testing.T) {
	met := meet(t, func(record func(collision.Meeting)) []plugin.Rule {
		return []plugin.Rule{host.Pair(bullet, bullet, func(_ plugin.Tick, m collision.Meeting) { record(m) })}
	}, &tagged{x: 100, bullet: true}, &tagged{x: 105, bullet: true})

	if len(met) != 1 {
		t.Errorf("behavior ran %d times, want once for one contact", len(met))
	}
}

// Anything stands for whatever is on the other side — tagged or not.
func TestBetween_Anything_MatchesWhateverIsThere(t *testing.T) {
	shot, wall := &tagged{x: 100, bullet: true}, &tagged{x: 105}

	met := meet(t, func(record func(collision.Meeting)) []plugin.Rule {
		return []plugin.Rule{host.Pair(bullet, tag.Any, func(_ plugin.Tick, m collision.Meeting) { record(m) })}
	}, shot, wall)

	if len(met) != 1 || met[0].Self != shot.id || met[0].Other != wall.id {
		t.Errorf("Meetings = %+v, want the bullet %v meeting the untagged %v once", met, shot.id, wall.id)
	}
}

func TestBetween_BehaviorsSharingATag_BothRun(t *testing.T) {
	var first, second int
	met := meet(t, func(func(collision.Meeting)) []plugin.Rule {
		return []plugin.Rule{
			host.Pair(bullet, target, func(plugin.Tick, collision.Meeting) { first++ }),
			host.Pair(bullet, tag.Any, func(plugin.Tick, collision.Meeting) { second++ }),
		}
	}, &tagged{x: 100, bullet: true}, &tagged{x: 105, target: true})

	if first != 1 || second != 1 || len(met) != 0 {
		t.Errorf("behaviors ran (%d, %d) times, want (1, 1)", first, second)
	}
}

func TestHook_RefusesWhatItCannotHost(t *testing.T) {
	engine := collision.New(testSpace(t), goke.New())

	for name, b := range map[string]plugin.Rule{
		"not a behavior at all":          "just a string",
		"a pair made for another host":   host.Pair(bullet, target, func(plugin.Tick, string) {}),
		"an entity made for another one": host.Each(func(plugin.Tick, *tagged, string) {}),
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

	if err := engine.Hook(bulletsAgainstTargets(func(collision.Meeting) {})[0]); !errors.Is(err, plugin.ErrHostBuilt) {
		t.Errorf("Hook after the systems were built = %v, want ErrHostBuilt", err)
	}
}

func TestHook_StopsAtTheFirstItCannotHost(t *testing.T) {
	var before, after int
	var refused error

	meetWith(t, func(engine registrar) {
		refused = engine.Hook(
			host.Pair(bullet, target, func(plugin.Tick, collision.Meeting) { before++ }),
			"not a behavior at all",
			host.Pair(bullet, target, func(plugin.Tick, collision.Meeting) { after++ }),
		)
	}, &tagged{x: 100, bullet: true}, &tagged{x: 105, target: true})

	if !errors.Is(refused, plugin.ErrUnhosted) {
		t.Errorf("Hook = %v, want ErrUnhosted", refused)
	}
	if before != 1 || after != 0 {
		t.Errorf("behaviors ran (before %d, after %d), want (1, 0)", before, after)
	}
}
