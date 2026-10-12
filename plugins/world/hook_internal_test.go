package world

import (
	"errors"
	"testing"

	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/rule"
)

func TestHook_RefusesARuleOfAnotherMoment(t *testing.T) {
	p := testPlugin()
	other := rule.Branch[struct{}]{Name: "of another moment", Do: rule.Steps()}
	if err := p.Hook(other); !errors.Is(err, plugin.ErrUnhosted) {
		t.Errorf("Hook = %v, want ErrUnhosted", err)
	}
}
