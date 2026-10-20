# 4a. goke: wersje zmian per chunk, `Mut[T]`, `Cursor.Ver`

**Repo, gałąź:** goke (`/home/karol/CODE/goke`, v3.5.0), `feat/chunk-versions` od `main`.
**Etap:** 4 (`doc/plans-perf.md` „Etap 4”, „Kształt” 1–5).
**Skille:** `goke` (z gram `.claude/skills/goke`: `scripts/api.sh`, release flow), goke
`CLAUDE.md`/`README.md`, `BENCHMARKS.md` goke (metoda).

## Cel

goke nie wie o zapisach danych: `Table.version` rośnie tylko przy usunięciu i relokacji.
Konsument (lustro chunków na GPU w etapie 5, przebiegi aabbworld, zapis gry) ma wiedzieć,
które chunki zmieniły dane od ostatniego spojrzenia, kosztem zera w ciele pętli.

## Kształt (z planu)

1. `vers []uint32` w `chunk.Pack` (`internal/chunk/pack.go`) równolegle do `chunks`;
   `AddChunks`, `trimTrailing`, `SwapChunks` utrzymują. Nie w pamięci chunka (kolumna
   identyfikatorów zostaje pod offsetem 0; MassMigrate na tym polega).
2. `Cursor.Ver *uint32` (`iter/cursor.go`), ustawiany w `Table.FillCursorNext` i w `Seek`.
3. Akcesor `Mut[T]` jak `Comp[T]` (`comp.go`), którego `Slice`/`At` robią inkrement przez
   `cur.Ver`. `Comp[T]` zostaje odczytem (konwencja; Go nie ma wycinków tylko do odczytu).
   Licznik monotoniczny per chunk, bez zegara globalnego; konsument pamięta ostatnią widzianą
   wartość.
4. `ChunkSnapshot.Ver`. Reguła konsumenta: zmieniła się `TableVer`, to cała tabela od nowa;
   nie zmieniła się, to porównanie `Ver` per chunk.
5. Tryb testowy (build tag albo opcja ECS): w `Sync` skrót bajtów chunka porównywany z
   poprzednim; zmiana bez `Mut` to panika z nazwą archetypu.

## Pliki

goke: `internal/chunk/pack.go`, `internal/colstore/table.go`, `iter/cursor.go`, `comp.go`
(+ `mut.go`), `internal/query/matcher.go` (`ChunkSnapshot`), `internal/bulk`, testy, `bench/`.
Metody generyczne są dostępne (Go 1.27). API publiczne sprawdzić `scripts/api.sh` przed i po;
tylko dodatki, bez łamania.

## Kroki

1. Benchmarki goke `bench/` przed (iteracja `Comp[T]` po 1 024 i 65 536 encjach, `Seek`).
2. Punkty 1–4; testy: wersja rośnie przy `Mut.Slice`/`At`, nie przy `Comp`; `SwapChunks` i
   `trimTrailing` przenoszą wersje z chunkami; `ChunkSnapshot.Ver` zgodne z `Cursor.Ver`;
   `TableVer` bez zmian w znaczeniu.
3. Punkt 5 za opcją ECS; test, że zapis bez `Mut` panikuje w trybie testowym, a bez opcji nic
   nie kosztuje (benchmark bez zmian).
4. Benchmarki po: koszt to jeden zapis wskaźnika na chunk na zapytanie i jeden inkrement na
   `Slice` przez `Mut`, zero w ciele pętli; `BENCHMARKS.md` goke (propozycja w raporcie).
5. Sekwencja weryfikacji goke (`make` / `go test ./...` z `-race`, `scripts/api.sh`).

## Poza zakresem

`RemoveComp` wielu komponentów jednym poleceniem CmdBuf: tylko uwaga w raporcie. Zmiany w
gram (4b). Wydanie (v3.6.0) i podbicie `gram/go.mod`: użytkownik po przeglądzie.

## Raport

API (sygnatury `Mut[T]`, `Cursor.Ver`, `ChunkSnapshot.Ver`, opcja trybu testowego), liczby
przed/po, propozycja wpisu do `CHANGELOG.md` goke i do skilla `goke` w gram.
