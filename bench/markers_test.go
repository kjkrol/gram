package bench_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

// states is the tag family the marker benchmarks flag with; marked the bit.
type states struct{}

const marked tag.Tag[states] = 0

// marker is a structural marker with a value, as navigation's CellEntered was; bare one without,
// as effects' Idle was.
type (
	marker struct{ Cell uint32 }
	bare   struct{}
)

// markerWorld is n entities of a collider's row — Base, Appearance, Collider, Physics and the
// family's Tags — and its query over all of them.
type markerWorld struct {
	ecs   *goke.ECS
	all   *goke.Query
	base  goke.Comp[world.Base]
	tags  goke.Comp[tag.Tags[states]]
	mark  goke.Comp[marker]
	byTag *goke.Query // the entities carrying marker, by the archetype
	held  goke.Comp[marker]
	ids   []goke.CompID
	pick  []bool // the entities marked, in the order walked
}

// picking has every-th entity of w marked.
func (w *markerWorld) picking(n, every int) {
	w.pick = make([]bool, n)
	for k := range w.pick {
		w.pick[k] = k%every == 0
	}
}

func newMarkerWorld(b *testing.B, n int) *markerWorld {
	b.Helper()
	w := &markerWorld{ecs: goke.New()}
	w.ecs.Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		var app goke.Comp[world.Appearance]
		var coll goke.Comp[collision.Collider]
		var phys goke.Comp[collision.Physics]
		f := si.NewFactory(&w.base, &app, &coll, &phys, &w.tags)
		f.Create(n)
		for f.Next() {
		}
		w.all = si.NewQueryBuilder(&w.base, &w.tags).Build()
		w.byTag = si.NewQueryBuilder(&w.held).Build()
		w.ids = []goke.CompID{si.RegComp[marker](), si.RegComp[bare]()}
	}})
	return w
}

// toggler is how a share of the entities gets the marker on one tick and loses it the next.
type toggler func(w *markerWorld, cb *goke.CmdBuf, on bool, every int)

// one adds and removes a bare marker entity by entity: Idle, Active, Outside, the facts.
func one(w *markerWorld, cb *goke.CmdBuf, on bool, _ int) {
	k := 0
	for w.all.All(); w.all.Next(); {
		for _, id := range w.all.Cursor().IDs {
			if k++; !w.pick[k-1] {
				continue
			}
			if on {
				cb.AddOne(id, w.ids[1], bare{})
			} else {
				cb.RemoveCompOne(id, w.ids[1])
			}
		}
	}
}

// batch adds a marker with a value chunk by chunk through a ValueEditor and removes it through an
// Editor: navigation's CellEntered as it was.
func batch(add *goke.ValueEditor, remove *goke.Editor) toggler {
	return func(w *markerWorld, cb *goke.CmdBuf, on bool, every int) {
		if on {
			k := 0
			for w.all.All(); w.all.Next(); {
				cur := w.all.Cursor()
				var ids []uid.UID64
				for _, id := range cur.IDs {
					if k++; w.pick[k-1] {
						ids = append(ids, id)
					}
				}
				vals := cb.AddCompValue(add, &w.mark, w.all.ChunkSnapshot(), ids)
				for i := range vals {
					vals[i] = marker{Cell: 1}
				}
			}
			return
		}
		for w.byTag.All(); w.byTag.Next(); {
			buf := w.byTag.BeginMigrate(cb)
			for _, id := range w.byTag.Cursor().IDs {
				buf.Add(id)
			}
			buf.Commit(remove)
		}
	}
}

// flag sets and clears a bit of the family's Tags: a value write, no migration.
func flag(w *markerWorld, _ *goke.CmdBuf, on bool, _ int) {
	k := 0
	for w.all.All(); w.all.Next(); {
		tags := w.tags.Slice(w.all.Cursor())
		for i := range tags {
			if k++; !w.pick[k-1] {
				continue
			}
			if on {
				tags[i] = tags[i].With(marked)
			} else {
				tags[i] = tags[i].Without(marked)
			}
		}
	}
}

