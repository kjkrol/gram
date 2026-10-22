package players

import (
	"fmt"

	"github.com/kjkrol/ceremony/command"
)

// Read reads the players, the same on every goroutine.
type Read struct{ p *Plugin }

// Read reads the players on worker w of the game's pool, 0 the game's own goroutine.
func (p *Plugin) Read(int) *Read { return &p.read }

// Write changes the players, on the game's goroutine.
type Write struct{ p *Plugin }

// Write changes the players, on the game's goroutine.
func (p *Plugin) Write() *Write { return &p.write }

// Players lists every player, in the order added.
func (r *Read) Players() []*Player { return r.p.players }

// Locals lists the players at this keyboard.
func (r *Read) Locals() []*Player {
	p := r.p
	var out []*Player
	for _, pl := range p.players {
		if pl.local {
			out = append(out, pl)
		}
	}
	return out
}

// ByID is the player with id, or nil — for Nobody too.
func (r *Read) ByID(id command.Issuer) *Player {
	p := r.p
	if id == command.Nobody || int(id) > len(p.players) {
		return nil
	}
	return p.players[id-1]
}

// Issue gives cmd as player (nil for Nobody); ErrUnknownCommand when no handler defines its
// type.
func (w *Write) Issue(player *Player, cmd command.Command) error {
	p := w.p
	id := command.Nobody
	if player != nil {
		id = player.ID
	}
	if !p.worldPlugin.Write().Carrier().Put(id, cmd) {
		return fmt.Errorf("%w: %T", ErrUnknownCommand, cmd)
	}
	return nil
}
