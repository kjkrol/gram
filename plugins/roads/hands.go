package roads

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/ceremony/host"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/ground"
	"github.com/kjkrol/gram/plugins/roads/site"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

// hand is the road in one player's hand: a route through the points marked, its last leg to the
// cell aimed at, or a piece in one cell toward a way out (way, -1 for none); its plan, priced
// and judged by the rules
type hand struct {
	issuer command.Issuer
	points []cell.ID
	ends   []int // len(legs) as each point was marked
	legs   []cell.ID
	to     cell.ID
	aimed  bool
	piece  bool
	at     cell.ID
	way    int

	cells   []cell.ID // what was routed, drawn where no plan came of it
	plan    site.Estimate
	planned bool
	moves   map[corner]float32
	price   float32
	err     error
	reason  string
	refused string
	asked   bool
	seen    uint64
	says    string // the estimate's words, made anew as the plan or the refusal changes
	said    string
	stale   bool
}

// forget is the ticks a hand nobody aims is kept: the cursor off the picture a moment
const forget = 30

var errChanged = errors.New("the ground changed since: plan it anew")

func (h *hand) fail(err error) {
	h.err, h.reason = err, ""
	if err != nil {
		h.reason = err.Error()
	}
}

func (p *Plugin) carryOut(d time.Duration) {
	p.tick++
	// a Refuse is the answer of the rules judging a hand, given while they judge it
	p.refusals.Drain(func(command.Issued[Refuse]) {})
	p.levels.Drain(func(i command.Issued[Levelling]) {
		if p.Res.Settings.Levelling == i.Command.On {
			return
		}
		p.Res.Settings.Levelling = i.Command.On
		for _, h := range p.hands {
			p.replan(h)
		}
	})
	p.aims.Drain(func(i command.Issued[Aim]) { p.aim(p.handOf(i.Issuer), i.Command.At) })
	p.marks.Drain(func(i command.Issued[Mark]) { p.mark(p.handOf(i.Issuer), i.Command.At) })
	p.pieces.Drain(func(i command.Issued[Piece]) { p.place(p.handOf(i.Issuer), i.Command) })
	p.builds.Drain(func(i command.Issued[Build]) {
		if h := p.handFor(i.Issuer); h != nil && h.asked {
			p.build(h)
		}
	})
	p.cancels.Drain(func(i command.Issued[Cancel]) {
		if h := p.handFor(i.Issuer); h != nil {
			p.cancel(h)
		}
	})
	p.hands = slices.DeleteFunc(p.hands, func(h *hand) bool {
		return !h.asked && p.tick-h.seen > forget
	})
	p.judge(d)
}

func (p *Plugin) handFor(id command.Issuer) *hand {
	for _, h := range p.hands {
		if h.issuer == id {
			return h
		}
	}
	return nil
}

func (p *Plugin) handOf(id command.Issuer) *hand {
	h := p.handFor(id)
	if h == nil {
		h = &hand{issuer: id, way: -1}
		p.hands = append(p.hands, h)
		p.hold(id)
	}
	h.seen = p.tick
	return h
}

// hold has the game stand in the tactical pause as a road is taken in hand, where the Config
// says; nothing lets it go again but the players
func (p *Plugin) hold(id command.Issuer) {
	if !p.cfg.InPause || p.paused == p.tick || p.worldPlugin.Read(0).Clock().State().Paused {
		return
	}
	p.paused = p.tick
	p.worldPlugin.Write().Carrier().Put(id, world.Pause{})
}

func (p *Plugin) aim(h *hand, at geom.Vec) {
	if h.asked || h.piece || len(h.points) == 0 {
		return
	}
	c, ok := p.board.Read(0).Board().CellAt(at)
	switch {
	case !ok || c == h.points[len(h.points)-1]:
		if h.aimed {
			h.aimed = false
			p.replan(h)
		}
	case !h.aimed || c != h.to:
		h.to, h.aimed = c, true
		p.replan(h)
	}
}

func (p *Plugin) mark(h *hand, at geom.Vec) {
	c, ok := p.board.Read(0).Board().CellAt(at)
	if !ok || h.asked {
		return
	}
	h.piece = false
	n := len(h.points)
	switch {
	case n == 0:
		h.points, h.legs = append(h.points[:0], c), append(h.legs[:0], c)
		h.ends = append(h.ends[:0], 1)
	case c == h.points[n-1]:
		h.points, h.ends = h.points[:n-1], h.ends[:n-1]
		if n > 1 {
			h.legs = h.legs[:h.ends[n-2]]
		} else {
			h.legs = h.legs[:0]
		}
	default:
		leg, err := p.routerOf(p.readied()).Route(h.points[n-1], c, p.leg[:0])
		p.leg = leg
		if err != nil {
			h.fail(err)
			return
		}
		h.points, h.legs = append(h.points, c), joined(h.legs, leg)
		h.ends = append(h.ends, len(h.legs))
	}
	h.aimed = false
	p.replan(h)
	h.asked = h.planned && len(h.points) > 1
}

