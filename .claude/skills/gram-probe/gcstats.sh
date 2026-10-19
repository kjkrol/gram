#!/bin/bash
# gcstats.sh <log>: from gctrace lines, the count of cycles, the span, cycles a second, the
# median and the highest heap after a cycle, the median goal and the cumulative CPU share
awk '
/^gc [0-9]+ @/ {
	n++; t=$3; sub(/^@/,"",t); sub(/s$/,"",t); last=t; if (n==1) first=t
	pct=$4; sub(/%:/,"",pct); cpu=pct
	for (i=1;i<=NF;i++) if ($i ~ /->/) { split($i,h,"->"); live[n]=h[3]+0; if (h[2]+0>peak) peak=h[2]+0; goal[n]=$(i+2)+0 }
}
function med(a,n,  i,b) { for (i=1;i<=n;i++) b[i]=a[i]; asort(b); return (n%2)? b[(n+1)/2] : (b[n/2]+b[n/2+1])/2 }
END {
	if (n==0) { print "gc: none"; exit }
	span=last-first; if (span<=0) span=1
	printf "gc: %d cycles over %.0fs = %.2f/s; live heap median %d MB, peak %d MB; goal median %d MB; cpu %s%%\n", n, span, n/span, med(live,n), peak, med(goal,n), cpu
}' "$1"
