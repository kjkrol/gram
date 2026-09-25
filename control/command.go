package control

import "reflect"

// PlayerID names a player of the game; Nobody is a command nobody in particular gave.
type PlayerID uint8

// Nobody is the PlayerID of a command with no player behind it — a test, a script.
const Nobody PlayerID = 0

// Issued is one command as its owner reads it: who gave it and what it says.
type Issued[C any] struct {
	Player  PlayerID
	Command C
}

// Queue holds the commands of one type until the plugin that owns it drains them in its own pass;
// the owner keeps it as a field and lists it in its plugin.CommandHandler's Queues.
type Queue[C any] struct{ items []Issued[C] }

// CommandQueue is a Queue with its command type erased, as a carrier sorts commands into them.
type CommandQueue interface {
	Accepts() reflect.Type
	Put(player PlayerID, cmd any)
	Clear()
}

var _ CommandQueue = (*Queue[struct{}])(nil)

func (q *Queue[C]) Accepts() reflect.Type        { return reflect.TypeFor[C]() }
func (q *Queue[C]) Put(player PlayerID, cmd any) { q.Add(player, cmd.(C)) }
func (q *Queue[C]) Clear()                       { q.items = q.items[:0] }

// Add puts cmd in as given by player.
func (q *Queue[C]) Add(player PlayerID, cmd C) { q.items = append(q.items, Issued[C]{player, cmd}) }

// Drain hands every command to fn in the order given and empties the queue.
func (q *Queue[C]) Drain(fn func(i Issued[C])) {
	for _, it := range q.items {
		fn(it)
	}
	q.items = q.items[:0]
}

// Empty reports whether nothing is waiting.
func (q *Queue[C]) Empty() bool { return len(q.items) == 0 }
