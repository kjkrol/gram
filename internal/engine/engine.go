package engine

import (
	"errors"
	"fmt"
	"image/color"
	"log"
	"os"
	"runtime"
	"time"

	"github.com/gogpu/gogpu"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/internal/parallel"
	"github.com/kjkrol/gram/plugin"
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
	order         []string // the scenes drawn, bottom to top, as the barrier found them shown
	started       time.Time
	last          plugin.FrameTimes // the last frame's, paced on the next barrier
	awaited       time.Duration     // the last frame's wait for its simulation
	timed         bool              // a frame timed since the last barrier

	serial    bool               // Props.Serial, GRAM_SERIAL: each frame ticked before it is drawn
	running   bool               // a simulation simulate started, not joined yet
	simulated chan time.Duration // how long it took
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
	serial := props.Serial || os.Getenv("GRAM_SERIAL") != ""
	e := &Engine{
		adapter:           adapter,
		game:              g,
		props:             props,
		inputs:            inputs,
		tps:               tps,
		pool:              parallel.New(workers(props.Workers, serial)),
		step:              time.Second / time.Duration(targetTPS),
		timeTracker:       newTracker(),
		controller:        controller,
		transitionOverlay: render.SolidBackground{Color: color.RGBA{A: 255}},
		serial:            serial,
		simulated:         make(chan time.Duration, 1),
	}
	controller.SetHandler(HandlerFn(e.dispatchEvents))
	return e
}

// workers is how many goroutines the game's pool has: as many as asked, else as many as there are
// CPUs, one left to the drawing where the ticks run while a frame is drawn
func workers(asked int, serial bool) int {
	if asked > 0 {
		return asked
	}
	n := runtime.GOMAXPROCS(0)
	if !serial {
		n--
	}
	return max(n, 1)
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
	e.join()
}

func (e *Engine) frame(dc *gogpu.Context) {
	start := time.Now()
	p := e.app.DeviceProvider()
	view := dc.SurfaceView()
	if p == nil || view == nil {
		return
	}
	if err := render.UseDevice(p.Device()); err != nil {
		log.Fatal(err)
	}
	t := plugin.FrameTimes{Wait: time.Since(start)}
	if !e.started.IsZero() {
		t.Frame = start.Sub(e.started)
	}
	e.started = start
	began := time.Now()
	w, h := e.Layout(dc.Size())
	if e.window == nil || e.window.Bounds().Dx() != w || e.window.Bounds().Dy() != h {
		if e.window != nil {
			e.window.Deallocate()
		}
		e.window = render.NewImage(w, h)
	}
	readied, awaited, err := e.advance(w, h, &t)
	if err != nil {
		if errors.Is(err, Termination) {
			e.app.Quit()
			return
		}
		log.Fatal(err)
	}
	drawing := time.Now()
	e.window.Clear()
	e.Draw(e.window)
	if e.shot != nil && e.shot.take(e.window) {
		e.join()
		e.app.Quit()
		return
	}
	t.Draw = time.Since(drawing)
	presenting := time.Now()
	fw, fh := dc.FramebufferSize()
	if enc := dc.CommandEncoder(); enc != nil {
		if err := render.Present(enc, view, p.SurfaceFormat(), fw, fh, e.window); err != nil {
			log.Printf("gram: a frame not shown: %v", err)
		}
	}
	t.Present = time.Since(presenting)
	if logRates && e.current.firsts < 3 {
		e.current.firsts++
		log.Printf("gram: the stage's frame %d in %v", e.current.firsts,
			time.Since(began).Round(10*time.Microsecond))
	}
	// a frame readying its stage is no measure of the next
	e.last, e.awaited, e.timed = t, awaited, !readied
}

// advance is a frame's work before it is drawn on a screen w by h: the simulation the last
// barrier started waited for, else the frame's input, ticks and publishing run now; the barrier,
// the input taken there when the ticks ran apart; and the next frame's simulation started to run
// while this one is drawn, where it may. It reports whether the stage was readied and how long
// the frame waited for its simulation; Termination once the game has quit.
func (e *Engine) advance(w, h int, t *plugin.FrameTimes) (readied bool, awaited time.Duration,
	err error) {
	began := time.Now()
	took, ran := e.join()
	if ran {
		awaited = time.Since(began)
	} else {
		if err := e.Update(); err != nil {
			return false, 0, err
		}
		e.publish()
		took = time.Since(began)
	}
	t.Simulate = took
	barred := time.Now()
	readied, err = e.barrier(w, h, ran)
	t.Barrier = time.Since(barred)
	if err == nil && e.concurrent() {
		e.simulate()
	}
	return readied, awaited, err
}

// concurrent reports whether the next frame's simulation may run while this one is drawn: not
// serially, nor while a stage waits to be entered or readied, nor once the game quits
func (e *Engine) concurrent() bool {
	return !e.serial && e.pendingSwitch == "" && e.current.prepared && !e.quit
}

