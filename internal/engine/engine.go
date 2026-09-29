package engine

import (
	"errors"
	"fmt"
	"image/color"
	"log"
	"math"
	"os"
	"time"

	"github.com/gogpu/gogpu"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/render/gpu"
)

const (
	defaultTargetTPS = 60
)

// Engine drives a game.Game through its window's loop (gogpu), one active Stage and its ECS at a
// time.
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
	adapter     *DesktopAdapter
	app         *gogpu.App    // the window, once Run opened it
	window      *render.Image // what a frame is drawn into, then presented on the window
	frames      int
	counted     time.Time
	spent       spent

	// pendingSwitch names the Stage to enter at the top of the next Update.
	pendingSwitch     string
	transitionOverlay render.SolidBackground

	quit bool
	// width and height are the window's, once a resizable one has been laid out.
	width, height int
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

// Quit ends the loop after this tick.
func (e *Engine) Quit() { e.quit = true }

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
	e.pendingSwitch = name
	return nil
}

// Init enters the initial Stage and makes the engine's step the only clock, loop not yet started.
func (e *Engine) Init() error {
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

// Run calls Init, then opens the window and runs the game in it, a tick and a picture a frame,
// until it quits.
func (e *Engine) Run() {
	if err := e.Init(); err != nil {
		log.Fatal(err)
	}
	gpu.Windowed()
	// paced by the swapchain alone: on Wayland gogpu otherwise waits for the compositor's word
	// after every frame, drawing each only once the last is on the screen
	if _, set := os.LookupEnv("GOGPU_WAYLAND_FRAME_CALLBACK"); !set {
		os.Setenv("GOGPU_WAYLAND_FRAME_CALLBACK", "0")
	}
	cfg := gogpu.DefaultConfig().WithTitle(e.props.Title).WithSize(e.props.ScreenWidth, e.props.ScreenHeight).
		WithResizable(e.props.Resizable).WithContinuousRender(true).WithVSync(os.Getenv("GRAM_VSYNC") != "off")
	if os.Getenv("GRAM_FULLSCREEN") != "" { // for measuring: the game starts fullscreen
		cfg = cfg.WithFullscreen()
	}
	e.app = gogpu.NewApp(cfg)
	e.adapter.app = e.app
	control.SetCursorCapture(e.adapter.capture)
	e.app.OnUpdate(func(float64) { e.adapter.Collect() })
	e.app.OnDraw(e.frame)
	if err := e.app.Run(); err != nil {
		log.Fatal(err)
	}
}

// frame runs a tick of the game and draws its picture on the window.
func (e *Engine) frame(dc *gogpu.Context) {
	p := e.app.DeviceProvider()
	view := dc.SurfaceView()
	if p == nil || view == nil {
		return
	}
	if err := gpu.Use(p.Device()); err != nil {
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
	e.window.Clear()
	e.Draw(e.window)
	drawn := time.Now()
	fw, fh := dc.FramebufferSize()
	if enc := dc.CommandEncoder(); enc != nil {
		if err := gpu.Present(enc, view, p.SurfaceFormat(), fw, fh, e.window.Texture()); err != nil {
			log.Printf("gram: a frame not shown: %v", err)
		}
	}
	e.frames++
	e.spent.update += updated.Sub(began)
	e.spent.draw += drawn.Sub(updated)
	e.spent.present += time.Since(drawn)
	if since := time.Since(e.counted); since >= time.Second {
		rate := float64(e.frames) / since.Seconds()
		render.SetRates(rate, rate)
		if logRates {
			n := time.Duration(e.frames)
			log.Printf("gram: %.1f FPS at %dx%d: a frame's tick %v, drawing %v, presenting %v", rate, fw, fh,
				(e.spent.update / n).Round(10*time.Microsecond), (e.spent.draw / n).Round(10*time.Microsecond), (e.spent.present / n).Round(10*time.Microsecond))
		}
		e.frames, e.counted, e.spent = 0, time.Now(), spent{}
	}
}

// logRates has the engine log its frame rate and where a frame's time goes, every second
// (GRAM_FPS_LOG set).
var logRates = os.Getenv("GRAM_FPS_LOG") != ""

// spent is the time a second's frames took, by step.
type spent struct{ update, draw, present time.Duration }

// =================================================================
// the loop's steps: Update a tick, Draw a picture, Layout the screen
// =================================================================

// Update runs a tick: the input captured, a switch of stage made, the world stepped as many times
// as the time gone says; Termination once the game has quit.
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
		current, err := e.enterStage(stage)
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

	if e.current.host.ecs.Paused() {
		return nil
	}

	steps := e.timeTracker.calculateSteps(e.step, maxStepsAFrame)
	for range steps {
		e.current.host.ecs.Tick(e.step)
		e.ticks++
	}
	if e.current.world != nil {
		e.current.world.Clock().Behind(steps == maxStepsAFrame)
		e.current.world.Clock().Pending(e.timeTracker.accumulator)
	}

	if e.timeTracker.processStatsInterval() {
		e.tps.Ticks = e.ticks
		e.ticks = 0
	}

	return nil
}

// maxStepsAFrame is how many ticks one frame catches up on at most; past it the engine is behind.
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

// Layout is the fixed screen of the Props, or with Resizable the window itself, which the active
// world's camera is resized to follow.
func (e *Engine) Layout(outsideWidth, outsideHeight int) (int, int) {
	if !e.props.Resizable || outsideWidth <= 0 || outsideHeight <= 0 {
		return e.props.ScreenWidth, e.props.ScreenHeight
	}
	if outsideWidth != e.width || outsideHeight != e.height {
		e.width, e.height = outsideWidth, outsideHeight
		e.fitViewports(geom.NewAABBAt(geom.Vec{}, float64(e.width), float64(e.height)))
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
func (e *Engine) fitViewports(screen geom.AABB) {
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
		w, h := pixels(screen)
		cam.SetViewport(float32(w), float32(h))
	}
}

// fit resizes vp's camera to its area, when it differs.
func fit(vp render.Viewport) {
	w, h := vp.Camera.Viewport()
	if aw, ah := pixels(vp.Area); int(w) != aw || int(h) != ah {
		vp.Camera.SetViewport(float32(aw), float32(ah))
	}
}

// pixels is the size of a screen rectangle in whole pixels.
func pixels(r geom.AABB) (w, h int) {
	return int(math.Round(r.BottomRight.X - r.TopLeft.X)), int(math.Round(r.BottomRight.Y - r.TopLeft.Y))
}

// screenBox is the screen image's rectangle.
func screenBox(screen *render.Image) geom.AABB {
	b := screen.Bounds()
	return geom.NewAABB(geom.NewVec(float64(b.Min.X), float64(b.Min.Y)), geom.NewVec(float64(b.Max.X), float64(b.Max.Y)))
}
