GO = go

COMMIT_HASH := $(shell git rev-parse --short HEAD)
COMMIT_DATE := $(shell git log -1 --format=%cd --date=format:%Y%m%d)
DIRTY       := $(shell git diff --quiet || echo "-dirty")
RESULT_FILE := bench_results/bench_$(COMMIT_DATE)_$(COMMIT_HASH)$(DIRTY).txt
BENCH_COUNT ?= 5

.PHONY: all demo-minimal demo-collision demo-appearance demo-navigation demo-navigation-hex demo-navigation-spawn demo-navigation-vision demo-navigation-vision-hex demo-board demo-board-topography demo-board-atlas demo-effect demo-bullet demo-trapdoor demo-pressure-plate demo-wire demo-split-screen demo-scenes demo-vision demo-dialog demo-platformer deps tidy test bench bench-save clean

all: demo-collision

## demo: Alias for run — fetches dependencies and launches the collision-demo example
demo-minimal: run-minimal

demo-collision: run-collision

demo-appearance: run-appearance

demo-navigation: run-navigation

demo-navigation-hex: run-navigation-hex

demo-navigation-spawn: run-navigation-spawn

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

demo-dialog: run-dialog

demo-material: run-material

demo-animation: run-animation

demo-ember: run-ember
demo-platformer: run-platformer

## run: Fetches dependencies and launches the collision-demo example
run-minimal: deps
	$(GO) run ./examples/start/minimal

run-collision: deps
	$(GO) run ./examples/motion/collision

run-navigation: deps
	$(GO) run ./examples/navigation/square

run-navigation-hex: deps
	$(GO) run ./examples/navigation/hex

run-navigation-spawn: deps
	$(GO) run ./examples/navigation/spawn

run-navigation-vision: deps
	$(GO) run ./examples/navigation/vision

run-navigation-vision-hex: deps
	$(GO) run ./examples/navigation/vision-hex

run-board: deps
	$(GO) run ./examples/board/flat

run-board-topography: deps
	$(GO) run ./examples/board/topography

run-board-atlas: deps
	$(GO) run ./examples/board/atlas

run-effect: deps
	$(GO) run ./examples/rules/effect

run-bullet: deps
	$(GO) run ./examples/motion/bullet

run-trapdoor: deps
	$(GO) run ./examples/rules/trapdoor

run-appearance: deps
	$(GO) run ./examples/look/appearance

run-pressure-plate: deps
	$(GO) run ./examples/rules/pressure-plate

run-wire: deps
	$(GO) run ./examples/rules/wire

run-split-screen: deps
	$(GO) run ./examples/start/split-screen

run-scenes: deps
	$(GO) run ./examples/start/scenes

run-vision: deps
	$(GO) run ./examples/rules/vision

run-dialog: deps
	$(GO) run ./examples/rules/dialog

run-material: deps
	$(GO) run ./examples/look/material

run-animation: deps
	$(GO) run ./examples/look/animation

run-ember: deps
	$(GO) run ./examples/look/ember

run-platformer: deps
	$(GO) run ./examples/motion/platformer

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
