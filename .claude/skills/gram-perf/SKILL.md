---
name: gram-perf
description: Indeks wydajności grama — gdzie leżą liczby (BENCHMARKS.md, doc/plans-perf.md, doc/benchmarks-history.md, CHANGELOG.md), protokół pomiaru zmiany (kopia bazowa przeciw drzewu, na zmianę, bezczynna maszyna, uczciwe rodzeństwo, min z przebiegów, -cpu=1, profile) ze skryptami measure.sh i compare.py, pułapki, i odsyłacze do skilli szczegółowych: gram-perf-tick (świat, kolizja, efekty), gram-perf-picture (klatka i obraz), gram-perf-navigation (A*, routing, wycena) i gram-perf-sight (wzrok, SIMD). Czytaj przy planowaniu etapu z doc/plans-perf.md, mierzeniu zmiany, szukaniu regresji albo dotykaniu gorącej ścieżki; po nim tylko skill warstwy, której zmiana dotyka.
---

# gram-perf

Liczby leżą w `BENCHMARKS.md` (sekcja na benchmark, metoda, klatki dem, mierzone na nowo przy
v0.6.0), historie etapów w `doc/plans-perf.md` (po polsku: plan i wynik etapu), liczby tak, jak
każdy etap je zmierzył, w `doc/benchmarks-history.md`; `CHANGELOG.md` niesie liczby tagu. Ten
skill jest indeksem i protokołem: czytaj go pierwszy, potem **jeden** skill warstwy, której
zmiana dotyka, nigdy wszystkie.

## Skille szczegółowe

- `gram-perf-tick`: tick świata (publikacja, sektory, kolumny chunków), kolizja i motion,
  efekty i plany, float32; benchmarki `World_*`, `Collision_*`, `Effects_Tick`, `Plans_Tick`,
  `Board_*`.
- `gram-perf-picture`: klatka i obraz w warstwach (pierścienie, dźwignie, figury ostre, lustro),
  płaska plansza na GPU; benchmarki `World_Draw`, `Frame_*`, `Text_Panel`, `Painter_Bake`.
- `gram-perf-navigation`: A* i routing (astar na tablicach po id, odpowiedzi pamiętane, pula),
  keepingi, **wycena kroków w udziałach** i jej niezmiennik; benchmarki `Navigation_*`,
  `Routing_*`, `PathFinder_Terrain`; skill astara pod
  `/home/karol/CODE/astar/.claude/skills/astar/SKILL.md` (czytany ścieżką).
- `gram-perf-sight`: wzrok w SIMD i tylko tam, gdzie czytany; benchmarki `Vision_*`,
  `Views_Copy`; skill `simd` aabbworlda pod
  `/home/karol/CODE/aabbworld/.claude/skills/simd/SKILL.md` (czytany ścieżką).

## Pomiar zmiany

1. **Bezczynna maszyna, jeden pomiar naraz.** Części i agenci niczego nie mierzą; `measure.sh`
   odmawia przy obciążeniu ponad 1,5. Liczba wzięta obok innego przebiegu nie jest liczbą.
2. **Kopia bazowa.** `cp -r` drzewa z commita bazowego do scratchpadu (nigdy `git stash` ani
   checkout drzewa użytkownika; `git worktree` poza `/home/karol/CODE` też służy).
   `/home/karol/CODE/go.work` rządzi wszystkim pod sobą; kopia gdzie indziej buduje z
   `GOWORK=off`: tagi z `go.mod` z cache, albo checkout rodzeństwa, do którego `go.mod` ma
   `replace`.
3. **Uczciwe rodzeństwo.** Obie strony budują goke, ceremony, aabbworld i astar tak samo, poza
   tym, w którym jest zmiana: własny `go.work` drzewa w scratchpadzie wymienia drzewo i ten
   jeden checkout (`use (/home/karol/CODE/gram /home/karol/CODE/astar)`), `GOWORK=<ten plik>`;
   baza bierze tag. Strona na cache przeciw stronie na nowszym checkoucie nietkniętego
   rodzeństwa mierzy rodzeństwo (brudne lokalne goke z 2026-10-28 to przykład).
4. **Ten sam build po obu stronach**: `GOEXPERIMENT=simd` (Makefile, CI) albo żaden; zmiana w
   kodzie SIMD mierzona w obu. Pierwszy przebieg strony kompiluje: najpierw przebieg próbny.
5. **Na zmianę, min.** `measure.sh <out> <regex> <baza> <drzewo> [rundy] [count]` z
   `BASE_ENV`/`TREE_ENV` (i `BENCHTIME`, `EXTRA`) biega po `./bench` na zmianę (3 rundy
   `-count=2` to zwykłe sześć), `compare.py` daje min i medianę na benchmark ze zmianą w
   procentach; pakiety poza `./bench` (`plugins/navigation`, `pathfind`) tą samą pętlą ręcznie;
   klatki dem biorą mediany linii na sekundę (`gram-probe`).
