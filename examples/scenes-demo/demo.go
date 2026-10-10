package main

import (
	"github.com/kjkrol/gram/game"
)

const (
	ScreenWidth  = 640
	ScreenHeight = 480
	TPS          = 60
)

func NewDemo() game.Game {
	return game.New(game.Props{
		Title:       "gram Stage/Scene demo",
		ScreenWidth: ScreenWidth, ScreenHeight: ScreenHeight,
		TargetTPS: TPS,
	}, NewMenuStage(GameplayStage), NewGameplayStage())
}
