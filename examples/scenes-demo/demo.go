package main

import (
	"github.com/kjkrol/gram/game"
)

const (
	ScreenWidth  = 640
	ScreenHeight = 480
	TPS          = 60
)

// =========================== Game ===========================

// NewDemo is the game of two Stages: the menu, which starts, and the gameplay it switches to. All
// real behavior lives on the menu's and the gameplay's arenas.
func NewDemo() game.Game {
	_, gameplay := NewGameplayStage("")
	return game.New(game.Props{
		Title:       "gram Stage/Scene demo",
		ScreenWidth: ScreenWidth, ScreenHeight: ScreenHeight,
		TargetTPS: TPS,
	}, NewMenuStage(gameplay.Name()), gameplay)
}
