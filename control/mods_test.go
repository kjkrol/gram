package control_test

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/kjkrol/gram/control"
)

func TestMods_HoldingAKeyIsATriggerOfItsOwn(t *testing.T) {
	plain := control.Mods{}
	withS := control.Mods{}.Holding(ebiten.KeyS)
	withA := control.Mods{}.Holding(ebiten.KeyA) // KeyA is ebiten's zero key
	if plain == withS || plain == withA || withS == withA {
		t.Errorf("Mods %v, %v and %v are not told apart", plain, withS, withA)
	}
	if _, held := plain.Held(); held {
		t.Error("plain Mods hold a key")
	}
	if k, held := withA.Held(); !held || k != ebiten.KeyA {
		t.Errorf("Holding(KeyA).Held() = %v, %v; want KeyA, true", k, held)
	}
	shiftS := control.Mods{Shift: true}.Holding(ebiten.KeyS)
	if k, _ := shiftS.Held(); !shiftS.Shift || k != ebiten.KeyS {
		t.Errorf("Shift + S = %+v, want both", shiftS)
	}
}
