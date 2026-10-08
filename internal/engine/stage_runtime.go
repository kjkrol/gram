package engine

import (
	"fmt"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

// stageRuntime is one active Stage: its ecsHost, its world.Plugin if it installed one, and every
// Scene's layers, built once at entry and keyed by Scene.Name().
type stageRuntime struct {
	host        *ecsHost
	stage       game.Stage
	world       *world.Plugin
	sceneLayers map[string][]render.Layer
}

// enterStage builds a Stage from scratch: a fresh ECS, Init, Restore or Spawn, renderers, Setup.
func (e *Engine) enterStage(stage game.Stage) (*stageRuntime, error) {
	host := newECSHost()

	w, h := e.screen()
	ctx := &initializer{host: host, tps: e.tps, screenWidth: w, screenHeight: h}
	if err := stage.Init(ctx); err != nil {
		return nil, err
	}
	if err := ctx.deliver(); err != nil {
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

	return &stageRuntime{host: host, stage: stage, world: ctx.world, sceneLayers: sceneLayers}, nil
}

// checkLayers refuses a layer that is no Renderer: a scene's layers are drawn on the screen, the
// world among them through a ui.Image of a render.Feed.
func checkLayers(sc game.Scene, layers []render.Layer) error {
	for _, l := range layers {
		if _, ok := l.(render.Renderer); ok {
			continue
		}
		if _, ok := l.(render.Picture); ok {
			return fmt.Errorf("gram: scene %q: %T is a world layer; show it through a ui.Image(render.NewFeed(camera, it))", sc.Name(), l)
		}
		if _, ok := l.(render.Source); ok {
			return fmt.Errorf("gram: scene %q: %T is a render.Source; list it in a render.NewComposer", sc.Name(), l)
		}
		return fmt.Errorf("gram: scene %q: %T is no Renderer", sc.Name(), l)
	}
	return nil
}

// drawScene draws sc's layers on the screen, bottom to top.
func (r *stageRuntime) drawScene(screen *render.Image, sc game.Scene) {
	for _, l := range r.sceneLayers[sc.Name()] {
		l.(render.Renderer).Draw(screen)
	}
}
