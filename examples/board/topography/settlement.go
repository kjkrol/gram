package main

import (
	"fmt"
	"math/rand/v2"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/act"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/ceremony/cond"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/lod"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/buildings"
	"github.com/kjkrol/gram/plugins/forest"
	"github.com/kjkrol/gram/plugins/light"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

// the plots a sector of a settlement may hold a lamp or a tree at: its cells' quarters
const (
	quarterColumns = 2 * sectorCells
	quarterPlots   = quarterColumns * quarterColumns
	// the built cells a settlement's sector lights a lamp among, at least
	litFrom = 16
)

// the plots off the middle of a sector's part of a settlement, each at a quarter of its cell
// whatever the part's size
var quarterLattice = func() (at [quarterPlots]geom.Vec) {
	for k := range at {
		i, j := k%quarterColumns, k/quarterColumns
		at[k] = geom.NewVec(
			float32((2*i+1)*CellSize/4-sectorCells*CellSize/2),
			float32((2*j+1)*CellSize/4-sectorCells*CellSize/2))
	}
	return at
}()

// the plots shaken a little within their quarters, so no street runs straight
var quarterShaken = func() (at [quarterPlots]geom.Vec) {
	shake := rand.New(rand.NewPCG(5, 13))
	for k := range at {
		q := float32(CellSize) / 16
		at[k] = quarterLattice[k].Add(geom.NewVec((shake.Float32()*2-1)*q, (shake.Float32()*2-1)*q))
	}
	return at
}()

// how high a settlement's palms and its dragon trees grow, metres, and of how many built
// cells one has a tree in its garden
const (
	palmsTo   = 300
	dragonsTo = 200
	gardens   = 8
)

// settlement stands over a field of the map's built cells, a building on each of them (settle):
// each sector of it come down to L3 raises them, one of litFrom cells or more lights a lamp in
// the yard of the cell nearest its middle, and by the sea a palm or a dragon tree, of the
// trees' kinds, grows in some yards.
type settlement struct{ trees *forest.Plugin }

var (
	_ board.Figure = settlement{}
	_ board.Ruled  = settlement{}
)

func (s settlement) Rules(b *board.Plugin) {
	w := b.Read(0).World()
	lit := kind.EntryOf(LampKind, lampRow{})
	palm, dragon := s.trees.Types().Entry(PalmKind), s.trees.Types().Entry(DragonKind)
	built := func(at geom.Vec) bool { return lotAt(b, at) }
	rules := make([]act.Rule, 0, quarterPlots+1)
	rules = append(rules, act.Branch[world.Unfold]{
		Name: "its buildings stand",
		When: cond.Self.If(cond.Always),
		Then: act.Self.Do(buildings.Raise{}),
	})
	for k := range quarterPlots {
		rules = append(rules, act.Branch[world.Unfold]{
			Name: fmt.Sprintf("plot %d", k),
			When: cond.Self.If(builtAt(b, k)),
			Then: act.OneOf(
				act.Branch[world.Unfold]{
					When: cond.Self.If(lampAt(b, k, built)),
					Then: act.Self.Do(world.SpawnPart{Entry: lit, At: quarterShaken[k]}),
				},
				act.Branch[world.Unfold]{
					When: cond.Self.If(gardenAt(b, k, 0, palmsTo)),
					Then: act.Self.Do(world.SpawnPart{Entry: palm, At: quarterShaken[k]}),
				},
				act.Branch[world.Unfold]{
					When: cond.Self.If(gardenAt(b, k, 1, dragonsTo)),
					Then: act.Self.Do(world.SpawnPart{Entry: dragon, At: quarterShaken[k]}),
				}),
		})
	}
	w.Roles().Define(SettlementRole, rules...)
}

func (settlement) Define(b *board.Plugin) {
	w := b.Read(0).World()
	kind.Define[fieldRow](w.Kinds(), SettlementKind, w.Roster().Spec(world.EntityTemplate,
		append(overField(b), ceremony.Plays(w.Roles().Named(SettlementRole)))...))
}

// its lamps and its yards' trees at L2 and L3; from L4 they stand in the sheet
func (settlement) Look(a *world.Atlas, w *world.Plugin) {
	a.Add(kind.Named[fieldRow](w.Kinds(), SettlementKind), fieldPx, unseen)
}

// builtAt is the word of the k-th plot of a sector of a settlement lying on a lot of the
// settlement's part there
func builtAt(b *board.Plugin, k int) cond.Word[world.Unfold] {
	return cond.NewWord(fmt.Sprintf("built %d", k), func(u world.Unfold) bool {
		at := middle(u.Box).Add(quarterLattice[k])
		return within(u.Box, at) && lotAt(b, at)
	})
}

// lotAt reports whether at lies on a lot: a built cell no road or river runs across
func lotAt(b *board.Plugin, at geom.Vec) bool {
	bd := b.Read(0).Board()
	c, ok := bd.Grid.CellAt(at)
	return ok && on(b, at, BuiltCell) && !bd.Way(c).Runs()
}

// lampAt is the word of the k-th plot of a sector of a settlement being the quarter across its
// cell from its lot's, in the yard of the built cell nearest the middle of its part there, of
// litFrom built cells or more
func lampAt(b *board.Plugin, k int, built func(geom.Vec) bool) cond.Word[world.Unfold] {
	return cond.NewWord(fmt.Sprintf("lamp %d", k), func(u world.Unfold) bool {
		at := middle(u.Box).Add(quarterLattice[k])
		if q, l := quarters(b, at); q != 3-l.home {
			return false
		}
		c, _ := b.Read(0).Board().Grid.CellAt(at)
		heart, n := heartOf(b, u.Box, built)
		return n >= litFrom && c == heart
	})
}

// gardenAt is the word of the k-th plot of a sector of a settlement being the other quarter of
// its cell's yard, the cell one of every gardens whose seed draws the number drawn and its
// ground lower than below, metres
func gardenAt(b *board.Plugin, k, drawn int, below float32) cond.Word[world.Unfold] {
	return cond.NewWord(fmt.Sprintf("garden %d %d", drawn, k), func(u world.Unfold) bool {
		at := middle(u.Box).Add(quarterLattice[k])
		if q, l := quarters(b, at); q != l.yard() {
			return false
		}
		heights := b.Read(0).Heights()
		if heights == nil || heights.At(at) >= scale.Units(below) {
			return false
		}
		bd := b.Read(0).Board()
		c, _ := bd.Grid.CellAt(at)
		i, _ := bd.Ordinal(c)
		return int(lod.Seed(0, uint64(i))>>8%gardens) == drawn
	})
}

// quarters are the quarter of its cell at lies in, 0 and 1 the upper left and right, 2 and 3
// the lower, and the cell's lot
func quarters(b *board.Plugin, at geom.Vec) (q int, l lot) {
	bd := b.Read(0).Board()
	c, _ := bd.Grid.CellAt(at)
	mid := bd.CellCenter(c)
	if at.X > mid.X {
		q |= 1
	}
	if at.Y > mid.Y {
		q |= 2
	}
	return q, lotOf(b, c)
}

// heartOf is the built cell of box nearest its middle, the first of a tie by rows, and how
// many cells of box are built
func heartOf(b *board.Plugin, box geom.AABB, built func(geom.Vec) bool) (heart cell.ID, n int) {
	bd := b.Read(0).Board()
	mid := middle(box)
	best := float32(-1)
	for y := box.TopLeft.Y + CellSize/2; y < box.BottomRight.Y; y += CellSize {
		for x := box.TopLeft.X + CellSize/2; x < box.BottomRight.X; x += CellSize {
			at := geom.NewVec(x, y)
			if !built(at) {
				continue
			}
			n++
			c, _ := bd.Grid.CellAt(at)
			off := at.Sub(mid)
			if d := off.X*off.X + off.Y*off.Y; best < 0 || d < best {
				heart, best = c, d
			}
		}
	}
	return heart, n
}

// on reports whether at lies on a cell of the kind named, snowed on or not
func on(b *board.Plugin, at geom.Vec, name string) bool {
	bd := b.Read(0).Board()
	c, ok := bd.Grid.CellAt(at)
	if !ok {
		return false
	}
	n := bd.Bare(c).Name
	return n == cell.Named(name) || n == cell.Named(snowyCell(name))
}

func middle(box geom.AABB) geom.Vec {
	return geom.NewVec((box.TopLeft.X+box.BottomRight.X)/2, (box.TopLeft.Y+box.BottomRight.Y)/2)
}

func within(box geom.AABB, at geom.Vec) bool {
	return at.X >= box.TopLeft.X && at.X < box.BottomRight.X && at.Y >= box.TopLeft.Y &&
		at.Y < box.BottomRight.Y
}

type lampRow struct{}

// lamp is a part of a settlement, a street lamp lit at night: it shines warm as far as it
// reaches, casts no shadow and takes none, and goes as its sector folds.
type lamp struct{}

var (
	_ board.Figure = lamp{}
	_ board.Ruled  = lamp{}
)

func (lamp) Rules(b *board.Plugin) {
	b.Read(0).World().Roles().Define(LampRole, act.Branch[world.Folding]{
		Name: "fold in",
		When: cond.Self.If(cond.Always),
		Then: act.Self.Do(world.Despawn{}),
	})
}

func (lamp) Define(b *board.Plugin) {
	w := b.Read(0).World()
	side := scale.Figure(1)
	kind.Define[lampRow](w.Kinds(), LampKind, w.Roster().Spec(board.StandingTemplate,
		// a part: SpawnPart lays its box off the middle of the settlement's sector
		comp.Const(world.Position{
			AABB: plane.NewAABB(geom.NewVec(-side/2, -side/2), side, side),
		}),
		comp.Const(world.Z{Height: scale.Figure(4)}),
		comp.Const(light.Source{Color: lanternColor, Strength: 1.5, Reach: scale.Figure(10)}),
		comp.Const(light.Shadow{Cast: light.NoShadow, Take: render.Shines}),
		ceremony.Plays(w.Roles().Named(LampRole))))
}

func (lamp) Look(a *world.Atlas, w *world.Plugin) {
	a.Add(kind.Named[lampRow](w.Kinds(), LampKind), 8, func(dst *render.Canvas, size int) {
		s := float32(size)
		dst.FillRect(s*0.4, s*0.5, s*0.2, s*0.5, pierColor)
		dst.FillCircle(s/2, s*0.3, s*0.3, lanternGlow)
	})
}
