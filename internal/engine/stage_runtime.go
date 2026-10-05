package engine

import (
	"fmt"
	"math"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

// stageRuntime is one active Stage: its ecsHost, its world.Plugin if it installed one,
// every Scene's layers, built once at entry and keyed by Scene.Name(), and the images its
// viewports are drawn into.
type stageRuntime struct {
	host        *ecsHost
	stage       game.Stage
	world       *world.Plugin
	sceneLayers map[string][]render.Layer
	views       map[viewKey]*render.Image
	passes      []pass // reused frame to frame
}

// viewKey names one viewport of one scene.
type viewKey struct {
	scene string
	index int
}

// enterStage builds a Stage from scratch: a fresh ECS, Init, Restore or Spawn, renderers, Setup.
func (e *Engine) enterStage(stage game.Stage) (*stageRuntime, error) {
	host := newECSHost()

	w, h := e.screen()
	ctx := &initializer{host: host, tps: e.tps, screenWidth: w, screenHeight: h}
	if err := stage.Init(ctx); err != nil {
		return nil, err
	}
	if err := ctx.hookPlayed(); err != nil {
		return nil, err
	}

	restored, err := stage.Restore(&persistence{host: host})
	if err != nil {
		return nil, err
	}
	if !restored {
		if err := stage.Spawn(); err != nil {
			return nil, err
		}
		if err := host.runPopulate(); err != nil {
			return nil, err
		}
	}

	// the game's Update lays the tick out; the world's clock then replays what simulates as many
	// times as its tempo says, none in the tactical pause
	if ctx.world != nil {
		clk := ctx.world.Clock()
		host.ecs.SetPlan(func(rc goke.RunCtx, d time.Duration) {
			stage.Update(rc, d)
			clk.Replay(rc, d)
		})
	} else {
		host.ecs.SetPlan(stage.Update)
	}

	sceneLayers := make(map[string][]render.Layer)
	for _, sc := range stage.Stack().All() {
		layers := sc.Layers()
		if err := checkLayers(sc, layers); err != nil {
			return nil, err
		}
		for _, l := range layers {
			host.registerLayer(l)
		}
		sceneLayers[sc.Name()] = layers
	}

	host.flushPendingSetup()

	return &stageRuntime{host: host, stage: stage, world: ctx.world, sceneLayers: sceneLayers,
		views: map[viewKey]*render.Image{}}, nil
}

// checkLayers refuses a layer that is neither a Renderer nor a WorldRenderer, or both, and world
// layers in a scene with no viewports to draw them through.
func checkLayers(sc game.Scene, layers []render.Layer) error {
	world := false
	for _, l := range layers {
		_, screen := l.(render.Renderer)
		_, shows := l.(render.WorldRenderer)
		switch {
		case screen && shows:
			return fmt.Errorf("gram: scene %q: %T is both a Renderer and a WorldRenderer", sc.Name(), l)
		case !screen && !shows:
			if _, ok := l.(render.Source); ok {
				return fmt.Errorf("gram: scene %q: %T is a render.Source; list it in a render.NewComposer", sc.Name(), l)
			}
			return fmt.Errorf("gram: scene %q: %T is neither a Renderer nor a WorldRenderer", sc.Name(), l)
		}
		world = world || shows
	}
	if _, ok := sc.(game.Viewer); world && !ok {
		return fmt.Errorf("gram: scene %q has world layers but no Viewports; make it a game.Viewer", sc.Name())
	}
	return nil
}

// drawScene draws sc's layers bottom to top: a Renderer on the screen, a run of WorldRenderers
// through each of the scene's viewports.
func (r *stageRuntime) drawScene(screen *render.Image, sc game.Scene) {
	var viewports []render.Viewport
	r.passes = passes(r.passes[:0], r.sceneLayers[sc.Name()], func() []render.Viewport {
		if viewports == nil {
			viewports = sc.(game.Viewer).Viewports(screenBox(screen))
		}
		return viewports
	})
	for _, p := range r.passes {
		if p.screen != nil {
			p.screen.Draw(screen)
			continue
		}
		r.drawViewport(screen, viewKey{sc.Name(), p.index}, p.viewport, p.world)
	}
}

// pass is one step of drawing a scene: a screen layer, or a run of world layers through the
// index-th viewport.
type pass struct {
	screen   render.Renderer
	world    []render.Layer
	viewport render.Viewport
	index    int
}

// passes appends to dst the steps drawing layers bottom to top, asking for the viewports only
// when a world layer comes.
func passes(dst []pass, layers []render.Layer, viewports func() []render.Viewport) []pass {
	for i := 0; i < len(layers); {
		if l, ok := layers[i].(render.Renderer); ok {
			dst = append(dst, pass{screen: l})
			i++
			continue
		}
		j := i
		for j < len(layers) {
			if _, ok := layers[j].(render.WorldRenderer); !ok {
				break
			}
			j++
		}
		for k, vp := range viewports() {
			dst = append(dst, pass{world: layers[i:j], viewport: vp, index: k})
		}
		i = j
	}
	return dst
}

// drawViewport draws the world layers through vp's camera: straight onto a screen it covers,
// otherwise onto an image of its area laid over the screen there.
func (r *stageRuntime) drawViewport(screen *render.Image, key viewKey, vp render.Viewport, layers []render.Layer) {
	w, h := pixels(vp.Area)
	if w <= 0 || h <= 0 {
		return
	}
	fit(vp)
	if vp.Area == screenBox(screen) {
		for _, l := range layers {
			l.(render.WorldRenderer).DrawWorld(screen, vp.Camera)
		}
		return
	}
	img := r.views[key]
	if img == nil || img.Bounds().Dx() != w || img.Bounds().Dy() != h {
		if img != nil {
			img.Deallocate()
		}
		img = render.NewImage(w, h)
		r.views[key] = img
	}
	img.Clear()
	for _, l := range layers {
		l.(render.WorldRenderer).DrawWorld(img, vp.Camera)
	}
	op := &render.DrawImageOptions{}
	op.GeoM.Translate(math.Round(vp.Area.TopLeft.X), math.Round(vp.Area.TopLeft.Y))
	screen.DrawImage(img, op)
}
