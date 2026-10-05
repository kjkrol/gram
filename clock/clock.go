package clock

import (
	"slices"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/uid"
)

// State is the tactical clock as it stands, saved with the game on the clock's entity: the game
// time gone by — the sum of the simulation's steps — the tempo it goes at, and whether it stands
// in the tactical pause.
type State struct {
	Time   time.Duration
	Tempo  float32
	Paused bool
}

// Phase is the family of the clock's tags: what effects cast on the clock switch on and off —
// night, winter — for a rule to run only while it holds (Clock.In). A game defines its phases with
// world.Kinds.DefineTag[clock.Phase].
type Phase struct{}

// Config is how the clock goes: the Tempos it can be set to, in order, with 1 among them
// (½, 1, 2 and 4 by default), and whether a tempo above 1 is BiggerStep — one step of the
// simulation as long as tempo steps — rather than as many steps of the same length.
type Config struct {
	Tempos     []float32
	BiggerStep bool
}

// defaultTempos are the tempos the clock offers unless the Config says otherwise.
var defaultTempos = []float32{0.5, 1, 2, 4}

// Block is a piece of the simulation: what a plugin runs under Simulate, as many times a tick as
// the clock says, each time over step.
type Block func(ctx goke.RunCtx, step time.Duration)

// Clock is the tactical clock of a world: the one time everything that simulates goes by. A plugin
// hands it what simulates (Simulate); after the game's Update the engine has it replay all of it as
// many times as the tempo says (Replay) — none in the tactical pause — moving game time on a step
// each time. The State is kept on the clock's own entity, so a save carries it.
type Clock struct {
	cfg   Config
	state State

	entity  uid.UID64 // the clock's entity, once the system found or made it
	blocks  []Block
	carry   float32 // the part of a step the tempo did not fill this tick
	step    time.Duration
	pending time.Duration // the real time the engine holds toward the next tick
	behind  int           // ticks in a row the engine fell behind
	slowed  bool          // the tempo was lowered for that
	system  *system
	changed func(State) // told after a command changed the state
}

// New makes a clock at tempo 1, not paused, of cfg.
func New(cfg Config) *Clock {
	if len(cfg.Tempos) == 0 {
		cfg.Tempos = slices.Clone(defaultTempos)
	}
	if !slices.Contains(cfg.Tempos, 1) {
		panic("clock: the tempos must hold 1")
	}
	c := &Clock{cfg: cfg, state: State{Tempo: 1}}
	c.system = &system{c: c}
	return c
}

// State is the clock as it stands: what its entity carries. For the world, which makes that
// entity as its own.
func (c *Clock) State() State { return c.state }

// Time is the game time gone by.
func (c *Clock) Time() time.Duration { return c.state.Time }

// In reports whether the clock is in phase: its entity carries the tag, as of this step of the
// simulation — an effect granting it in one step is seen in the next.
func (c *Clock) In(phase tag.Tag[Phase]) bool {
	s := c.system
	if s.query == nil {
		return false
	}
	for s.query.All(); s.query.Next(); {
		cursor := s.query.Cursor()
		return s.tags.Present(cursor) && s.tags.Slice(cursor)[0].Has(phase)
	}
	return false
}

// Entity is the clock's entity — what a Phase is granted to, by an effect cast on it; 0 before the
// game is set up.
func (c *Clock) Entity() uid.UID64 { return c.entity }

// Simulate hands the clock a piece of the simulation for this tick: block runs at Replay, as many
// times as the tempo says, in the order the pieces came. Call it from a plugin's RunPlan for the
// systems that must stand in the tactical pause and go with the tempo.
func (c *Clock) Simulate(_ goke.RunCtx, block Block) { c.blocks = append(c.blocks, block) }

// Replay runs every piece handed to Simulate this tick — 0 times in the tactical pause, every
// other tick at ½, 2 or 4 times at 2 or 4 — over step, moving game time on with each; the engine
// calls it after the game's Update. A BiggerStep clock runs them once, over step times the tempo.
func (c *Clock) Replay(ctx goke.RunCtx, step time.Duration) {
	defer func() { c.blocks = c.blocks[:0] }()
	c.step = step
	if c.state.Paused {
		c.carry = 0
		return
	}
	if c.cfg.BiggerStep && c.state.Tempo > 1 {
		c.run(ctx, time.Duration(float64(step)*float64(c.state.Tempo)))
		return
	}
	c.carry += c.state.Tempo
	for ; c.carry >= 1; c.carry-- {
		c.run(ctx, step)
	}
}

