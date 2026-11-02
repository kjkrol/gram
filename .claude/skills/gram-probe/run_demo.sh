#!/bin/bash
# run_demo.sh build <demo dir> <probe.go or a probes/ dir> <work dir>: copies the demo into a
# module of its own under the work dir, hooks the probe into the copy (no tempo change) and
# builds <work dir>/bin. PROBE_TREE builds another copy of gram (a baseline), as probe.sh does;
# PROBE_SEED fixes the demo's random seed, PROBE_SED edits the copied demo's .go files (a sed
# expression).
# run_demo.sh run <work dir> <real seconds> <log file>: runs the binary under timeout, the
# environment passed through, the whole output kept in the log file.
set -e
R=$(git -C "$(dirname "$0")" rev-parse --show-toplevel)
G=${PROBE_TREE:-$R}
case "$1" in
build)
	W=$4/probews
	rm -rf "$W"; mkdir -p "$W/demo"
	cp -r "$G/$2"/* "$W/demo/"
	if [ -d "$3" ]; then cp "$3"/*.go "$W/demo/"; else cp "$3" "$W/demo/zz_probe.go"; fi
	[ -n "$PROBE_SEED" ] && sed -i "s/uint64(time.Now().UnixNano())/$PROBE_SEED/" "$W/demo/demo.go"
	[ -n "$PROBE_SED" ] && sed -i "$PROBE_SED" "$W"/demo/*.go
	GOV=$(sed -n 's/^go //p' "$G/go.mod")
	printf 'module probe\n\ngo %s\n' "$GOV" > "$W/demo/go.mod"
	{
		printf 'go %s\n\nuse (\n\t./demo\n\t%s\n' "$GOV" "$G"
		sed -n 's#^\t\./\(.*\)$#\1#p' "$R/../go.work" | grep -vx gram |
			while read -r m; do printf '\t%s\n' "$(cd "$R/.." && pwd)/$m"; done
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
	r'\1\tdefer a.probe.RunPlan(ctx, d)\n', s, count=1)
open(p, 'w').write(s)
PY
	cd "$W/demo"
	GOWORK=$W/go.work go build -o "$4/bin" .
	;;
run)
	set +e
	timeout "$3"s "$2/bin" > "$4" 2>&1
	echo "exit $? -> $4"
	;;
esac