// Benchmark_Marker_Toggle is a share of 10000 colliders marked on one tick and unmarked the next
// — the op is the two ticks — three ways: a bare marker entity by entity (effects' Idle and Active
// as they were, the facts), a marker with a value chunk by chunk (navigation's CellEntered as it
// was), a bit of a tag family (a marker, comp.Marks).
func Benchmark_Marker_Toggle(b *testing.B) {
	const n = 10000
	for _, share := range []int{1, 10, 100} {
		every := 100 / share
		for _, way := range []string{"one", "batch", "flag"} {
			b.Run(fmt.Sprintf("share=%d%%/%s", share, way), func(b *testing.B) {
				w := newMarkerWorld(b, n)
				w.picking(n, every)
				var t toggler
				switch way {
				case "one":
					t = one
				case "batch":
					var add *goke.ValueEditor
					var remove *goke.Editor
					add = w.all.NewValueEditorBuilder(&w.mark).Build()
					remove = w.byTag.NewEditorBuilder().Remove(goke.Remove[marker]()).Build()
					t = batch(add, remove)
				case "flag":
					t = flag
				}
				on := false
				run := w.ecs.RegSys(goke.SystemFn{OnUpdate: func(cb *goke.CmdBuf, _ time.Duration) {
					on = !on
					t(w, cb, on, every)
				}})
				w.ecs.SetPlan(func(ctx goke.RunCtx, d time.Duration) {
					ctx.Run(run, d)
					ctx.Sync()
				})
				b.ResetTimer()
				for b.Loop() {
					w.ecs.Tick(time.Millisecond)
					w.ecs.Tick(time.Millisecond)
				}
				b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N)/float64(2*n/every), "ns/toggle")
			})
		}
	}
}

// Benchmark_Marker_Find counts the marked among 10000 colliders, a share of them marked: by the
// archetype — a query over the marker, which visits the marked alone — or by the bit, visiting
// every entity of the family's Tags.
func Benchmark_Marker_Find(b *testing.B) {
	const n = 10000
	for _, share := range []int{1, 10, 100} {
		every := 100 / share
		for _, way := range []string{"archetype", "flag"} {
			b.Run(fmt.Sprintf("share=%d%%/%s", share, way), func(b *testing.B) {
				w := newMarkerWorld(b, n)
				w.picking(n, every)
				mark := w.ecs.RegSys(goke.SystemFn{OnUpdate: func(cb *goke.CmdBuf, _ time.Duration) {
					k := 0
					for w.all.All(); w.all.Next(); {
						tags := w.tags.Slice(w.all.Cursor())
						for i, id := range w.all.Cursor().IDs {
							if k++; w.pick[k-1] {
								cb.AddOne(id, w.ids[0], marker{Cell: 1})
								tags[i] = tags[i].With(marked)
							}
						}
					}
				}})
				w.ecs.SetPlan(func(ctx goke.RunCtx, d time.Duration) { ctx.Run(mark, d); ctx.Sync() })
				w.ecs.Tick(time.Millisecond)
				found := 0
				b.ResetTimer()
				for b.Loop() {
					found = 0
					if way == "archetype" {
						for w.byTag.All(); w.byTag.Next(); {
							found += len(w.byTag.Cursor().IDs)
						}
						continue
					}
					for w.all.All(); w.all.Next(); {
						for _, t := range w.tags.Slice(w.all.Cursor()) {
							if t.Has(marked) {
								found++
							}
						}
					}
				}
				if found != n/every {
					b.Fatalf("found %d, want %d", found, n/every)
				}
			})
		}
	}
}

// Benchmark_Marker_Tick is the two ticks of Benchmark_Marker_Toggle with nothing toggled: its
// system walking every entity, the syncs — what the toggles' figures stand on.
func Benchmark_Marker_Tick(b *testing.B) {
	w := newMarkerWorld(b, 10000)
	w.picking(10000, 1)
	run := w.ecs.RegSys(goke.SystemFn{OnUpdate: func(*goke.CmdBuf, time.Duration) {
		k := 0
		for w.all.All(); w.all.Next(); {
			for range w.tags.Slice(w.all.Cursor()) {
				if k++; !w.pick[k-1] {
					continue
				}
			}
		}
	}})
	w.ecs.SetPlan(func(ctx goke.RunCtx, d time.Duration) { ctx.Run(run, d); ctx.Sync() })
	for b.Loop() {
		w.ecs.Tick(time.Millisecond)
		w.ecs.Tick(time.Millisecond)
	}
}