// simulate starts the next frame's ticks and publishing on a goroutine of their own
func (e *Engine) simulate() {
	e.running = true
	go func() {
		began := time.Now()
		e.tick()
		e.publish()
		e.simulated <- time.Since(began)
	}()
}

// join waits for the simulation simulate started, reporting how long it took; false where none
// runs
func (e *Engine) join() (time.Duration, bool) {
	if !e.running {
		return 0, false
	}
	e.running = false
	return <-e.simulated, true
}

// pace hands the last frame's times to what is paced, and once a second notes the frame rate and
// where a frame's time went (render.SetRates, SetSpent), logged with GRAM_FPS_LOG
func (e *Engine) pace() {
	if !e.timed {
		return
	}
	e.timed = false
	t := e.last
	for _, p := range e.current.pacers {
		p.Pace(t)
	}
	e.frames++
	e.spent.update += t.Simulate
	e.spent.awaited += e.awaited
	e.spent.barrier += t.Barrier
	e.spent.draw += t.Draw
	e.spent.present += t.Present
	e.spent.wait += t.Wait
	since := time.Since(e.counted)
	if since < time.Second {
		return
	}
	rate := float64(e.frames) / since.Seconds()
	render.SetRates(rate)
	n := time.Duration(e.frames)
	render.SetSpent(e.spent.update/n, e.spent.draw/n, e.spent.present/n)
	if logRates {
		us := func(d time.Duration) time.Duration { return (d / n).Round(10 * time.Microsecond) }
		log.Printf("gram: %.1f FPS at %dx%d: a frame's tick %v (publishing %v, awaited %v), "+
			"barrier %v, drawing %v, presenting %v, waiting %v", rate, e.window.Bounds().Dx(),
			e.window.Bounds().Dy(), us(e.spent.update), us(e.spent.publish), us(e.spent.awaited),
			us(e.spent.barrier), us(e.spent.draw), us(e.spent.present), us(e.spent.wait))
	}
	e.frames, e.counted, e.spent = 0, time.Now(), spent{}
}

var logRates = os.Getenv("GRAM_FPS_LOG") != ""

// barrier is the frame's step where nothing ticks and nothing draws: what was published swapped
// in to be drawn, the frame's input with input (on the layout last drawn), the last frame's times
// paced, the scenes shown since published, a stage entered readied before its first frame, and
// the scenes shown laid out over a screen w by h (their cameras copied for drawing), reporting
// whether the stage was readied; Termination once the game has quit
func (e *Engine) barrier(w, h int, input bool) (readied bool, err error) {
	e.current.swap()
	if input {
		if err := e.input(); err != nil {
			return false, err
		}
	}
	e.order = append(e.order[:0], e.current.stage.Stack().Composition().Order()...)
	e.pace()
	if e.pendingSwitch != "" {
		return false, nil
	}
	e.current.publishShown(e.order)
	if readied = !e.current.prepared; readied {
		e.prepare()
	}
	for _, name := range e.order {
		for _, d := range e.current.sceneDrawers[name] {
			if l, ok := d.(game.Laying); ok {
				l.Lay(w, h)
			}
		}
	}
	return readied, nil
}

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

type spent struct{ update, publish, awaited, barrier, draw, present, wait time.Duration }

// Frame runs a frame's Update and hands the renderer what it draws: every Publisher of the stage
// and of the scenes shown publishes, then swaps; not while a stage switch is pending. It returns
// Termination once the game has quit.
func (e *Engine) Frame() error {
	if err := e.Update(); err != nil {
		return err
	}
	e.publish()
	e.current.swap()
	return nil
}

// Update runs a frame's input, then its ticks. It returns Termination once the game has quit.
func (e *Engine) Update() error {
	if err := e.input(); err != nil {
		return err
	}
	e.tick()
	return nil
}

// input takes the frame's input: a switch of stage asked for, the events handed to the active
// scene, the engine's own commands (save, load, quit). It returns Termination once the game has
// quit.
func (e *Engine) input() error {
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
	return nil
}

// tick steps the world as many times as the time gone says, none while it stands
func (e *Engine) tick() {
	if e.current.host.ecs.Paused() {
		return
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
}

// publish has the stage's Publishers read what is drawn next; not while a stage switch is pending
func (e *Engine) publish() {
	if e.pendingSwitch != "" {
		return
	}
	began := time.Now()
	e.current.publish(e.current.stage.Stack().Composition().Order())
	e.spent.publish += time.Since(began)
}

func (e *Engine) takeLoad() *string {
	slot := e.pendingLoad
	e.pendingLoad = nil
	return slot
}

// a frame further behind drops the rest: the game slows instead of spiralling
const maxStepsAFrame = 5

// Draw draws the active stage's scenes shown as the barrier found them, bottom to top, onto
// screen.
func (e *Engine) Draw(screen *render.Image) {
	if e.pendingSwitch != "" {
		e.transitionOverlay.Draw(screen)
		return
	}
	stack := e.current.stage.Stack()
	for _, name := range e.order {
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
