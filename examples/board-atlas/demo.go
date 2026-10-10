package main

import (
	"image/color"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/game/stage"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/driving"
	"github.com/kjkrol/gram/plugins/navigation"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/steering"
	"github.com/kjkrol/gram/ui"
)

const (
	TPS          = 60
	GridWidth    = 24
	GridHeight   = 16
	CellSize     = 48
	WorldWidth   = GridWidth * CellSize
	WorldHeight  = GridHeight * CellSize
	ScreenWidth  = 1024
	ScreenHeight = 768
	EntitySize   = 24
	UnitSpeed    = CellSize * 2
	UnitCount    = 4
)

type arena struct {
	world     *world.Plugin
	board     *board.Plugin
	nav       *navigation.Plugin
	driving   *driving.Plugin
	collision *collision.Plugin
	selection *selection.Plugin
	players   *players.Plugin
	cameras   *cameras.Plugin
	player    *players.Player
}

func NewDemo(s *arena) game.Game {
	return game.New(game.Props{
		Title:       "gram board atlas",
		Description: "A board from the game's own atlas.",
		ScreenWidth: ScreenWidth, ScreenHeight: ScreenHeight, Resizable: true,
		TargetTPS: TPS,
	},
		stage.New(BoardAtlasStage).
			World(world.Config{
				Space:    world.SpaceCfg{Width: WorldWidth, Height: WorldHeight},
				Entities: world.EntitiesCfg{MaxCount: 2 * UnitCount, MinSize: EntitySize, MaxSize: EntitySize},
			}).
			Plugins(s.makePlugins).
			Players(s.definePlayer).
			Kinds(s.defineCells, s.defineKinds).
			Spawn(s.spawnCells, s.spawnUnits).
			Scenes(s.defineScenes).
			Update(s.update))
}

func (s *arena) makePlugins(w *world.Plugin) {
	s.world = w
	grid := grid.DefaultGrids{}.Square(GridWidth, GridHeight, CellSize)
	s.collision = collision.NewPlugin(s.world)
	s.board = board.NewPlugin(grid, &cell.SingleOccupancy{}, s.world).WithCollision(s.collision)
	s.selection = selection.NewPlugin(s.world)
	s.driving = driving.NewPlugin(s.world, s.selection).WithGround(s.board)
	s.nav = navigation.NewPlugin(s.board, s.world, s.selection, s.driving).WithCollision(s.collision)
	s.cameras = cameras.NewPlugin(s.world)
	s.players = players.NewPlugin(s.world, s.cameras, s.board, s.selection, s.nav, s.driving)
}

func (s *arena) definePlayer() error {
	s.player = s.players.Local("player")
	return s.player.Bind(s.players.Defaults()...)
}

func (s *arena) defineCells() {
	kinds := s.board.CellKinds()
	// TODO: przypisywanie koloru, to nie ten etap! wyglad okreslamy pozniej!
	kinds.Define(GrassCell, cell.Kind{Cost: 2, Allows: cell.Land, Color: color.RGBA{R: 96, G: 150, B: 70, A: 255}})
	kinds.Define(WaterCell, cell.Kind{Cost: 1, Allows: cell.Water, Color: color.RGBA{R: 50, G: 100, B: 180, A: 255}})
	kinds.Define(RoadCell, cell.Kind{Cost: 1, Allows: cell.Land, Color: color.RGBA{R: 160, G: 140, B: 110, A: 255}})
	kinds.Define(WoodCell, cell.Kind{Cost: 4, Allows: cell.Land, Veil: 0.6, Color: color.RGBA{R: 40, G: 100, B: 50, A: 255}})
}

func (s *arena) defineScenes(ctx game.Initializer) []game.Scene {
	main := &mainScene{arena: s, tps: ctx.TPS()}
	return []game.Scene{ui.NewScene(MainScene, main.screen()).Input(s.players.Handle)}
}

type unitRow struct{ start, target cell.ID }

func (s *arena) defineKinds() {
	brd := s.board.Res.Logic.Board
	units := board.NewUnits[unitRow](s.board, board.Shape{Size: EntitySize}, func(u unitRow) geom.Vec { return brd.CellCenter(u.start) })
	order := comp.Load(func(u unitRow) navigation.MoveOrder { return navigation.MoveOrder{Target: u.target} })
	// TODO: co ja mam za formatowanie kodu ustawione, ze linie sa tak dlugie ?!
	units.Define(UnitKind, unit.Mover{Domain: cell.Land}, steering.Steering{MaxSpeed: UnitSpeed, Accel: UnitSpeed * 2, Brake: UnitSpeed * 4, V0: UnitSpeed / 2, TurnRate: 0.15},
		order)
}

