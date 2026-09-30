package trigger_test

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/collision/trigger"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/act"
	"github.com/kjkrol/uid"
)

// collide runs the real collision engine for one tick over two elastic boxes closing head-on.
func collide(t *testing.T, opts ...trigger.LogOption) (idA, idB uid.UID64) {
	t.Helper()
	space, err := aabbworld.NewSpace(aabbworld.Config{
		Width: 1000, Height: 1000,
		BucketSize: 64,
	})
	if err != nil {
		t.Fatalf("aabbworld.NewSpace: %v", err)
	}

	ecs := goke.New()
	engine := collision.New(space, ecs)
	if err := engine.Hook(act.Trigger[collision.Meeting]("log contacts").Do(trigger.LogContacts(opts...))); err != nil {
		t.Fatalf("Hook: %v", err)
	}

	ecs.Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		var base goke.Comp[world.Base]
		var coll goke.Comp[collision.Collider]
		var physics goke.Comp[collision.Physics]
		f := si.NewFactory(&base, &coll, &physics)
		f.Create(2)
		f.Next()
		idA, idB = f.IDs[0], f.IDs[1]
		for i, x := range []float64{100, 105} {
			box := plane.NewAABB(geom.NewVec(x, 100), 10, 10)
			base.Slice(&f.Cursor)[i].Pos = world.Position{AABB: box}
			physics.Slice(&f.Cursor)[i] = collision.Physics{Restitution: 1}
		}
		base.Slice(&f.Cursor)[0].Vel.SetDelta(geom.NewVec(5, 0))
		base.Slice(&f.Cursor)[1].Vel.SetDelta(geom.NewVec(-5, 0))
	}})
	engine.RegSystems(ecs)
	ecs.SetPlan(engine.RunPlan)
	ecs.Tick(time.Millisecond)
	return idA, idB
}

func TestLogContacts_LogsEachContactOnce(t *testing.T) {
	var out bytes.Buffer

	idA, idB := collide(t, trigger.LogTo(&out))

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

	collide(t, trigger.LogTo(&out), trigger.LogAs(func(m collision.Meeting) string {
		return fmt.Sprintf("struck with %.0f", m.Impact)
	}))

	if got := strings.TrimSpace(out.String()); got != "struck with 10" {
		t.Errorf("line = %q, want the custom format", got)
	}
}
