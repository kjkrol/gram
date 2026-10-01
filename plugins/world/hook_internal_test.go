package world

import (
	"errors"
	"testing"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugin"
)

// Hook hosts rules alone: a goke system or anything else that is no rule of the world's moments
// is refused.
func TestHook_RefusesWhatIsNoRule(t *testing.T) {
	p := testPlugin()
	for name, b := range map[string]plugin.Rule{
		"a goke system": goke.SystemFn{},
		"not a rule":    struct{}{},
	} {
		if err := p.Hook(b); !errors.Is(err, plugin.ErrUnhosted) {
			t.Errorf("Hook(%s) = %v, want ErrUnhosted", name, err)
		}
	}
}
