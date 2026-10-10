package game_test

import (
	"testing"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/game"
)

// named is a Stage of a test that is only its name.
type named string

func (n named) Name() string                    { return string(n) }
func (named) Init(game.Initializer) error       { return nil }
func (named) Loaded() error                     { return nil }
func (named) Spawn() error                      { return nil }
func (named) Update(goke.RunCtx, time.Duration) {}
func (named) Stack() game.Scenes                { return nil }

func TestNew_StartsWithTheFirstStageUnlessToldOtherwise(t *testing.T) {
	g := game.New(game.Props{Title: "t"}, named("menu"), named("play"))
	stages, first := g.Stages()
	if len(stages) != 2 || first != "menu" || g.Props().Title != "t" {
		t.Fatalf("stages %v starting with %q, props %+v; want menu and play, menu first", stages, first, g.Props())
	}
	if _, first := g.Starting("play").Stages(); first != "play" {
		t.Errorf("Starting(play) starts with %q", first)
	}
}

func TestNew_PanicsForTwoStagesOfOneName(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("two stages called menu were taken, want a panic")
		}
	}()
	game.New(game.Props{}, named("menu"), named("menu"))
}
