package steering

import (
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity"
	"github.com/kjkrol/uid"
)

func commanded(t *testing.T, give func(s *System, steered, other uid.UID64)) geom.Vec {
	t.Helper()
	s := NewSystem()
	var base goke.Comp[entity.Base]
	var steer goke.Comp[Steering]
	var course goke.Comp[Course]
	var steered, other uid.UID64
	at := func(x, y float64) entity.Position {
		return entity.Position{AABB: plane.NewAABB(geom.NewVec(x-1, y-1), 2, 2)}
	}
	var dir geom.Vec
	goke.New().Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		f := si.NewFactory(&base, &steer, &course)
		f.Create(1)
		for f.Next() {
			steered = f.Cursor.IDs[0]
			base.Slice(&f.Cursor)[0] = entity.Base{Pos: at(10, 10), Vel: entity.Velocity{Dir: geom.NewVec(1, 0)}}
		}
		g := si.NewFactory(&base)
		g.Create(1)
		for g.Next() {
			other = g.Cursor.IDs[0]
			base.Slice(&g.Cursor)[0] = entity.Base{Pos: at(20, 10)}
		}
		s.Init(si)
		give(s, steered, other)
		s.Update(nil, time.Second/60)
		q := si.NewQueryBuilder(&base, &steer).Build()
		for q.All(); q.Next(); {
			dir = base.Slice(q.Cursor())[0].Vel.Dir
		}
	}})
	return dir
}

func put(s *System, id uid.UID64, cmd control.Command) {
	for _, q := range s.Queues() {
		if q.Accepts() == reflect.TypeOf(cmd) {
			q.PutFrom(id, cmd)
			return
		}
	}
}

func near(a, b geom.Vec) bool { return math.Abs(a.X-b.X) < 1e-9 && math.Abs(a.Y-b.Y) < 1e-9 }

func TestCommands_HeadTheEntity(t *testing.T) {
	for name, tc := range map[string]struct {
		give func(steered, other uid.UID64) control.Command
		want geom.Vec
	}{
		"away":   {func(_, o uid.UID64) control.Command { return Away{From: o} }, geom.NewVec(-1, 0)},
		"toward": {func(_, o uid.UID64) control.Command { return Toward{To: o} }, geom.NewVec(1, 0)},
		"turn":   {func(uid.UID64, uid.UID64) control.Command { return Turn{Angle: math.Pi / 2} }, geom.NewVec(0, 1)},
	} {
		got := commanded(t, func(s *System, steered, other uid.UID64) { put(s, steered, tc.give(steered, other)) })
		if !near(got, tc.want) {
			t.Errorf("%s: goes %v, want %v", name, got, tc.want)
		}
	}
}

func TestCommands_OneAStep(t *testing.T) {
	got := commanded(t, func(s *System, steered, other uid.UID64) {
		put(s, steered, Toward{To: other})
		put(s, steered, Away{From: other})
		// from itself: no way at all, and too late
		put(s, steered, Away{From: steered})
	})
	if !near(got, geom.NewVec(-1, 0)) {
		t.Errorf("goes %v, want away from the other", got)
	}
}

func TestCommands_OnlyAnEntitysOwnForItself(t *testing.T) {
	got := commanded(t, func(s *System, steered, other uid.UID64) {
		s.told.aways.Add(control.PlayerID(1), Away{From: other})
		put(s, other, Toward{To: steered})
	})
	if !near(got, geom.NewVec(1, 0)) {
		t.Errorf("goes %v, want on east as before", got)
	}
}
