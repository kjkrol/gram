package navigation_test

import (
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/navigation"
)

func TestMoveTo_RoundTrip(t *testing.T) {
	path := t.TempDir() + "/save.bin"

	want := navigation.MoveOrder{Target: board.CellID(7), Path: navigation.Path{Length: 3, Index: 1}, Queued: 2}
	want.Waypoints[0], want.Waypoints[1] = navigation.Goal{Cell: 9}, navigation.Goal{Cell: 11, Spot: geom.NewVec(3.5, 7.25), At: geom.NewVec(4, 7)}
	want.Spot, want.At, want.Struck, want.Hit, want.Mets = geom.NewVec(1.5, 2.5), geom.NewVec(1, 2), geom.NewVec(0, -1), 42, 1
	want.Met[0], want.Avoid[0], want.Avoids, want.AsideFor = 42, 13, 1, 250*time.Millisecond
	want.Path.Steps[0] = board.CellID(10)
	want.Path.Steps[1] = board.CellID(11)
	want.Path.Steps[2] = board.CellID(12)

	ecs := goke.New()
	var moveToComp goke.Comp[navigation.MoveOrder]
	ecs.Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		f := si.NewFactory(&moveToComp)
		f.Create(1)
		f.Next()
		moveToComp.Slice(&f.Cursor)[0] = want
	}})

	ecs.Pause()
	if err := ecs.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	ecs2 := goke.New()
	if err := ecs2.Load(path, goke.LoadComp[navigation.MoveOrder]()); err != nil {
		t.Fatalf("Load: %v", err)
	}
	var moveToComp2 goke.Comp[navigation.MoveOrder]
	var q *goke.Query
	ecs2.Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		q = si.NewQueryBuilder(&moveToComp2).Build()
	}})
	q.All()
	found := false
	for q.Next() {
		got := moveToComp2.Slice(q.Cursor())
		for i := range got {
			found = true
			if got[i] != want {
				t.Errorf("MoveOrder = %+v, want %+v", got[i], want)
			}
		}
	}
	if !found {
		t.Fatal("no entity found after Load")
	}
}