func (p *Plugin) place(h *hand, cmd Piece) {
	c, ok := p.board.Read(0).Board().CellAt(cmd.At)
	if !ok || h.asked {
		return
	}
	h.points, h.legs, h.ends, h.aimed = h.points[:0], h.legs[:0], h.ends[:0], false
	h.piece, h.at, h.way = true, c, p.wayOut(c, cmd.Toward)
	p.replan(h)
	h.asked = h.planned
}

// wayOut is the way out of c toward the point toward: to the neighbour under it, else the
// nearest; -1 for toward in c or past the board's edge
func (p *Plugin) wayOut(c cell.ID, toward geom.Vec) int {
	g := p.board.Read(0).Board().Grid
	if t, ok := g.CellAt(toward); ok {
		if t == c {
			return -1
		}
		if i, ok := grid.Way(g, c, t); ok {
			return i
		}
	}
	m := g.CellCenter(c)
	i := grid.Nearest(g, geom.NewVec(toward.X-m.X, toward.Y-m.Y))
	if _, ok := g.Toward(c, i); !ok {
		return -1
	}
	return i
}

func (p *Plugin) build(h *hand) {
	if h.refused != "" {
		return
	}
	shown := h.plan
	p.replan(h)
	switch {
	case !h.planned:
	case !same(shown, h.plan):
		h.fail(errChanged)
	case len(h.plan.Blocked) > 0:
		h.fail(site.Refuse(p.board.Read(0).Board().Grid, h.plan.Blocked[0],
			"levels the ground under a unit: build it once the unit moves off"))
	default:
		p.Res.Orders = append(p.Res.Orders,
			Order{Issuer: h.issuer, Estimate: h.plan, Price: h.price})
		*h = hand{issuer: h.issuer, way: -1, seen: h.seen, stale: true}
	}
}

// same reports whether b is the plan a was: its cells, its corners and its earth
func same(a, b site.Estimate) bool {
	return slices.Equal(a.Cells, b.Cells) && slices.Equal(a.Corners, b.Corners) &&
		a.Cut == b.Cut && a.Fill == b.Fill
}

func (p *Plugin) cancel(h *hand) {
	h.asked = false
	if h.piece {
		h.piece = false
		p.replan(h)
	}
}

// replan plans what h holds anew, asking nothing while there is no plan
func (p *Plugin) replan(h *hand) {
	h.plan, h.planned, h.moves, h.price = site.Estimate{}, false, nil, 0
	h.refused, h.stale = "", true
	plan, planned, err := p.planOf(h)
	h.fail(err)
	if err != nil || !planned {
		h.asked = false
		return
	}
	h.plan, h.planned, h.moves = plan, true, p.movesOf(plan, nil)
	if p.cfg.Tariff != nil {
		h.price = p.cfg.Tariff(plan)
	}
}

// planOf is the plan of what h holds; false where it holds less than a road
func (p *Plugin) planOf(h *hand) (site.Estimate, bool, error) {
	g := p.readied()
	if h.piece {
		h.cells = []cell.ID{h.at}
		e, err := p.pieceIn(g, h.at, h.way)
		return e, err == nil, err
	}
	// a new slice every plan: site's Estimate may hold on to the cells it is given
	cells := make([]cell.ID, 0, len(h.legs)+len(p.leg))
	cells = append(cells, h.legs...)
	if h.aimed {
		leg, err := p.routerOf(g).Route(h.points[len(h.points)-1], h.to, p.leg[:0])
		p.leg = leg
		if err != nil {
			h.cells = cells
			return site.Estimate{}, false, err
		}
		cells = joined(cells, leg)
	}
	h.cells = cells
	if len(cells) < 2 {
		return site.Estimate{}, false, nil
	}
	e, err := p.plan(g, p.cfg.Rules, cells, p.cfg.Kind, p.width(), p.kind().Graded, p.levelling(),
		p.cfg.MaxStep, nil)
	return e, err == nil, err
}

// joined is the cells of leg after those of dst, the cell where they meet once
func joined(dst, leg []cell.ID) []cell.ID {
	if len(leg) > 0 && len(dst) > 0 && leg[0] == dst[len(dst)-1] {
		leg = leg[1:]
	}
	return append(dst, leg...)
}

