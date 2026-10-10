package main

import (
	"github.com/kjkrol/gram/game"
)

const (
	ScreenWidth  = 640
	ScreenHeight = 480
	TPS          = 60
)

// TODO: to jest napisane zupelnie z czapy: najpierw stage menu, potem gra

func NewDemo() game.Game {
	_, gameplay := NewGameplayStage("")
	return game.New(game.Props{
		Title:       "gram Stage/Scene demo",
		ScreenWidth: ScreenWidth, ScreenHeight: ScreenHeight,
		TargetTPS: TPS,
	}, NewMenuStage(gameplay.Name()), gameplay)
}
