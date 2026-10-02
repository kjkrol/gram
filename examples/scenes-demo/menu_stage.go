package main

import (
	"image/color"
	"log"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/render"
)

// =========================== Stage ===========================

// MenuStage is the splash/menu — no entities, no gameplay plugins.
type MenuStage struct {
	gameplayName string

	stack game.Scenes
}

// NewMenuStage builds a MenuStage that switches to the Stage named gameplayName on start.
func NewMenuStage(gameplayName string) *MenuStage {
	return &MenuStage{gameplayName: gameplayName}
}

var _ game.Stage = (*MenuStage)(nil)

func (m *MenuStage) Name() string { return "menu" }

func (m *MenuStage) Init(ctx game.Initializer) error {
	main := &menuScene{gameplayName: m.gameplayName}
	stack, err := game.NewStack(main)
	if err != nil {
		return err
	}
	m.stack = stack
	comp := stack.Composition()
	comp.Show(main.Name())
	return ctx.Track(comp)
}

func (m *MenuStage) Restore(game.Persistence) (bool, error) { return false, nil }

func (m *MenuStage) Spawn() error { return nil }

func (m *MenuStage) Update(goke.RunCtx, time.Duration) {}

func (m *MenuStage) Stack() game.Scenes { return m.stack }

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
