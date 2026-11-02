package engine_test

import (
	"testing"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/game/stage"
	"github.com/kjkrol/gram/internal/engine"
	"github.com/kjkrol/gram/plugins/world"
)

type keptTreasury struct{ Gold float32 }

type keptDiary struct{ Days int }

func keptStage(res *keptTreasury, diary *keptDiary) game.Stage {
	return stage.New("kept").
		World(testWorldConfig()).
		Plugins(func(*world.Plugin) {}).
		Keeps(res, diary).
		Update(func(goke.RunCtx, time.Duration) {})
}

func TestStage_KeepsTheGamesResourcesWithTheSave(t *testing.T) {
	props := game.Props{Saves: t.TempDir()}
	res, diary := &keptTreasury{Gold: 500}, &keptDiary{}
	eng := engine.NewEngine(oneStageGame{stage: keptStage(res, diary), props: props})
	if err := eng.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	res.Gold, diary.Days = 335.5, 12
	if err := eng.Save(""); err != nil {
		t.Fatalf("Save: %v", err)
	}
	loaded, days := &keptTreasury{Gold: 500}, &keptDiary{}
	eng2 := engine.NewEngine(oneStageGame{stage: keptStage(loaded, days), props: props})
	if err := eng2.Load("", ""); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := eng2.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if loaded.Gold != 335.5 || days.Days != 12 {
		t.Errorf("loaded %v gold and %d days, want the 335.5 and 12 saved: two resources kept apart",
			loaded.Gold, days.Days)
	}
}
