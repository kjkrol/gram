package main

import (
	"image/color"
	"log"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/game/stage"
	"github.com/kjkrol/gram/render"
)

// =========================== Stage ===========================

// MenuStage is the splash/menu — no entities, no gameplay plugins: a Stage of one scene.
type MenuStage struct {
	game.Stage // defined in sections: NewMenuStage
}

// NewMenuStage builds a MenuStage that switches to the Stage named gameplayName on start.
func NewMenuStage(gameplayName string) *MenuStage {
	return &MenuStage{Stage: stage.New("menu").
		Scenes(func() []game.Scene { return []game.Scene{&menuScene{gameplayName: gameplayName}} }).
		Update(func(goke.RunCtx, time.Duration) {})}
}

// =========================== Scene ===========================

// menuScene shows the splash text, enters the gameplay Stage on Enter and quits on Escape.
type menuScene struct{ gameplayName string }

var _ game.Scene = (*menuScene)(nil)

func (m *menuScene) Name() string { return "menu" }

func (m *menuScene) Layers() []render.Layer { return []render.Layer{&menuRenderer{}} }

func (m *menuScene) HandleEvents(events *control.InputEvents, runtime game.Runtime, composition game.Composition) {
	for _, k := range events.KeyEvents {
		if k.Action != control.ActionPress {
			continue
		}
		switch k.Key {
		case control.KeyEnter:
			if err := runtime.SwitchStage(m.gameplayName); err != nil {
				log.Printf("switch stage: %v", err)
			}
		case control.KeyEscape:
			runtime.Quit()
		}
	}
}

func (m *menuScene) Focusable() bool { return true }

type menuRenderer struct{}

func (r *menuRenderer) Init(*goke.SysInit) {}

func (r *menuRenderer) Draw(screen *render.Image) {
	screen.Fill(color.RGBA{R: 20, G: 20, B: 30, A: 255})
	render.DebugPrintAt(screen, "gram Stage/Scene demo\n\nPress ENTER to start", 20, 20)
}
