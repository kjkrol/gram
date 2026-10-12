package world

import (
	"testing"

	"github.com/kjkrol/goke/v3"
)

func testWorld() *module {
	return NewPlugin(Config{
		Space:    SpaceCfg{Width: 1000, Height: 1000},
		Entities: EntitiesCfg{MaxCount: 10, MinSize: 1, MaxSize: 100},
	}).module
}

func TestWorld_PostLoad_SetsCountToZeroWhenNothingSeeded(t *testing.T) {
	w := testWorld()

	goke.New().Setup(w.PostLoad())

	if w.telemetry.Count != 0 {
		t.Errorf("telemetry.Count = %d, want 0 (no Position entities seeded)", w.telemetry.Count)
	}
}

func TestWorld_PostLoad_SetsCountFromLoadedEntities(t *testing.T) {
	w := testWorld()

	pos := Position{}
	pos.Size.X, pos.Size.Y = 10, 10

	var baseComp goke.Comp[Base]
	ecs := goke.New()
	ecs.Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		f := si.NewFactory(&baseComp)
		f.Create(3)
		for f.Next() {
			for i := range baseComp.Slice(&f.Cursor) {
				baseComp.Slice(&f.Cursor)[i].Pos = pos
			}
		}
	}}, w.PostLoad())

	if w.telemetry.Count != 3 {
		t.Errorf("telemetry.Count = %d, want 3", w.telemetry.Count)
	}
}
