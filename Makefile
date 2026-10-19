GO = go

COMMIT_HASH := $(shell git rev-parse --short HEAD)
COMMIT_DATE := $(shell git log -1 --format=%cd --date=format:%Y%m%d)
DIRTY       := $(shell git diff --quiet || echo "-dirty")
RESULT_FILE := bench_results/bench_$(COMMIT_DATE)_$(COMMIT_HASH)$(DIRTY).txt
BENCH_COUNT ?= 5

.PHONY: all demo-start-minimal demo-start-scenes demo-start-split-screen demo-look-animation demo-look-material demo-look-sky demo-look-light demo-rules-dialog demo-rules-effect demo-rules-guard demo-rules-mechanisms demo-rules-vision demo-board-atlas demo-board-flat demo-board-topography demo-navigation-hex demo-navigation-spawn demo-navigation-square demo-motion-bullet demo-motion-collision demo-motion-expanse demo-motion-platformer deps tidy test bench bench-save clean

all: demo-motion-collision

## demo-<group>-<name>: fetches dependencies and runs examples/<group>/<name>

# start
demo-start-minimal: deps
	$(GO) run ./examples/start/minimal
demo-start-scenes: deps
	$(GO) run ./examples/start/scenes
demo-start-split-screen: deps
	$(GO) run ./examples/start/split-screen

# look
demo-look-animation: deps
	$(GO) run ./examples/look/animation
demo-look-material: deps
	$(GO) run ./examples/look/material
demo-look-sky: deps
	$(GO) run ./examples/look/sky
demo-look-light: deps
	$(GO) run ./examples/look/light

# rules
demo-rules-dialog: deps
	$(GO) run ./examples/rules/dialog
demo-rules-effect: deps
	$(GO) run ./examples/rules/effect
demo-rules-guard: deps
	$(GO) run ./examples/rules/guard
demo-rules-mechanisms: deps
	$(GO) run ./examples/rules/mechanisms
demo-rules-vision: deps
	$(GO) run ./examples/rules/vision

# board
demo-board-atlas: deps
	$(GO) run ./examples/board/atlas
demo-board-flat: deps
	$(GO) run ./examples/board/flat
demo-board-topography: deps
	$(GO) run ./examples/board/topography

# navigation
demo-navigation-hex: deps
	$(GO) run ./examples/navigation/hex
demo-navigation-spawn: deps
	$(GO) run ./examples/navigation/spawn
demo-navigation-square: deps
	$(GO) run ./examples/navigation/square

# motion
demo-motion-bullet: deps
	$(GO) run ./examples/motion/bullet
demo-motion-collision: deps
	$(GO) run ./examples/motion/collision
demo-motion-expanse: deps
	$(GO) run ./examples/motion/expanse
demo-motion-platformer: deps
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
