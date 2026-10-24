package engine

import (
	"reflect"
	"slices"
	"time"

	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

type stageRuntime struct {
	host         *ecsHost
	stage        game.Stage
	world        *world.Plugin
	carrier      *command.Carrier
	sceneDrawers map[string][]render.Renderer
	publishers   []plugin.Publisher            // the tracked values' and the sources'
	scenes       map[string][]plugin.Publisher // each scene's drawers'
	publishing   []plugin.Publisher            // those of a frame
	read         []string                      // the scenes whose drawers the frame's publish read
	unswapped    bool                          // the frame's publish not swapped in yet
	pacers       []plugin.Paced                // the tracked values paced on the barrier
	prepared     bool                          // its first frame readied (Engine.prepare)
	firsts       int                           // its first frames logged
}

func (e *Engine) enterStage(stage game.Stage, load *string) (*stageRuntime, error) {
	host := newECSHost()

	ctx := &initializer{host: host, tps: e.tps, pool: e.pool,
		handlers: []plugin.CommandHandler{&e.commands}}
	if err := stage.Init(ctx); err != nil {
		return nil, err
	}
	if err := ctx.deliver(); err != nil {
		return nil, err
	}
	ctx.look()
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

	// the clock replays what simulates as many times as its tempo says, none in the pause, and
	// settles what reads goke's memory after every Sync of the Stage's own Update too
	if ctx.world != nil {
		clk := ctx.world.Write().Clock()
		host.ecs.SetPlan(func(rc goke.RunCtx, d time.Duration) {
			stage.Update(clk.Settling(rc), d)
			clk.Replay(rc, d)
		})
	} else {
		host.ecs.SetPlan(stage.Update)
	}

	for _, s := range ctx.draws {
		if b, ok := s.(plugin.Bound); ok {
			host.registerBound(b)
		}
	}
	sceneDrawers := make(map[string][]render.Renderer)
	scenes := make(map[string][]plugin.Publisher)
	for _, sc := range stage.Stack().All() {
		drawers := sc.Drawers()
		var drawn []any
		for _, l := range drawers {
			if b, ok := l.(plugin.Bound); ok {
				host.registerBound(b)
			}
			drawn = append(drawn, l)
		}
		sceneDrawers[sc.Name()] = drawers
		scenes[sc.Name()] = publishersOf(nil, nil, drawn)
	}

	host.flushPendingSetup()

	return &stageRuntime{
		host:         host,
		stage:        stage,
		world:        ctx.world,
		carrier:      carrier,
		sceneDrawers: sceneDrawers,
		publishers:   publishersOf(host.tracked, ctx.draws, nil),
		scenes:       scenes,
		pacers:       pacersOf(host.tracked),
	}, nil
}

// publishersOf are the Publishers among the tracked values, the sources drawn and the scenes'
// drawers, each once, in that order
func publishersOf(tracked []any, sources []render.Source, drawers []any) []plugin.Publisher {
	all := append([]any(nil), tracked...)
	for _, s := range sources {
		all = append(all, s)
	}
	all = append(all, drawers...)
	var out []plugin.Publisher
	seen := map[any]bool{}
	for _, v := range all {
		p, ok := v.(plugin.Publisher)
		if !ok {
			continue
		}
		if reflect.TypeOf(v).Comparable() {
			if seen[v] {
				continue
			}
			seen[v] = true
		}
		out = append(out, p)
	}
	return out
}

// publish has every Publisher of the stage and of the scenes shown publish: a hidden scene's
// drawers read nothing
func (r *stageRuntime) publish(shown []string) {
	now := append(r.publishing[:0], r.publishers...)
	for _, name := range shown {
		for _, p := range r.scenes[name] {
			if !holds(now, p) {
				now = append(now, p)
			}
		}
	}
	r.publishing, r.read, r.unswapped = now, append(r.read[:0], shown...), true
	for _, p := range now {
		p.Publish()
	}
}

// swap has what the last publish read drawn; nothing without one
func (r *stageRuntime) swap() {
	if !r.unswapped {
		return
	}
	r.unswapped = false
	for _, p := range r.publishing {
		p.Swap()
	}
}

// publishShown publishes and swaps at once, on the barrier, the drawers of the scenes shown that
// the frame's publish did not read: none is drawn unread
func (r *stageRuntime) publishShown(shown []string) {
	for _, name := range shown {
		if slices.Contains(r.read, name) {
			continue
		}
		r.read = append(r.read, name)
		for _, p := range r.scenes[name] {
			if holds(r.publishing, p) {
				continue
			}
			p.Publish()
			p.Swap()
			r.publishing = append(r.publishing, p)
		}
	}
}

// pacersOf are the Paced among the tracked values, in their order
func pacersOf(tracked []any) []plugin.Paced {
	var out []plugin.Paced
	for _, v := range tracked {
		if p, ok := v.(plugin.Paced); ok {
			out = append(out, p)
		}
	}
	return out
}

// holds reports whether ps holds p itself
func holds(ps []plugin.Publisher, p plugin.Publisher) bool {
	if !reflect.TypeOf(p).Comparable() {
		return false
	}
	for _, q := range ps {
		if q == p {
			return true
		}
	}
	return false
}

func (r *stageRuntime) drawScene(screen *render.Image, sc game.Scene) {
	for _, l := range r.sceneDrawers[sc.Name()] {
		l.Draw(screen)
	}
}
