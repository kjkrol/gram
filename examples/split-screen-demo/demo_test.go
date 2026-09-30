package main

import (
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/internal/engine"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/players/owner"
	"github.com/kjkrol/gram/plugins/world/steering"
)

func TestDemo_TwoHalvesAndAMinimapOfTheWholeArena(t *testing.T) {
	d := NewDemo()
	if err := engine.NewEngine(d).Init(); err != nil {
		t.Fatal(err)
	}
	s := d.stage
	main, _ := s.stack.Get("main")
	minimap, _ := s.stack.Get("minimap")
	screen := geom.NewAABB(geom.NewVec(0, 0), geom.NewVec(ScreenWidth, ScreenHeight))

	halves := main.(*mainScene).Viewports(screen)
	if len(halves) != 2 || halves[0].Camera != s.redPlayer.Camera || halves[1].Camera != s.bluePlayer.Camera ||
		halves[0].Area != geom.NewAABB(geom.NewVec(0, 0), geom.NewVec(ScreenWidth/2, ScreenHeight)) {
		t.Fatalf("main viewports %+v, want red's camera on the left half and blue's on the right", halves)
	}
	if halves[0].Camera == s.world.Camera() || halves[0].Camera == halves[1].Camera {
		t.Error("the players look through a shared camera, want one of their own each")
	}

	vp := minimap.(*minimapScene).Viewports(screen)
	if len(vp) != 1 {
		t.Fatalf("%d minimap viewports, want 1", len(vp))
	}
	a := vp[0].Area
	if a.TopLeft.X < 0 || a.TopLeft.Y < 0 || a.BottomRight.X > ScreenWidth || a.BottomRight.Y > ScreenHeight || a.BottomRight.X-a.TopLeft.X != MinimapWidth {
		t.Fatalf("minimap viewport %+v, want one %d wide on the screen", vp, MinimapWidth)
	}
	b := vp[0].Camera.Bounds()
	if b.TopLeft.X > 0 || b.TopLeft.Y > 0 || b.BottomRight.X < WorldWidth-1 || b.BottomRight.Y < WorldHeight-1 {
		t.Errorf("the minimap shows %v, want the whole %dx%d arena", b, WorldWidth, WorldHeight)
	}
}

func TestDriveSystem_SteersTheBlockOfThePlayerWhoDrivesAndBrakesTheOther(t *testing.T) {
	var drives control.Queue[Drive]
	sys := &driveSystem{drives: &drives}
	var owners goke.Comp[plugin.Tags[owner.Family]]
	var steer goke.Comp[steering.Steering]
	ecs := goke.New()
	ecs.Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		f := si.NewFactory(&owners, &steer)
		f.Create(2)
		for f.Next() {
			for i := range f.Cursor.IDs {
				owners.Slice(&f.Cursor)[i] = plugin.Tags[owner.Family](0).With(owner.Of(control.PlayerID(i + 1)))
				steer.Slice(&f.Cursor)[i] = steering.Steering{MaxSpeed: 100, WantSpeed: 50}
			}
		}
	}}, sys)
	drives.Add(1, Drive{Dir: geom.NewVec(1, 0)})
	drives.Add(1, Drive{Dir: geom.NewVec(0, 1)})
	sys.Update(nil, time.Second/60)

	for sys.query.All(); sys.query.Next(); {
		cur := sys.query.Cursor()
		for i := range cur.IDs {
			owned, st := sys.owners.Slice(cur)[i], sys.steer.Slice(cur)[i]
			switch {
			case owned.Has(owner.Of(1)):
				if st.WantSpeed != 100 || st.Want.X <= 0 || st.Want.Y <= 0 {
					t.Errorf("player 1's block wants %v at %v, want down-right at full speed", st.Want, st.WantSpeed)
				}
			case owned.Has(owner.Of(2)):
				if st.WantSpeed != 0 {
					t.Errorf("player 2's block wants speed %v with no Drive, want 0", st.WantSpeed)
				}
			}
		}
	}
}
