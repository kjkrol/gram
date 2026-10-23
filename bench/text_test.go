package bench_test

import (
	"fmt"
	"image/color"
	"strings"
	"testing"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/render/gputest"
	"github.com/kjkrol/gram/ui"
	"github.com/kjkrol/uid"
)

// the screen the ui frames are drawn into
const screenW, screenH = 1280, 720

// lines is a panel's text of n lines, every one changing each frame, as a telemetry panel's.
type lines struct {
	n     int
	frame int
}

func (l *lines) Text(uid.UID64, bool) (string, bool) {
	var sb strings.Builder
	for i := range l.n {
		fmt.Fprintf(&sb, "line %d: %d\n", i, l.frame*7+i)
	}
	return sb.String(), true
}

// drawersOf has the scene's drawers set up with the ECS and hands them back.
func drawersOf(ctx *headless, scene *ui.Scene) []render.Drawer {
	drawers := scene.Drawers()
	ctx.pending = append(ctx.pending, func() []goke.System {
		var systems []goke.System
		for _, d := range drawers {
			systems = append(systems, goke.SystemFn{OnInit: d.Init})
		}
		return systems
	})
	return drawers
}

func drawFrame(screen *render.Image, drawers []render.Drawer) {
	screen.Clear()
	for _, d := range drawers {
		d.(render.Renderer).Draw(screen)
	}
	render.Submit()
}

// Benchmark_Text_Panel is one frame of a panel of 20 lines of text changing every frame, as a
// telemetry panel's, drawn into a 1280x720 screen on a headless GPU.
func Benchmark_Text_Panel(b *testing.B) {
	gputest.Need(b)
	ctx := newHeadless()
	l := &lines{n: 20}
	panel := ui.TopLeft(ui.Panel(ui.LabelOf(l)).Fill(color.RGBA{A: 160}).Steady()).Margin(8)
	drawers := drawersOf(ctx, ui.NewScene("panel", panel))
	ctx.start(b, func(goke.RunCtx, time.Duration) {})
	screen := render.NewImage(screenW, screenH)
	b.ReportAllocs()
	for b.Loop() {
		l.frame++
		drawFrame(screen, drawers)
	}
}

// Benchmark_Frame_UI is one frame of a scene as a game's: the world of 5000 boxes shown through
// a ui.Image of a feed over the whole 1280x720 screen, a panel of 20 lines of text and a button
// over it, on a headless GPU.
func Benchmark_Frame_UI(b *testing.B) {
	gputest.Need(b)
	ctx := newHeadless()
	l := &lines{n: 20}
	var drawers []render.Drawer
	ecs := benchWorldViewed(b, ctx, 5000, 56, 2000, func(w *world.Plugin, movers kind.Of[mover]) {
		atlas := render.NewAtlas()
		atlas.Add(movers.SpriteID(), 20, render.Solid(color.RGBA{R: 90, G: 200, B: 110, A: 255}))
		atlas.Close()
		w.Write().SetAtlas(atlas)
		ctx.look()
		screen := ui.Layers(
			ui.Image(render.NewFeed(ctx.cam, w.Read(0).Picture())),
			ui.TopLeft(ui.Panel(ui.LabelOf(l)).Fill(color.RGBA{A: 160}).Steady()).Margin(8),
			ui.BottomRight(ui.Button("Quit")).Margin(8),
		)
		drawers = drawersOf(ctx, ui.NewScene("main", screen))
	})
	ecs.Tick(step)
	screen := render.NewImage(screenW, screenH)
	b.ReportAllocs()
	for b.Loop() {
		l.frame++
		drawFrame(screen, drawers)
	}
}
