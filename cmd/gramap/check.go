package main

import (
	"fmt"
	"github.com/kjkrol/aabbworld/geom"
	"slices"
	"testing/fstest"
	"time"

	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/ceremony/host"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/internal/parallel"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/bridges"
	"github.com/kjkrol/gram/plugins/buildings"
	"github.com/kjkrol/gram/plugins/topography"
	"github.com/kjkrol/gram/plugins/underground"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

type fordRow struct {
	At  cell.ID `yaml:"at"`
	Way [2]int  `yaml:"way"`
}

type treeRow struct {
	At [2]float32 `yaml:"at"`
}

type fieldRow struct {
	From  cell.ID `yaml:"from"`
	To    cell.ID `yaml:"to"`
	Cells int     `yaml:"cells"`
}

// check lays the map on a board as a game would and runs it a step: what a board refuses fails
// here.
func (w *chart) check(data []byte) (err error) {
	units := w.r.Area.Units
	wp := world.NewPlugin(world.Config{
		Space: world.SpaceCfg{Width: uint32(w.cols) * units, Height: uint32(w.rows) * units},
		Entities: world.EntitiesCfg{
			MaxCount: w.cols * w.rows,
			MinSize:  1,
			MaxSize:  units,
		},
		Heights: true,
		Scale:   world.Scale{Metres: float32(w.perUnit)},
	})
	g := grid.DefaultGrids{}.Square(uint32(w.cols), uint32(w.rows), uint32(units))
	b := board.NewPlugin(g, &cell.MultipleOccupancy{}, wp)
	plugins := []plugin.Plugin{wp, b}
	var topo *topography.Plugin
	if !w.flat {
		topo = topography.NewPlugin(wp, b, nil, topography.Config{})
		plugins = append(plugins, topo)
	}
	k := w.r.Kinds
	ground := cell.Land | cell.Air
	kinds := b.Read(0).CellKinds()
	kinds.Define(k.Sea, cell.Kind{Pace: 1, Allows: cell.Water | cell.Air})
	for _, name := range append(append(k.land(), w.r.Cover.kinds()...), w.r.town()...) {
		kinds.Define(name, cell.Kind{Pace: 1, Allows: ground})
	}
	for _, name := range []string{k.Brook, k.Stream, k.River} {
		if _, ok := kinds.Get(name); !ok {
			kinds.Define(name, cell.Kind{Pace: 1, Allows: cell.Water | cell.Air})
		}
	}
	kinds.Define(k.Road, cell.Kind{Pace: 1, Allows: ground, Graded: w.roads.streets == nil})
	br := bridges.NewPlugin(wp, b, bridges.Config{Rules: w.roads.bridgeRules()})
	plugins = append(plugins, br)
	for _, name := range []string{k.Bridge, k.Arch, k.Viaduct, w.r.Coast.Pontoon} {
		if name == "" || slices.Contains(br.Types().Names(), name) {
			continue
		}
		br.Types().Define(name, bridges.Spec{
			Allows:    cell.Land,
			Width:     float32(w.deck()),
			Thickness: float32(w.r.Roads.Thickness * w.figures() / w.perUnit),
			Pace:      1,
			Rail:      cell.Rail{Keeps: cell.Land},
			Pier:      1,
		})
	}
	var un *underground.Plugin
	if !w.flat && len(w.roads.tunnels) > 0 {
		un = underground.NewPlugin(wp, b, underground.Config{Rules: w.roads.tunnelRules(0)})
		plugins = append(plugins, un)
		// a town's tunnels are as wide as its recipe says, as the game's are
		width := float32(w.deck())
		if w.roads.streets != nil {
			width = float32(w.r.Tunnels.Width * w.figures() / w.perUnit)
		}
		un.Types().Define(k.Tunnel, underground.Spec{
			Allows: cell.Land,
			Width:  width,
			Clear:  w.roads.clear(),
			Pace:   1,
		})
	}
	fords := board.NewEntities(b, board.Shape{Size: float32(w.size)},
		func(r fordRow) cell.ID { return r.At })
	fords.Define(k.Ford, comp.Const(cell.Floor{Allows: cell.Land}))
	// a field is a game's kind: its rows are read and taken, nothing spawned
	for _, f := range w.r.Fields {
		if _, _, _, ok := wp.Kinds().Lookup(f.Kind); !ok {
			kind.Define[fieldRow](wp.Kinds(), f.Kind,
				wp.Roster().Spec(world.EntityTemplate, comp.Const(world.Position{})))
		}
	}
	// a tree standing by itself is a game's kind too
	for _, t := range w.r.Greens.Trees {
		if _, _, _, ok := wp.Kinds().Lookup(t.Kind); !ok {
			kind.Define[treeRow](wp.Kinds(), t.Kind,
				wp.Roster().Spec(world.EntityTemplate, comp.Const(world.Position{})))
		}
	}
	var stood *buildings.Plugin
	if names := w.r.Buildings.names(); len(names) > 0 && !w.flat {
		stood = buildings.NewPlugin(wp, b)
		plugins = append(plugins, stood)
		// the role its figure would define in a Stage
		wp.Roles().Define(buildings.BuildingRole)
		for _, name := range names {
			stood.Types().Define(name, buildings.Spec{Storey: 1, Size: geom.NewVec(1, 1)})
		}
	}
	level, err := b.Lay(fstest.MapFS{"map.yaml": {Data: data}}, "map.yaml")
	if err != nil {
		return err
	}
	if stood != nil {
		if err := stood.Write().Spawn(level); err != nil {
			return err
		}
	}
	if err := board.Spawn[fordRow](level, nil); err != nil {
		return err
	}
	if err := board.Each(level, func(string, fieldRow, string) error { return nil }); err != nil {
		return err
	}
	if err := board.Each(level, func(string, treeRow, string) error { return nil }); err != nil {
		return err
	}
	if err := br.Write().Spawn(level); err != nil {
		return err
	}
	if un != nil {
		if err := un.Write().Spawn(level); err != nil {
			return err
		}
	}
	if topo != nil {
		topo.Write().Seed(level.Heights())
	}
	ctx := &installer{ecs: goke.New()}
	for _, p := range plugins {
		if err := p.Install(ctx); err != nil {
			return err
		}
	}
	for _, p := range plugins {
		if pp, ok := p.(plugin.Populator); ok {
			if err := pp.Populate(); err != nil {
				return err
			}
		}
	}
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%v", r)
		}
	}()
	ctx.ecs.Setup(ctx.systems()...)
	ctx.ecs.SetPlan(func(rc goke.RunCtx, d time.Duration) {
		settling := wp.Write().Clock().Settling(rc)
		wp.RunPlan(settling, d)
		b.RunPlan(settling, d)
		if topo != nil {
			topo.RunPlan(settling, d)
		}
		settling.Sync()
		wp.Write().Clock().Replay(rc, d)
	})
	ctx.ecs.Tick(time.Second / 60)
	return nil
}

type installer struct {
	ecs     *goke.ECS
	pending []func() []goke.System
}

func (c *installer) UseModule(m goke.Module) {
	reg := goke.SystemFn{OnInit: func(*goke.SysInit) { m.RegSystems(c.ecs) }}
	c.pending = append(c.pending, func() []goke.System { return append(m.SetupSystems(), reg) })
}

func (c *installer) Setup(providers ...goke.SetupProvider) {
	for _, p := range providers {
		c.pending = append(c.pending, p.SetupSystems)
	}
}

func (c *installer) RegSys(factory func() goke.System) goke.Runnable {
	return c.ecs.RegSys(factory())
}

func (c *installer) ECS() *goke.ECS { return c.ecs }

func (c *installer) Hosts(...host.Host)     {}
func (c *installer) Draws(...render.Source) {}
func (c *installer) Looks(func())           {}
func (*installer) Pool() *parallel.Pool     { return nil }

func (c *installer) systems() []goke.System {
	var out []goke.System
	for _, produce := range c.pending {
		out = append(out, produce()...)
	}
	return out
}
