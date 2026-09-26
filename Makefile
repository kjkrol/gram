GO = go

COMMIT_HASH := $(shell git rev-parse --short HEAD)
COMMIT_DATE := $(shell git log -1 --format=%cd --date=format:%Y%m%d)
DIRTY       := $(shell git diff --quiet || echo "-dirty")
RESULT_FILE := bench_results/bench_$(COMMIT_DATE)_$(COMMIT_HASH)$(DIRTY).txt
BENCH_COUNT ?= 5

.PHONY: all demo-minimal demo-collision demo-navigation demo-navigation-hex demo-navigation-vision demo-navigation-vision-hex demo-island demo-island-25 demo-island-isometric demo-effect demo-split-screen demo-scenes demo-vision deps tidy test bench bench-save clean

all: demo-collision

## demo: Alias for run — fetches dependencies and launches the collision-demo example
demo-minimal: run-minimal

demo-collision: run-collision

demo-navigation: run-navigation

demo-navigation-hex: run-navigation-hex

demo-navigation-vision: run-navigation-vision

demo-navigation-vision-hex: run-navigation-vision-hex

demo-island: run-island

demo-island-25: run-island-25

demo-island-isometric: run-island-isometric

demo-effect: run-effect

demo-split-screen: run-split-screen

demo-scenes: run-scenes

demo-vision: run-vision

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

run-island: deps
	$(GO) run ./examples/island-demo

run-island-25: deps
	$(GO) run ./examples/island-25-demo

run-island-isometric: deps
	$(GO) run ./examples/island-isometric-demo

run-island-isometric: tidy
	$(GO) run ./examples/island-isometric-demo

run-effect: deps
	$(GO) run ./examples/effect-demo

run-split-screen: deps
	$(GO) run ./examples/split-screen-demo

run-scenes: deps
	$(GO) run ./examples/scenes-demo

run-vision: deps
	$(GO) run ./examples/vision-demo

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
