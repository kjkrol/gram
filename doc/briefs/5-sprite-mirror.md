# 5. Lustro chunków na GPU dla renderera encji (brief wstępny, do uzupełnienia po decyzji)

**Repo, gałąź:** gram, `m/mirror` od `main` po etapach 3 i 4 (worktree
`/home/karol/worktrees/gram/mirror`). Rusza dopiero po decyzji integratora o znaczniku `Still`
(raport 4b) i po jej wdrożeniu; bramka otwarta (etap 0 pkt 5: składanie instancji 59% klatki
`World_Draw`).
**Etap:** 5 (`doc/plans-perf.md` „Etap 5”, pełny tekst tam; kontrakt wgpu w „Etap 1: wynik”).
**Skille:** `gram-render`, `gram-plugins` (world, `world/internal/look`), `goke`.

## Cel

Instancje sprite'ów trzymane na GPU per chunk w slabach, odświeżane tylko dla chunków, których
`Ver` lub `TableVer` się zmieniły (konwersja `Base` do float32 przy wysyłce przez pierścień
staging); rysowanie jak dziś jednym batchem w `Frame.DrawAt`. Warstwy i widoczność z reguł
wyglądu per klatka jako mała kolumna obok trwałych instancji.

## Pliki (wstępnie)

`render/sprites.go`, `render/gpu/staging.go`, nowy typ lustra w `render/gpu` albo
`plugins/world/internal/look` (rejestr chunków po `ChunkPtr` z wersją, alokator offsetów,
wysyłka brudnych, zwalnianie), `plugins/world/internal/look/flat.go`, testy, `bench/world_test.go`
(`Benchmark_World_Draw` w trzech kadrach, scena 5 000 encji z 10% w ruchu).

## Kroki (z planu)

1. Projekt na stronie w raporcie wstępnym (co trwałe, co per klatka, jak batch wpisuje się w
   `DrawAt` i w sort Composera) zanim ruszy kod; integrator zatwierdza.
2. Lustro; zero zależności goke od gogpu (wejściem `ChunkSnapshot` i przebiegi z etapu 3).
3. Pomiar `World_Draw` przed/po z profilem; `GRAM_SHOT` motion/collision i board/topography:
   obrazy identyczne.

## Raport

Jak w README; dodatkowo decyzja, czy lustro jest w `render/gpu`, czy w `look`, i dlaczego.
