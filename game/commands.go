package game

import "github.com/kjkrol/gram/control"

// Save is the command to save the active Stage in the save Slot, "" the quicksave
// (Runtime.Save).
type Save struct{ Slot string }

// Load is the command to enter Stage — the active one for "" — from its save Slot, "" the
// quicksave: the Stage built anew, its world loaded instead of spawned (Runtime.Load).
type Load struct{ Stage, Slot string }

// Enter is the command to enter Stage afresh (Runtime.SwitchStage).
type Enter struct{ Stage string }

// Quit is the command to end the game (Runtime.Quit).
type Quit struct{}

func (Save) Command()  {}
func (Load) Command()  {}
func (Enter) Command() {}
func (Quit) Command()  {}

var (
	_ control.Command = Save{}
	_ control.Command = Load{}
	_ control.Command = Enter{}
	_ control.Command = Quit{}
)
