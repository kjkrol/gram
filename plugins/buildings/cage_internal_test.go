package buildings

import (
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/world"
)

// caged is the plugin's cage over a board of 8 x 8 cells of 32 holding a house with a room on two
// cells, from (96, 96) to (160, 128), its floor 10 up where the ground rises from 4 on its north
// side to 10 on its south, its door in the south wall round x 112, under a hipped roof rising 8
// from its eaves at 22; and a solid house without one from (32, 192) to (96, 224) on level ground
// at 10, its eaves at 30 under a gable rising 16
func caged(t *testing.T) *Plugin {
	t.Helper()
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 256, Height: 256},
		Entities: world.EntitiesCfg{MaxCount: 8, MinSize: 1, MaxSize: 64},
		Heights:  true,
	})
	brd := board.NewPlugin(grid.DefaultGrids{}.Square(8, 8, 32), &cell.MultipleOccupancy{}, w)
	p := NewPlugin(w, brd)
	room := Spec{Storey: 12, Floors: 1, Door: cell.Gap{Width: 4, Height: 8},
		Parts: []Part{{Roof: Hip, Pitch: 0.5}}}
	at := room.of(Row{Mid: [2]float32{128, 112}, Size: [2]float32{64, 32}})
	h := house{spec: room, at: at, floor: 10, door: brd.Read(0).Board().Grid.At(3, 3),
		doorAt: geom.NewVec(112, 128)}
	slope := func(q geom.Vec) float32 { return 4 + (q.Y-96)*6/32 }
	solid := Spec{Storey: 10, Floors: 2, Parts: []Part{{Roof: Gable, Pitch: 1}}}
	plain := solid.of(Row{Mid: [2]float32{64, 208}, Size: [2]float32{64, 32}})
	c := &p.cage
	c.stale, c.cells, c.low, c.high, c.rooms = false, map[cell.ID][]int32{}, 0, 50, true
	c.halls = append(c.halls, hallOf(room, at, h))
	cellAt := brd.Read(0).Board().CellAt
	for _, part := range heldOf(room, at, slope, &h, 0) {
		c.parts = append(c.parts, part)
		c.index(cellAt, 32, int32(len(c.parts)-1))
	}
	level := func(geom.Vec) float32 { return 10 }
	for _, part := range heldOf(solid, plain, level, nil, -1) {
		c.parts = append(c.parts, part)
		c.index(cellAt, 32, int32(len(c.parts)-1))
	}
	return p
}

func TestCage_HoldsTheEyeOutOfASolidHouseUnderItsRoofAndLetsItOver(t *testing.T) {
	p := caged(t)
	for name, tc := range map[string]struct {
		at   [3]float32
		free bool
	}{
		"within its walls":             {[3]float32{64, 208, 15}, false},
		"under the gable by the eaves": {[3]float32{64, 194, 31.9}, false},
		"over the slope by the eaves":  {[3]float32{64, 194, 32.1}, true},
		"under the ridge":              {[3]float32{64, 208, 45.9}, false},
		"over the ridge":               {[3]float32{64, 208, 46.1}, true},
		"beside it":                    {[3]float32{110, 208, 15}, true},
	} {
		if got := p.read.Free(tc.at[0], tc.at[1], tc.at[2]); got != tc.free {
			t.Errorf("%s, at %v: free %v, want %v", name, tc.at, got, tc.free)
		}
	}
}

