package engine

import (
	"fmt"
	"image"
	"image/color"
	"log"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/render"
)

const (
	defaultTargetTPS = 60
)

// Engine drives a game.Game through the Ebitengine loop, one active Stage and its ECS at a time.
// Stage names are always resolved against game.Stages().
type Engine struct {
	game    game.Game
	current *stageRuntime

	ticks       int
	step        time.Duration
	timeTracker *tracker
	props       game.Props
	inputs      *control.InputEvents
	tps         *game.TPS
	controller  *DefaultController

	// pendingSwitch names the Stage to enter at the top of the next Update.
	pendingSwitch     string
	transitionOverlay render.SolidBackground

	quit bool
	// width and height are the window's, once a resizable one has been laid out.
	width, height int
}

var _ ebiten.Game = (*Engine)(nil)
var _ game.Runtime = (*Engine)(nil)

// NewEngine builds an Engine driving g, configured by g.Props().
func NewEngine(g game.Game) *Engine {
	props := g.Props()
	inputs := &control.InputEvents{}
	tps := &game.TPS{}

	targetTPS := defaultTargetTPS
	if props.TargetTPS != 0 {
		targetTPS = props.TargetTPS
	}
	controller := NewDefaultController(&DesktopAdapter{}, inputs)
	e := &Engine{
		game:              g,
		props:             props,
		inputs:            inputs,
		tps:               tps,
		step:              time.Second / time.Duration(targetTPS),
		timeTracker:       newTracker(),
		controller:        controller,
		transitionOverlay: render.SolidBackground{Color: color.RGBA{A: 255}},
	}
	controller.SetHandler(HandlerFn(e.dispatchEvents))
	return e
}

// TPS returns the engine's measured-ticks-per-second counter.
func (e *Engine) TPS() *game.TPS { return e.tps }

// Persistence returns the active Stage's Save/Load/List surface.
func (e *Engine) Persistence() game.Persistence { return &persistence{host: e.current.host} }

func (e *Engine) Paused() bool { return e.current.host.ecs.Paused() }

func (e *Engine) Pause() { e.current.host.ecs.Pause() }

func (e *Engine) Resume() { e.current.host.ecs.Resume() }

func (e *Engine) TogglePause() {
	if e.current.host.ecs.Paused() {
		e.current.host.ecs.Resume()
	} else {
		e.current.host.ecs.Pause()
	}
}

// Camera returns the active Stage's world camera, or nil if the Stage has no world.
func (e *Engine) Camera() camera.Camera {
	if e.current.world == nil {
		return nil
	}
	return e.current.world.Camera()
}

// Quit ends the Ebitengine loop after this tick.
func (e *Engine) Quit() { e.quit = true }

// ToggleFullscreen switches the window to fullscreen and back.
func (e *Engine) ToggleFullscreen() { ebiten.SetFullscreen(!ebiten.IsFullscreen()) }

// SwitchStage requests a transition to the Stage called name, made at the start of the next Update.
func (e *Engine) SwitchStage(name string) error {
	stages, _ := e.game.Stages()
	if _, ok := stages[name]; !ok {
		return fmt.Errorf("gram: unknown stage %q", name)
	}
	e.pendingSwitch = name
	return nil
}

// Init enters the initial Stage and makes the engine's step the only clock, loop not yet started.
func (e *Engine) Init() error {
	ebiten.SetTPS(ebiten.SyncWithFPS)
	stages, initial := e.game.Stages()
	stage, ok := stages[initial]
	if !ok {
		return fmt.Errorf("gram: initial stage %q not found among registered Stages", initial)
	}

	current, err := e.enterStage(stage)
	if err != nil {
		return err
	}
	e.current = current
	return nil
}

// Run calls Init, then starts the Ebitengine loop.
func (e *Engine) Run() {
	if err := e.Init(); err != nil {
		log.Fatal(err)
	}

	ebiten.SetWindowSize(e.props.ScreenWidth, e.props.ScreenHeight)
	ebiten.SetWindowTitle(e.props.Title)
	if e.props.Resizable {
		ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	}
	if err := ebiten.RunGame(e); err != nil {
		log.Fatal(err)
	}
}

