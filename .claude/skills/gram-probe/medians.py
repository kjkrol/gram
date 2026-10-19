import sys, statistics as st
from collections import defaultdict
rows = defaultdict(list)
pending = None
for line in open(sys.argv[1]):
    f = line.split()
    if not f: continue
    if line.startswith("Benchmark"):
        if "ns/op" in line:
            name, rest = f[0], f[1:]
        else:
            pending = f[0]; continue
    elif pending and "ns/op" in line:
        name, rest = pending, f; pending = None
    else:
        continue
    vals = {}
    for i in range(1, len(rest)-1):
        try: v = float(rest[i])
        except ValueError: continue
        vals[rest[i+1]] = v
    rows[name].append(vals)
for name in sorted(rows):
    rs = rows[name]
    keys = [k for k in rs[0] if k != "ns/op"]
    out = [f"{st.median([r['ns/op'] for r in rs]):12.0f} ns/op"]
    for k in keys:
        out.append(f"{st.median([r.get(k, 0) for r in rs]):10.0f} {k}")
    print(f"{name:55s} " + " ".join(out) + f"  (n={len(rs)})")
