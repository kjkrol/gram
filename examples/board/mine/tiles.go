package main

import (
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/render"
)

type grass struct{}

var _ board.Tile = grass{}

func (grass) Define(b *board.Plugin) {
	b.Read(0).CellKinds().Define(GrassCell, cell.Kind{Pace: 1, Allows: cell.Land})
}

func (grass) Look(a *board.Atlas, _ *board.Plugin) {
	a.Add(GrassCell, func(dst *render.Canvas, size int) {
		dst.Fill(grassColor)
		for i := 0; i < size; i += 7 {
			dst.FillRect(float32(i), float32((i*5)%size), 2, 4, bladeColor)
		}
	})
}

// scree is the hill's stony grass
type scree struct{}

var _ board.Tile = scree{}

func (scree) Define(b *board.Plugin) {
	b.Read(0).CellKinds().Define(ScreeCell, cell.Kind{Pace: 0.8, Allows: cell.Land})
}

func (scree) Look(a *board.Atlas, _ *board.Plugin) {
	a.Add(ScreeCell, func(dst *render.Canvas, size int) {
		dst.Fill(screeColor)
		for i := 0; i < size; i += 5 {
			dst.FillRect(float32((i*7)%size), float32(i), 3, 2, stoneColor)
		}
	})
}

// track is the beaten earth up to the mine's mouth
type track struct{}

var _ board.Tile = track{}

func (track) Define(b *board.Plugin) {
	b.Read(0).CellKinds().Define(TrackCell, cell.Kind{Pace: 1, Allows: cell.Land, Graded: true})
}

func (track) Look(a *board.Atlas, _ *board.Plugin) {
	a.Add(TrackCell, func(dst *render.Canvas, size int) {
		dst.Fill(earthColor)
		for i := 0; i < size; i += 6 {
			dst.FillRect(float32(i), float32((i*3)%size), 4, 1, rutColor)
		}
	})
}
