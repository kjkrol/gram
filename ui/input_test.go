package ui

import (
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/render"
)

type order struct{ what string }

func (order) Command() {}

type player struct {
	area   geom.AABB
	shown  render.Surface
	clicks []geom.Vec
	wheel  float32
}

func (p *player) Over(area geom.AABB, shown render.Surface) { p.area, p.shown = area, shown }

func (p *player) handle(ev *control.InputEvents, _ game.Runtime, _ game.Composition) {
	for _, c := range ev.ClickQueue {
		p.clicks = append(p.clicks, c.Pos)
	}
	p.wheel += ev.ScrollDelta
}

func built(s *Scene) *Scene {
	s.Drawers()
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

// a picture the player only clicks: what it heard, and whether the cursor was on it
type minimap struct {
	player
	pressed []geom.Vec
	on      bool
}

func (m *minimap) Click(at geom.Vec) { m.pressed = append(m.pressed, at) }
func (m *minimap) Hover(on bool)     { m.on = on }

type hovering struct {
	player
	on bool
}

func (h *hovering) Hover(on bool) { h.on = on }

func TestInput_AClickedPictureTakesItsPressAndWheelAndLetsAReleaseGoOn(t *testing.T) {
	red, mini := &hovering{}, &minimap{}
	s := built(NewScene("main", Layers(Image(&sized{}).Input(red),
		TopRight(Image(&sized{}).Input(mini)).Size(200, 200))).Input(red.handle))
	over := geom.NewVec(1100, 100)
	ev := click(over)
	ev.ScrollDelta = 1
	s.HandleEvents(ev, nil, nil)
	// the minimap from 1000 across: the press 100 into it
	if len(mini.pressed) != 1 || mini.pressed[0] != geom.NewVec(100, 100) || !mini.on || red.on {
		t.Errorf("the minimap heard %v, on it %v, the world under it %v; want the press at "+
			"(100, 100) on it, the world covered", mini.pressed, mini.on, red.on)
	}
	if len(red.clicks) != 0 || red.wheel != 0 {
		t.Errorf("the world got clicks %v and wheel %v through the minimap, want none", red.clicks,
			red.wheel)
	}
	s.HandleEvents(&control.InputEvents{MousePos: over, ClickQueue: []control.ClickEvent{
		{Button: control.MouseButtonLeft, Action: control.ActionRelease, Pos: over}}}, nil, nil)
	if len(red.clicks) != 1 || len(mini.pressed) != 1 {
		t.Errorf("a release over the minimap reached the world %d times and the minimap %d, want "+
			"the world once", len(red.clicks), len(mini.pressed)-1)
	}
	s.HandleEvents(click(geom.NewVec(600, 400)), nil, nil)
	if !red.on || mini.on {
		t.Errorf("the cursor on the world: the world hovered %v, the minimap %v", red.on, mini.on)
	}
}

func TestElement_HideAndShowHaveItHitOrNot(t *testing.T) {
	e := Panel(Label("hud")).Size(200, 100)
	built(NewScene("main", TopLeft(e)))
	at := geom.NewVec(50, 50)
	e.Hide()
	if e.Hits(at) {
		t.Error("hidden, the element is hit")
	}
	e.Show()
	if !e.Hits(at) {
		t.Error("shown again, the element is not hit")
	}
}

func TestInput_APassingPanelOfWordsLetsTheClickTheWheelAndTheHoverThrough(t *testing.T) {
	red := &hovering{}
	s := built(NewScene("main", Layers(Image(&sized{}).Input(red),
		TopMiddle(Panel(Columns(Fit(Label("08:15")), Fit(Label("summer, 3 August")))).Passing()).
			Size(300, 40))).Input(red.handle))
	ev := click(geom.NewVec(600, 20))
	ev.ScrollDelta = 1
	s.HandleEvents(ev, nil, nil)
	if !red.on || len(red.clicks) != 1 || red.wheel != 1 {
		t.Errorf("under the passing panel the world is hovered %v, got clicks %v and wheel %v; "+
			"want it hovered, the click and the wheel", red.on, red.clicks, red.wheel)
	}
}

func TestInput_AButtonInAPassingPanelTakesItsPressAndTheWordsBesideItDoNot(t *testing.T) {
	red := &hovering{}
	var issued []command.Command
	go_ := Button("go", order{"go"})
	panel := TopLeft(Panel(Columns(Fit(Label("a word")), Fit(go_))).Passing()).Size(300, 40)
	s := built(NewScene("main", Layers(Image(&sized{}).Input(red), panel)).Input(red.handle).
		Issue(func(cmd command.Command) error { issued = append(issued, cmd); return nil }))
	b := go_.Box()
	onButton := geom.NewVec((b.TopLeft.X+b.BottomRight.X)/2, (b.TopLeft.Y+b.BottomRight.Y)/2)
	onWords := geom.NewVec(10, 20)
	if !panel.Hits(onButton) || panel.Hits(onWords) {
		t.Errorf("the passing panel is hit on its button %v and on its words %v, want the button "+
			"alone", panel.Hits(onButton), panel.Hits(onWords))
	}
	s.HandleEvents(click(onButton), nil, nil)
	if len(issued) != 1 || issued[0] != (order{"go"}) || len(red.clicks) != 0 || red.on {
		t.Errorf("a press on the button issued %v, the world got %v and is hovered %v; want the "+
			"order alone", issued, red.clicks, red.on)
	}
	s.HandleEvents(click(onWords), nil, nil)
	if len(issued) != 1 || len(red.clicks) != 1 || !red.on {
		t.Errorf("a press on the words issued %v, the world got %v and is hovered %v; want the "+
			"world's click, the world hovered", issued, red.clicks, red.on)
	}
}

func TestInput_APassingPictureOverAClickedOneLeavesItThePressAndTheWorldTheCorner(t *testing.T) {
	red, mini := &hovering{}, &minimap{}
	// the minimap 200 a side in a box 240 a side top-right, a transparent compass over the box
	s := built(NewScene("main", Layers(Image(&sized{}).Input(red),
		TopRight(Layers(
			Center(Image(&sized{}).Input(mini).Masked(Circle)).Size(200, 200),
			Image(&sized{}).Passing(),
		)).Size(240, 240))).Input(red.handle))
	inCircle := geom.NewVec(1130, 120)
	s.HandleEvents(click(inCircle), nil, nil)
	if len(mini.pressed) != 1 || mini.pressed[0] != geom.NewVec(150, 100) || !mini.on || red.on {
		t.Errorf("a press in the circle: the minimap heard %v, on it %v, the world hovered %v; "+
			"want the press at (150, 100) on it, the world covered", mini.pressed, mini.on, red.on)
	}
	corner := click(geom.NewVec(1195, 5))
	corner.ScrollDelta = 1
	s.HandleEvents(corner, nil, nil)
	if len(mini.pressed) != 1 || mini.on || !red.on || len(red.clicks) != 1 || red.wheel != 1 {
		t.Errorf("a press in the box's corner, off the circle: the minimap heard %v, on it %v, "+
			"the world hovered %v with clicks %v and wheel %v; want the world's", mini.pressed,
			mini.on, red.on, red.clicks, red.wheel)
	}
}
