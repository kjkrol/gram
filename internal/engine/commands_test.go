package engine

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/game/stage"
)

func twoStages(t *testing.T) (*Engine, *int) {
	t.Helper()
	s := &sectioned{}
	spawns := 0
	meadow := stage.New("meadow").
		World(testWorldConfig()).Plugins(s.makeWorld).
		Spawn(func() { spawns++ }).
		Scenes(func() []game.Scene { return []game.Scene{&scene{name: "main"}} }).
		Update(s.update)
	menu := stage.New("menu").
		Scenes(func() []game.Scene { return []game.Scene{&scene{name: "menu"}} }).
		Update(func(goke.RunCtx, time.Duration) {})
	eng := NewEngine(game.New(game.Props{Saves: t.TempDir()}, meadow, menu))
	if err := eng.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	return eng, &spawns
}

func give(t *testing.T, eng *Engine, cmd command.Command) {
	t.Helper()
	if err := eng.Issue(cmd); err != nil {
		t.Fatalf("Issue %T: %v", cmd, err)
	}
	if err := eng.Update(); err != nil {
		t.Fatalf("Update: %v", err)
	}
}

func TestCommands_SaveEnterAndLoadAcrossStages(t *testing.T) {
	eng, spawns := twoStages(t)
	give(t, eng, game.Save{})
	if _, err := os.Stat(filepath.Join(eng.props.Saves, "meadow.game.save")); err != nil {
		t.Fatalf("Save wrote nothing where the game keeps its saves: %v", err)
	}
	give(t, eng, game.Enter{Stage: "menu"})
	if err := eng.Update(); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if got := eng.current.stage.Name(); got != "menu" {
		t.Fatalf("after Enter the stage is %q, want menu", got)
	}
	give(t, eng, game.Load{Stage: "meadow"})
	if err := eng.Update(); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if got := eng.current.stage.Name(); got != "meadow" {
		t.Fatalf("after Load the stage is %q, want meadow", got)
	}
	if *spawns != 1 {
		t.Errorf("the meadow spawned %d times, want once: loaded, it spawns nothing", *spawns)
	}
	if got := eng.current.stage.Stack().Composition().Order(); len(got) != 1 || got[0] != "main" {
		t.Errorf("the loaded meadow shows %v, want its scene", got)
	}
}

func TestCommands_LoadingASaveNotThereChangesNothing(t *testing.T) {
	eng, _ := twoStages(t)
	if err := eng.Load("meadow", "nowhere"); err == nil {
		t.Fatal("Load of a save not there: no error")
	}
	if eng.pendingSwitch != "" || eng.pendingLoad != nil {
		t.Errorf("a failed Load left %q, %v pending", eng.pendingSwitch, eng.pendingLoad)
	}
}

func TestCommands_QuitEndsTheGame(t *testing.T) {
	eng, _ := twoStages(t)
	give(t, eng, game.Quit{})
	if err := eng.Update(); !errors.Is(err, Termination) {
		t.Errorf("the frame after Quit: %v, want Termination", err)
	}
}
