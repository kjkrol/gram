package collision_test

import (
	"bytes"
	"fmt"
	"log"
	"strings"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/collision/internal/collisiontest"
	"github.com/kjkrol/gram/plugins/motion"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

type slider struct{ x, speed float64 }

func colliding(t *testing.T, elastic bool, ticks int, with func(*collision.Plugin)) (left, right uid.UID64) {
	t.Helper()
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 1000, Height: 1000},
		Entities: world.EntitiesCfg{MaxCount: 2, MinSize: 10, MaxSize: 10},
	})
	m := motion.NewPlugin(w)
	c := collision.NewPlugin(w, m)
	with(c)
	spec := kind.Spec{
		comp.Load(func(b slider) world.Position {
			return world.Position{AABB: plane.NewAABB(geom.NewVec(b.x, 100), 10, 10)}
		}),
		comp.Load(func(b slider) world.Velocity {
			var v world.Velocity
			v.SetDelta(geom.NewVec(b.speed, 0))
			return v
		}),
		comp.Const(collision.Collider{}),
	}
	rows := []slider{{x: 100}, {x: 105}}
	if elastic {
		spec = append(spec, comp.Const(collision.Physics{Restitution: 1}))
		rows = []slider{{x: 100, speed: 5}, {x: 105, speed: -5}}
	}
	kind.Define[slider](w.Kinds(), "slider", spec)
	k := kind.Named[slider](w.Kinds(), "slider")
	for _, r := range rows {
		w.Write().Seed(k.Entry(r))
	}
	if err := w.Populate(); err != nil {
		t.Fatalf("Populate: %v", err)
	}
	ecs := collisiontest.Start(t, w, m, c, goke.SystemFn{OnInit: func(si *goke.SysInit) {
		var base goke.Comp[world.Base]
		q := si.NewQueryBuilder(&base).Build()
		for q.All(); q.Next(); {
			cur := q.Cursor()
			for i, id := range cur.IDs {
				if base.Slice(cur)[i].Pos.TopLeft.X < 102 {
					left = id
				} else {
					right = id
				}
			}
		}
	}})
	for range ticks {
		ecs.Tick(time.Second / 60)
	}
	return left, right
}

func TestStats_CountsEachContactOnce(t *testing.T) {
	var s collision.ContactStats
	colliding(t, false, 1, func(c *collision.Plugin) { c.WithStats(&s) })
	if s.Counter != 1 {
		t.Errorf("Counter = %d, want 1 for one pair in contact", s.Counter)
	}
}

func TestStats_AccumulateAcrossTicks(t *testing.T) {
	s := collision.ContactStats{Counter: 5}
	colliding(t, false, 3, func(c *collision.Plugin) { c.WithStats(&s) })
	if s.Counter != 8 {
		t.Errorf("Counter = %d, want 8 — three more ticks in contact on top of the 5 it started with", s.Counter)
	}
}

func TestLog_WritesEachContactOnce(t *testing.T) {
	var out bytes.Buffer
	idA, idB := colliding(t, true, 1, func(c *collision.Plugin) { c.WithLog(log.New(&out, "", 0)) })

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("logged %d lines, want 1 for one contact: %q", len(lines), out.String())
	}
	for _, want := range []string{fmt.Sprint(idA), fmt.Sprint(idB), "10.00"} {
		if !strings.Contains(lines[0], want) {
			t.Errorf("line %q is missing %q", lines[0], want)
		}
	}
}
