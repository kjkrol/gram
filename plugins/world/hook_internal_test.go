package world

import (
	"errors"
	"testing"

	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/rule"
)

// Hook hosts the rules of the world's moments alone: one of another moment is refused.
func TestHook_RefusesARuleOfAnotherMoment(t *testing.T) {
	p := testPlugin()
	other := rule.Then[struct{}]("of another moment", rule.All, rule.Steps())
	if err := p.Hook(other); !errors.Is(err, plugin.ErrUnhosted) {
		t.Errorf("Hook = %v, want ErrUnhosted", err)
	}
}
