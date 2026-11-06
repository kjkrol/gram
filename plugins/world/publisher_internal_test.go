package world

import (
	"image/color"
	"slices"
	"testing"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/lod"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	ilook "github.com/kjkrol/gram/plugins/world/internal/look"
	"github.com/kjkrol/gram/plugins/world/sector"
	"github.com/kjkrol/gram/plugins/world/view"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/uid"
)

// thinnable is a world's atlas of a plain kind, a kind never thinned and one with a look declared
// at L4, closed
type thinnable struct {
	a                       *Atlas
	plain, always, declared render.SpriteID
}

func newThinnable() thinnable {
	p := NewPlugin(Config{
		Space:    SpaceCfg{Width: 1024, Height: 1000},
		Entities: EntitiesCfg{MaxCount: 1000, MinSize: 1, MaxSize: 10},
	})
	k := thinnable{a: p.NewAtlas()}
	k.plain, k.always, k.declared = p.Kinds().NewSprite(), p.Kinds().NewSprite(),
		p.Kinds().NewSprite()
	red := render.Solid(color.RGBA{R: 255, A: 255})
	k.a.Add(k.plain, 8, red)
	k.a.Add(k.always, 8, red).Always()
	k.a.Add(k.declared, 8, red).At(lod.L4, render.Solid(color.RGBA{G: 255, A: 255}))
	k.a.Close()
	return k
}

// group is n entities of a kind, keen or not
type group struct {
	n      int
	sprite render.SpriteID
	keen   bool
}

// published is the records the renderer publishes through a camera seeing the world as halves
// with ladder says of groups, the entities of each in a row along the right sector, at L4 under
// the thresholds 0.5 and 0.2, and ten more in the left at L2; the ids of those of each group
// published, the left ones last
func published(t *testing.T, k thinnable, ladder lod.Ladder,
	groups ...group) (*figures, [][]uid.UID64) {
	t.Helper()
	cam := farther{Camera: cameratest.NewFromSpace(1024, 1000, 0)}
	v := view.NewSeen(cam, halves{ladder: ladder})
	look := ilook.NewFlat(1024, 1000, false, false)
	r := newRenderer(k.a, seenBy(cam, v), &Rules{}, func() Look { return look })
	readied(k.a)(r)
	space, err := aabbworld.NewSpace(aabbworld.Config{Width: 1024, Height: 1000, BucketSize: 128})
	if err != nil {
		t.Fatal(err)
	}
	var base goke.Comp[Base]
	var appearance goke.Comp[render.Appearance]
	var keenOf goke.Comp[sector.Keen]
	ids := make([][]uid.UID64, len(groups)+1)
	ecs := goke.New()
	ecs.Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		var items []aabbworld.Item
		add := func(g int, n int, sprite render.SpriteID, keen bool, x0 float32) {
			f := si.NewFactory(&base, &appearance)
			if keen {
				f = si.NewFactory(&base, &appearance, &keenOf)
			}
			f.Create(n)
			for i := 0; f.Next(); {
				for j, id := range f.Cursor.IDs {
					box := plane.NewAABB(geom.NewVec(x0+float32(i%40)*12, float32(i/40)*12), 4, 4)
					base.Slice(&f.Cursor)[j] = Base{Pos: Position{AABB: box}}
					appearance.Slice(&f.Cursor)[j] = render.Appearance{SpriteID: sprite}
					items = append(items, aabbworld.Item{ID: id, Box: box})
					ids[g] = append(ids[g], id)
					i++
				}
			}
		}
		for g, gr := range groups {
			add(g, gr.n, gr.sprite, gr.keen, 520)
		}
		add(len(groups), 10, k.plain, false, 10)
		space.Rebuild(items)
		r.Init(si)
	}})
	views := []*view.View{v}
	view.NewSystem(space, &views, 1024, 1000).Update(nil, 0)
	r.Publish()
	r.Swap()
	figs := r.figures.Front()
	kept := make([][]uid.UID64, len(ids))
	for _, e := range figs.all {
		for g := range ids {
			if slices.Contains(ids[g], e.id) {
				kept[g] = append(kept[g], e.id)
			}
		}
	}
	return figs, kept
}

