#!/bin/bash
# probe.sh <demo dir> <probe.go> <real seconds> <work dir>: copies the demo into a module of its
# own under the work dir, hooks the probe into the copy, runs it at tempo x4 and prints the lines
# logged with "P:". The demo's own files are never touched.
set -e
G=$(git -C "$(dirname "$0")" rev-parse --show-toplevel)
W=$4/probews
rm -rf "$W"; mkdir -p "$W/demo"
cp "$G/$1"/* "$W/demo/"
cp "$2" "$W/demo/zz_probe.go"
GOV=$(sed -n 's/^go //p' "$G/go.mod")
printf 'module probe\n\ngo %s\n' "$GOV" > "$W/demo/go.mod"
# the probe module, gram and the modules gram's go.work joins
{
	printf 'go %s\n\nuse (\n\t./demo\n\t%s\n' "$GOV" "$G"
	sed -n 's#^\t\./\(.*\)$#\1#p' "$G/../go.work" | grep -vx gram |
		while read -r m; do printf '\t%s\n' "$(cd "$G/.." && pwd)/$m"; done
	printf ')\n'
} > "$W/go.work"
python3 - "$W/demo/demo.go" <<'PY'
import re, sys
p = sys.argv[1]; s = open(p).read()
m = re.search(r'type arena struct \{\n', s)
s = s[:m.end()] + '\tprobe *probe\n' + s[m.end():]
i = s.index('a.players = players.NewPlugin(')
j = s.index('\n}\n', i)
s = s[:j] + '\n\ta.probe = newProbe(a)' + s[j:]
s = re.sub(r'(func \(a \*arena\) update\(ctx goke\.RunCtx, d time\.Duration\) \{\n)',
	r'\1\tif a.probe.tick == 0 {\n\t\ta.world.Clock().Faster()\n\t\ta.world.Clock().Faster()\n\t}\n'
	r'\tdefer a.probe.RunPlan(ctx, d)\n', s, count=1)
open(p, 'w').write(s)
PY
cd "$W/demo"
rm -f "$4/probe_bin"
GOWORK=$W/go.work go build -o "$4/probe_bin" .
set +e
timeout "$3"s "$4/probe_bin" > "$4/probe.log" 2>&1
echo "exit $?"
grep -h 'P:\|panic' "$4/probe.log" | sed 's/^.*P://'
