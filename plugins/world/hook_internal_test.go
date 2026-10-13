package world

import (
	"errors"
	"testing"

	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/rule/act"
	"github.com/kjkrol/gram/rule/cond"
)

func TestHook_RefusesARuleOfAnotherMoment(t *testing.T) {
	p := testPlugin()
	other := act.Branch[struct{}]{
		Name: "of another moment",
		When: cond.Self.If(cond.Always),
		Then: act.Steps(),
	}
	if err := p.Hook(other); !errors.Is(err, plugin.ErrUnhosted) {
		t.Errorf("Hook = %v, want ErrUnhosted", err)
	}
}
