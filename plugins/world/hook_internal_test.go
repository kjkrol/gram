package world

import (
	"errors"
	"testing"

	"github.com/kjkrol/ceremony/act"
	"github.com/kjkrol/ceremony/cond"
	"github.com/kjkrol/ceremony/host"
)

func TestHook_RefusesARuleOfAnotherMoment(t *testing.T) {
	p := testPlugin()
	other := act.Branch[struct{}]{
		Name: "of another moment",
		When: cond.Self.If(cond.Always),
		Then: act.Steps(),
	}
	if err := p.Hook(other); !errors.Is(err, host.ErrUnhosted) {
		t.Errorf("Hook = %v, want ErrUnhosted", err)
	}
}
