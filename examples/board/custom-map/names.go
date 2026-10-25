package main

const (
	GrassCell  = "grass"
	RiseCell   = "grass-rise"
	SlopeCell  = "grass-slope"
	SummitCell = "grass-summit"

	UnitKind = "unit"

	PlayerSide = "player"

	CustomMapStage = "board-custom-map"
	MainScene      = "main"
)

// the grass's shades, from the flat to the summits
var grassShades = []string{GrassCell, RiseCell, SlopeCell, SummitCell}
