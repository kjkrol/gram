# 4b. gram: systemy piszące `Base` przez `Mut`, pomiar brudu, materiał do decyzji `Still`

**Repo, gałąź:** gram, `v/mut` od `perf/chunk-runs` scalonego do `main` i po podbiciu goke
w `go.mod` (worktree `/home/karol/worktrees/gram/mut`). Zależy od 3b i wydania 4a.
**Etap:** 4 (`doc/plans-perf.md` „Etap 4”, akapit „W gram, po wydaniu”).
**Skille:** `goke`, `gram-plugin`, `gram-plugins` (world, motion, collision, navigation,
bullet), `gram-probe`.

## Cel

Systemy piszące `Base` (motion, collision, navigation, bullet, spawn) piszą przez `Mut[T]`,
reszta czyta przez `Comp[T]`; tryb testowy goke włączony w testach gram łapie zapisy bez
`Mut`. Potem liczba: ile chunków z `Base` jest brudnych na tick w demach planszy (5%
poruszających się) i w motion (100%). Od niej zależy decyzja integratora przed etapem 5:
znacznik `Still` nadawany przez velocity, gdy prędkość spada do zera, i wykluczany z zapytania
ruchu (koszt migracji z `BENCHMARKS.md` „Markers”), albo rozbicie `Base` (duże), albo nic.

## Pliki

Wolno: systemy piszące `Base` w `plugins/{motion,collision,navigation,bullet,world}` (tylko
zamiana akcesora i to, co z niej wynika), testy, `bench/world_test.go` (benchmark „brudne
chunki na tick” na scenie 5 000 encji z 10% w ruchu), sonda w kopii dem. Nie wolno: typ `Base`,
goke, aabbworld, ceremony, `examples/`.

## Kroki

1. `grep` wszystkich zapisów do `Base` (również przez wskaźnik ze `Seek`), lista w raporcie.
2. Zamiana na `Mut`; testy pakietów pod opcją trybu testowego goke; sekwencja.
3. Benchmark i sonda: udział brudnych chunków `Base` na tick w board/topography, motion/expanse
   (pasma), motion/collision; liczby do raportu jako wejście decyzji.
4. Pomiar, że `Mut` nic nie kosztuje: `Benchmark_World_Tick` przed/po w granicach dryfu.

## Poza zakresem

Sama decyzja i znacznik `Still` (integrator po raporcie; osobna mała sesja, jeśli wejdzie).
Lustro GPU (5).

## Raport

Lista miejsc zapisu, liczby brudu per demo, `World_Tick` przed/po, propozycje wpisów.
