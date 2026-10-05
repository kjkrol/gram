package control

import (
	"fmt"
	"reflect"

	"github.com/kjkrol/uid"
)

// PlayerID names a player of the game; Nobody is a command nobody in particular gave.
type PlayerID uint8

// Nobody is the PlayerID of a command with no player behind it — a test, a script.
const Nobody PlayerID = 0

// Issued is one command as its owner reads it: who gave it — a player, or with ByEntity an
// entity for itself, its tree — and what it says.
type Issued[C any] struct {
	Player   PlayerID
	Entity   uid.UID64
	ByEntity bool
	Command  C
}

// Queue holds the commands of one type until the plugin that owns it drains them in its own pass;
// the owner keeps it as a field and lists it in its plugin.CommandHandler's Queues.
type Queue[C any] struct{ items []Issued[C] }

// CommandQueue is a Queue with its command type erased, as a carrier sorts commands into them.
type CommandQueue interface {
	Accepts() reflect.Type
	Put(player PlayerID, cmd any)
	PutFrom(entity uid.UID64, cmd any)
	Empty() bool
}

var _ CommandQueue = (*Queue[struct{}])(nil)

func (q *Queue[C]) Accepts() reflect.Type             { return reflect.TypeFor[C]() }
func (q *Queue[C]) Put(player PlayerID, cmd any)      { q.Add(player, cmd.(C)) }
func (q *Queue[C]) PutFrom(entity uid.UID64, cmd any) { q.AddFrom(entity, cmd.(C)) }

// Add puts cmd in as given by player.
func (q *Queue[C]) Add(player PlayerID, cmd C) {
	q.items = append(q.items, Issued[C]{Player: player, Command: cmd})
}

// AddFrom puts cmd in as given by entity for itself.
func (q *Queue[C]) AddFrom(entity uid.UID64, cmd C) {
	q.items = append(q.items, Issued[C]{Entity: entity, ByEntity: true, Command: cmd})
}

// Drain hands every command to fn in the order given and empties the queue.
func (q *Queue[C]) Drain(fn func(i Issued[C])) {
	for _, it := range q.items {
		fn(it)
	}
	q.items = q.items[:0]
}

// Empty reports whether nothing is waiting.
func (q *Queue[C]) Empty() bool { return len(q.items) == 0 }

// Carrier takes each command to the queue of its type: the world keeps the one carrier of a stage,
// for the commands its players give — through the players plugin — and those its entities give
// themselves. Nothing is dropped: a command waits in its queue for its handler's pass.
type Carrier struct{ queues map[reflect.Type]CommandQueue }

// Carry adds queues to those c takes commands to; a type another queue takes already is an error.
func (c *Carrier) Carry(queues ...CommandQueue) error {
	if c.queues == nil {
		c.queues = map[reflect.Type]CommandQueue{}
	}
	for _, q := range queues {
		if other, taken := c.queues[q.Accepts()]; taken && other != q {
			return fmt.Errorf("control: %v has two queues", q.Accepts())
		}
		c.queues[q.Accepts()] = q
	}
	return nil
}

// Put takes cmd, given by player, to the queue of its type; false when none takes it.
func (c *Carrier) Put(player PlayerID, cmd any) bool {
	cmd = Unwrap(cmd)
	q, ok := c.queue(cmd)
	if ok {
		q.Put(player, cmd)
	}
	return ok
}

// PutFrom takes cmd, given by entity for itself, to the queue of its type; false when none takes
// it.
func (c *Carrier) PutFrom(entity uid.UID64, cmd any) bool {
	cmd = Unwrap(cmd)
	q, ok := c.queue(cmd)
	if ok {
		q.PutFrom(entity, cmd)
	}
	return ok
}

// Takes reports whether a queue takes commands of type t.
func (c *Carrier) Takes(t reflect.Type) bool {
	if c == nil {
		return false
	}
	_, ok := c.queues[t]
	return ok
}

// Empty reports whether every command given has been drained.
func (c *Carrier) Empty() bool {
	if c == nil {
		return true
	}
	for _, q := range c.queues {
		if !q.Empty() {
			return false
		}
	}
	return true
}

// Routed is a command written one way and carried as another: Routed is the command its handler
// takes — a rule.Command, by whom it is for.
type Routed interface{ Routed() any }

// Unwrap is cmd as its handler takes it: itself, or what a Routed command says.
func Unwrap(cmd any) any {
	for {
		r, ok := cmd.(Routed)
		if !ok {
			return cmd
		}
		next := r.Routed()
		if reflect.TypeOf(next) == reflect.TypeOf(cmd) {
			return next
		}
		cmd = next
	}
}

// queue is the queue of cmd's type.
func (c *Carrier) queue(cmd any) (CommandQueue, bool) {
	if c == nil {
		return nil, false
	}
	q, ok := c.queues[reflect.TypeOf(cmd)]
	return q, ok
}
