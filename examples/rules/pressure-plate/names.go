package main

const (
	GrassCell = "grass"
	PitCell   = "pit"

	OpenEf = "open"

	PlateRole  = "plate"
	MortalRole = "mortal"

	ScoutKind    = "scout"
	WandererKind = "wanderer"
	TrapdoorKind = "trapdoor"
	PlateKind    = "plate"

	PatrolPlan = "patrol"

	PlayerSide = "player"

	PressurePlateStage = "pressure-plate-demo"
	MainScene          = "main"
)

func openCmd(group string) string { return "open " + group }
