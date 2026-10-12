package engine

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"

	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/plugin"
)

// commands are the engine's own queues — the game package's commands — in every Stage's carrier,
// carried out between the input and the ticks.
type commands struct {
	saves  control.Queue[game.Save]
	loads  control.Queue[game.Load]
	enters control.Queue[game.Enter]
	quits  control.Queue[game.Quit]
}

var _ plugin.CommandHandler = (*commands)(nil)

func (c *commands) Queues() []control.CommandQueue {
	return []control.CommandQueue{&c.saves, &c.loads, &c.enters, &c.quits}
}

func (*commands) DefaultBindings() []control.Binding { return nil }

// carry carries out the engine's commands given since the last frame.
func (e *Engine) carry() {
	e.commands.saves.Drain(func(i control.Issued[game.Save]) {
		if err := e.Save(i.Command.Slot); err != nil {
			log.Printf("gram: save: %v", err)
		}
	})
	e.commands.loads.Drain(func(i control.Issued[game.Load]) {
		if err := e.Load(i.Command.Stage, i.Command.Slot); err != nil {
			log.Printf("gram: load: %v", err)
		}
	})
	e.commands.enters.Drain(func(i control.Issued[game.Enter]) {
		if err := e.SwitchStage(i.Command.Stage); err != nil {
			log.Printf("gram: enter: %v", err)
		}
	})
	e.commands.quits.Drain(func(control.Issued[game.Quit]) { e.Quit() })
}

// Save saves the active Stage in the save slot, "" the quicksave, where Props.Saves says.
func (e *Engine) Save(slot string) error {
	if e.props.Saves != "" {
		if err := os.MkdirAll(e.props.Saves, 0o755); err != nil {
			return err
		}
	}
	name := e.current.stage.Name()
	if err := e.Persistence().Save(e.savePath(name), slot); err != nil {
		return err
	}
	log.Printf("gram: saved %q in %s", name, filePath(e.savePath(name), slot))
	return nil
}

// Load enters stage — the active one for "", the starting one before Init — from its save slot at
// the start of the next Update, or in Init; a save not there is an error, and nothing changes.
func (e *Engine) Load(stage, slot string) error {
	stages, initial := e.game.Stages()
	switch {
	case stage != "":
	case e.current != nil:
		stage = e.current.stage.Name()
	default:
		stage = initial
	}
	if _, ok := stages[stage]; !ok {
		return fmt.Errorf("gram: unknown stage %q", stage)
	}
	saves, err := listSaves(e.savePath(stage))
	if err != nil {
		return err
	}
	if !slices.Contains(saves, slot) {
		return fmt.Errorf("gram: stage %q has no save %q in %q", stage, slot, e.props.Saves)
	}
	e.pendingSwitch, e.pendingLoad = stage, &slot
	return nil
}

// Issue gives cmd as nobody to whoever carries it out in the active Stage.
func (e *Engine) Issue(cmd control.Command) error {
	if !e.current.carrier.Put(control.Nobody, cmd) {
		return fmt.Errorf("gram: nobody carries out %T in stage %q", cmd, e.current.stage.Name())
	}
	return nil
}

// savePath is where the saves of the Stage called name lie.
func (e *Engine) savePath(name string) string { return filepath.Join(e.props.Saves, name) }
