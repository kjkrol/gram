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

func TestGameplayStage_Composition_SurvivesSaveLoad(t *testing.T) {
	basePath := t.TempDir() + "/save"

	playedArena, played := NewGameplayStage(basePath)
	eng := engine.NewEngine(oneStageGame{stage: played, props: testProps()})
	if err := eng.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}

	played.Stack().Composition().Show(playedArena.panel.Name())
	if got, want := played.Stack().Composition().Active(), playedArena.panel.Name(); got != want {
		t.Fatalf("Active() before save = %q, want %q", got, want)
	}

	if err := eng.Persistence().Save(basePath, ""); err != nil {
		t.Fatalf("Save: %v", err)
	}

	arena2, stage2 := NewGameplayStage(basePath)
	eng2 := engine.NewEngine(oneStageGame{stage: stage2, props: testProps()})
	if err := eng2.Init(); err != nil {
		t.Fatalf("Init (fresh process/engine): %v", err)
	}

	wantOrder := []string{"world", "hud", "panel"}
	if got := stage2.Stack().Composition().Order(); !equalStrings(got, wantOrder) {
		t.Errorf("Order() after Load = %v, want %v", got, wantOrder)
	}
	if got, want := stage2.Stack().Composition().Active(), arena2.panel.Name(); got != want {
		t.Errorf("Active() after Load = %q, want %q (the panel should still be on top and focused)", got, want)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
