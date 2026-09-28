package engine

import (
	"log"
	"os"
	"time"
)

// TEMP-MEASURE: under GRAM_FULLSCREEN=1 the engine logs, once a second, how long its ticks and
// its Draw took and how many ticks a frame ran. To be removed with the rest of the measurement.
var (
	measuring             = os.Getenv("GRAM_FULLSCREEN") == "1"
	measureSince          time.Time
	tickTime, drawTime    time.Duration
	tickFrames, tickSteps int
	drawFrames            int
	measureCapped         int
)

func measureTicks(d time.Duration, steps int) {
	if !measuring {
		return
	}
	tickTime += d
	tickFrames++
	tickSteps += steps
	if steps == maxStepsAFrame {
		measureCapped++
	}
}

func measureDraw(start time.Time) {
	if !measuring {
		return
	}
	drawTime += time.Since(start)
	drawFrames++
	if measureSince.IsZero() {
		measureSince = start
	}
	if time.Since(measureSince) < time.Second {
		return
	}
	log.Printf("MEASURE-ENGINE ticks/frame=%.2f update=%.1fms/frame draw=%.1fms/frame capped=%d/%d",
		float64(tickSteps)/float64(max(tickFrames, 1)), float64(tickTime.Milliseconds())/float64(max(tickFrames, 1)),
		float64(drawTime.Milliseconds())/float64(max(drawFrames, 1)), measureCapped, tickFrames)
	measureSince, tickTime, drawTime, tickFrames, tickSteps, drawFrames, measureCapped = time.Now(), 0, 0, 0, 0, 0, 0
}
