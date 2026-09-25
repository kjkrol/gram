package engine

import (
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

// stageRuntime is one active Stage: its ecsHost, its world.Plugin if it installed one,
// and every Scene's renderers, built once at entry and keyed by Scene.Name().
type stageRuntime struct {
	host        *ecsHost
	stage       game.Stage
	world       *world.Plugin
	sceneLayers map[string][]render.Renderer
}

// enterStage builds a Stage from scratch: a fresh ECS, Init, Restore or Spawn, renderers, Setup.
func (e *Engine) enterStage(stage game.Stage) (*stageRuntime, error) {
	host := newECSHost()

	w, h := e.screen()
	ctx := &initializer{host: host, tps: e.tps, screenWidth: w, screenHeight: h}
	if err := stage.Init(ctx); err != nil {
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

	host.ecs.SetPlan(stage.Update)

	sceneLayers := make(map[string][]render.Renderer)
	for _, sc := range stage.Stack().All() {
		var layers []render.Renderer
		for _, r := range sc.Layers() {
			layers = append(layers, host.registerRenderer(r))
		}
		sceneLayers[sc.Name()] = layers
	}

	host.flushPendingSetup()

	return &stageRuntime{host: host, stage: stage, world: ctx.world, sceneLayers: sceneLayers}, nil
}