func (c *Clock) run(ctx goke.RunCtx, step time.Duration) {
	for _, b := range c.blocks {
		b(ctx, step)
	}
	c.state.Time += step
}

// Behind tells the clock the engine could not run every tick this frame: a tempo above 1 held
// for a while past that comes down a notch, and the report says so.
func (c *Clock) Behind(behind bool) {
	if !behind {
		c.behind = 0
		return
	}
	c.behind++
	if c.behind < behindFor || c.state.Tempo <= 1 {
		return
	}
	c.behind = 0
	if i := slices.Index(c.cfg.Tempos, c.state.Tempo); i > 0 {
		c.state.Tempo, c.slowed = c.cfg.Tempos[i-1], true
		c.tell()
	}
}

// Pending tells the clock how much real time the engine holds toward its next tick, each frame.
func (c *Clock) Pending(d time.Duration) { c.pending = d }

// Shown is the game time what is drawn goes by: Time, and past it the part of a step the tempo
// has filled and the real time held toward the next tick at the tempo, so it moves every frame.
func (c *Clock) Shown() time.Duration {
	if c.state.Paused {
		return c.state.Time
	}
	ahead := float64(c.pending) * float64(c.state.Tempo)
	if !c.cfg.BiggerStep || c.state.Tempo <= 1 {
		ahead += float64(c.carry) * float64(c.step)
	}
	return c.state.Time + time.Duration(ahead)
}

// behindFor is how many frames in a row the engine must fall behind before the tempo comes down.
const behindFor = 30

func (c *Clock) tell() {
	if c.changed != nil {
		c.changed(c.state)
	}
}

// TogglePause holds the simulation in the tactical pause, or lets it go on.
func (c *Clock) TogglePause() { c.state.Paused = !c.state.Paused; c.tell() }

// Faster moves the tempo a notch up the Config's list, Slower a notch down; neither past its end.
func (c *Clock) Faster() { c.shift(1) }

// Slower — see Faster.
func (c *Clock) Slower() { c.shift(-1) }

func (c *Clock) shift(by int) {
	i := slices.Index(c.cfg.Tempos, c.state.Tempo) + by
	if i < 0 || i >= len(c.cfg.Tempos) {
		return
	}
	c.state.Tempo, c.slowed = c.cfg.Tempos[i], false
	c.tell()
}

// System is the clock's own system, for the world's module to register and run in the interface
// part of its tick: it finds the clock's entity or makes one, carries out the commands and keeps
// the entity's State as the clock's.
func (c *Clock) System() goke.System { return c.system }

// system keeps the clock's entity: the State on it is what a save carries, the Phase tags on it
// what effects set, read by In.
type system struct {
	c     *Clock
	query *goke.Query
	state goke.Comp[State]
	tags  goke.OptComp[tag.Tags[Phase]]
	spawn goke.Comp[State]
}

func (s *system) Init(si *goke.SysInit) {
	s.query = si.NewQueryBuilder(&s.state).Optional(&s.tags).Build()
	for s.query.All(); s.query.Next(); {
		cursor := s.query.Cursor()
		s.c.state, s.c.entity = s.state.Slice(cursor)[0], cursor.IDs[0] // a loaded game brought its clock
		return
	}
	f := si.NewFactory(&s.spawn)
	f.Create(1)
	for f.Next() {
		s.spawn.Slice(&f.Cursor)[0] = s.c.state
		s.c.entity = f.Cursor.IDs[0]
	}
}

func (s *system) Update(_ *goke.CmdBuf, _ time.Duration) {
	for s.query.All(); s.query.Next(); {
		s.state.Slice(s.query.Cursor())[0] = s.c.state
		return
	}
}

// Simulate hands block to c, or runs it at once over step where c is nil: a module run on its
// own, without a world — in a test.
func Simulate(c *Clock, ctx goke.RunCtx, step time.Duration, block Block) {
	if c == nil {
		block(ctx, step)
		return
	}
	c.Simulate(ctx, block)
}
