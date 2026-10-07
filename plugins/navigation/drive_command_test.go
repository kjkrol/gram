package navigation

import (
	"math"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/steering"
	"github.com/kjkrol/uid"
)

// hand is what the driving tests read off a unit: its Driven, if it carries one, whether it is
// Driving, its speed and where it stands.
type hand struct {
	driven  *steering.Driven
	driving bool
	speed   float64
	at      geom.Vec
}

// hands reads every unit's hand by id, through a query of its own.
func hands(rw *roadWorld) func() map[uid.UID64]hand {
	var base goke.Comp[world.Base]
	var driven goke.OptComp[steering.Driven]
	var states goke.OptComp[tag.Tags[States]]
	var course goke.OptComp[steering.Course]
	var q *goke.Query
	rw.ecs.RegSys(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		q = si.NewQueryBuilder(&base).Optional(&driven).Optional(&states).Optional(&course).Build()
	}})
	return func() map[uid.UID64]hand {
		out := map[uid.UID64]hand{}
		for q.All(); q.Next(); {
			cur := q.Cursor()
			drivens, marks, courses := driven.Slice(cur), states.Slice(cur), course.Slice(cur)
			for i, id := range cur.IDs {
				h := hand{at: base.Slice(cur)[i].Pos.Center()}
				if drivens != nil {
					d := drivens[i]
					h.driven = &d
				}
				if marks != nil {
					h.driving = marks[i].Has(Driving)
				}
				if courses != nil {
					h.speed = courses[i].Speed
				}
				out[id] = h
			}
		}
		return out
	}
}

func (rw *roadWorld) drive(by control.PlayerID, cmd players.Drive) {
	if !rw.nav.worldPlugin.Carrier().Put(by, cmd) {
		rw.t.Fatal("the world carries no Drive")
	}
}

func (rw *roadWorld) tick(n int) {
	for range n {
		rw.ecs.Tick(time.Second / 60)
	}
}

// A Drive moves the selected units of the player who gave it and no one else's; the unit driven
// carries a Driven and is Driving.
func TestDrive_WalksTheSelectedUnitsOfThePlayerAlone(t *testing.T) {
	rw := newRoadWorld(t, 10, []roadUnit{{start: 0}})
	units := []roadUnit{
		{start: rw.at(2, 1), selected: true, owner: 1},
		{start: rw.at(6, 1), selected: true, owner: 2},
	}
	rw = newRoadWorld(t, 10, units)
	read := hands(rw)
	mine, theirs := rw.byRow[0], rw.byRow[1]
	before := read()
	for range 3 {
		rw.drive(1, players.Drive{Ahead: 1})
		rw.tick(1)
	}
	after := read()
	if after[mine].at == before[mine].at {
		t.Errorf("the player's unit stands at %v still, want it driven on", after[mine].at)
	}
	if after[theirs].at != before[theirs].at || after[theirs].driven != nil {
		t.Errorf("the other player's unit moved to %v or got a Driven %v, want it left alone", after[theirs].at, after[theirs].driven)
	}
	if h := after[mine]; h.driven == nil || h.driven.Ahead != 1 || !h.driving {
		t.Errorf("the unit driven carries %+v, Driving %v; want Ahead 1 and Driving", h.driven, h.driving)
	}
}

// Several Drives in one tick add up: on and turning at once.
func TestDrive_TwoKeysInOneTickAddUp(t *testing.T) {
	rw := newRoadWorld(t, 10, []roadUnit{{start: 0}})
	rw = newRoadWorld(t, 10, []roadUnit{{start: rw.at(2, 1), selected: true, owner: 1}})
	read := hands(rw)
	rw.drive(1, players.Drive{Ahead: 1})
	rw.drive(1, players.Drive{Turn: 1})
	rw.drive(1, players.Drive{Turn: 1})
	rw.tick(1)
	if h := read()[rw.byRow[0]]; h.driven == nil || *h.driven != (steering.Driven{Ahead: 1, Turn: 1}) {
		t.Errorf("the unit carries %+v, want Ahead 1 and Turn 1, each way held within one", h.driven)
	}
}

// A tick without a Drive brakes the unit; once it stands its Driven is taken off and it is not
// Driving any more.
func TestDrive_ATickWithoutADriveBrakesAndThenLetsGo(t *testing.T) {
	rw := newRoadWorld(t, 10, []roadUnit{{start: 0}})
	rw = newRoadWorld(t, 10, []roadUnit{{start: rw.at(2, 1), selected: true, owner: 1}})
	read := hands(rw)
	id := rw.byRow[0]
	for range 10 {
		rw.drive(1, players.Drive{Ahead: 1})
		rw.tick(1)
	}
	if h := read()[id]; h.speed == 0 {
		t.Fatal("after ten ticks driven the unit stands still")
	}
	rw.tick(1)
	if h := read()[id]; h.driven == nil || *h.driven != (steering.Driven{}) || !h.driving {
		t.Fatalf("a tick without a Drive: the unit carries %+v, Driving %v; want a zero Driven, still Driving", h.driven, h.driving)
	}
	rw.tick(30)
	if h := read()[id]; h.driven != nil || h.driving || h.speed != 0 {
		t.Errorf("thirty ticks later the unit carries %+v, Driving %v, speed %v; want let go, standing", h.driven, h.driving, h.speed)
	}
}

