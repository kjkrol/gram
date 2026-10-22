package weathering

import (
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/atmosphere/air"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/world"
)

func TestWeathering_SnowFallsOnlyUnderTheSnowingMass(t *testing.T) {
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 640, Height: 480},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 8},
		Heights:  true,
	})
	brd := board.NewPlugin(grid.DefaultGrids{}.Square(20, 15, 32), &cell.MultipleOccupancy{}, w)
	dome := air.NewDome(640, 480, 2000)
	dome.Own()
	// a snowing mass over the west, two of the dome's cells deep
	dome.Carry([2]float32{}, func(x, _ float32) (air.Mass, int16) {
		if x < 2*air.BoardCell {
			return air.Mass{Cover: 1, Billow: 1, Falls: 1}, 0
		}
		return air.Mass{}, 0
	}, true)
	weather := air.Weather{Dome: dome, Temperature: -5}
	wt := &Weathering{board: brd}
	var under, beyond float32
	for x := range 20 {
		for y := range 15 {
			c, _ := brd.Read(0).Board().CellAt(geom.NewVec(float64(x*32+16), float64(y*32+16)))
			s := wt.snowing(weather, c)
			switch {
			case x*32+32 <= 1.5*air.BoardCell:
				under += s
			case x*32 >= 2.5*air.BoardCell:
				beyond += s
			}
		}
	}
	if under <= 0 || beyond != 0 {
		t.Errorf("snow falls %v on the cells under the snowing mass and %v beyond it, want it on "+
			"the first alone", under, beyond)
	}
	weather.Temperature = 5
	corner, _ := brd.Read(0).Board().CellAt(geom.NewVec(16, 16))
	if s := wt.snowing(weather, corner); s != 0 {
		t.Errorf("in the warmth %v snow falls, want rain alone", s)
	}
}