// pieceIn is the plan of a piece of road in c running out by way and into the roads ending
// toward c, its links given to site.Plan as the cell's extra ones and laid in the plan here too,
// a Graded piece checked to lie as a road on the corners the plan leaves.
func (p *Plugin) pieceIn(g *boardGround, c cell.ID, way int) (site.Estimate, error) {
	r := p.cfg.Rules
	k := p.kind()
	gr := g.Grid()
	links, joins := g.Road(c), cell.Links(0)
	if r.Crossings {
		joins = g.joins(c)
	}
	links |= joins
	if way >= 0 {
		links |= 1 << way
	}
	if links == 0 {
		return site.Estimate{}, site.Refuse(gr, c, "joins no road: drag it toward where it runs")
	}
	e, err := p.plan(g, r, []cell.ID{c}, p.cfg.Kind, p.width(), k.Graded, p.levelling(),
		p.cfg.MaxStep, []cell.Links{links})
	if err != nil {
		return site.Estimate{}, err
	}
	e, laid := p.linked(e, c, links, joins != 0)
	if k.Graded && p.board.Read(0).Heights() != nil {
		p.outline, p.heights = p.heightsOf(c, p.movesOf(e, nil), p.outline, p.heights)
		if !ground.RoadFits(gr, c, p.heights, laid) {
			return site.Estimate{}, site.Refuse(gr, c,
				"would bend, fork or end on a slope: a road lies level there")
		}
	}
	return e, nil
}

// linked is e with links laid in c, its cell, and the links its way runs on then; joined says
// whether c meets the roads standing
func (p *Plugin) linked(e site.Estimate, c cell.ID, links cell.Links,
	joined bool) (site.Estimate, cell.Links) {
	if k := slices.Index(e.Cells, c); k >= 0 && len(e.Links) == len(e.Cells) {
		e.Links = slices.Clone(e.Links)
		e.Links[k] |= links
	} else {
		e.Cells, e.Links = []cell.ID{c}, []cell.Links{links}
	}
	e.Ways = slices.Clone(e.Ways)
	k := slices.IndexFunc(e.Ways, func(w cell.WayEntry) bool { return w.Cell == c })
	if k < 0 {
		k = len(e.Ways)
		e.Ways = append(e.Ways, cell.WayEntry{Kind: p.cfg.Kind, Cell: c, Width: p.width()})
	}
	e.Ways[k].Links |= links
	if joined && !slices.Contains(e.Crossings, c) {
		e.Crossings = append(slices.Clone(e.Crossings), c)
	}
	return e, e.Ways[k].Links
}

// judge has the rules of Estimated judge every plan in hand, a Refuse given while they judge
// one its refusal, and words the estimates anew where they changed
func (p *Plugin) judge(d time.Duration) {
	ruled := !p.estimated.Empty()
	var tick host.Tick
	if ruled {
		tick = p.worldPlugin.Read(0).Tick(d)
	}
	for _, h := range p.hands {
		if ruled && h.planned {
			h.refused = ""
			p.estimated.Run(tick, Estimated{Self: p.Entity(), Estimate: h.plan, Price: h.price})
			p.refusals.Drain(func(i command.Issued[Refuse]) {
				if h.refused == "" {
					h.refused = i.Command.Why
				}
			})
		}
		if h.stale || h.refused != h.said {
			h.says, h.said, h.stale = p.words(h), h.refused, false
		}
	}
}

func (p *Plugin) words(h *hand) string {
	if !h.planned {
		return ""
	}
	e := h.plan
	var b strings.Builder
	n := len(e.Cells)
	if n == 1 {
		b.WriteString("It lies in one cell")
	} else {
		fmt.Fprintf(&b, "It runs %d cells", n)
	}
	switch {
	case e.Laid == 0:
		b.WriteString(", a road already")
	case e.Laid < n:
		fmt.Fprintf(&b, ", %d of them laid anew", e.Laid)
	}
	if e.Cut > 0 || e.Fill > 0 {
		fmt.Fprintf(&b, ", digging %.0f and raising %.0f of earth", e.Cut, e.Fill)
	}
	b.WriteString(".")
	if p.cfg.Tariff != nil {
		fmt.Fprintf(&b, " It costs %.0f.", h.price)
	}
	if len(e.Blocked) > 0 {
		b.WriteString(" Something stands where it levels.")
	}
	if h.refused != "" {
		fmt.Fprintf(&b, " Refused: %s.", strings.TrimSuffix(h.refused, "."))
	}
	return b.String()
}

// text says one line about the roads in hand, the first hand's that has it to say, for the
// window and the reason
type text struct {
	p   *Plugin
	say func(h *hand) (string, bool)
}

func (t text) Text(uid.UID64, bool) (string, bool) {
	for _, h := range t.p.hands {
		if s, ok := t.say(h); ok {
			return s, true
		}
	}
	return "", false
}

func askText(h *hand) (string, bool) { return "Build the road?", h.asked }

func estimateText(h *hand) (string, bool) { return h.says, h.asked && h.planned }

func reasonText(h *hand) (string, bool) { return h.reason, h.err != nil }