// keeping is a ladder drawing the right sector at L4 keeping share there
func keepingL4(share float32) lod.Ladder {
	l := lod.Ladder{Scales: [lod.Levels - 1]float32{0, 0, 0.5, 0.2}}
	l.Keep[lod.L4] = share
	return l
}

func TestRenderer_Publish_ThinsALevelByItsKeepTheSameEveryTime(t *testing.T) {
	k := newThinnable()
	plain := group{n: 400, sprite: k.plain}
	_, half := published(t, k, keepingL4(0.5), plain)
	_, again := published(t, k, keepingL4(0.5), plain)
	_, more := published(t, k, keepingL4(0.75), plain)
	if n := len(half[0]); n < 140 || n > 260 {
		t.Errorf("kept %d of 400 at a half, want about 200", n)
	}
	if !slices.Equal(half[0], again[0]) {
		t.Error("kept another set published again, want the same")
	}
	for _, id := range half[0] {
		if !slices.Contains(more[0], id) {
			t.Errorf("%v kept at a half, not at three quarters, want every one kept", id)
		}
	}
	if len(more[0]) <= len(half[0]) || len(half[1]) != 10 {
		t.Errorf("kept %d at three quarters, %d at a half, %d of 10 at L2; want more, all",
			len(more[0]), len(half[0]), len(half[1]))
	}
}

func TestRenderer_Publish_KeepsTheKeenTheKindsNeverThinnedAndThoseDeclaredThere(t *testing.T) {
	k := newThinnable()
	_, kept := published(t, k, keepingL4(0.01), group{n: 100, sprite: k.plain},
		group{n: 100, sprite: k.plain, keen: true}, group{n: 100, sprite: k.always},
		group{n: 100, sprite: k.declared})
	if len(kept[0]) > 10 || len(kept[1]) != 100 || len(kept[2]) != 100 || len(kept[3]) != 100 {
		t.Errorf("kept %d plain, %d keen, %d never thinned, %d declared of 100 each; want few, "+
			"then all", len(kept[0]), len(kept[1]), len(kept[2]), len(kept[3]))
	}
}

func TestRenderer_Publish_KeepingAllPublishesAsWithoutAKeep(t *testing.T) {
	k := newThinnable()
	groups := []group{{n: 100, sprite: k.plain}, {n: 20, sprite: k.always}}
	none, _ := published(t, k, keepingL4(0), groups...)
	// a share at a level none is drawn at: every entity weighed, none thinned
	l3 := keepingL4(0)
	l3.Keep[lod.L5] = 0.5
	weighed, _ := published(t, k, l3, groups...)
	if !slices.Equal(none.all, weighed.all) || !slices.Equal(none.layers, weighed.layers) ||
		len(none.all) != 130 {
		t.Errorf("published %d records keeping all and %d weighing every one, want the same 130",
			len(none.all), len(weighed.all))
	}
}

// keepingL5 is a ladder drawing the right sector at L5 keeping share there
func keepingL5(share float32) lod.Ladder {
	l := lod.Ladder{Scales: [lod.Levels - 1]float32{0, 0, 0.5, 0.2, 0.15}}
	l.Keep[lod.L5] = share
	return l
}

func TestRenderer_Publish_RecordsWhatItSeesAtL5ThinnedByItsKeep(t *testing.T) {
	k := newThinnable()
	groups := []group{{n: 100, sprite: k.plain}, {n: 100, sprite: k.plain, keen: true},
		{n: 100, sprite: k.always}, {n: 100, sprite: k.declared}}
	_, all := published(t, k, keepingL5(0), groups...)
	for g, ids := range all[:len(groups)] {
		if len(ids) != 100 {
			t.Errorf("recorded %d of the group %d's 100 at L5 keeping all, want every one",
				len(ids), g)
		}
	}
	_, kept := published(t, k, keepingL5(0.01), groups...)
	if len(kept[0]) > 10 || len(kept[1]) != 100 || len(kept[2]) != 100 || len(kept[3]) != 100 {
		t.Errorf("kept %d plain, %d keen, %d never thinned, %d declared of 100 each at L5; want "+
			"few, then all", len(kept[0]), len(kept[1]), len(kept[2]), len(kept[3]))
	}
}
