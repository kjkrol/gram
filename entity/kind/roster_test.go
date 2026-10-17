package kind_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/gram/entity/kind"
)

type armour struct{ Plate int }

func types(spec kind.Spec) []string {
	var out []string
	for _, c := range spec {
		out = append(out, fmt.Sprintf("%T", c))
	}
	return out
}

var (
	thing   = kind.NewTemplate("thing", nil)
	soldier = kind.NewTemplate("soldier", thing)
)

// a roster asks writes as it is first read
func rosterOf(asks func(r *kind.Roster)) *kind.Roster { return kind.NewRoster(asks) }

func TestRoster_SpecBringsTheDefaultsAndTheGamesOwn(t *testing.T) {
	r := rosterOf(func(r *kind.Roster) { r.Default(soldier, comp.Const(stat{HP: 3})) })

	spec := r.Spec(soldier, comp.Load(func(r row) armour { return armour{Plate: r.hp} }))

	if len(spec) != 2 {
		t.Fatalf("spec has %d components, want the default stat and the game's armour: %v", len(spec), types(spec))
	}
	if got := spec[0].(comp.Template[stat]).Resolve(nil); got.HP != 3 {
		t.Errorf("the default resolved to %+v, want HP 3", got)
	}
}

func TestRoster_TheGamesOwnReplacesADefaultOfTheSameType(t *testing.T) {
	r := rosterOf(func(r *kind.Roster) { r.Default(soldier, comp.Const(stat{HP: 3})) })

	spec := r.Spec(soldier, comp.Const(stat{HP: 9}))

	if len(spec) != 1 {
		t.Fatalf("spec has %d components, want the game's stat alone: %v", len(spec), types(spec))
	}
	if got := spec[0].(comp.Template[stat]).Resolve(nil); got.HP != 9 {
		t.Errorf("stat resolved to %+v, want the game's 9", got)
	}
}

func TestRoster_WithoutDropsADefaultAndLeavesNoTrace(t *testing.T) {
	r := rosterOf(func(r *kind.Roster) {
		r.Default(soldier, comp.Const(stat{HP: 3}))
		r.Default(soldier, comp.Const(armour{Plate: 1}))
	})

	spec := r.Spec(soldier, comp.Without[stat]())

	if len(spec) != 1 {
		t.Fatalf("spec has %d components, want the armour alone: %v", len(spec), types(spec))
	}
	if _, isArmour := spec[0].(comp.Template[armour]); !isArmour {
		t.Errorf("spec keeps %v, want the armour", types(spec))
	}
}

func TestRoster_ARequirementIsMetByAConstOrALoad(t *testing.T) {
	for name, own := range map[string]comp.Comp{
		"const": comp.Const(stat{}),
		"load":  comp.Load(func(r row) stat { return stat{HP: r.hp} }),
	} {
		t.Run(name, func(t *testing.T) {
			r := rosterOf(func(r *kind.Roster) {
				kind.Require[stat](r, soldier, "board", "its health")
			})
			if spec := r.Spec(soldier, own); len(spec) != 1 {
				t.Errorf("spec has %d components, want the one given", len(spec))
			}
		})
	}
}

func TestRoster_SpecPanicsNamingEveryUnmetRequirement(t *testing.T) {
	r := rosterOf(func(r *kind.Roster) {
		kind.Require[stat](r, soldier, "board", "its health")
		kind.Require[armour](r, soldier, "collision", "what it is clad in")
	})
	defer func() {
		msg, _ := recover().(string)
		for _, want := range []string{
			"kind: soldier:",
			"board requires kind_test.stat (its health)",
			"collision requires kind_test.armour (what it is clad in)",
		} {
			if !strings.Contains(msg, want) {
				t.Errorf("panic %q does not mention %q", msg, want)
			}
		}
	}()

	r.Spec(soldier)
	t.Error("Spec accepted a soldier missing both requirements")
}

func TestRoster_ATemplateTakesItsParentsAsksItsOwnDefaultsFirst(t *testing.T) {
	r := rosterOf(func(r *kind.Roster) {
		r.Default(thing, comp.Const(stat{HP: 1}))
		r.Default(thing, comp.Const(armour{Plate: 1}))
		kind.Require[row](r, thing, "world", "where it stands")
		r.Default(soldier, comp.Const(stat{HP: 5}))
	})

	spec := r.Spec(soldier, comp.Const(row{}))

	got := map[string]bool{}
	for _, c := range spec {
		got[fmt.Sprintf("%T", c)] = true
		if s, ok := c.(comp.Template[stat]); ok && s.Resolve(nil).HP != 5 {
			t.Errorf("a soldier's stat is %+v, want its own 5 over the thing's 1", s.Resolve(nil))
		}
	}
	if len(spec) != 3 || !got[fmt.Sprintf("%T", comp.Const(armour{}))] {
		t.Errorf("a soldier's spec is %v, want its stat, the thing's armour and its row",
			types(spec))
	}
	if spec := r.Spec(thing, comp.Const(row{})); len(spec) != 3 {
		t.Errorf("a thing's spec is %v, want none of a soldier's", types(spec))
	}
}

func TestRoster_IsWrittenOnlyAsItIsFirstRead(t *testing.T) {
	r := rosterOf(func(*kind.Roster) {})
	wrote := func() (msg string) {
		defer func() { msg, _ = recover().(string) }()
		r.Default(soldier, comp.Const(stat{}))
		return ""
	}
	if msg := wrote(); !strings.Contains(msg, "outside a plugin's KindTemplates") {
		t.Errorf("a default written before the roster was read panics %q, want it refused", msg)
	}
	r.Spec(soldier)
	if msg := wrote(); !strings.Contains(msg, "outside a plugin's KindTemplates") {
		t.Errorf("a default written after the roster was read panics %q, want it refused", msg)
	}
	reading := rosterOf(func(r *kind.Roster) { r.Spec(thing) })
	defer func() {
		if msg, _ := recover().(string); !strings.Contains(msg, "read while") {
			t.Errorf("a roster read while written panics %q, want it refused", msg)
		}
	}()
	reading.Spec(thing)
}
