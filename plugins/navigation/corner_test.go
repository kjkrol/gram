package navigation

import (
	"math"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/ground"
	"github.com/kjkrol/gram/plugins/board/look"
	"github.com/kjkrol/gram/plugins/navigation/internal/spacing"
)

// sheer is a map level to the eye whose listed steps rise sheer: no hiking paces them
type sheer map[[2]cell.ID]bool

var _ board.Map = sheer{}

func (sheer) Look() look.Look         { return look.Flat }
func (sheer) Heights() ground.Heights { return levelGround{} }
func (sheer) Dressing() look.Dressing { return nil }

func (sheer) Top(cell.ID, float32) (corners [4]float32, level float32) { return corners, 0 }

func (s sheer) Rise(from, to cell.ID) float64 {
	if s[[2]cell.ID{from, to}] {
		return math.Inf(1)
	}
	return 0
}

// the ground's heights, all nought, given so a step's slope counts
type levelGround struct{}

func (levelGround) At(geom.Vec) float32 { return 0 }
func (levelGround) Step() float32       { return fieldCell }

func TestNavigation_CuttingTheCornerOfAnAllowedSlantwiseStepStopsNoUnit(t *testing.T) {
	for _, s := range []Spacing{BodySpacing, CellSpacing} {
		t.Run(map[Spacing]string{BodySpacing: "bodies", CellSpacing: "cells"}[s],
			func(t *testing.T) { crossesTheCorner(t, s) })
	}
}

// the only way to (2,0) is the slantwise step from (1,1): the straight ones into it from the
// corners that step cuts are sheer, so a unit crossing a corner must not be priced by them
func crossesTheCorner(t *testing.T, s Spacing) {
	g := grid.DefaultGrids{}.Square(3, 2, fieldCell)
	start, north, east, goal := g.At(0, 1), g.At(1, 0), g.At(2, 1), g.At(2, 0)
	cliffs := sheer{{north, goal}: true, {east, goal}: true}
	fw := newFieldWorldWith(t, 3, 2, s, nil, []fieldUnit{{
		at:    g.CellCenter(start),
		side:  6,
		paced: true,
		order: &MoveOrder{Target: goal},
	}}, func(p *Plugin) { p.boardPlugin.WithMap(cliffs) })
	stall := int(spacing.StallAfter / (time.Second / 60))
	cut, still := false, 0
	for tick := range 20 * 60 {
		before, _ := fw.centre(0)
		fw.ecs.Tick(time.Second / 60)
		at, o := fw.centre(0)
		c, _ := fw.grid.CellAt(at)
		cut = cut || c == north || c == east
		if o == nil {
			if c != goal || !cut {
				t.Fatalf("tick %d: done in %v, a corner crossed %v; want in %v past one", tick,
					c, cut, goal)
			}
			return
		}
		if still++; at != before {
			still = 0
		}
		if still == stall {
			t.Fatalf("tick %d: stands at %v in %v with its order, a corner crossed %v", tick,
				at, c, cut)
		}
	}
	t.Fatal("never arrived")
}
