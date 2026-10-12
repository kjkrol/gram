package engine

import (
	"fmt"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

type stageRuntime struct {
	host        *ecsHost
	stage       game.Stage
	world       *world.Plugin
	carrier     *control.Carrier
	sceneLayers map[string][]render.Layer
}

func (e *Engine) enterStage(stage game.Stage, load *string) (*stageRuntime, error) {
	host := newECSHost()

	ctx := &initializer{host: host, tps: e.tps, handlers: []plugin.CommandHandler{&e.commands}}
	if err := stage.Init(ctx); err != nil {
		return nil, err
	}
	if err := ctx.deliver(); err != nil {
		return nil, err
	}
	carrier, err := ctx.carrier()
	if err != nil {
		return nil, err
	}

	if load != nil {
		if err := (&persistence{host: host}).Load(e.savePath(stage.Name()), *load); err != nil {
			return nil, err
		}
		if err := stage.Loaded(); err != nil {
			return nil, err
		}
	} else {
		if err := stage.Spawn(); err != nil {
			return nil, err
		}
		if err := host.runPopulate(); err != nil {
			return nil, err
		}
	}

	// the clock replays what simulates as many times as its tempo says, none in the pause
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

	return &stageRuntime{host: host, stage: stage, world: ctx.world, carrier: carrier, sceneLayers: sceneLayers}, nil
}

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

func (r *stageRuntime) drawScene(screen *render.Image, sc game.Scene) {
	for _, l := range r.sceneLayers[sc.Name()] {
		l.(render.Renderer).Draw(screen)
	}
}
