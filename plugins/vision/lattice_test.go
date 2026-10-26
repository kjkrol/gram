package vision_test

import (
	"math"
	"math/rand/v2"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/ground"
	"github.com/kjkrol/gram/plugins/topography"
	"github.com/kjkrol/gram/plugins/vision"
	"github.com/kjkrol/gram/plugins/world"
)

// cornered is the topography's relief as sight takes it: the heights, their profile along a ray
// and the corners they stand on.
type cornered interface {
	ground.Heights
	ground.Profile
	Lattice() (cols, rows int, cell float32, heights []float32, ok bool)
}

// alongOnly is a relief without its corners, read along each ray through its profile.
type alongOnly struct {
	ground.Heights
	profile ground.Profile
	alongs  *atomic.Int64
}

func (a alongOnly) Along(origin, dir geom.Vec, length float32, visit func(d, alt float32) bool) {
	a.alongs.Add(1)
	a.profile.Along(origin, dir, length, visit)
}

// latticed is a relief as it is, counting the rays read along its profile.
type latticed struct {
	cornered
	alongs *atomic.Int64
}

func (l latticed) Along(origin, dir geom.Vec, length float32, visit func(d, alt float32) bool) {
	l.alongs.Add(1)
	l.cornered.Along(origin, dir, length, visit)
}

const reliefCell = 25

// hilly is a relief over cols by rows cells: rolling hills, terraces whose cells lie level (their
// folds a tie), ridges across the cells, and ground off 0 at the edge, where it steps down.
func hilly(t *testing.T, cols, rows uint32, wrapX bool) cornered {
	t.Helper()
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: cols * reliefCell, Height: rows * reliefCell},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: reliefCell},
		Heights:  true,
	})
	g := grid.DefaultGrids{}.Square(cols, rows, reliefCell)
	brd := board.NewPlugin(g, &cell.MultipleOccupancy{}, w)
	if wrapX {
		// the topography refuses a world that wraps, so the grid wraps by hand: the relief's own
		// walk round a torus
		g.(interface{ SetWrap(x, y bool) }).SetWrap(true, false)
	}
	topo := topography.NewPlugin(w, brd, nil, topography.Config{})
	topo.Write().Relief().SetHeights(func(p geom.Vec) float32 {
		h := 18 + 12*f32.Sin(p.X/97)*f32.Cos(p.Y/73)
		if f32.Sin(p.X/41+p.Y/53) > 0.6 {
			h = f32.Round(h/5) * 5
		}
		if int(p.X/reliefCell+p.Y/reliefCell)%7 == 0 {
			h += 9
		}
		return h
	})
	r, ok := topo.Heights().(cornered)
	if !ok {
		t.Fatalf("the relief %T has no corners to read", topo.Heights())
	}
	return r
}

// watchers is n observers with outlines and as many walkers, walls and hawks standing on r within
// the box from (x0, y0) to (x1, y1), every one where r has it.
func watchers(seed uint64, r ground.Heights, n int, x0, y0, x1, y1 float32) []spawn {
	rnd := rand.New(rand.NewPCG(seed, seed+1))
	in := func(lo, hi float32) float32 { return lo + (hi-lo)*float32(rnd.Float64()) }
	on := func(s spawn, lift, height float32) spawn {
		box := at(s).AABB
		middle := geom.NewVec((box.TopLeft.X+box.BottomRight.X)/2,
			(box.TopLeft.Y+box.BottomRight.Y)/2)
		s.z = z(r.At(middle)+lift, height)
		return s
	}
	var out []spawn
	for i := range 2 * n {
		s := spawn{x: in(x0, x1), y: in(y0, y1), size: in(2, 30)}
		if i < n {
			a := 2 * math.Pi * rnd.Float64()
			eye := world.Eye{Height: 1.5, Angle: in(0.6, 2.2)}
			if i%3 == 0 {
				eye.Height = 30
			}
			s.size, s.outline = 6, true
			s.sight = &look{Sight: vision.Sight{Facing: geom.NewVec(f32.Cos(float32(a)),
				f32.Sin(float32(a))), Radius: in(200, 600)}, Eye: eye}
			out = append(out, on(s, 0, 2))
			continue
		}
		switch i % 4 {
		case 0:
			out = append(out, on(s, 30, 4)) // a hawk
		case 1:
			out = append(out, on(s, 0, 12)) // a wall
		default:
			out = append(out, on(s, 0, 2)) // a walker
		}
	}
	return out
}

