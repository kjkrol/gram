package engine_test

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/internal/engine"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/uid"
)

// roles is the family of tags these tests name.
type roles struct{}

const (
	hunter tag.Tag[roles] = iota
	hunted
)

// token is a component every entity of a made-up host carries.
type token struct{ N int }

// poke is a made-up host's moment of one entity.
type poke struct{ self uid.UID64 }

func (p poke) Who() uid.UID64 { return p.self }

// glance is a made-up host's moment of a pair: one looking at another.
type glance struct{ self, other uid.UID64 }

func (g glance) Who() uid.UID64            { return g.self }
func (g glance) Whom(each func(uid.UID64)) { each(g.other) }

// dawn is a made-up moment of the world as a whole, about its own entity, as a clock.Moment is.
type dawn struct{ world uid.UID64 }

func (d dawn) Who() uid.UID64 { return d.world }

// noted is a command a rule's entity gives itself.
type noted struct{}

func noteOf[P any](name string, filter rule.Filter) rule.Rule {
	return rule.Then[P](name, filter, rule.Order(noted{}))
}

// narrowed is r for the players of a role alone, as a role's Obeys makes it.
func narrowed(r rule.Rule) rule.Rule { return rule.Role("hookon scout").Obeys(r).Rules()[0] }

// host is a made-up plugin hosting the rules its adder takes — a plugin.Rules, PairRules or
// StepRules — wrapping a refusal in its own name, as the built-in plugins' Hook does.
type host struct {
	name  string
	adds  interface{ Add(rule any) error }
	asked int
	took  []rule.Rule
}

func (h *host) Hook(rules ...rule.Rule) error {
	for _, r := range rules {
		h.asked++
		if err := h.adds.Add(r); err != nil {
			return fmt.Errorf("%w in %s", err, h.name)
		}
		h.took = append(h.took, r)
	}
	return nil
}

// broken is a made-up plugin whose Hook fails for another reason than the moment.
type broken struct{ asked int }

var errBroken = errors.New("broken host")

func (b *broken) Hook(...rule.Rule) error {
	b.asked++
	return errBroken
}

// otherHook has a Hook of another shape: no host of rules.
type otherHook struct{}

func (otherHook) Hook(...any) error { panic("no host of rules: never asked") }

// HookOn hooks each rule on the host of its moment, skipping whatever is no Host and whoever
// refuses the rule's moment, and never asks a host after the one that took it.
func TestHookOn_HooksEachRuleOnTheHostOfItsMoment(t *testing.T) {
	pokes := &host{name: "pokes", adds: &plugin.Rules[poke]{}}
	glances := &host{name: "glances", adds: &plugin.PairRules[glance]{}}
	dawns := &host{name: "dawns", adds: &plugin.StepRules[dawn]{}}
	among := []any{nil, "not a host", otherHook{}, glances, dawns, pokes}

	wake := noteOf[dawn]("wake", rule.All)
	spot := noteOf[poke]("spot", rule.Self(hunter))
	stare := noteOf[glance]("stare", rule.Between(hunter, hunted))
	drown := noteOf[poke]("drown", rule.All)
	scouting := narrowed(drown)
	scoutStare := narrowed(stare)

	if err := engine.HookOn(among, wake, spot, stare, drown, scouting, scoutStare); err != nil {
		t.Fatalf("HookOn = %v", err)
	}
	for _, c := range []struct {
		h     *host
		took  []rule.Rule
		asked int
	}{
		{glances, []rule.Rule{stare, scoutStare}, 6},
		{dawns, []rule.Rule{wake}, 4},
		{pokes, []rule.Rule{spot, drown, scouting}, 3},
	} {
		if !slices.Equal(c.h.took, c.took) {
			t.Errorf("%s took %v; want %v", c.h.name, c.h.took, c.took)
		}
		if c.h.asked != c.asked {
			t.Errorf("%s was asked %d times; want %d", c.h.name, c.h.asked, c.asked)
		}
	}
}

