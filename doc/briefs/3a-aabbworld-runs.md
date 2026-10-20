# 3a. aabbworld: `Rebuild` z przebiegów kolumn zamiast kopii Item

**Repo, gałąź:** aabbworld (`/home/karol/CODE/aabbworld`, poza `go.work`), `feat/runs` od `main`
(v1.13.0).
**Etap:** 3 (`doc/plans-perf.md` „Etap 3”, kroki 1–2; gram adaptuje w 3b).
**Skille:** `goke` (model chunków, `ChunkSnapshot`) z gram `.claude/skills/goke`, aabbworld
`CLAUDE.md`.

## Cel

Dziś gram kopiuje co tick każdy `Base` (104 B) do `aabbworld.Item` (80 B) i woła `remember`
per identyfikator (54% ticku świata po etapie 1), a silnik kolizji pcha pudełka w kopii, które
gram przepisuje do `Base` przez `Seek`. aabbworld ma czytać pudełka wprost z kolumn chunków
goke przez przebiegi i pisać pchnięte pudełka wprost pod ich adres.

## Kształt (z planu)

Nowe wejście `Rebuild` obok starego: przebieg to `ids []uid.UID64`, wskaźnik bazowy kolumny,
stride, offset pudełka (`plane.AABB`), offset `Capability`, długość. `Grid.index`
(`internal/spatial/grid.go`) i `Pairs` czytają pudełko spod `base + i*stride + boxOff` bez
funkcji per element (między modułami nie inlinują się). Pozycja `at int32` płaska przez sumy
prefiksowe długości przebiegów; `gridCell` z kopią identyfikatora, żeby `Query` nie szukało
przebiegu. Mała lista Item doklejana za przebiegami dla sweepów (pudełko rozciągnięte,
którego nie ma w kolumnie). Silnik (`internal/collide/engine.go` `Tick`) pisze pchnięte
pudełka pod adres przebiegu. `Items()` dla silnika jako widok po `at`.

## Pliki

aabbworld: `space*.go`, `internal/spatial/grid.go`, `internal/collide/engine.go`, testy,
`bench/`. Stare wejście zostaje (testy, lista sweepów). Nie dotykać `plane`, ani tego, co faza
szeroka zwraca.

## Kroki

1. Kontrakt w godoc `Rebuild`: pamięć przebiegów musi stać do ostatniego `Query` lub `Tick`
   przed następnym `Rebuild`; konsument zbiera przebiegi na nowo, gdy zmieniła się wersja
   którejkolwiek tabeli (goke `TableVer`).
2. `Grid` z przebiegami; `broad_phase_test.go` w obu wejściach daje te same pary (test
   parametryzowany wejściem). Testy `Tick`: pchnięcie ląduje w pamięci przebiegu.
3. Benchmarki aabbworld `bench/` przed i po na scenie, która udaje gram: 5 000 pudełek w
   kolumnie o stride 104 (slice struktur z pudełkiem pod offsetem 0), `Rebuild` + `Query`
   + `Tick`; `BENCHMARKS.md` aabbworld (propozycja w raporcie).
4. Sekwencja weryfikacji aabbworld; commity na `feat/runs`.

## Poza zakresem

Wersje per chunk (etap 4): tu wystarczy `TableVer` po stronie konsumenta. Zmiana w zwracanych
parach. Wydanie (tag v1.14.0) i podbicie `gram/go.mod`: użytkownik po przeglądzie.

## Raport

API nowego wejścia (sygnatury), kontrakt pamięci, liczby przed/po, propozycja wpisu do
`CHANGELOG.md` aabbworld i do skilla `gram-plugins` (world/motion/collision: co się zmieni w 3b).
