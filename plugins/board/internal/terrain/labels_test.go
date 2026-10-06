package terrain_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity"
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/internal/boardtest"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/uid"
)

// cellProbe reads every cell entity's roles, label and fuel once the cells are made.
type cellProbe struct {
	q     *goke.Query
	plot  goke.Comp[cell.Plot]
	roles goke.OptComp[tag.Tags[rule.Roles]]
	label goke.OptComp[entity.Label]
	fuel  goke.OptComp[fuel]
}

// fuel is a game's own component on every cell, given through the world's roster.
type fuel struct{ Left int }

func (p *cellProbe) SetupSystems() []goke.System {
	return []goke.System{goke.SystemFn{OnInit: func(si *goke.SysInit) {
		p.q = si.NewQueryBuilder(&p.plot).Optional(&p.roles, &p.label, &p.fuel).Build()
	}}}
}

// cellState is what a cell entity carries of roles and of what it is called.
type cellState struct {
	id       uid.UID64
	hasRoles bool
	roles    tag.Tags[rule.Roles]
	labelled bool
	label    entity.Label
	fuel     *fuel // nil for none
}

// read is every cell entity's state, by its cell.
func (p *cellProbe) read(t *testing.T) map[cell.ID]cellState {
	t.Helper()
	out := map[cell.ID]cellState{}
	for p.q.All(); p.q.Next(); {
		cur := p.q.Cursor()
		roles, labels, fuels := p.roles.Slice(cur), p.label.Slice(cur), p.fuel.Slice(cur)
		for i, plot := range p.plot.Slice(cur) {
			st := cellState{id: cur.IDs[i], hasRoles: roles != nil, labelled: labels != nil}
			if fuels != nil {
				st.fuel = &fuels[i]
			}
			if roles != nil {
				st.roles = roles[i]
			}
			if labels != nil {
				st.label = labels[i]
			}
			if _, dup := out[plot.Cell]; dup {
				t.Fatalf("two entities hold cell %d", plot.Cell)
			}
			out[plot.Cell] = st
		}
	}
	return out
}

// installCells installs w and brd alone, with the probe's query, and sets the ECS up; prepare
// runs before, to define roles and seed the Layout.
func installCells(t *testing.T, g grid.Grid, prepare func(w *world.Plugin, brd *board.Plugin)) (*goke.ECS, *world.Plugin, *board.Plugin, *cellProbe) {
	t.Helper()
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 4 * boardtest.CellSize, Height: 4 * boardtest.CellSize},
		Entities: world.EntitiesCfg{MaxCount: 4, MinSize: boardtest.UnitSize, MaxSize: boardtest.UnitSize},
	})
	brd := board.NewPlugin(g, &cell.MultipleOccupancy{}, w)
	brd.Res.Logic.Board.SetAll(cell.Kind{Name: cell.Named("grass"), Cost: 1, Allows: cell.Land})
	prepare(w, brd)
	if err := brd.Populate(); err != nil {
		t.Fatal(err)
	}
	ctx := boardtest.NewInstallCtx()
	if err := w.Install(ctx); err != nil {
		t.Fatal(err)
	}
	if err := brd.Install(ctx); err != nil {
		t.Fatal(err)
	}
	probe := &cellProbe{}
	ctx.ECS().Setup(append(ctx.Systems(), probe.SetupSystems()...)...)
	return ctx.ECS(), w, brd, probe
}

func cellGrids() map[string]grid.Grid {
	return map[string]grid.Grid{
		"square": grid.DefaultGrids{}.Square(4, 4, boardtest.CellSize),
		"hex":    grid.DefaultGrids{}.Hex(4, 4, boardtest.CellSize/2),
	}
}

