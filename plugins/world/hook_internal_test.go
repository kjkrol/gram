package world

import (
	"errors"
	"testing"

	"github.com/kjkrol/gram/rule"
)

// Hook hosts the rules of the world's moments alone: one of another moment is refused.
func TestHook_RefusesARuleOfAnotherMoment(t *testing.T) {
	p := testPlugin()
	other := rule.On("of another moment", rule.All, func(m *rule.Moment[struct{}]) rule.Step { return m.Steps() })
	if err := p.Hook(other); !errors.Is(err, rule.ErrUnhosted) {
		t.Errorf("Hook = %v, want ErrUnhosted", err)
	}
}