// Of two hosts of one moment the first has the rule; the second is never asked.
func TestHookOn_TheFirstHostToTakeTheRuleHasIt(t *testing.T) {
	first := &host{name: "first", adds: &plugin.Rules[poke]{}}
	second := &host{name: "second", adds: &plugin.Rules[poke]{}}
	drown := noteOf[poke]("drown", rule.All)

	if err := engine.HookOn([]any{first, second}, drown); err != nil {
		t.Fatalf("HookOn = %v", err)
	}
	if !slices.Equal(first.took, []rule.Rule{drown}) || second.asked != 0 {
		t.Errorf("first took %v, second was asked %d times; want first to take it, second never asked", first.took, second.asked)
	}
}

// Any error but plugin.ErrUnhosted stops HookOn at once: no later host is asked, no later rule
// tried.
func TestHookOn_StopsAtAnotherError(t *testing.T) {
	b := &broken{}
	pokes := &host{name: "pokes", adds: &plugin.Rules[poke]{}}
	drown, spot := noteOf[poke]("drown", rule.All), noteOf[poke]("spot", rule.Self(hunter))

	err := engine.HookOn([]any{b, pokes}, drown, spot)
	if !errors.Is(err, errBroken) || errors.Is(err, plugin.ErrUnhosted) {
		t.Errorf("HookOn = %v; want the broken host's error, not plugin.ErrUnhosted", err)
	}
	if b.asked != 1 || pokes.asked != 0 {
		t.Errorf("the broken host was asked %d times, the next %d; want once and never", b.asked, pokes.asked)
	}
}

// A host whose system is built refuses with plugin.ErrHostBuilt: HookOn reports it and asks no
// later host, though one would take the rule.
func TestHookOn_StopsAtAHostAlreadyBuilt(t *testing.T) {
	bound := &plugin.Rules[poke]{}
	goke.New().Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		var tokens goke.Comp[token]
		bound.Bind(si.NewQueryBuilder(&tokens))
	}})
	built := &host{name: "built", adds: bound}
	later := &host{name: "later", adds: &plugin.Rules[poke]{}}

	err := engine.HookOn([]any{built, later}, noteOf[poke]("drown", rule.All))
	if !errors.Is(err, plugin.ErrHostBuilt) {
		t.Errorf("HookOn = %v; want plugin.ErrHostBuilt", err)
	}
	if later.asked != 0 {
		t.Errorf("the later host was asked %d times; want never", later.asked)
	}
}

// A rule no host takes is plugin.ErrUnhosted naming the rule — a narrowed rule of the world as a
// whole among them — and so is any rule among no hosts at all.
func TestHookOn_RefusesARuleNoHostTakes(t *testing.T) {
	pokes := &host{name: "pokes", adds: &plugin.Rules[poke]{}}
	dawns := &host{name: "dawns", adds: &plugin.StepRules[dawn]{}}
	stare := noteOf[glance]("stare", rule.Between(hunter, hunted))
	narrowedWake := narrowed(noteOf[dawn]("wake", rule.All))

	for _, c := range []struct {
		name  string
		among []any
		r     rule.Rule
	}{
		{"a moment nobody hosts", []any{pokes, "not a host", dawns}, stare},
		{"a narrowed rule of the world", []any{pokes, dawns}, narrowedWake},
		{"no hosts", nil, stare},
		{"nothing that is a host", []any{"not a host", otherHook{}}, stare},
	} {
		t.Run(c.name, func(t *testing.T) {
			err := engine.HookOn(c.among, c.r)
			if !errors.Is(err, plugin.ErrUnhosted) {
				t.Fatalf("HookOn = %v; want plugin.ErrUnhosted", err)
			}
			if !strings.Contains(err.Error(), c.r.String()) {
				t.Errorf("HookOn = %q; want it naming the rule %s", err, c.r)
			}
		})
	}
	if len(pokes.took) != 0 || len(dawns.took) != 0 {
		t.Errorf("pokes took %v, dawns %v; want nothing taken", pokes.took, dawns.took)
	}
}

// No rules is nothing to hook.
func TestHookOn_NoRulesIsNothing(t *testing.T) {
	if err := engine.HookOn(nil); err != nil {
		t.Errorf("HookOn(nil) = %v; want nil", err)
	}
}
