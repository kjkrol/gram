package terrain_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/cond"
	"github.com/kjkrol/ceremony/tag"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/internal/boardtest"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

type cellProbe struct {
	q     *goke.Query
	plot  goke.Comp[cell.Plot]
	roles goke.OptComp[tag.Tags[cond.Roles]]
	label goke.OptComp[ceremony.Label]
	fuel  goke.OptComp[fuel]
}

type fuel struct{ Left int }

func (p *cellProbe) SetupSystems() []goke.System {
	return []goke.System{goke.SystemFn{OnInit: func(si *goke.SysInit) {
		p.q = si.NewQueryBuilder(&p.plot).Optional(&p.roles, &p.label, &p.fuel).Build()
	}}}
}

type cellState struct {
	id       uid.UID64
	hasRoles bool
	roles    tag.Tags[cond.Roles]
	labelled bool
	label    ceremony.Label
	fuel     *fuel
}

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

func installCells(t *testing.T, g grid.Grid, prepare func(w *world.Plugin, brd *board.Plugin)) (*goke.ECS, *world.Plugin, *board.Plugin, *cellProbe) {
	t.Helper()
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 4 * boardtest.CellSize, Height: 4 * boardtest.CellSize},
		Entities: world.EntitiesCfg{MaxCount: 4, MinSize: boardtest.UnitSize, MaxSize: boardtest.UnitSize},
		Heights:  true,
	})
	brd := board.NewPlugin(g, &cell.MultipleOccupancy{}, w)
	brd.Write().Board().SetAll(cell.Kind{Name: cell.Named("grass"), Pace: 1, Allows: cell.Land})
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

func TestCells_RolesAndLabelsReachTheCellEntities(t *testing.T) {
	for name, g := range cellGrids() {
		t.Run(name, func(t *testing.T) {
			var trapdoor, plate *ceremony.Part
			cells := make([]cell.ID, 0, 5)
			g.EachCell(func(c cell.ID) {
				if len(cells) < 5 {
					cells = append(cells, c)
				}
			})
			a, b, c, d, e := cells[0], cells[1], cells[2], cells[3], cells[4]
			snow := cell.Kind{Name: cell.Named("snow"), Pace: 1.0 / 3, Allows: cell.Land}
			_, _, brd, probe := installCells(t, g, func(w *world.Plugin, brd *board.Plugin) {
				w.Roles().Define("trapdoor")
				w.Roles().Define("plate")
				trapdoor, plate = w.Roles().Named("trapdoor"), w.Roles().Named("plate")
				land := func() cell.Kind { return cell.Kind{Pace: 1, Allows: cell.Land} }
				brd.Read(0).CellKinds().Define("door", land(), trapdoor)
				brd.Read(0).CellKinds().Define("plate", land(), plate)
				brd.Read(0).CellKinds().Define("both", land(), trapdoor, plate)
				brd.Write().Board().Set(e, snow)
				brd.Write().Seed(board.Layout{Cells: []cell.Entry{
					{Kind: "door", Cell: a, Group: "west"},
					{Kind: "plate", Cell: b, Name: "plate", Group: "west"},
					{Kind: "both", Cell: c, Group: "east"},
					{Kind: "door", Cell: d},
					{Cell: e, Name: "lever"},
				}})
			})
			want := map[cell.ID]cellState{
				a: {hasRoles: true, roles: tag.Tags[cond.Roles](0).With(trapdoor.Tag()), labelled: true, label: ceremony.LabelOf("", "west")},
				b: {hasRoles: true, roles: tag.Tags[cond.Roles](0).With(plate.Tag()), labelled: true, label: ceremony.LabelOf("plate", "west")},
				c: {hasRoles: true, roles: tag.Tags[cond.Roles](0).With(trapdoor.Tag(), plate.Tag()), labelled: true, label: ceremony.LabelOf("", "east")},
				d: {hasRoles: true, roles: tag.Tags[cond.Roles](0).With(trapdoor.Tag())},
				e: {hasRoles: true, labelled: true, label: ceremony.LabelOf("lever", "")},
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
				if id, _ := brd.Read(0).Board().CellEntity(at); id != st.id {
					t.Errorf("cell %d's entity is %d, the one holding its Plot %d", at, id, st.id)
				}
			}
			if k := brd.Read(0).Board().Kind(e); k != snow {
				t.Errorf("labelled cell %d is %q, want the snow the seed gave it", e, k.Name)
			}
			if k := brd.Read(0).Board().Kind(a); k.Name.String() != "door" {
				t.Errorf("labelled cell %d is %q, want the door the Layout laid", a, k.Name)
			}
		})
	}
}

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
