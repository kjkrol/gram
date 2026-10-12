package control_test

import (
	"strings"
	"testing"

	"github.com/kjkrol/gram/control"
)

type kept struct{ name string }

func (kept) Command() {}

func (k kept) Defined() bool { return k.name != "" }

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
