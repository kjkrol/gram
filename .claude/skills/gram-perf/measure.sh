#!/bin/bash
# measure.sh <out dir> <bench regex> <base dir> <tree dir> [rounds=3] [count=2]: runs the
# benchmarks matching the regex in the base and the tree alternately, rounds times with -count
# each, on an idle machine, appending the whole output to <out dir>/base.txt and tree.txt (a
# benchmark's log line splits a result from its name; compare.py joins them, a grep would lose
# the result), then prints compare.py's table. BASE_ENV and TREE_ENV are each side's environment
# ("GOEXPERIMENT=simd GOWORK=off"; a GOWORK file of the tree's own for the fair siblings),
# BENCHTIME go test's -benchtime (1s), EXTRA more flags for both sides ("-cpu=1").
set -u
out=$1 bench=$2 base=$3 tree=$4 rounds=${5:-3} count=${6:-2}
load=$(cut -d' ' -f1 /proc/loadavg)
awk "BEGIN{exit !($load < 1.5)}" || { echo "busy: load $load; not measuring" >&2; exit 1; }
mkdir -p "$out"
rm -f "$out/base.txt" "$out/tree.txt"
run() { # run <dir> <env string> <file>
	(cd "$1" && env $2 go test ./bench -run xxx -bench "$bench" -benchtime="${BENCHTIME:-1s}" \
		-count="$count" ${EXTRA:-} >> "$3" 2>&1)
}
echo "start $(date +%T), load $load, $rounds rounds of -count=$count" >&2
for _ in $(seq "$rounds"); do
	run "$base" "${BASE_ENV:-}" "$out/base.txt"
	run "$tree" "${TREE_ENV:-}" "$out/tree.txt"
done
echo "end $(date +%T), load $(cut -d' ' -f1 /proc/loadavg)" >&2
python3 "$(dirname "$0")/compare.py" "$out/base.txt" "$out/tree.txt"
