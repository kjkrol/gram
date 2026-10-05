package control_test

import (
	"strings"
	"testing"

	"github.com/kjkrol/gram/control"
)

// kept is a command a register keeps: defined or not.
type kept struct{ name string }

func (k kept) Defined() bool { return k.name != "" }

// A command a register keeps is given to a key only as the register hands it back: one made on
// the spot is refused, naming the key.
func TestGive_RefusesACommandNoRegisterHolds(t *testing.T) {
	control.Give(control.KeyPress{Key: control.KeyJ}, "Hasten", kept{name: "hasten"}) // defined: taken
	defer func() {
		msg, _ := recover().(string)
		if !strings.Contains(msg, `"Freeze"`) || !strings.Contains(msg, "define it by name") {
			t.Errorf("panic %q, want it to name the key and say to define the command", msg)
		}
	}()
	control.Give(control.KeyPress{Key: control.KeyF}, "Freeze", kept{})
}