func TestCage_LetsTheEyeIntoAHousesRoomAndByItsDoorAndHoldsItInItsWalls(t *testing.T) {
	p := caged(t)
	c := &p.cage
	shut := func(int32) bool { return false }
	open := func(int32) bool { return true }
	board := p.board.Read(0).Board()
	for name, tc := range map[string]struct {
		at         [3]float32
		free, open bool
	}{
		"in its room":                 {[3]float32{128, 112, 15}, true, true},
		"in its north wall":           {[3]float32{128, 96.25, 15}, false, true},
		"in its door":                 {[3]float32{112, 127.75, 14}, true, true},
		"in its door a wall in":       {[3]float32{112, 127.1, 14}, true, true},
		"in its wall beside the door": {[3]float32{118, 127.75, 14}, false, true},
		"in its wall a wall in":       {[3]float32{118, 127.1, 14}, false, true},
		"by its wall a wall in":       {[3]float32{118, 126.9, 14}, true, true},
		"in its wall over the door":   {[3]float32{112, 127.75, 19}, false, true},
		"in a window":                 {[3]float32{144, 96.25, 16}, false, true},
		"in its ceiling":              {[3]float32{128, 112, 21.8}, false, true},
		"under its roof":              {[3]float32{128, 112, 25}, false, true},
		"over its roof":               {[3]float32{128, 112, 30.1}, true, true},
		"in its plinth":               {[3]float32{128, 100, 7}, false, true},
	} {
		at, _ := board.CellAt(geom.NewVec(tc.at[0], tc.at[1]))
		if got := c.freeIn(at, tc.at[0], tc.at[1], tc.at[2], shut); got != tc.free {
			t.Errorf("%s, at %v, the house shut: free %v, want %v", name, tc.at, got, tc.free)
		}
		if got := c.freeIn(at, tc.at[0], tc.at[1], tc.at[2], open); got != tc.open {
			t.Errorf("%s, at %v, the house open: free %v, want %v", name, tc.at, got, tc.open)
		}
	}
}

func TestRead_GrainsTheWayNearARoomAlone(t *testing.T) {
	p := caged(t)
	band := Spec{Storey: 12}.wall()
	if step, thin := p.read.Grain(128, 112); step != band/2 || thin != band {
		t.Errorf("by the room the grain is %v and %v, want a step of %v by walls %v thin", step,
			thin, band/2, band)
	}
	if step, thin := p.read.Grain(64, 208); step != 0 || thin != 0 {
		t.Errorf("by the solid house the grain is %v and %v, want none", step, thin)
	}
}

func TestOpens_AHouseBeforeTheCamerasItsSettingsSayAndNeverInsideAUnit(t *testing.T) {
	for name, tc := range map[string]struct {
		settings Settings
		how      camera.How
		want     bool
	}{
		"loose":        {Settings{}, camera.Loose, true},
		"centred":      {Settings{}, camera.Centred, true},
		"loose, shut":  {Settings{Shut: true}, camera.Loose, false},
		"behind":       {Settings{Shut: true}, camera.Behind, true},
		"behind, shut": {Settings{ShutBehind: true}, camera.Behind, false},
		"inside":       {Settings{}, camera.Inside, false},
	} {
		if got := opens(tc.settings, tc.how); got != tc.want {
			t.Errorf("%s: opens %v, want %v", name, got, tc.want)
		}
	}
}

func TestSettings_SwitchTheRoomsOpenWhereAKindHasOne(t *testing.T) {
	p := caged(t)
	if got := p.Settings(); len(got) != 0 {
		t.Fatalf("%d settings with no kind with a room, want none", len(got))
	}
	p.types.rooms = true // a kind of house with a door defined
	got := p.Settings()
	if len(got) != 2 {
		t.Fatalf("%d settings, want the tactical view's and behind a unit's", len(got))
	}
	for i, want := range []command.Command{OpenRooms{On: false}, OpenRoomsBehind{On: false}} {
		sw, ok := got[i].(control.Switch)
		if !ok || !sw.Get() || sw.Set(false) != want {
			t.Errorf("setting %d: %+v, want a switch on from the start setting %#v", i, got[i],
				want)
		}
	}
	p.Write().OpenRooms(false)
	p.Write().OpenRoomsBehind(false)
	if want := (Settings{Shut: true, ShutBehind: true}); p.Res.Settings != want {
		t.Errorf("the settings %+v, want %+v", p.Res.Settings, want)
	}
}

func TestSpec_AHousesWallsAreATwelfthOfAStoreyThickUnlessItSays(t *testing.T) {
	if got := (Spec{Storey: 12}).wall(); got != 1 {
		t.Errorf("walls of a storey of 12 are %v thick, want 1", got)
	}
	if got := (Spec{Storey: 12, Wall: 2.5}).wall(); got != 2.5 {
		t.Errorf("walls the kind says are 2.5 thick are %v", got)
	}
	for _, s := range []Spec{{Storey: 12}, {Storey: 12, Wall: 2.5}} {
		if got := s.roomFloor(roomRow{}, cell.Gap{Width: 4}).Walls; got != s.wall() {
			t.Errorf("a room's floor of %+v has walls %v thick on the board, want %v", s, got,
				s.wall())
		}
	}
}