6. **Co się liczy.** Maszyna dryfuje ~10% między przebiegami; ±1–2% w zestawie to nic. Zysk
   liczy się od ≥ 3% mediany w reżimie, w który celuje, przy nienakładających się seriach obu
   stron i bez straty w innym reżimie; mniej alokacji przy czasie nie gorszym to też zysk.
   Stratę w jednym reżimie wyjaśnia się przed scaleniem: `EXTRA='-cpu=1'` oddziela pracę od
   szeregowania, build zwykły zegar AVX2 od kerneli, profil CPU (`-cpuprofile`, `go tool pprof
   -top`) przed i po gorącą pętlę; zmiana gorącej pętli bez profilu po obu stronach nie jest
   zmierzona.
7. **Reżimy.** Szczeble benchmarku to reżimy (open i crowded, 100 i 1 000, `profile` i `step`,
   `map` i `relief`); zmianę mierzy się w każdym, jaki ma, spokojnym i najcięższym, nigdy tylko
   w tym, dla którego ją pisano.
8. **Potem zapis.** Liczby do `BENCHMARKS.md` (sekcja, metoda, data), wynik etapu do
   `doc/plans-perf.md`, liczby tagu do `CHANGELOG.md`; kopie, wyniki i skrypty sesji zostają w
   scratchpadzie.

## Pułapki

- `replace` w `go.mod` (`=> ../aabbworld`) bierze checkout nawet z `GOWORK=off`: wiedz, co
  każda strona buduje (`go list -m all | grep aabbworld` po obu).
- `GOEXPERIMENT` jest Makefile'a, nie powłoki: goły `go test` to build zwykły.
- Go dopasowuje `-bench` poziom po poziomie i benchmark płytszy niż wzorzec nie biegnie:
  `Solve/Size/CostWeight/Dict` (cztery poziomy) i `SolveInto/Size/Dict` (trzy) to dwa wzorce.
- Benchmarki wzroku skanują każdego obserwatora (`WithScanAll()`), gra tylko czytanych: udział
  skanu w grze jest mniejszy, niż mówi benchmark.
- Readback klatki headless; panel telemetrii, losowa pogoda i losowa scena na obrazie: szum,
  nie różnica (`gram-probe`, „Checking a change").
- Linia loga benchmarku (świat: `maxEntities=… → bucket=…`) wchodzi między nazwę wyniku a jego
  liczby, więc `grep '^Benchmark'` gubi każdy wynik po pierwszym w `-count=N`: do 2026-10-28
  `measure.sh` tak robił i „trzy rundy `-count=2`" było trzema próbkami, nie sześcioma.
  Zapisuje się całe wyjście, a `compare.py` przypisuje gołą linię liczb ostatniej nazwie.
- Reżim równoległy na laptopie: cztery rdzenie, osiem wątków, zegar spada pod AVX2 albo z
  ciepła; liczba szeregowa (`-cpu=1`) jest stabilniejsza do porównania pracy.
- Długi przebieg grzeje maszynę: w 15-minutowym zestawie z 2026-10-27 benchmarki późne w jego
  kolejności (`Text_Panel`, `Frame_UI`, `Vision_Relief`) wyszły 25–150% wolniej niż osobno,
  ticki przed nimi w 5%. Zapis (`make bench-save`) sprawdza się, mierząc późne osobno;
  porównanie i tak zmienia strony.
- `/tmp` to tmpfs z limitem na użytkownika: porzucone `/tmp/go-build*` i scratchpady innych
  sesji zapełniają go, a linker odmawia („disk quota exceeded"); sprzątać tylko swoje
  (`GOTMPDIR` na dysku omija limit).
- **Boost pierwszej rundy** (2026-10-29): `measure.sh` zaczyna każdy zestaw od bazy zaraz po
  czekaniu na bezczynność, a pierwsze ~25–30 s biegną z boostem zegara: próbki bazy z pierwszej
  rundy wychodzą 10–20% szybciej niż każda późniejsza (zamiana stron odwraca znak). Duży ruch
  rungu, który biegnie pierwszy w kolejności źródła, sprawdzić z zamienionymi stronami albo
  bez pierwszej rundy; rungi 2–60 µs przy 200x trwają ~10 ms i mówią tylko szum: mierzyć je z
  `BENCHTIME=1s`.
- **Transparent huge pages** (`/sys/kernel/mm/transparent_hugepage/enabled` `[always]` tu):
  układ pamięci daje dwumodalne czasy po obu stronach (`Routing_Routes/workers=1` 230–245
  wobec 257–277 ms; `crowded,100` przy 2000x +28% z THP, −0,2% bez). Duży ruch bez śladu w
  profilu sprawdzić z `GODEBUG=disablethp=1` po obu stronach, zanim nazwie się go regresją.

## Odsyłacze

- `BENCHMARKS.md`: „How to benchmark", „Where the time goes", „The demos", „The island's frame";
  `doc/benchmarks-history.md`: te same sekcje tak, jak etapy je mierzyły, do v0.6.0.
- `doc/plans-perf.md`: „Co wiadomo na starcie", „Wspólne dla każdego etapu", „wynik" etapu.
- `gram-probe`: „Checking a change", „Measuring a demo's frames and its garbage collector".
- `CLAUDE.md`, „A stage in parts": kto mierzy (integrator sam, po częściach) i kiedy.
