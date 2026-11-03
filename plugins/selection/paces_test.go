package selection_test

import (
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/ceremony/tag"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/lod"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	"github.com/kjkrol/gram/plugins/players/owner"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/sector"
	"github.com/kjkrol/uid"
)

// the world's one observer, a camera from above looking at (x, y)
type eye struct{ cam camera.Camera }

var _ sector.Observers = eye{}

func lookingAt(w, h uint32, x, y float32) eye {
	cam := cameratest.NewFromSpace(w, h, 0)
	cam.SetViewport(2, 2)
	cam.CenterOn(x, y, 0)
	return eye{cam}
}

func (e eye) Observers(each func(cam camera.Camera)) { each(e.cam) }

func TestSelect_SelectsNoUnitAsleepAtL4(t *testing.T) {
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 1000, Height: 1000},
		Entities: world.EntitiesCfg{MaxCount: 4, MinSize: 10, MaxSize: 10},
		Sectors:  world.SectorsCfg{Buckets: 1},
		LOD:      lod.Ladder{Sight: 300},
	})
	w.Write().Observe(lookingAt(1000, 1000, 100, 100))
	sel := selection.NewPlugin(w)
	tags := sel.Tags()
	kind.Define[float32](w.Kinds(), "unit", kind.Spec{
		comp.Load(func(x float32) world.Position {
			return world.Position{AABB: plane.NewAABB(geom.NewVec(x, 100), 10, 10)}
		}),
		comp.Const(world.Velocity{}),
		comp.Const(tag.Tags[selection.Family](0).With(tags.Selectable)),
		comp.Const(tag.Tags[owner.Family](0).With(owner.Of(1))),
	})
	k := kind.Named[float32](w.Kinds(), "unit")
	w.Write().Seed(k.Entry(100), k.Entry(900))
	if err := w.Populate(); err != nil {
		t.Fatal(err)
	}
	ctx := &installCtx{ecs: goke.New()}
	if err := w.Install(ctx); err != nil {
		t.Fatal(err)
	}
	if err := sel.Install(ctx); err != nil {
		t.Fatal(err)
	}
	if err := w.Carry(sel); err != nil {
		t.Fatal(err)
	}
	var systems []goke.System
	for _, produce := range ctx.pending {
		systems = append(systems, produce()...)
	}
	var base goke.Comp[world.Base]
	var marks goke.Comp[tag.Tags[selection.Family]]
	var q *goke.Query
	systems = append(systems, goke.SystemFn{OnInit: func(si *goke.SysInit) {
		q = si.NewQueryBuilder(&base, &marks).Build()
	}})
	ctx.ecs.Setup(systems...)
	ctx.ecs.SetPlan(func(rc goke.RunCtx, d time.Duration) {
		sel.RunPlan(rc, d)
		w.RunPlan(rc, d)
		w.Write().Clock().Replay(rc, d)
		rc.Sync()
	})
	ctx.ecs.Tick(time.Second / 10)
	byX := map[float32]uid.UID64{}
	selected := map[float32]bool{}
	read := func() {
		for q.All(); q.Next(); {
			cur := q.Cursor()
			for i, id := range cur.IDs {
				x := base.Slice(cur)[i].Pos.TopLeft.X
				byX[x], selected[x] = id, marks.Slice(cur)[i].Has(tags.Selected)
			}
		}
	}
	read()
	w.Write().Carrier().Put(1, selection.Select{IDs: []uid.UID64{byX[100], byX[900]}})
	ctx.ecs.Tick(time.Second / 10)
	read()
	if !selected[100] || selected[900] {
		t.Errorf("selected at L0 %v, asleep %v; want only the first", selected[100], selected[900])
	}
}
