package selection_test

import (
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/ceremony/tag"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/internal/parallel"
	"github.com/kjkrol/gram/plugins/players/owner"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

// whom each of six players has chosen, as one goroutine reads it
type choice struct {
	id [6]uid.UID64
	ok [6]bool
}

func chooses(r *selection.Read) choice {
	var c choice
	for by := range 6 {
		c.id[by], c.ok[by] = r.Chosen(command.Issuer(by + 1))
	}
	return c
}

func TestRead_TheWorkersReadWhomThePlayersChoseAtOnceAsTheGameDoes(t *testing.T) {
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 1000, Height: 1000},
		Entities: world.EntitiesCfg{MaxCount: 16, MinSize: 10, MaxSize: 10},
	})
	sel := selection.NewPlugin(w)
	tags := sel.Tags()
	kind.Define[unit](w.Kinds(), "unit", kind.Spec{
		comp.Load(func(u unit) world.Position {
			return world.Position{AABB: plane.NewAABB(geom.NewVec(u.x, 100), 10, 10)}
		}),
		comp.Const(world.Velocity{}),
		comp.Load(func(u unit) tag.Tags[selection.Family] {
			t := tag.Tags[selection.Family](0).With(tags.Selectable)
			if u.selected {
				t = t.With(tags.Selected)
			}
			return t
		}),
		comp.Load(func(u unit) tag.Tags[owner.Family] {
			return tag.Tags[owner.Family](0).With(owner.Of(u.by))
		}),
	})
	k := kind.Named[unit](w.Kinds(), "unit")
	// players 1–4 one unit selected of two, 5 two of two, 6 none of two
	for by := range 6 {
		for i := range 2 {
			selected := by < 4 && i == 0 || by == 4
			w.Write().Seed(k.Entry(unit{x: float64(20 + 150*by + 60*i),
				by: command.Issuer(by + 1), selected: selected}))
		}
	}
	if err := w.Populate(); err != nil {
		t.Fatal(err)
	}
	pool := parallel.New(4)
	ctx := &installCtx{ecs: goke.New(), pool: pool}
	if err := w.Install(ctx); err != nil {
		t.Fatal(err)
	}
	if err := sel.Install(ctx); err != nil {
		t.Fatal(err)
	}
	var systems []goke.System
	for _, produce := range ctx.pending {
		systems = append(systems, produce()...)
	}
	ctx.ecs.Setup(systems...)
	ctx.ecs.SetPlan(func(rc goke.RunCtx, d time.Duration) {
		sel.RunPlan(rc, d)
		w.RunPlan(rc, d)
		w.Write().Clock().Replay(rc, d)
	})
	ctx.ecs.Tick(time.Second / 10)
	want := chooses(sel.Read(0))
	if !want.ok[0] || want.ok[4] || want.ok[5] {
		t.Fatalf("the game's goroutine reads %+v: the selection proves nothing", want)
	}
	got := make([]choice, 2*pool.Size())
	b := pool.Begin(func(wk, i int) {
		for range 50 {
			got[i] = chooses(sel.Read(wk))
		}
	}, nil)
	b.Add(len(got))
	b.Wait()
	for i := range got {
		if got[i] != want {
			t.Fatalf("task %d read %+v, the game's goroutine %+v", i, got[i], want)
		}
	}
}
