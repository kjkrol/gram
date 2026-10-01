package hooks_test

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/collision/hooks"
	"github.com/kjkrol/uid"
)

// collide runs the world with collision for one tick over two elastic boxes closing head-on, the
// contacts logged as opts say.
func collide(t *testing.T, opts ...hooks.LogOption) (idA, idB uid.UID64) {
	t.Helper()
	return colliding(t, true, 1, hooks.LogContacts(opts...))
}

func TestLogContacts_LogsEachContactOnce(t *testing.T) {
	var out bytes.Buffer

	idA, idB := collide(t, hooks.LogTo(&out))

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("logged %d lines, want 1 for one contact: %q", len(lines), out.String())
	}
	for _, want := range []string{fmt.Sprint(idA), fmt.Sprint(idB), "10.00"} {
		if !strings.Contains(lines[0], want) {
			t.Errorf("line %q is missing %q", lines[0], want)
		}
	}
}

func TestLogContacts_LogAs_ReplacesTheLine(t *testing.T) {
	var out bytes.Buffer

	collide(t, hooks.LogTo(&out), hooks.LogAs(func(m collision.Meeting) string {
		return fmt.Sprintf("struck with %.0f", m.Impact)
	}))

	if got := strings.TrimSpace(out.String()); got != "struck with 10" {
		t.Errorf("line = %q, want the custom format", got)
	}
}
