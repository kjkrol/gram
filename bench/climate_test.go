package bench_test

import (
	"testing"

	"github.com/kjkrol/gram/plugins/atmosphere"
	"github.com/kjkrol/gram/plugins/atmosphere/climate"
	"github.com/kjkrol/gram/plugins/atmosphere/climate/weather"
	"github.com/kjkrol/gram/plugins/world"
)

// Benchmark_Climate_Step is a tick of the atmosphere over a world as big as Madeira's, its sky's
// air masses out to the horizon: carried by the wind, settling over the world as a weather set
// now is forced, settling all over without wind; patchy, the horizon's lanes each their own.
func Benchmark_Climate_Step(b *testing.B) {
	for _, c := range []struct {
		name          string
		forced, still bool
		patchy        bool
	}{{"wind", false, false, false}, {"forced", true, false, false},
		{"still", false, true, false}, {"patchy", false, false, true}} {
		b.Run(c.name, func(b *testing.B) {
			ctx := newHeadless()
			w := ctx.UseWorld(world.Config{
				Space:    world.SpaceCfg{Width: 6528, Height: 3392},
				Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 8},
				Scale:    world.Scale{Metres: 333.0 / 32},
				Heights:  true,
			})
			running := atmosphere.AllRunning()
			running.Wind = !c.still
			a := atmosphere.NewPlugin(w, nil, atmosphere.Config{
				Climate: climate.Config{Start: weather.Rain, Seed: 7, Patchy: c.patchy},
				Running: &running,
			})
			if err := ctx.Use(a); err != nil {
				b.Fatal(err)
			}
			ecs := ctx.start(b, a.RunPlan)
			for range 600 {
				ecs.Tick(step)
			}
			b.ReportAllocs()
			n := 0
			for b.Loop() {
				// a weather set now settles in for a minute, the dome every quarter of a second
				if c.forced && n%3000 == 0 {
					a.Climate().Set(weather.Storm)
				}
				n++
				ecs.Tick(step)
			}
		})
	}
}
