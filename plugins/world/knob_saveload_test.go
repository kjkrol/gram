package world_test

import (
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/ceremony/effect"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/game/stage"
	"github.com/kjkrol/gram/internal/engine"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

type tint struct{ V int32 }

func (tint) Knob() {}

var _ effect.Knob = tint{}

// a post tinted 1 by its kind, an effect "dyed" tinting it 5 for a second
type dyedPost struct {
	w    *world.Plugin
	ecs  *goke.ECS
	dyed effect.Effect
	tint goke.Comp[tint]
	all  *goke.Query
}

func (d *dyedPost) SetupSystems() []goke.System {
	return []goke.System{goke.SystemFn{OnInit: func(si *goke.SysInit) {
		d.all = si.NewQueryBuilder(&d.tint).Build()
	}}}
}

func (d *dyedPost) post() (uid.UID64, int32) {
	for d.all.All(); d.all.Next(); {
		cur := d.all.Cursor()
		return cur.IDs[0], d.tint.Slice(cur)[0].V
	}
	return 0, 0
}

func (d *dyedPost) run(steps int) {
	for range steps {
		d.ecs.Tick(time.Second / 60)
	}
}

func newDyedPost() (*engine.Engine, *dyedPost) {
	d := &dyedPost{}
	st := stage.New("dye").
		World(testWorldConfig()).
		Plugins(func(w *world.Plugin) { d.w = w }).
		Effects(func(ctx game.Initializer) {
			d.ecs = ctx.ECS()
			ctx.Setup(d)
			d.w.Write().Effects().Define("dyed", effect.Spec{
				effect.Lasts(time.Second),
				effect.Alter(func(t *tint) { t.V = 5 })})
			d.dyed = d.w.Write().Effects().Named("dyed")
		}).
		Kinds(func() {
			kind.Define[struct{}](d.w.Kinds(), "post", kind.Spec{
				comp.Const(world.Position{AABB: plane.NewAABB(geom.NewVec(100, 100), 10, 10)}),
				comp.Const(world.Velocity{}),
				comp.Const(tint{V: 1}),
			})
		}).
		Spawn(func() error {
			d.w.Write().Seed(kind.Named[struct{}](d.w.Kinds(), "post").Entry(struct{}{}))
			return nil
		}).
		Update(func(ctx goke.RunCtx, dt time.Duration) { d.w.RunPlan(ctx, dt) })
	return engine.NewEngine(oneStageGame{stage: st}), d
}

func TestKnob_AnEffectsTurnOutlastsASaveAndEndsAfterTheLoad(t *testing.T) {
	t.Chdir(t.TempDir())
	eng, was := newDyedPost()
	if err := eng.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	was.run(2)
	id, _ := was.post()
	was.w.Write().Effects().Cast(id, was.dyed)
	was.run(30)
	if _, v := was.post(); v != 5 {
		t.Fatalf("the post is tinted %d under the effect, want 5", v)
	}
	if err := eng.Save(""); err != nil {
		t.Fatalf("Save: %v", err)
	}
	back, now := newDyedPost()
	if err := back.Load("", ""); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := back.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	now.run(1)
	if _, v := now.post(); v != 5 {
		t.Errorf("read back from a save under the effect the post is tinted %d, want 5", v)
	}
	now.run(60)
	if _, v := now.post(); v != 1 {
		t.Errorf("the effect over after the load, the post is tinted %d, want its kind's 1", v)
	}
}
