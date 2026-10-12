package main

import (
	"github.com/kjkrol/gram"
	"github.com/kjkrol/gram/game"
)

const (
	ScreenWidth  = 640
	ScreenHeight = 480
	TPS          = 60
)

func main() {
	props := game.Props{
		Title:       "gram Stage/Scene demo",
		ScreenWidth: ScreenWidth, ScreenHeight: ScreenHeight,
		TargetTPS: TPS,
	}
	gram.Run(game.New(props, NewMenuStage(GameplayStage), NewGameplayStage()))
}
