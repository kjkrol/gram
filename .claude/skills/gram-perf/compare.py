#!/usr/bin/env python3
"""compare.py <base.txt> <tree.txt>: the benchmarks of two files of `go test -bench` lines side
by side — the min and the median of ns/op a benchmark, the change of each in percent, and
allocs/op where the lines carry them. A benchmark in one file alone is marked new or gone."""
import re
import statistics as st
import sys
from collections import defaultdict

LINE = re.compile(r'^(Benchmark\S+?)(?:-\d+)?\s+\d+\s+([\d.]+) ns/op(.*)$')


def load(path):
    runs = defaultdict(list)
    for line in open(path):
        m = LINE.match(line)
        if not m:
            continue
        allocs = re.search(r'([\d.]+) allocs/op', m.group(3))
        runs[m.group(1)].append((float(m.group(2)), float(allocs.group(1)) if allocs else None))
    return runs


def pct(a, b):
    return f"{100 * (b - a) / a:+6.1f}%" if a else "      "


base, tree = load(sys.argv[1]), load(sys.argv[2])
names = sorted(set(base) | set(tree))
width = max((len(k) for k in names), default=20)
print(f"{'benchmark':{width}}  {'base min':>11} {'tree min':>11} {'change':>7}  "
      f"{'base med':>11} {'tree med':>11} {'change':>7}  allocs/op")
for k in names:
    b, t = base.get(k), tree.get(k)
    if not b or not t:
        print(f"{k:{width}}  {'new' if t else 'gone'}")
        continue
    bn, tn = [v for v, _ in b], [v for v, _ in t]
    bmin, tmin, bmed, tmed = min(bn), min(tn), st.median(bn), st.median(tn)
    ba, ta = b[0][1], t[0][1]
    allocs = f"{ba:.0f} → {ta:.0f}" if ba is not None and ta is not None else ""
    print(f"{k:{width}}  {bmin:11.0f} {tmin:11.0f} {pct(bmin, tmin)}  "
          f"{bmed:11.0f} {tmed:11.0f} {pct(bmed, tmed)}  {allocs}")