// lookOver scans the observers among spawns once over g in a world of cfg: what each saw and its
// outline, in the order of spawns.
func lookOver(t *testing.T, cfg world.Config, g ground.Heights, spawns ...spawn) (
	[]vision.Sighted, []vision.SightOutline) {
	t.Helper()
	w := world.NewPlugin(cfg)
	v := vision.NewPlugin(w).WithHeights(func() ground.Heights { return g })
	ctx := &installCtx{ecs: goke.New()}
	if err := w.Install(ctx); err != nil {
		t.Fatal(err)
	}
	if err := v.Install(ctx); err != nil {
		t.Fatal(err)
	}
	for i, s := range spawns {
		spec := kind.Spec{comp.Load(at), comp.Const(world.Velocity{}), comp.Const(*s.z)}
		if s.sight != nil {
			spec = append(spec, comp.Const(s.sight.Sight), comp.Const(s.sight.Eye),
				comp.Const(vision.Sighted{}), comp.Const(vision.SightOutline{}))
		}
		kind.Define[spawn](w.Kinds(), kindName(i), spec)
		w.Write().Seed(kind.Named[spawn](w.Kinds(), kindName(i)).Entry(s))
	}
	if err := w.Populate(); err != nil {
		t.Fatal(err)
	}
	var sighted goke.Comp[vision.Sighted]
	var outline goke.Comp[vision.SightOutline]
	var query *goke.Query
	var systems []goke.System
	for _, produce := range ctx.pending {
		systems = append(systems, produce()...)
	}
	systems = append(systems, goke.SystemFn{OnInit: func(si *goke.SysInit) {
		query = si.NewQueryBuilder(&sighted, &outline).Build()
	}})
	ctx.ecs.Setup(systems...)
	ctx.ecs.SetPlan(func(rc goke.RunCtx, d time.Duration) {
		w.RunPlan(rc, d)
		v.RunPlan(rc, d)
		w.Write().Clock().Replay(rc, d)
	})
	ctx.ecs.Tick(time.Second / 60)
	var seen []vision.Sighted
	var outlines []vision.SightOutline
	query.All()
	for query.Next() {
		seen = append(seen, sighted.Slice(query.Cursor())...)
		outlines = append(outlines, outline.Slice(query.Cursor())...)
	}
	return seen, outlines
}

// Sight over the topography's relief reads its corners inside the scan, and sees what it saw
// reading the relief's profile ray by ray: the same entities in the same order at the same
// distances, the ground hidden in the same stretches; past the board's edge, round a torus, and
// on a world bending away under the eye.
func TestLattice_SightReadsTheReliefsCornersAsItsProfile(t *testing.T) {
	for _, c := range []struct {
		name           string
		wrapX          bool
		scale          world.Scale
		x0, y0, x1, y1 float32 // where the entities stand
	}{
		{"past the edge", false, world.Scale{}, 900, 500, 1580, 1180},
		{"past the edge, bent", false, world.Scale{Metres: 1000}, 900, 500, 1580, 1180},
		{"round a torus", true, world.Scale{}, 1300, 200, 1600, 900},
	} {
		t.Run(c.name, func(t *testing.T) {
			r := hilly(t, 64, 48, c.wrapX)
			cfg := world.Config{
				Space:    world.SpaceCfg{Width: 2000, Height: 1600},
				Entities: world.EntitiesCfg{MaxCount: 64, MinSize: 1, MaxSize: 100},
				Heights:  true,
				Scale:    c.scale,
			}
			if c.wrapX {
				cfg.Space.Width, cfg.Space.Edges = 64*reliefCell, aabbworld.WrapX
			}
			spawns := watchers(7, r, 24, c.x0, c.y0, c.x1, c.y1)
			var alongs, read atomic.Int64
			want, wantOut := lookOver(t, cfg, alongOnly{r, r, &alongs}, spawns...)
			got, gotOut := lookOver(t, cfg, latticed{r, &read}, spawns...)
			if alongs.Load() == 0 || read.Load() != 0 {
				t.Fatalf("the profile read along %d rays, the corners along %d; want the corners "+
					"read inside the scan and the profile along every ray", alongs.Load(),
					read.Load())
			}
			compared, shadows := 0, 0
			for i := range want {
				w, g := want[i], got[i]
				if !slices.Equal(g.IDs[:g.Count], w.IDs[:w.Count]) ||
					!slices.Equal(g.Dists[:g.Count], w.Dists[:w.Count]) {
					t.Errorf("observer %d sees %v at %v over the corners, %v at %v over the "+
						"profile", i, g.IDs[:g.Count], g.Dists[:g.Count], w.IDs[:w.Count],
						w.Dists[:w.Count])
				}
				compared += int(w.Count)
				tolerance := 1e-3 * spawns[i].sight.Radius
				wo, gi := wantOut[i], gotOut[i]
				if gi.Count != wo.Count {
					t.Fatalf("observer %d: %d angles over the corners, %d over the profile",
						i, gi.Count, wo.Count)
				}
				for k := range int(wo.Count) {
					if f32.Abs(gi.Depths[k]-wo.Depths[k]) > tolerance {
						t.Errorf("observer %d, angle %d: reach %v over the corners, %v over the "+
							"profile", i, k, gi.Depths[k], wo.Depths[k])
					}
					for b := range wo.Shadows[k] {
						gb, wb := gi.Shadows[k][b], wo.Shadows[k][b]
						if f32.Abs(gb.From-wb.From) > tolerance ||
							f32.Abs(gb.To-wb.To) > tolerance {
							t.Errorf("observer %d, angle %d: shadow %+v over the corners, %+v "+
								"over the profile", i, k, gb, wb)
						}
						if wb != (vision.Band{}) {
							shadows++
						}
					}
				}
			}
			if compared < 20 || shadows < 100 {
				t.Errorf("%d entities seen and %d shadows in all, too few to compare", compared,
					shadows)
			}
		})
	}
}
