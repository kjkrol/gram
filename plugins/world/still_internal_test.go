package world

import (
	"slices"
	"testing"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/uid"
)

// a world of posts, standing for good, and of things, which do not
func stillWorld(t *testing.T) (p *Plugin, posts, things kind.Of[thing], ecs *goke.ECS) {
	t.Helper()
	p = NewPlugin(Config{
		Space:    SpaceCfg{Width: 1000, Height: 1000},
		Entities: EntitiesCfg{MaxCount: 8, MinSize: 5, MaxSize: 20},
	})
	at := comp.Load(func(r thing) Position {
		return Position{AABB: plane.NewAABB(geom.NewVec(r.x, r.y), r.size, r.size)}
	})
	kind.Define[thing](p.Kinds(), "post", kind.Spec{at, comp.Const(Velocity{}),
		comp.Const(Fixed{})})
	kind.Define[thing](p.Kinds(), "thing", kind.Spec{at, comp.Const(Velocity{})})
	ecs = goke.New()
	ecs.Setup(p.module.SetupSystems()...)
	p.module.RegSystems(ecs)
	return p, kind.Named[thing](p.Kinds(), "post"), kind.Named[thing](p.Kinds(), "thing"), ecs
}

func ids(runs []aabbworld.Run) []uid.UID64 {
	var all []uid.UID64
	for _, r := range runs {
		all = append(all, r.IDs...)
	}
	return all
}

// a post moved in place, which nothing standing for good does, shows the still layer is read
// as it was gathered whatever the moving one does, and gathered anew when a post comes or goes
func TestIndex_WhatStandsForGoodLiesInTheStillLayerGatheredOnlyAsItComesAndGoes(t *testing.T) {
	p, posts, things, ecs := stillWorld(t)
	wm := p.module
	p.write.Spawn(posts.Entry(thing{x: 100, y: 100, size: 10}),
		things.Entry(thing{x: 300, y: 100, size: 10}))
	run(ecs, wm, func(*goke.CmdBuf) {})
	finds := func(x, y float32) []uid.UID64 {
		var found []uid.UID64
		wm.space.Query(geom.NewAABBAt(geom.NewVec(x, y), 10, 10), aabbworld.AnyCapability,
			func(id uid.UID64) { found = append(found, id) })
		return found
	}
	still, moving := ids(wm.still.runs), ids(wm.moving.runs)
	if len(still) != 1 || len(moving) != 1 {
		t.Fatalf("the still layer holds %v, the moving one %v; want a post and a thing", still,
			moving)
	}
	post, walker := still[0], moving[0]
	if found := finds(100, 100); !slices.Equal(found, []uid.UID64{post}) {
		t.Fatalf("at the post the space finds %v, want %v", found, post)
	}
	if found := finds(300, 100); !slices.Equal(found, []uid.UID64{walker}) {
		t.Fatalf("at the thing the space finds %v, want %v", found, walker)
	}

	x := wm.indexer
	for x.still.All(); x.still.Next(); {
		x.stillBase.Slice(x.still.Cursor())[0].Pos.AABB = plane.NewAABB(geom.NewVec(600, 100),
			10, 10)
	}
	for x.query.All(); x.query.Next(); {
		x.base.Slice(x.query.Cursor())[0].Pos.AABB = plane.NewAABB(geom.NewVec(300, 500), 10, 10)
	}
	p.write.Spawn(things.Entry(thing{x: 800, y: 100, size: 10}))
	run(ecs, wm, func(*goke.CmdBuf) {})
	if found := finds(300, 500); !slices.Equal(found, []uid.UID64{walker}) {
		t.Errorf("the thing moved, its chunk grown: the space finds %v where it went, want %v",
			found, walker)
	}
	if found := finds(600, 100); len(found) != 0 {
		t.Errorf("the still layer gathered anew though no post came or went: %v found where "+
			"the post was moved", found)
	}
	if found := finds(100, 100); !slices.Equal(found, []uid.UID64{post}) {
		t.Errorf("the space finds %v where the post was gathered, want %v", found, post)
	}

	p.write.Spawn(posts.Entry(thing{x: 100, y: 700, size: 10}))
	run(ecs, wm, func(*goke.CmdBuf) {})
	if found := finds(600, 100); !slices.Equal(found, []uid.UID64{post}) {
		t.Errorf("a post came: the space finds %v where the first was moved, want %v", found,
			post)
	}
	if found := finds(100, 700); len(found) != 1 {
		t.Errorf("the space finds %v at the post come, want it", found)
	}

	wm.commands.PutFrom(post, Despawn{})
	run(ecs, wm, func(*goke.CmdBuf) {})
	if found := finds(600, 100); len(found) != 0 {
		t.Errorf("the post gone, the space still finds %v where it stood", found)
	}
	if still := ids(wm.still.runs); len(still) != 1 || still[0] == post {
		t.Errorf("the still layer holds %v, want the other post alone", still)
	}
}

func TestIndex_ALoadGathersBothLayers(t *testing.T) {
	p, posts, things, ecs := stillWorld(t)
	wm := p.module
	p.write.Spawn(posts.Entry(thing{x: 100, y: 100, size: 10}),
		things.Entry(thing{x: 300, y: 100, size: 10}))
	run(ecs, wm, func(*goke.CmdBuf) {})
	wm.space.RebuildRuns(nil, nil)
	wm.space.RebuildStill(nil, nil)

	ecs.RegSys(wm.PostLoad())
	if still, moving := ids(wm.still.runs), ids(wm.moving.runs); len(still) != 1 ||
		len(moving) != 1 {
		t.Fatalf("after a load the still layer holds %v, the moving one %v; want one each",
			still, moving)
	}
	found := 0
	wm.space.Query(geom.NewAABB(geom.NewVec(0, 0), geom.NewVec(1000, 1000)),
		aabbworld.AnyCapability, func(uid.UID64) { found++ })
	if found != 2 {
		t.Errorf("after a load the space finds %d, want the post and the thing", found)
	}
}
