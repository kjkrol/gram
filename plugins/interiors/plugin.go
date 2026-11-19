package interiors

import (
	"fmt"
	"time"

	"github.com/kjkrol/ceremony/host"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

var _ plugin.Plugin = (*Plugin)(nil)

// Plugin keeps the rooms of the board's cells and who stands in each, and tells the rules as one
// comes in and goes out.
type Plugin struct {
	*world.Self

	worldPlugin *world.Plugin
	board       *board.Plugin
	module      *module
	read        Read

	entered host.Rules[Entered]
	left    host.Rules[Left]

	// derived, found anew as the board's passage changes: the rooms by their cells' storeys
	rooms   map[place]uid.UID64
	roomsAt uint64 // one more than that passage; 0 none
	// counted each step: the rooms an uncovered entity stands in, and the lamps of those under a
	// shelter
	bared map[uid.UID64]bool
	lamps map[uid.UID64][3]float32
}

// a cell's storey a room's floor lies on
type place struct {
	cell   cell.ID
	storey uint8
}

// NewPlugin builds the plugin of the rooms of the board brd's cells in the world w.
func NewPlugin(w *world.Plugin, brd *board.Plugin) *Plugin {
	p := &Plugin{worldPlugin: w, board: brd, rooms: map[place]uid.UID64{},
		bared: map[uid.UID64]bool{}, lamps: map[uid.UID64][3]float32{}}
	p.read.p = p
	p.Self = world.NewSelf(w, p)
	return p
}

func (p *Plugin) Name() string { return "gram.interiors" }

func (p *Plugin) Install(ctx plugin.Installer) error {
	p.module = &module{p: p}
	ctx.UseModule(p.module)
	ctx.Hosts(&p.entered, &p.left)
	return nil
}

// RunPlan finds who stands in which room in the simulation; call it after the board's.
func (p *Plugin) RunPlan(ctx goke.RunCtx, d time.Duration) { p.module.RunPlan(ctx, d) }

func (p *Plugin) EventHandler() control.EventHandler { return nil }

// Serializable is nil: the rooms and who is in them are entities, saved as any.
func (p *Plugin) Serializable() plugin.Serializable { return nil }

func (p *Plugin) String() string { return fmt.Sprintf("interiors, %d rooms", len(p.rooms)) }

// Read reads the rooms on the game's goroutine.
type Read struct{ p *Plugin }

// Read reads the rooms on the game's goroutine.
func (p *Plugin) Read(int) *Read { return &p.read }

// At is the room whose floor lies on the storey of c, its whole, as the last step found.
func (r *Read) At(c cell.ID, storey uint8) (uid.UID64, bool) {
	room, ok := r.p.rooms[place{c, storey}]
	return room, ok
}

// Bared reports whether an entity drawn uncovered stands in the room whose whole is room, as the
// last step found.
func (r *Read) Bared(room uid.UID64) bool { return r.p.bared[room] }

// Lamp is the light of the lamp of the room whose whole is room, its light.Shelter's, as the
// last step found; zero, it has none.
func (r *Read) Lamp(room uid.UID64) [3]float32 { return r.p.lamps[room] }