// corners are where the units start, each bound for the opposite one.
var corners = [][2]int{{1, 1}, {22, 1}, {22, 14}, {1, 14}}

// at is the cell at column x, row y.
func (s *arena) at(x, y int) cell.ID {
	c := s.board.Res.Logic.Board.CellIndex(uint32(x), uint32(y))
	return c
}

// TODO: mysle, ze najwyzsza pora skonczyc z tym prceduralnym definiowaniem planszy. Proponuje opis trzymac w pliku (trzeba wymyslic format) i zaczytywac go tutaj.
func (s *arena) spawnCells() {
	brd, at := s.board.Res.Logic.Board, s.at
	layout := board.Layout{Default: GrassCell}
	for y := 5; y < 11; y++ {
		for x := 9; x < 15; x++ {
			layout.Cells = append(layout.Cells, cell.Entry{Kind: WaterCell, Cell: at(x, y)})
		}
	}
	for y := 1; y < 6; y++ {
		for x := 16; x < 22; x++ {
			layout.Cells = append(layout.Cells, cell.Entry{Kind: WoodCell, Cell: at(x, y)})
		}
	}
	// the road: a ring round the pond, and spurs out to the corners
	ring := []cell.ID{}
	for x := 7; x <= 16; x++ {
		ring = append(ring, at(x, 3))
	}
	for y := 4; y <= 12; y++ {
		ring = append(ring, at(16, y))
	}
	for x := 15; x >= 7; x-- {
		ring = append(ring, at(x, 12))
	}
	for y := 11; y >= 4; y-- {
		ring = append(ring, at(7, y))
	}
	road := s.board.CellKinds().Named(RoadCell).Kind()
	link := func(a, b cell.ID) {
		if bit, ok := grid.Link(brd, a, b); ok {
			w := layoutWay(&layout, a, road)
			w.Links |= bit
		}
		if bit, ok := grid.Link(brd, b, a); ok {
			w := layoutWay(&layout, b, road)
			w.Links |= bit
		}
	}
	for i := range ring {
		link(ring[i], ring[(i+1)%len(ring)])
	}
	near := [][2]int{{7, 3}, {16, 3}, {16, 12}, {7, 12}}
	for k, c := range corners {
		x, y := c[0], c[1]
		for x != near[k][0] || y != near[k][1] {
			nx, ny := x, y
			if x != near[k][0] {
				nx += sign(near[k][0] - x)
			} else {
				ny += sign(near[k][1] - y)
			}
			link(at(x, y), at(nx, ny))
			x, y = nx, ny
		}
	}
	s.board.Seed(layout)
}

// TODO: to powinna byc sekcja w pliku mapy, patrz powyzsze TODO
func (s *arena) spawnUnits() {
	unitKind := kind.Named[unitRow](s.world.Kinds(), UnitKind)
	var entries []kind.Entry
	for k, c := range corners {
		o := corners[(k+2)%4]
		entries = append(entries, unitKind.Entry(unitRow{start: s.at(c[0], c[1]), target: s.at(o[0], o[1])}).Told(players.Give{To: s.player.ID}, selection.Allow{Selected: true}))
	}
	s.world.Seed(entries...)
}

func sign(v int) int {
	if v < 0 {
		return -1
	}
	return 1
}

// TODO: alez to jest hacky!
func layoutWay(layout *board.Layout, c cell.ID, kind cell.Kind) *cell.WayEntry {
	for i := range layout.Ways {
		if layout.Ways[i].Cell == c {
			return &layout.Ways[i]
		}
	}
	layout.Ways = append(layout.Ways, cell.WayEntry{Kind: kind.Name.String(), Cell: c, Width: 14})
	return &layout.Ways[len(layout.Ways)-1]
}

func (s *arena) update(ctx goke.RunCtx, d time.Duration) {
	s.world.RunPlan(ctx, d)
	s.collision.RunPlan(ctx, d)
	s.board.RunPlan(ctx, d)
	s.nav.RunPlan(ctx, d)
	s.driving.RunPlan(ctx, d)
	s.selection.RunPlan(ctx, d)
	s.cameras.RunPlan(ctx, d)
	s.players.RunPlan(ctx, d)
	ctx.Sync()
}
