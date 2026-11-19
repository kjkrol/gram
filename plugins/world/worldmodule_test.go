package world

import (
	"testing"

	"github.com/kjkrol/goke/v3"
)

func TestWorld_Populate_EndToEnd(t *testing.T) {
	wm := testWorld()

	wm.populate(testKind(spawnerTestPos(), Velocity{}), []any{nil, nil, nil})

	ecs := goke.New()
	var base goke.Comp[Base]
	var q *goke.Query
	systems := append(wm.SetupSystems(), goke.SystemFn{OnInit: func(si *goke.SysInit) {
		q = si.NewQueryBuilder(&base).Build()
	}})
	ecs.Setup(systems...)

	if wm.telemetry.Count != 3 {
		t.Errorf("telemetry.Count = %d, want 3", wm.telemetry.Count)
	}

	count := 0
	q.All()
	for q.Next() {
		for _, b := range base.Slice(q.Cursor()) {
			count++
			if b.Pos.TopLeft != spawnerTestPos().TopLeft {
				t.Errorf(
					"spawned entity position = %+v, want %+v",
					b.Pos.TopLeft,
					spawnerTestPos().TopLeft,
				)
			}
		}
	}
	if count != 3 {
		t.Errorf("found %d entities with Position, want 3", count)
	}
}

func TestWorld_RegSystems_IsIdempotent(t *testing.T) {
	wm := testWorld()
	ecs := goke.New()

	wm.RegSystems(ecs)
	wm.RegSystems(ecs)
}