// The kinds' roles and the Layout's names and groups reach the cell entities: every cell carries its roles, none
// for most; only the cells called something carry a Label; and the cells labelled apart are still
// each its own cell's entity, of the kind the seed gave it.
func TestCells_RolesAndLabelsReachTheCellEntities(t *testing.T) {
	for name, g := range cellGrids() {
		t.Run(name, func(t *testing.T) {
			var trapdoor, plate *rule.Part
			cells := make([]cell.ID, 0, 5)
			g.EachCell(func(c cell.ID) {
				if len(cells) < 5 {
					cells = append(cells, c)
				}
			})
			a, b, c, d, e := cells[0], cells[1], cells[2], cells[3], cells[4]
			snow := cell.Kind{Name: cell.Named("snow"), Cost: 3, Allows: cell.Land}
			_, _, brd, probe := installCells(t, g, func(w *world.Plugin, brd *board.Plugin) {
				w.Roles().Define("trapdoor")
				w.Roles().Define("plate")
				trapdoor, plate = w.Roles().Named("trapdoor"), w.Roles().Named("plate")
				land := func() cell.Kind { return cell.Kind{Cost: 1, Allows: cell.Land} }
				brd.CellKinds().Define("door", land(), trapdoor)
				brd.CellKinds().Define("plate", land(), plate)
				brd.CellKinds().Define("both", land(), trapdoor, plate)
				brd.Res.Logic.Board.Set(e, snow)
				brd.Seed(board.Layout{Cells: []cell.Entry{
					{Kind: "door", Cell: a, Group: "west"},
					{Kind: "plate", Cell: b, Name: "plate", Group: "west"},
					{Kind: "both", Cell: c, Group: "east"},
					{Kind: "door", Cell: d},
					{Cell: e, Name: "lever"},
				}})
			})
			want := map[cell.ID]cellState{
				a: {hasRoles: true, roles: tag.Tags[rule.Roles](0).With(trapdoor.Tag()), labelled: true, label: entity.LabelOf("", "west")},
				b: {hasRoles: true, roles: tag.Tags[rule.Roles](0).With(plate.Tag()), labelled: true, label: entity.LabelOf("plate", "west")},
				c: {hasRoles: true, roles: tag.Tags[rule.Roles](0).With(trapdoor.Tag(), plate.Tag()), labelled: true, label: entity.LabelOf("", "east")},
				d: {hasRoles: true, roles: tag.Tags[rule.Roles](0).With(trapdoor.Tag())},
				e: {hasRoles: true, labelled: true, label: entity.LabelOf("lever", "")},
			}
			got := probe.read(t)
			if len(got) != g.CellCount() {
				t.Fatalf("%d cell entities, want one per cell, %d", len(got), g.CellCount())
			}
			for at, st := range got {
				w, ok := want[at]
				if !ok {
					w = cellState{hasRoles: true}
				}
				w.id = st.id
				if st != w {
					t.Errorf("cell %d carries %+v, want %+v", at, st, w)
				}
				if id, _ := brd.CellEntity(at); id != st.id {
					t.Errorf("cell %d's entity is %d, the one holding its Plot %d", at, id, st.id)
				}
			}
			if k := brd.Res.Logic.Board.Kind(e); k != snow {
				t.Errorf("labelled cell %d is %q, want the snow the seed gave it", e, k.Name)
			}
			if k := brd.Res.Logic.Board.Kind(a); k.Name.String() != "door" {
				t.Errorf("labelled cell %d is %q, want the door the Layout laid", a, k.Name)
			}
		})
	}
}

// panicsWith runs f and fails the test unless it panics with a message holding every one of want.
func panicsWith(t *testing.T, f func(), want ...string) {
	t.Helper()
	defer func() {
		t.Helper()
		r := recover()
		if r == nil {
			t.Errorf("no panic, want one mentioning %q", want)
			return
		}
		msg := fmt.Sprint(r)
		for _, w := range want {
			if !strings.Contains(msg, w) {
				t.Errorf("panic %q, want one mentioning %q", msg, w)
			}
		}
	}()
	f()
}