// A unit under an order drops it at the first Drive.
func TestDrive_AUnitWithAnOrderDropsIt(t *testing.T) {
	rw := newRoadWorld(t, 10, []roadUnit{{start: 0}})
	rw = newRoadWorld(t, 10, []roadUnit{{start: rw.at(2, 1), target: rw.at(8, 1), ordered: true, selected: true, owner: 1}})
	id := rw.byRow[0]
	rw.tick(2)
	if _, o := rw.state(id); o == nil {
		t.Fatal("the unit lost its order before any Drive")
	}
	rw.drive(1, players.Drive{Ahead: 1})
	rw.tick(2)
	if _, o := rw.state(id); o != nil {
		t.Errorf("the unit keeps its order %+v after a Drive, want it dropped", o)
	}
}

// A hand's Way drives the unit that way: it turns to it and walks on.
func TestDrive_AWayDrivesTheUnitThatWay(t *testing.T) {
	rw := newRoadWorld(t, 10, []roadUnit{{start: 0}})
	rw = newRoadWorld(t, 10, []roadUnit{{start: rw.at(2, 1), selected: true, owner: 1}})
	read := hands(rw)
	id := rw.byRow[0]
	before := read()[id].at
	for range 8 {
		rw.drive(1, players.Drive{Ahead: 1, Way: geom.NewVec(0, 1)})
		rw.tick(1)
	}
	after := read()[id]
	if after.at.Y <= before.Y+4 || math.Abs(after.at.X-before.X) > 4 {
		t.Errorf("driven south from %v the unit is at %v, want it gone down the screen", before, after.at)
	}
	if after.driven == nil || after.driven.Face != geom.NewVec(0, 1) {
		t.Errorf("the unit carries %+v, want its Face the way driven", after.driven)
	}
}

// A player's hand is on the unit its camera is fastened to, selected or not, and not on the
// units it has selected then.
func TestDrive_TheHandIsOnTheUnitTheCameraIsFastenedTo(t *testing.T) {
	rw := newRoadWorld(t, 10, []roadUnit{{start: 0}})
	rw = newRoadWorld(t, 10, []roadUnit{
		{start: rw.at(2, 1), selected: true, owner: 1},
		{start: rw.at(6, 1), owner: 1},
	})
	read := hands(rw)
	selected, fastened := rw.byRow[0], rw.byRow[1]
	rw.players.ByID(1).Camera.(camera.Fastenable).Fasten(camera.Fastening{Entity: fastened, How: camera.Centred})
	before := read()
	for range 3 {
		rw.drive(1, players.Drive{Ahead: 1})
		rw.tick(1)
	}
	after := read()
	if after[fastened].at == before[fastened].at || !after[fastened].driving {
		t.Errorf("the unit the camera is fastened to stands at %v still, Driving %v; want it driven on", after[fastened].at, after[fastened].driving)
	}
	if after[selected].at != before[selected].at || after[selected].driven != nil {
		t.Errorf("the selected unit moved to %v or got a Driven %v, want it left alone while the camera is fastened to another", after[selected].at, after[selected].driven)
	}
}

// A Driven navigation did not give — a camera's — is left as it is.
func TestDrive_AForeignDrivenIsLeftAlone(t *testing.T) {
	rw := newRoadWorld(t, 10, []roadUnit{{start: 0}})
	rw = newRoadWorld(t, 10, []roadUnit{{start: rw.at(2, 1), selected: true, owner: 1}})
	read := hands(rw)
	id := rw.byRow[0]
	var drivenID goke.CompID
	put := rw.ecs.RegSys(goke.SystemFn{
		OnInit:   func(si *goke.SysInit) { drivenID = si.RegComp[steering.Driven]() },
		OnUpdate: func(cb *goke.CmdBuf, _ time.Duration) { cb.AddOne(id, drivenID, steering.Driven{Turn: 1}) },
	})
	rw.ecs.SetPlan(func(rc goke.RunCtx, d time.Duration) {
		rc.Run(put, d)
		rc.Sync()
	})
	rw.tick(1)
	rw.ecs.SetPlan(func(rc goke.RunCtx, d time.Duration) {
		rw.nav.worldPlugin.RunPlan(rc, d)
		rw.nav.RunPlan(rc, d)
		rc.Sync()
		rw.nav.worldPlugin.Clock().Replay(rc, d)
	})
	rw.tick(3)
	if h := read()[id]; h.driven == nil || *h.driven != (steering.Driven{Turn: 1}) || h.driving {
		t.Errorf("the camera's Driven became %+v, Driving %v; want left as it was, not Driving", h.driven, h.driving)
	}
}
