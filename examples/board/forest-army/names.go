package main

import "fmt"

const (
	GrassCell = "grass"
	WaterCell = "water"
	FordCell  = "ford"
	WallCell  = "wall"

	ThinnedEf  = "thinned"
	WoundedEf  = "wounded"
	BlowEf     = "blow"
	ChargingEf = "charging"
	EngagedEf  = "engaged"

	ChargeCmd = "charge"

	ForestRole      = "forest"
	ThinnedRole     = "thinned forest"
	TreeRole        = "tree"
	FellerRole      = "feller"
	MortalRole      = "mortal"
	RedRole         = "red"
	BlueRole        = "blue"
	RedArmyRole     = "red army"
	BlueArmyRole    = "blue army"
	RedSoldierRole  = "red soldier"
	BlueSoldierRole = "blue soldier"

	RedArmyPlan     = "red army"
	BlueArmyPlan    = "blue army"
	RedSoldierPlan  = "red soldier"
	BlueSoldierPlan = "blue soldier"

	ForestKind      = "forest"
	TreeKind        = "tree"
	RedArmyKind     = "red army"
	BlueArmyKind    = "blue army"
	RedSoldierKind  = "red soldier"
	BlueSoldierKind = "blue soldier"

	RedArmyName  = "red army"
	BlueArmyName = "blue army"

	RedSoldiersGroup  = "red soldiers"
	BlueSoldiersGroup = "blue soldiers"

	PlayerSide = "player"

	ForestArmyStage = "board-forest-army"
	MainScene       = "main"
)

func lossEf(k int) string { return fmt.Sprintf("short of %d", k) }

func fightEf(n int) string { return fmt.Sprintf("fighting %d", n) }
