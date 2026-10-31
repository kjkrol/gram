---
name: gram-perf
description: Indeks wydajności grama — gdzie leżą liczby, protokół pomiaru zmiany (kopia bazowa przeciw drzewu na zmianę, bezczynna maszyna, uczciwe rodzeństwo, -cpu=1, profile, 5–10 minut) ze skryptami measure.sh i compare.py, pułapki pomiaru i odsyłacze do gram-perf-tick (świat, kolizja, efekty), gram-perf-picture (klatka), gram-perf-navigation (A*, routing, pole, HPA*, wycena) i gram-perf-sight (wzrok, SIMD). Czytaj przy planowaniu etapu z doc/plans-perf.md, mierzeniu zmiany, szukaniu regresji albo dotykaniu gorącej ścieżki; po nim tylko skill warstwy, której zmiana dotyka.
---

# gram-perf

Indeks i protokół (użytkownik, 2026-10-28: indeks skilli szczegółowych, nie jeden blob): czytaj
go pierwszy, potem **jeden** skill warstwy, której zmiana dotyka. Liczby: `BENCHMARKS.md`
(sekcja na benchmark, „Where the time goes", „The demos", „How to benchmark"); plan i wynik
etapów: `doc/plans-perf.md`; liczby tak, jak etapy je zmierzyły do v0.6.0:
`doc/benchmarks-history.md`; liczby tagu: `CHANGELOG.md`.

## Skille szczegółowe

- `gram-perf-tick`: świat, sektory, kolizja i motion, efekty i plany, float32.
- `gram-perf-picture`: klatka i obraz w warstwach, płaska plansza na GPU.
- `gram-perf-navigation`: A*, routing, pole grupy, hierarchia, wycena; skill hpastara.
- `gram-perf-sight`: wzrok w SIMD; skill `simd` aabbworlda.

Pomysłu z ich „Odrzucone" nie proponuje się bez nowego powodu.

## Skrypty

- `measure.sh <out> <regex> <baza> <drzewo> [rundy] [count]`: `go test ./bench` na zmianę w
  bazie i drzewie, całe wyjście w `<out>`, na końcu tabela `compare.py`; odmawia przy obciążeniu
  ≥ 1,5. Środowisko stron i flagi w nagłówku (`BASE_ENV`, `TREE_ENV`, `BENCHTIME`, `EXTRA`).
  Pakiety poza `./bench` mierzy się tą samą pętlą ręcznie.
- `compare.py <base.txt> <tree.txt>`: min i mediana ns/op na benchmark ze zmianą w procentach,
  allocs/op.

## Protokół

1. **Krótko** (użytkownik, 2026-10-29): cały pomiar etapu to 5–10 minut maszyny, jedna–dwie
   rundy `-count=1` na stronę. Mierzy się każdy reżim, który zmiana ma (open i crowded, 100 i
   1 000, `map` i `relief`; spokojny i najcięższy), i tylko te. Klatki dem: `gram-probe`.
2. **Bezczynna maszyna, jeden pomiar naraz**; części pracujące równolegle nie mierzą (`CLAUDE.md`,
   „A stage in parts").
3. **Kopia bazowa**: `cp -r` z commita bazowego do scratchpadu albo `git worktree` poza
   `/home/karol/CODE`, nigdy `git stash` ani checkout drzewa użytkownika. Pod `/home/karol/CODE`
   rządzi `go.work`, poza nim buduje się z `GOWORK=off` (tagi z `go.mod`).
4. **Uczciwe rodzeństwo**: goke, ceremony, aabbworld i hpastar obie strony budują tak samo poza
   tym, w którym jest zmiana (własny `go.work` w scratchpadzie przez `GOWORK=<plik>`, baza na
   tagu); nowszy checkout nietkniętego rodzeństwa po jednej stronie mierzy rodzeństwo.
5. **Ten sam build**: `GOEXPERIMENT=simd` albo żaden po obu stronach, zmianę kodu SIMD w obu.
   Binaria testów kompiluje się przed pomiarem.
6. **Zysk** (użytkownik, 2026-10-21): ≥ 3% mediany w reżimie, w który zmiana celuje, serie obu
   stron się nie nakładają, żaden reżim nie traci, także reżimy wołającego; mniej alokacji przy
   czasie nie gorszym to też zysk. ±1–2% to szum, maszyna dryfuje ~10%.
7. **Strata się wyjaśnia przed scaleniem**: `EXTRA='-cpu=1'` oddziela pracę od szeregowania,
   build zwykły zegar AVX2 od kerneli, profil CPU po obu stronach gorącą pętlę; zmiana gorącej
   pętli bez profilu po obu stronach nie jest zmierzona.
8. **Zapis**: `BENCHMARKS.md` (sekcja, metoda, data), wynik etapu w `doc/plans-perf.md`, liczby
   tagu w `CHANGELOG.md`; kopie, wyniki i skrypty sesji zostają w scratchpadzie.

## Pułapki

- `replace` w `go.mod` (`=> ../aabbworld`) bierze checkout nawet z `GOWORK=off`: `go list -m all`
  po obu stronach mówi, co każda buduje. `GOEXPERIMENT` jest Makefile'a: goły `go test` to build
  zwykły.
- `-bench` dopasowuje poziom po poziomie po `/`, a benchmark płytszy niż wzorzec nie biegnie:
  różne głębokości to osobne wzorce.
- Linia loga benchmarku (świat: `bucket=…`) wchodzi między nazwę wyniku a liczby, więc
  `grep '^Benchmark'` gubi wyniki po pierwszym w `-count=N`: zapisuje się całe wyjście.
- Boost: pierwsze ~25–30 s po bezczynności biegną szybciej, a `measure.sh` zaczyna od bazy; duży
  ruch rungu biegnącego pierwszy sprawdza się z zamienionymi stronami. Rungi kilkudziesięciu µs
  przy `200x` to szum: `BENCHTIME=1s`.
- THP (`/sys/kernel/mm/transparent_hugepage/enabled`: `always`) daje dwumodalne czasy po obu
  stronach: duży ruch bez śladu w profilu sprawdza się z `GODEBUG=disablethp=1` po obu stronach,
  zanim nazwie się go regresją.
- Cztery rdzenie, osiem wątków, zegar spada pod AVX2 i z ciepła: `-cpu=1` porównuje pracę
  stabilniej. Benchmarki późne w długim zestawie wychodzą wolniej niż osobno: mierzy się je
  osobno.
- Rozmiar komponentu przesuwa tick bez śladu w zmienianym kodzie: `unit.Mover` 16 → 20 B
  przyspieszył `Navigation_Tick` o 3–19% (`world.moved`, `motion.moveSystem`). Zmianę dodającą
  pole do komponentu porównuje się też z bazą o tym samym układzie (overlay z samym polem).
- `/tmp` to tmpfs z limitem na użytkownika: porzucone `/tmp/go-build*` zapełniają go i linker
  odmawia („disk quota exceeded"). Sprząta się tylko swoje; `GOTMPDIR` na dysku omija limit.
