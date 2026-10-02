package kind_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/entity/kind/comp"
)

type armour struct{ Plate int }

// types lists the component types of a spec, in order.
func types(spec kind.Spec) []string {
	var out []string
	for _, c := range spec {
		out = append(out, fmt.Sprintf("%T", c))
	}
	return out
}

func TestTemplate_SpecBringsTheDefaultsAndTheGamesOwn(t *testing.T) {
	r := kind.NewRoster()
	r.Unit.Default(comp.Const(stat{HP: 3}))

	spec := r.Unit.Spec(comp.Load(func(r row) armour { return armour{Plate: r.hp} }))

	if len(spec) != 2 {
		t.Fatalf("spec has %d components, want the default stat and the game's armour: %v", len(spec), types(spec))
	}
	if got := spec[0].(comp.Template[stat]).Resolve(nil); got.HP != 3 {
		t.Errorf("the default resolved to %+v, want HP 3", got)
	}
}

func TestTemplate_TheGamesOwnReplacesADefaultOfTheSameType(t *testing.T) {
	r := kind.NewRoster()
	r.Unit.Default(comp.Const(stat{HP: 3}))

	spec := r.Unit.Spec(comp.Const(stat{HP: 9}))

	if len(spec) != 1 {
		t.Fatalf("spec has %d components, want the game's stat alone: %v", len(spec), types(spec))
	}
	if got := spec[0].(comp.Template[stat]).Resolve(nil); got.HP != 9 {
		t.Errorf("stat resolved to %+v, want the game's 9", got)
	}
}

func TestTemplate_WithoutDropsADefaultAndLeavesNoTrace(t *testing.T) {
	r := kind.NewRoster()
	r.Unit.Default(comp.Const(stat{HP: 3}))
	r.Unit.Default(comp.Const(armour{Plate: 1}))

	spec := r.Unit.Spec(comp.Without[stat]())

	if len(spec) != 1 {
		t.Fatalf("spec has %d components, want the armour alone: %v", len(spec), types(spec))
	}
	if _, isArmour := spec[0].(comp.Template[armour]); !isArmour {
		t.Errorf("spec keeps %v, want the armour", types(spec))
	}
}

func TestTemplate_ARequirementIsMetByAConstOrALoad(t *testing.T) {
	for name, own := range map[string]comp.Comp{
		"const": comp.Const(stat{}),
		"load":  comp.Load(func(r row) stat { return stat{HP: r.hp} }),
	} {
		t.Run(name, func(t *testing.T) {
			r := kind.NewRoster()
			kind.Require[stat](&r.Unit, "board", "its health")
			if spec := r.Unit.Spec(own); len(spec) != 1 {
				t.Errorf("spec has %d components, want the one given", len(spec))
			}
		})
	}
}

func TestTemplate_SpecPanicsNamingEveryUnmetRequirement(t *testing.T) {
	r := kind.NewRoster()
	kind.Require[stat](&r.Unit, "board", "its health")
	kind.Require[armour](&r.Unit, "collision", "what it is clad in")
	defer func() {
		msg, _ := recover().(string)
		for _, want := range []string{"kind: unit:", "board requires kind_test.stat (its health)", "collision requires kind_test.armour (what it is clad in)"} {
			if !strings.Contains(msg, want) {
				t.Errorf("panic %q does not mention %q", msg, want)
			}
		}
	}()

	r.Unit.Spec()
	t.Error("Spec accepted a unit missing both requirements")
}
