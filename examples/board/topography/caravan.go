package main

import (
	"image/color"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/act"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/ceremony/cond"
	"github.com/kjkrol/ceremony/plan"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/lod"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/minimap"
	"github.com/kjkrol/gram/plugins/navigation"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

const wagonPx = 12

// a wagon's box across and down, narrow enough to cross a bridge's deck slantwise between its
// railings, and how tall it stands
var (
	wagonWide = scale.Figure(1.5)
	wagonDeep = scale.Figure(1)
	wagonTall = scale.Figure(1.6)
)

// where a caravan's wagon and its three porters stand as it unfolds: a cell apart in file, the
// wagon first
var inFile = [4]geom.Vec{
	geom.NewVec(0, 0),
	geom.NewVec(-CellSize, 0),
	geom.NewVec(-2*CellSize, 0),
	geom.NewVec(-3*CellSize, 0),
}

type caravanRow struct {
	Start  cell.ID `yaml:"start"`
	Target cell.ID `yaml:"target"`
}

// caravan is a wagon and its porters seen from afar, one point going to and fro between two
// places on the abstract graph: come down to L3 it stands them in file, each taking its way
// over, and folded it takes the way back from them.
type caravan struct {
	color color.RGBA
	// what its wagon and porters are told as they are made: whose they are, each picked itself
	carried func() []command.Command
}

var (
	_ board.Figure = caravan{}
	_ board.Ruled  = caravan{}
)

func (c caravan) Rules(b *board.Plugin) {
	w := b.Read(0).World()
	unpack := make([]act.Step, 0, len(inFile))
	for i, at := range inFile {
		part := kind.EntryOf(PorterKind, porterRow{})
		if i == 0 {
			part = kind.EntryOf(WagonKind, wagonRow{})
		}
		part = part.Told(navigation.Inherit{}).Told(c.carried()...)
		unpack = append(unpack, act.Self.Do(world.SpawnPart{Entry: part, At: at}))
	}
	w.Roles().Define(CaravanRole,
		act.Branch[world.Unfold]{
			Name: "unpack",
			When: cond.Self.If(cond.Always),
			Then: act.Steps(unpack...),
		},
		act.Branch[world.Fold]{
			Name: "pack",
			When: cond.Self.If(cond.Always),
			Then: act.Self.Do(navigation.Inherit{}),
		})
}

func (c caravan) Define(b *board.Plugin) {
	w := b.Read(0).World()
	units := board.NewUnits[caravanRow](b,
		board.Shape{Size: wagonDeep, Width: wagonWide, Height: wagonTall},
		func(r caravanRow) geom.Vec { return b.Read(0).Board().CellCenter(r.Start) })
	units.Define(CaravanKind, unit.Mover{Domain: cell.Land}, walking,
		comp.Load(func(r caravanRow) navigation.MoveOrder {
			return navigation.MoveOrder{Target: r.Target}
		}),
		comp.Load(func(r caravanRow) navigation.Round {
			return navigation.NewRound(r.Target, r.Start)
		}),
		comp.Const(world.Aggregate{}),
		// its wagon and porters stand round it, pushed by nothing of its own
		comp.Without[collision.Collider](),
		comp.Without[collision.Physics](),
		shuttle(CaravanPlan, 0),
		ceremony.Plays(w.Roles().Named(CaravanRole)))
}

// unseen while its wagon and porters stand, three figures on a disc from L4
func (c caravan) Look(a *world.Atlas, w *world.Plugin) {
	a.Add(kind.Named[caravanRow](w.Kinds(), CaravanKind), iconPx, unseen).
		At(lod.L4, caravanIcon(c.color)).
		Styled(world.BannerStyle{Fill: c.color}).
		Styled(selection.Highlight{Line: c.color, Width: 2}).
		Styled(navigation.RouteStyle{Line: c.color, Goal: c.color, Width: 1.5}).
		Styled(minimap.Mark{Color: c.color, Size: 5})
}

// shuttle is the plan of one going to and fro between the two cells of its Round, resting rest
// at either end
func shuttle(name string, rest time.Duration) plan.Planned {
	leg := plan.Loop[any]{
		Name:  "leg",
		When:  cond.Self.If(cond.Always),
		Then:  plan.Self.Do(navigation.Onward{}),
		Until: cond.Self.If(navigation.Arrived{}),
	}
	if rest == 0 {
		return plan.New(name, leg)
	}
	return plan.New(name, plan.Steps(leg,
		plan.Loop[any]{Name: "rest", When: cond.Self.If(cond.Always), Then: plan.Self.Wait(rest)}))
}

type wagonRow struct{}

// wagon is a caravan's cart, keeping its porters' pace: it goes as its caravan folds.
type wagon struct{}

var _ board.Figure = wagon{}

func (wagon) Define(b *board.Plugin) {
	w := b.Read(0).World()
	// a part: SpawnPart lays its box off the caravan's centre
	units := board.NewUnits[wagonRow](b,
		board.Shape{Size: wagonDeep, Width: wagonWide, Height: wagonTall},
		func(wagonRow) geom.Vec { return geom.Vec{} })
	units.Define(WagonKind, unit.Mover{Domain: cell.Land}, walking,
		ceremony.Plays(w.Roles().Named(MortalRole), w.Roles().Named(CarrierRole)),
		leaping())
}

func (wagon) Look(a *world.Atlas, w *world.Plugin) {
	a.Add(kind.Named[wagonRow](w.Kinds(), WagonKind), wagonPx, cart).
		Upright().
		Styled(minimap.Mark{Color: wagonColor, Size: 4})
}

// a cart seen from its side, its two wheels under it
func cart(dst *render.Canvas, size int) {
	s := float32(size)
	dst.FillRect(0, 0, s, s*0.75, wagonColor)
	dst.FillCircle(s*0.25, s*0.8, s*0.18, wheelColor)
	dst.FillCircle(s*0.75, s*0.8, s*0.18, wheelColor)
}

type porterRow struct{}

// porter is one of a caravan's men, on foot as a walker but looking out for nothing: it goes as
// its caravan folds.
type porter struct{ color color.RGBA }

var _ board.Figure = porter{}

func (porter) Define(b *board.Plugin) {
	w := b.Read(0).World()
	// a part: SpawnPart lays its box off the caravan's centre
	units := board.NewUnits[porterRow](b,
		board.Shape{Size: scale.Figure(0.6), Height: figure},
		func(porterRow) geom.Vec { return geom.Vec{} })
	units.Define(PorterKind, unit.Mover{Domain: cell.Land}, walking,
		ceremony.Plays(w.Roles().Named(MortalRole), w.Roles().Named(CarrierRole)),
		leaping())
}

func (p porter) Look(a *world.Atlas, w *world.Plugin) {
	a.Add(kind.Named[porterRow](w.Kinds(), PorterKind), walkerPx, render.Solid(p.color)).
		Upright().
		Styled(minimap.Mark{Color: p.color, Size: 3})
}
