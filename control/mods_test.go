package control_test

import (
	"testing"

	"github.com/kjkrol/gram/control"
)

func TestMods_HoldingAKeyIsATriggerOfItsOwn(t *testing.T) {
	plain := control.Mods{}
	withS := control.Mods{}.Holding(control.KeyS)
	withA := control.Mods{}.Holding(control.KeyA) // the first key
	if plain == withS || plain == withA || withS == withA {
		t.Errorf("Mods %v, %v and %v are not told apart", plain, withS, withA)
	}
	if _, held := plain.Held(); held {
		t.Error("plain Mods hold a key")
	}
	if k, held := withA.Held(); !held || k != control.KeyA {
		t.Errorf("Holding(KeyA).Held() = %v, %v; want KeyA, true", k, held)
	}
	shiftS := control.Mods{Shift: true}.Holding(control.KeyS)
	if k, _ := shiftS.Held(); !shiftS.Shift || k != control.KeyS {
		t.Errorf("Shift + S = %+v, want both", shiftS)
	}
}