// =================================================================
// ebiten.Game contract implementation
// =================================================================

func (e *Engine) Update() error {
	if e.quit {
		return ebiten.Termination
	}

	if e.pendingSwitch != "" {
		name := e.pendingSwitch
		e.pendingSwitch = ""
		stages, _ := e.game.Stages()
		stage, ok := stages[name]
		if !ok {
			return fmt.Errorf("gram: unknown stage %q", name)
		}
		current, err := e.enterStage(stage)
		if err != nil {
			return err
		}
		e.current = current
	}

	e.controller.Capture(e.inputs)
	for _, k := range e.inputs.KeyEvents {
		if k.Key == ebiten.KeyF && k.Action == control.ActionPress && e.inputs.Modifiers.Shift {
			e.ToggleFullscreen()
		}
	}
	e.controller.Update(nil, 0)
	e.inputs.ResetTransient()

	if e.current.host.ecs.Paused() {
		return nil
	}

	steps := e.timeTracker.calculateSteps(e.step, 5)
	for range steps {
		e.current.host.ecs.Tick(e.step)
		e.ticks++
	}

	if e.timeTracker.processStatsInterval() {
		e.tps.Ticks = e.ticks
		e.ticks = 0
	}

	return nil
}

func (e *Engine) Draw(screen *ebiten.Image) {
	if e.pendingSwitch != "" {
		e.transitionOverlay.Draw(screen)
		return
	}
	stack := e.current.stage.Stack()
	for _, name := range stack.Composition().Order() {
		if sc, ok := stack.Get(name); ok {
			e.current.drawScene(screen, sc)
		}
	}
}

// Layout is the fixed screen of the Props, or with Resizable the window itself, which the active
// world's camera is resized to follow.
func (e *Engine) Layout(outsideWidth, outsideHeight int) (int, int) {
	if !e.props.Resizable || outsideWidth <= 0 || outsideHeight <= 0 {
		return e.props.ScreenWidth, e.props.ScreenHeight
	}
	if outsideWidth != e.width || outsideHeight != e.height {
		e.width, e.height = outsideWidth, outsideHeight
		e.fitViewports(image.Rect(0, 0, e.width, e.height))
	}
	return e.width, e.height
}

// screen is the size the screen has now: the window's once a resizable one has been laid out.
func (e *Engine) screen() (int, int) {
	if e.props.Resizable && e.width > 0 && e.height > 0 {
		return e.width, e.height
	}
	return e.props.ScreenWidth, e.props.ScreenHeight
}

// =================================================================

// dispatchEvents hands input to the active Scene's HandleEvents and to nothing else.
func (e *Engine) dispatchEvents(events *control.InputEvents) {
	stack := e.current.stage.Stack()
	comp := stack.Composition()
	active := comp.Active()
	if active == "" {
		return
	}
	if sc, ok := stack.Get(active); ok {
		sc.HandleEvents(events, e, comp)
	}
}

// fitViewports sizes every camera of the visible scenes' viewports to its area on screen, and the
// world's camera to the whole screen when no visible scene shows the world.
func (e *Engine) fitViewports(screen image.Rectangle) {
	if e.current == nil {
		return
	}
	stack := e.current.stage.Stack()
	fitted := false
	for _, name := range stack.Composition().Order() {
		sc, _ := stack.Get(name)
		if v, ok := sc.(game.Viewer); ok {
			for _, vp := range v.Viewports(screen) {
				fit(vp)
			}
			fitted = true
		}
	}
	if cam := e.Camera(); !fitted && cam != nil {
		cam.SetViewport(float32(screen.Dx()), float32(screen.Dy()))
	}
}

// fit resizes vp's camera to its area, when it differs.
func fit(vp render.Viewport) {
	w, h := vp.Camera.Viewport()
	if int(w) != vp.Area.Dx() || int(h) != vp.Area.Dy() {
		vp.Camera.SetViewport(float32(vp.Area.Dx()), float32(vp.Area.Dy()))
	}
}
