package engine

import (
	"errors"
	"fmt"
	"image/color"
	"log"
	"os"
	"time"

	"github.com/gogpu/gogpu"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/internal/parallel"
	"github.com/kjkrol/gram/render"
)

const (
	defaultTargetTPS = 60
)

// Engine drives a game.Game through its window's loop (gogpu), one active Stage and its ECS at a
// time.
type Engine struct {
	game    game.Game
	current *stageRuntime

	ticks       int
	step        time.Duration
	timeTracker *tracker
	props       game.Props
	inputs      *control.InputEvents
	tps         *game.TPS
	pool        *parallel.Pool
	controller  *DefaultController
	adapter     *DesktopAdapter
	app         *gogpu.App // the window, once Run opened it
	window      *render.Image
	frames      int
	counted     time.Time
	spent       spent

	pendingSwitch     string
	pendingLoad       *string // nil: entered afresh
	commands          commands
	transitionOverlay render.SolidBackground

	quit          bool
	width, height int
	shot          *shot
}

var _ game.Runtime = (*Engine)(nil)

// Termination is what Update returns once the game has quit.
var Termination = errors.New("gram: the game quit")

// NewEngine builds an Engine driving g, configured by g.Props().
func NewEngine(g game.Game) *Engine {
	props := g.Props()
	inputs := &control.InputEvents{}
	tps := &game.TPS{}

	targetTPS := defaultTargetTPS
	if props.TargetTPS != 0 {
		targetTPS = props.TargetTPS
	}
	adapter := &DesktopAdapter{}
	controller := NewDefaultController(adapter, inputs)
	e := &Engine{
		adapter:           adapter,
		game:              g,
		props:             props,
		inputs:            inputs,
		tps:               tps,
		pool:              parallel.New(props.Workers),
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

// Quit ends the loop after this tick.
func (e *Engine) Quit() { e.quit = true }

// Props are the game's, read once as the engine was made.
func (e *Engine) Props() game.Props { return e.props }

// ToggleFullscreen switches the window to fullscreen and back.
func (e *Engine) ToggleFullscreen() {
	if e.app != nil {
		e.app.ToggleFullscreen()
	}
}

// SwitchStage requests a transition to the Stage called name, made at the start of the next Update.
func (e *Engine) SwitchStage(name string) error {
	stages, _ := e.game.Stages()
	if _, ok := stages[name]; !ok {
		return fmt.Errorf("gram: unknown stage %q", name)
	}
	e.pendingSwitch, e.pendingLoad = name, nil
	return nil
}

// Init enters the initial Stage, or the one Load asked for from its save.
func (e *Engine) Init() error {
	stages, initial := e.game.Stages()
	if e.pendingSwitch != "" {
		initial, e.pendingSwitch = e.pendingSwitch, ""
	}
	stage, ok := stages[initial]
	if !ok {
		return fmt.Errorf("gram: initial stage %q not found among registered Stages", initial)
	}

	current, err := e.enterStage(stage, e.takeLoad())
	if err != nil {
		return err
	}
	e.current = current
	return nil
}

// Run calls Init, then opens the window and runs the game in it, a tick and a picture a frame,
// until it quits.
func (e *Engine) Run() {
	render.Windowed()
	if err := e.Init(); err != nil {
		log.Fatal(err)
	}
	e.shot = shotFromEnv()
	// paced by the swapchain alone: on Wayland gogpu otherwise waits for the compositor's word
	// after every frame, drawing each only once the last is on the screen
	if _, set := os.LookupEnv("GOGPU_WAYLAND_FRAME_CALLBACK"); !set {
		os.Setenv("GOGPU_WAYLAND_FRAME_CALLBACK", "0")
	}
	cfg := gogpu.DefaultConfig().WithTitle(e.props.Title).WithSize(e.props.ScreenWidth, e.props.ScreenHeight).
		WithResizable(e.props.Resizable).WithContinuousRender(true).WithVSync(os.Getenv("GRAM_VSYNC") != "off")
	if os.Getenv("GRAM_FULLSCREEN") != "" {
		cfg = cfg.WithFullscreen()
	}
	e.app = gogpu.NewApp(cfg)
	e.adapter.app = e.app
	control.SetCursorCapture(e.adapter.capture)
	e.app.OnUpdate(func(float64) { e.adapter.Collect() })
	e.app.OnDraw(e.frame)
	e.timeTracker.restart()
	if err := e.app.Run(); err != nil {
		log.Fatal(err)
	}
}

func (e *Engine) frame(dc *gogpu.Context) {
	p := e.app.DeviceProvider()
	view := dc.SurfaceView()
	if p == nil || view == nil {
		return
	}
	if err := render.UseDevice(p.Device()); err != nil {
		log.Fatal(err)
	}
	began := time.Now()
	if err := e.Update(); err != nil {
		if errors.Is(err, Termination) {
			e.app.Quit()
			return
		}
		log.Fatal(err)
	}
	updated := time.Now()
	w, h := e.Layout(dc.Size())
	if e.window == nil || e.window.Bounds().Dx() != w || e.window.Bounds().Dy() != h {
		if e.window != nil {
			e.window.Deallocate()
		}
		e.window = render.NewImage(w, h)
	}
	if !e.current.prepared && e.pendingSwitch == "" {
		e.prepare()
		began, updated = time.Now(), time.Now()
	}
	e.window.Clear()
	e.Draw(e.window)
	if e.shot != nil && e.shot.take(e.window) {
		e.app.Quit()
		return
	}
	drawn := time.Now()
	fw, fh := dc.FramebufferSize()
	if enc := dc.CommandEncoder(); enc != nil {
		if err := render.Present(enc, view, p.SurfaceFormat(), fw, fh, e.window); err != nil {
			log.Printf("gram: a frame not shown: %v", err)
		}
	}
	if logRates && e.current.firsts < 3 {
		e.current.firsts++
		log.Printf("gram: the stage's frame %d in %v", e.current.firsts,
			time.Since(began).Round(10*time.Microsecond))
	}
	e.frames++
	e.spent.update += updated.Sub(began)
	e.spent.draw += drawn.Sub(updated)
	e.spent.present += time.Since(drawn)
	if since := time.Since(e.counted); since >= time.Second {
		rate := float64(e.frames) / since.Seconds()
		render.SetRates(rate)
		n := time.Duration(e.frames)
		render.SetSpent(e.spent.update/n, e.spent.draw/n, e.spent.present/n)
		if logRates {
			log.Printf("gram: %.1f FPS at %dx%d: a frame's tick %v, drawing %v, presenting %v", rate, fw, fh,
				(e.spent.update / n).Round(10*time.Microsecond), (e.spent.draw / n).Round(10*time.Microsecond), (e.spent.present / n).Round(10*time.Microsecond))
		}
		e.frames, e.counted, e.spent = 0, time.Now(), spent{}
	}
}

var logRates = os.Getenv("GRAM_FPS_LOG") != ""

// prepare readies a stage entered for its first frame: every shader compiled, what waits on the
// CPU on the GPU, and a frame drawn and thrown away, for all that is baked, laid out or built as
// it is first drawn. The time it takes is not the world's.
func (e *Engine) prepare() {
	began := time.Now()
	if err := render.Prepare(); err != nil {
		log.Fatal(err)
	}
	compiled := time.Now()
	e.window.Clear()
	e.Draw(e.window)
	render.Submit()
	e.current.prepared = true
	e.timeTracker.restart()
	if logRates {
		log.Printf("gram: the stage readied in %v: shaders and uploads %v, a first frame %v",
			time.Since(began).Round(10*time.Microsecond),
			compiled.Sub(began).Round(10*time.Microsecond),
			time.Since(compiled).Round(10*time.Microsecond))
	}
}

type spent struct{ update, draw, present time.Duration }

// Update runs a tick: the input, a switch of stage, the world stepped as many times as the time
// gone says. It returns Termination once the game has quit.
func (e *Engine) Update() error {
	if e.quit {
		return Termination
	}

	if e.pendingSwitch != "" {
		name := e.pendingSwitch
		e.pendingSwitch = ""
		stages, _ := e.game.Stages()
		stage, ok := stages[name]
		if !ok {
			return fmt.Errorf("gram: unknown stage %q", name)
		}
		current, err := e.enterStage(stage, e.takeLoad())
		if err != nil {
			return err
		}
		e.current = current
	}

	e.controller.Capture(e.inputs)
	for _, k := range e.inputs.KeyEvents {
		if k.Key == control.KeyF11 && k.Action == control.ActionPress {
			e.ToggleFullscreen()
		}
	}
	e.controller.Update(nil, 0)
	e.inputs.ResetTransient()
	e.carry()

	if e.current.host.ecs.Paused() {
		return nil
	}

	steps := e.timeTracker.calculateSteps(e.step, maxStepsAFrame)
	if e.shot != nil {
		steps, e.timeTracker.accumulator = 1, 0
	}
	for range steps {
		e.current.host.ecs.Tick(e.step)
		e.ticks++
	}
	if e.current.world != nil {
		e.current.world.Write().Clock().Behind(steps == maxStepsAFrame)
		e.current.world.Write().Clock().Pending(e.timeTracker.accumulator)
	}

	if e.timeTracker.processStatsInterval() {
		e.tps.Ticks = e.ticks
		e.ticks = 0
	}

	return nil
}

func (e *Engine) takeLoad() *string {
	slot := e.pendingLoad
	e.pendingLoad = nil
	return slot
}

// a frame further behind drops the rest: the game slows instead of spiralling
const maxStepsAFrame = 5

// Draw draws the active stage's scenes, bottom to top, onto screen.
func (e *Engine) Draw(screen *render.Image) {
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

// Layout is the screen's size: the Props' fixed one, or with Resizable the window's.
func (e *Engine) Layout(outsideWidth, outsideHeight int) (int, int) {
	if !e.props.Resizable || outsideWidth <= 0 || outsideHeight <= 0 {
		return e.props.ScreenWidth, e.props.ScreenHeight
	}
	e.width, e.height = outsideWidth, outsideHeight
	return e.width, e.height
}

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
