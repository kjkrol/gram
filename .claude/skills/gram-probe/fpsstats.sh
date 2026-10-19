#!/bin/bash
# fpsstats.sh <log> [skip]: medians of FPS, tick, drawing and presenting from the per-second
# lines after the first skip (default 8) of them, in microseconds
awk -v skip="${2:-8}" '
function us(v,  x) { x=v; if (x ~ /ms/) { sub(/ms.*/,"",x); return x*1000 } sub(/µs.*/,"",x); return x+0 }
/gram: [0-9.]+ FPS/ {
	k++; if (k<=skip) next
	n++
	for (i=1;i<=NF;i++) {
		if ($i=="FPS") fps[n]=$(i-1)+0
		if ($i=="tick") tick[n]=us($(i+1))
		if ($i=="drawing") draw[n]=us($(i+1))
		if ($i=="presenting") pres[n]=us($(i+1))
	}
}
function med(a,n,  i,b) { for (i=1;i<=n;i++) b[i]=a[i]; asort(b); return (n%2)? b[(n+1)/2] : (b[n/2]+b[n/2+1])/2 }
END {
	if (n==0) { print "fps: no lines"; exit }
	printf "fps: %d s measured; FPS median %.0f; tick %.0f us; drawing %.0f us; presenting %.0f us\n", n, med(fps,n), med(tick,n), med(draw,n), med(pres,n)
}' "$1"
