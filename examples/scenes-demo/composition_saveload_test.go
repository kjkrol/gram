package main

import (
	"testing"

	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/internal/engine"
)

func testProps() game.Props {
	return game.Props{ScreenWidth: ScreenWidth, ScreenHeight: ScreenHeight}
}

// oneStageGame is a minimal game.Game wrapping a single Stage.
type oneStageGame struct {
	stage game.Stage
	props game.Props
}

func (g oneStageGame) Props() game.Props { return g.props }

func (g oneStageGame) Stages() (map[string]game.Stage, string) {
	return map[string]game.Stage{g.stage.Name(): g.stage}, g.stage.Name()
}

// The panel open as the game is saved is open as it is loaded.
func TestGameplayStage_PanelOpenSurvivesSaveLoad(t *testing.T) {
	basePath := t.TempDir() + "/save"

	played, stage := NewGameplayStage(basePath)
	eng := engine.NewEngine(oneStageGame{stage: stage, props: testProps()})
	if err := eng.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	played.scene.Show(PanelElement)
	if err := eng.Persistence().Save(basePath, ""); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, stage2 := NewGameplayStage(basePath)
	eng2 := engine.NewEngine(oneStageGame{stage: stage2, props: testProps()})
	if err := eng2.Init(); err != nil {
		t.Fatalf("Init (fresh process/engine): %v", err)
	}
	if !loaded.scene.Shown(PanelElement) {
		t.Error("the panel open when saved is closed when loaded")
	}
	if got := stage2.Stack().Composition().Active(); got != WorldScene {
		t.Errorf("Active() after Load = %q, want %q", got, WorldScene)
	}
}
