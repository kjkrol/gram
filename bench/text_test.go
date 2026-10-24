package bench_test

import (
	"fmt"
	"image/color"
	"strings"
	"testing"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugin"
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
func drawersOf(ctx *headless, scene *ui.Scene) []render.Renderer {
	drawers := scene.Drawers()
	ctx.pending = append(ctx.pending, func() []goke.System {
		var systems []goke.System
		for _, d := range drawers {
			if b, ok := d.(plugin.Bound); ok {
				systems = append(systems, goke.SystemFn{OnInit: b.Init})
			}
		}
		return systems
	})
	return drawers
}

// drawFrame publishes what the drawers draw and draws it into screen.
func drawFrame(screen *render.Image, drawers []render.Renderer) {
	for _, d := range drawers {
		if p, ok := d.(plugin.Publisher); ok {
			p.Publish()
			p.Swap()
		}
	}
	screen.Clear()
	for _, d := range drawers {
		d.Draw(screen)
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
	var drawers []render.Renderer
	ecs := benchWorldViewed(b, ctx, 5000, 56, 2000, func(w *world.Plugin, movers kind.Of[mover]) {
		atlas := render.NewAtlas()
		atlas.Add(movers.SpriteID(), 20, render.Solid(color.RGBA{R: 90, G: 200, B: 110, A: 255}))
		atlas.Close()
		w.Write().SetAtlas(atlas)
		ctx.look()
		ctx.pending = append(ctx.pending, ctx.bound)
		screen := ui.Layers(
			ui.Image(render.NewFeed(ctx.cam, w.Read(0).Picture())),
			ui.TopLeft(ui.Panel(ui.LabelOf(l)).Fill(color.RGBA{A: 160}).Steady()).Margin(8),
			ui.BottomRight(ui.Button("Quit")).Margin(8),
		)
		drawers = drawersOf(ctx, ui.NewScene("main", screen))
	})
	ecs.Tick(step)
	ctx.publish()
	screen := render.NewImage(screenW, screenH)
	b.ReportAllocs()
	for b.Loop() {
		l.frame++
		drawFrame(screen, drawers)
	}
}
