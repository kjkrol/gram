package world

import (
	"errors"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/world/entity/kind/comp"
)

// behaviorTag marks the one kind TestBehavior_Include applies to.
type behaviorTag struct{}

// driveVelocity is a Behavior that sets every entity it visits moving, counts
// them, and notes the order it ran in.
type driveVelocity struct {
	tagged bool
	log    *[]string
	name   string

	visited int
	query   *goke.Query
	base    goke.Comp[Base]
}

func (b *driveVelocity) Init(si *goke.SysInit) {
	qb := si.NewQueryBuilder(&b.base)
	if b.tagged {
		qb.Include(goke.Include[behaviorTag]())
	}
	b.query = qb.Build()
}

func (b *driveVelocity) Update(*goke.CmdBuf, time.Duration) {
	if b.log != nil {
		*b.log = append(*b.log, b.name)
	}
	b.query.All()
	for b.query.Next() {
		cursor := b.query.Cursor()
		bases := b.base.Slice(cursor)
		for i := range cursor.IDs {
			bases[i].Vel.Dir = geom.NewVec(1.0, 0.0)
			bases[i].Vel.Value = 600
			b.visited++
		}
	}
}

func spawnAt(wm *module, x float64, extras ...comp.Comp) {
	pos := Position{AABB: plane.NewAABB(geom.NewVec(x, 100), 10, 10)}
	wm.populate(testKind(pos, Velocity{}, extras...), []any{nil})
}

// tickWorld runs one full world tick through the module's own RunPlan.
func tickWorld(t *testing.T, wm *module) []float64 {
	t.Helper()

	var base goke.Comp[Base]
	var query *goke.Query
	ecs := goke.New()
	ecs.Setup(append(wm.SetupSystems(), goke.SystemFn{OnInit: func(si *goke.SysInit) {
		query = si.NewQueryBuilder(&base).Build()
	}})...)
	wm.RegSystems(ecs)
	ecs.SetPlan(func(rc goke.RunCtx, d time.Duration) { wm.RunPlan(rc, d); wm.clock.Replay(rc, d) })
	ecs.Tick(time.Second / 10)

	var xs []float64
	query.All()
	for query.Next() {
		cursor := query.Cursor()
		bases := base.Slice(cursor)
		for i := range cursor.IDs {
			xs = append(xs, bases[i].Pos.TopLeft.X)
		}
	}
	return xs
}

func TestBehavior_RunsBeforeMovement(t *testing.T) {
	p := testPlugin()
	p.Hook(&driveVelocity{})
	wm := p.module
	spawnAt(wm, 100)

	xs := tickWorld(t, wm)
	if len(xs) != 1 {
		t.Fatalf("found %d entities, want 1", len(xs))
	}
	if xs[0] <= 100 {
		t.Errorf("entity sits at x=%v after a tick, want it moved — the behavior ran too late to be integrated", xs[0])
	}
}

func TestBehavior_RunsInRegistrationOrder(t *testing.T) {
	var log []string
	wm := testWorld()
	wm.Hook(&driveVelocity{log: &log, name: "first"})
	wm.Hook(&driveVelocity{log: &log, name: "second"})
	spawnAt(wm, 100)

	tickWorld(t, wm)
	if len(log) != 2 || log[0] != "first" || log[1] != "second" {
		t.Errorf("behaviors ran as %v, want [first second]", log)
	}
}

func TestBehavior_IncludeVisitsOnlyTaggedEntities(t *testing.T) {
	b := &driveVelocity{tagged: true}
	wm := testWorld()
	wm.Hook(b)
	spawnAt(wm, 100)
	spawnAt(wm, 300, comp.Const(behaviorTag{}))

	tickWorld(t, wm)
	if b.visited != 1 {
		t.Errorf("behavior visited %d entities, want only the tagged one", b.visited)
	}
}

// A world nobody registered a behavior with must tick exactly as before.
func TestBehavior_NoneRegisteredLeavesTheTickUnchanged(t *testing.T) {
	wm := testWorld()
	spawnAt(wm, 100)

	if xs := tickWorld(t, wm); len(xs) != 1 || xs[0] != 100 {
		t.Errorf("positions = %v, want the entity still at 100", xs)
	}
}

func TestHook_RefusesWhatIsNotASystem(t *testing.T) {
	p := testPlugin()

	if err := p.Hook(&driveVelocity{}); err != nil {
		t.Errorf("Hook(a system) = %v, want nil", err)
	}
	if err := p.Hook(struct{}{}); !errors.Is(err, plugin.ErrUnhosted) {
		t.Errorf("Hook(not a system) = %v, want ErrUnhosted", err)
	}
}
