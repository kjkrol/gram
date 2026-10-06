GO = go

COMMIT_HASH := $(shell git rev-parse --short HEAD)
COMMIT_DATE := $(shell git log -1 --format=%cd --date=format:%Y%m%d)
DIRTY       := $(shell git diff --quiet || echo "-dirty")
RESULT_FILE := bench_results/bench_$(COMMIT_DATE)_$(COMMIT_HASH)$(DIRTY).txt
BENCH_COUNT ?= 5

.PHONY: all demo-minimal demo-collision demo-appearance demo-navigation demo-navigation-hex demo-navigation-vision demo-navigation-vision-hex demo-board demo-board-topography demo-board-atlas demo-effect demo-bullet demo-trapdoor demo-pressure-plate demo-wire demo-split-screen demo-scenes demo-vision deps tidy test bench bench-save clean

all: demo-collision

## demo: Alias for run — fetches dependencies and launches the collision-demo example
demo-minimal: run-minimal

demo-collision: run-collision

demo-appearance: run-appearance

demo-navigation: run-navigation

demo-navigation-hex: run-navigation-hex

demo-navigation-vision: run-navigation-vision

demo-navigation-vision-hex: run-navigation-vision-hex

demo-board: run-board

demo-board-topography: run-board-topography

demo-board-atlas: run-board-atlas

demo-effect: run-effect

demo-bullet: run-bullet

demo-trapdoor: run-trapdoor

demo-pressure-plate: run-pressure-plate

demo-wire: run-wire

demo-split-screen: run-split-screen

demo-scenes: run-scenes

demo-vision: run-vision

demo-material: run-material

demo-animation: run-animation

## run: Fetches dependencies and launches the collision-demo example
run-minimal: deps
	$(GO) run ./examples/minimal

run-collision: deps
	$(GO) run ./examples/collision-demo

run-navigation: deps
	$(GO) run ./examples/navigation-demo

run-navigation-hex: deps
	$(GO) run ./examples/navigation-hex-demo

run-navigation-vision: deps
	$(GO) run ./examples/navigation-vision-demo

run-navigation-vision-hex: deps
	$(GO) run ./examples/navigation-vision-hex-demo

run-board: deps
	$(GO) run ./examples/board

run-board-topography: deps
	$(GO) run ./examples/board-topography

run-board-atlas: deps
	$(GO) run ./examples/board-atlas

run-effect: deps
	$(GO) run ./examples/effect-demo

run-bullet: deps
	$(GO) run ./examples/bullet-demo

run-trapdoor: deps
	$(GO) run ./examples/trapdoor-demo

run-appearance: deps
	$(GO) run ./examples/appearance-demo

run-pressure-plate: deps
	$(GO) run ./examples/pressure-plate-demo

run-wire: deps
	$(GO) run ./examples/wire-demo

run-split-screen: deps
	$(GO) run ./examples/split-screen-demo

run-scenes: deps
	$(GO) run ./examples/scenes-demo

run-vision: deps
	$(GO) run ./examples/vision-demo

run-material: deps
	$(GO) run ./examples/material-demo

run-animation: deps
	$(GO) run ./examples/animation-demo

deps:
	$(GO) mod tidy

tidy:
	$(GO) mod tidy

test:
	$(GO) test ./...

## bench: runs the whole suite once, with allocations
bench:
	$(GO) test -bench=. -benchmem -count=1 ./bench/...

## bench-save: runs the suite BENCH_COUNT times and keeps the raw output under bench_results/
bench-save:
	@mkdir -p bench_results
	$(GO) test -bench=. -benchmem -count=$(BENCH_COUNT) ./bench/... > $(RESULT_FILE)
	@echo "Results saved into: $(RESULT_FILE)"

clean:
	$(GO) clean
