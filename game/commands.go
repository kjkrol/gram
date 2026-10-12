package game

import "github.com/kjkrol/gram/control"

// Save is the command to save the active Stage in the save Slot, "" the quicksave.
type Save struct{ Slot string }

// Load is the command to enter Stage ("" the active one) from its save Slot ("" the quicksave),
// its world loaded instead of spawned.
type Load struct{ Stage, Slot string }

// Enter is the command to enter Stage afresh.
type Enter struct{ Stage string }

// Quit is the command to end the game.
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
