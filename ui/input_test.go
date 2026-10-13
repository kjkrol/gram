package ui

import (
	"testing"

	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/gram/render"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/game"
)

type order struct{ what string }

func (order) Command() {}

type player struct {
	area   geom.AABB
	shown  render.Surface
	clicks []geom.Vec
	wheel  float64
}

func (p *player) Over(area geom.AABB, shown render.Surface) { p.area, p.shown = area, shown }

func (p *player) handle(ev *control.InputEvents, _ game.Runtime, _ game.Composition) {
	for _, c := range ev.ClickQueue {
		p.clicks = append(p.clicks, c.Pos)
	}
	p.wheel += ev.ScrollDelta
}

func built(s *Scene) *Scene {
	s.Layers()
	s.root.lay(screen)
	return s
}

func click(at geom.Vec) *control.InputEvents {
	return &control.InputEvents{MousePos: at, ClickQueue: []control.ClickEvent{
		{Button: control.MouseButtonLeft, Action: control.ActionPress, Pos: at}}}
}

func TestInput_AClickOnTheWorldReachesThePlayerAndOneOnAPanelDoesNot(t *testing.T) {
	red, world := &player{}, &sized{}
	s := built(NewScene("main", Layers(Image(world).Input(red), TopLeft(Panel(Label("hud"))).Size(200, 100))).Input(red.handle))
	if red.shown != nil {
		t.Fatal("the player was told of its picture before the scene handled input")
	}
	s.HandleEvents(click(geom.NewVec(600, 300)), nil, nil)
	if red.area != screen || red.shown != world {
		t.Fatalf("handling input the scene told the player %v showing %v, want the screen showing its picture", red.area, red.shown)
	}
	s.HandleEvents(click(geom.NewVec(50, 50)), nil, nil)
	if len(red.clicks) != 1 || red.clicks[0] != geom.NewVec(600, 300) {
		t.Fatalf("the player got clicks %v, want only the one on the world", red.clicks)
	}
}

func TestInput_AButtonGivesItsCommandThroughIssue(t *testing.T) {
	var issued []command.Command
	s := built(NewScene("main", Center(Button("go", order{"go"})).Size(100, 40)).Issue(func(cmd command.Command) error { issued = append(issued, cmd); return nil }))
	s.HandleEvents(click(geom.NewVec(600, 300)), nil, nil)
	if len(issued) != 1 || issued[0] != (order{"go"}) {
		t.Fatalf("issued %v, want the button's order", issued)
	}
}

type issuing struct {
	game.Runtime
	issued []command.Command
}

func (r *issuing) Issue(cmd command.Command) error {
	r.issued = append(r.issued, cmd)
	return nil
}

func TestInput_AButtonWithoutIssueGivesItsCommandThroughTheRuntime(t *testing.T) {
	rt := &issuing{}
	s := built(NewScene("menu", Center(Button("go", order{"go"})).Size(100, 40)))
	s.HandleEvents(click(geom.NewVec(600, 300)), rt, nil)
	if len(rt.issued) != 1 || rt.issued[0] != (order{"go"}) {
		t.Fatalf("the runtime was given %v, want the button's order", rt.issued)
	}
}

func TestInput_AButtonOfTheScenesOwnShowsAndHidesByName(t *testing.T) {
	panel := Panel(Label("panel")).Named("panel").Hidden()
	s := built(NewScene("main", Layers(TopLeft(Button("open", Toggle{"panel"})).Size(100, 40), panel)))
	s.HandleEvents(click(geom.NewVec(20, 20)), nil, nil)
	if panel.hidden {
		t.Fatal("the button left the panel hidden")
	}
}

func TestInput_AModalWindowHoldsTheClicksAndTheWheelOutsideIt(t *testing.T) {
	red := &player{}
	var issued []command.Command
	s := built(NewScene("main", Layers(
		Image(&sized{}).Input(red),
		TopLeft(Button("under", order{"under"})).Size(100, 40),
		Center(Window("decide", Button("yes", order{"yes"}))).Size(300, 200).Named("decision").Modal(),
	)).Input(red.handle).Issue(func(cmd command.Command) error { issued = append(issued, cmd); return nil }))

	outside := click(geom.NewVec(20, 20))
	outside.ScrollDelta = 1
	s.HandleEvents(outside, nil, nil)
	if len(issued) != 0 || len(red.clicks) != 0 || red.wheel != 0 {
		t.Fatalf("outside the modal window: issued %v, clicks %v, wheel %v; want nothing", issued, red.clicks, red.wheel)
	}

	s.Hide("decision")
	s.HandleEvents(click(geom.NewVec(20, 20)), nil, nil)
	if len(issued) != 1 || issued[0] != (order{"under"}) {
		t.Fatalf("with the window hidden: issued %v, want the button under it", issued)
	}
}

func TestInput_AModalWindowLeftOutHoldsNothing(t *testing.T) {
	silent := Of(func(*int) (string, bool) { return "", false })
	for name, window := range map[string]*Element{
		"the anchor modal": Center(WindowOf(silent, Button("yes", order{"yes"}))).Size(300, 200).
			Modal(),
		"the window modal": Center(WindowOf(silent, Button("yes", order{"yes"})).Modal()),
	} {
		red := &player{}
		s := built(NewScene("main", Layers(Image(&sized{}).Input(red), window)).Input(red.handle))
		ev := click(geom.NewVec(150, 150))
		ev.ScrollDelta = 1
		s.HandleEvents(ev, nil, nil)
		if len(red.clicks) == 0 || red.wheel == 0 {
			t.Errorf("%s, left out: clicks %v, wheel %v; want both through", name, red.clicks,
				red.wheel)
		}
	}
}

func TestInput_TheScenesKeysGiveTheirCommands(t *testing.T) {
	panel := Label("panel").Named("panel").Hidden()
	s := built(NewScene("main", Layers(panel)).
		Keys(control.Give(control.KeyPress{Key: control.KeyP}, "Panel", Toggle{"panel"})))
	ev := &control.InputEvents{KeyEvents: []control.KeyEvent{{Key: control.KeyP, Action: control.ActionPress}}}
	s.HandleEvents(ev, nil, nil)
	if panel.hidden {
		t.Fatal("P left the panel hidden")
	}
}
