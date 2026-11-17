package main

import (
	"image/color"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/lod"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/buildings"
)

var (
	ochre  = color.RGBA{R: 232, G: 204, B: 148, A: 255}
	basalt = color.RGBA{R: 132, G: 126, B: 120, A: 255}
)

// how long and how wide a building stands, metres: no wider than the island's world holds a
// thing, and narrow enough to leave its cell a yard
const (
	homeLong = 6
	homeWide = 4
)

// homes is the kinds of buildings a settlement stands, their sizes in metres as in life: a
// cottage under a hipped roof, a house of a town's streets and a chapel with its tower.
type homes struct{}

var _ buildings.Figure = homes{}

func (homes) Define(p *buildings.Plugin) {
	m := scale.Figure
	size := geom.NewVec(m(homeLong), m(homeWide))
	p.Types().Define(HouseKind, buildings.Spec{
		Storey: m(3),
		Floors: 1,
		Size:   size,
		Walls:  wallColor,
		Tiles:  roofColor,
		Bay:    m(2),
		Parts:  []buildings.Part{{Roof: buildings.Hip, Pitch: 0.5}},
	})
	p.Types().Define(TownhouseKind, buildings.Spec{
		Storey: m(3),
		Floors: 3,
		Size:   size,
		Walls:  ochre,
		Tiles:  roofColor,
		Bay:    m(1.5),
		Parts:  []buildings.Part{{Roof: buildings.Gable, Pitch: 0.45}},
	})
	p.Types().Define(ChapelKind, buildings.Spec{
		Storey: m(4.5),
		Floors: 1,
		Size:   size,
		Walls:  wallColor,
		Tiles:  roofColor,
		Bay:    m(3),
		Parts: []buildings.Part{
			{Roof: buildings.Gable, Pitch: 0.8},
			{
				Roof:  buildings.Pyramid,
				Pitch: 1.4,
				Size:  geom.NewVec(m(1.8), m(1.8)),
				Off:   geom.NewVec(-0.7, 0),
				Above: m(3.5),
				Tiles: basalt,
				Blind: true,
			},
		},
	})
}

// lot is how a built cell is laid out: its building stands against the edge the quarter home
// lies by, 0 and 1 the upper left and right, 2 and 3 the lower, the upper or the lower edge, or
// upright the left or the right one, the building running north to south; the quarters across
// the cell from it are its yard.
type lot struct {
	home    int
	upright bool
}

func lotOf(b *board.Plugin, c cell.ID) lot {
	i, _ := b.Read(0).Board().Ordinal(c)
	seed := lod.Seed(0, uint64(i))
	return lot{home: int(seed % 4), upright: seed>>2%2 == 1}
}

// yard is the quarter of the lot's yard beside the one across the cell from home
func (l lot) yard() int {
	if l.upright {
		return l.home ^ 1
	}
	return l.home ^ 2
}

// stands is where the lot's building stands in the cell round mid and how it is turned: as far
// from the edge it stands against as from those at its ends
func (l lot) stands(mid geom.Vec) (at geom.Vec, turn float32) {
	off := (scale.Figure(homeLong) - scale.Figure(homeWide)) / 2
	if l.upright {
		if l.home&1 == 0 {
			off = -off
		}
		return geom.NewVec(mid.X+off, mid.Y), 90
	}
	if l.home&2 == 0 {
		off = -off
	}
	return geom.NewVec(mid.X, mid.Y+off), 0
}

// laid says whether a point lies on a cell level lays built with no way across it, while the
// board has no cells yet
func laid(b *board.Plugin, level *board.Level) func(geom.Vec) bool {
	grid := b.Read(0).Board().Grid
	return func(at geom.Vec) bool {
		c, ok := grid.CellAt(at)
		if !ok {
			return false
		}
		name, crossed := level.Kind(c)
		return name == BuiltCell && !crossed
	}
}

// settle stands the buildings of the settlement over the field r: on each built cell of it the
// one its neighbours say, the chapel on its heart where it has litFrom cells or more
func settle(b *board.Plugin, w *buildings.Write, r fieldRow, built func(geom.Vec) bool) {
	bd := b.Read(0).Board()
	from, to := bd.CellCenter(r.From), bd.CellCenter(r.To)
	half := geom.NewVec(CellSize/2, CellSize/2)
	box := geom.AABB{TopLeft: from.Sub(half), BottomRight: to.Add(half)}
	heart, n := heartOf(b, box, built)
	for y := from.Y; y <= to.Y; y += CellSize {
		for x := from.X; x <= to.X; x += CellSize {
			mid := geom.NewVec(x, y)
			if !built(mid) {
				continue
			}
			c, _ := bd.Grid.CellAt(mid)
			name, floors := buildingOn(mid, n >= litFrom && c == heart, built)
			at, turn := lotOf(b, c).stands(mid)
			w.Stand(name, buildings.Row{
				Mid:    [2]float32{at.X, at.Y},
				Turn:   turn,
				Floors: floors,
			})
		}
	}
}

// buildingOn is the kind of building the built cell round mid holds and its storeys: the chapel
// on a settlement's heart, a town's house where all the cells round it are built but one at
// most, a cottage else, of two storeys among four built cells or more
func buildingOn(mid geom.Vec, heart bool, built func(geom.Vec) bool) (name string, floors int) {
	if heart {
		return ChapelKind, 1
	}
	round := 0
	for _, d := range [8][2]float32{
		{-1, -1}, {0, -1}, {1, -1}, {-1, 0}, {1, 0}, {-1, 1}, {0, 1}, {1, 1},
	} {
		if built(geom.NewVec(mid.X+d[0]*CellSize, mid.Y+d[1]*CellSize)) {
			round++
		}
	}
	switch {
	case round >= 7:
		return TownhouseKind, 3
	case round >= 4:
		return HouseKind, 2
	}
	return HouseKind, 1
}
