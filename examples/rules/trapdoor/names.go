package main

const (
	GrassCell = "grass"
	PitCell   = "pit"

	OpenEf  = "open"
	HasteEf = "haste"

	MortalRole = "mortal"

	HastenCmd = "hasten"

	ScoutKind    = "scout"
	WandererKind = "wanderer"
	TrapdoorKind = "trapdoor"

	PlayerSide = "player"

	TrapdoorStage = "trapdoor-demo"
	MainScene     = "main"
)

func pullCmd(lever string) string { return "pull " + lever }
