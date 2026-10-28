#!/usr/bin/env python3
"""compare.py <base.txt> <tree.txt>: the benchmarks of two files of `go test -bench` output side
by side — the min and the median of ns/op a benchmark, the change of each in percent, and
allocs/op where the lines carry them. A benchmark in one file alone is marked new or gone. A
line a benchmark logs (the world's bucket line) lands between a result's name and its numbers,
so a bare line of numbers belongs to the last name seen: keep the whole output, grep nothing."""
import re
import statistics as st
import sys
from collections import defaultdict

NAME = re.compile(r'^(Benchmark\S+?)(?:-\d+)?\s')
RESULT = re.compile(r'(?:^|\s)\d+\s+([\d.]+) ns/op(.*)$')


def load(path):
    runs = defaultdict(list)
    last = None
    for line in open(path):
        m = NAME.match(line)
        if m:
            last = m.group(1)
        r = RESULT.search(line)
        if not r or last is None:
            continue
        allocs = re.search(r'([\d.]+) allocs/op', r.group(2))
        runs[last].append((float(r.group(1)), float(allocs.group(1)) if allocs else None))
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
