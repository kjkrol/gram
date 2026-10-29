# Etap A: przejezdność planszy i liczniki nawigacji (2026-10-29)

Gałąź `feat/passage` od `main` (v0.10.0), worktree `/home/karol/worktrees/gram/passage`.
Plan: `doc/plans-perf.md` „HPA*" (etap A). Zasady wspólne: `doc/briefs/README.md`. Skille
przed kodem: `gram-plugin`, `gram-plugins` (board: `Cells`, `CellVersion`, `Changes`,
`Version`; navigation: odpowiedzi pamiętane, pola, `flatStep`), `gram-perf-navigation`
(niezmiennik: co szukanie czyta, liczy się w wersji przed następnym szukaniem). **Bez
pomiarów czasu** (osobny brief dla agenta mierzącego).

## Cel

Plansza odróżnia zmiany tego, co krok czyta (przejezdność), od zmian tylko rysowanych albo
regułowych: drugi licznik obok `Version`. Nawigacja pamięta po przejezdności, więc kołysanie
lasu (`Kind.Sway` przez `effect.Alter` co przejście wiatru) i śnieg bez zmiany cen nie gaszą
odpowiedzi pamiętanych, pól i udziałów płaskich; lód (zmiana `Allows`/`Pace`), kształtowanie
reliefu i mosty gaszą je jak dziś. Liczniki nawigacji odsłonięte dla sond i panelu.

## Kontrakty (nazwy ustalone; odstępstwa w raporcie)

- **Rzut przejezdny komórki** (`plugins/board/internal/terrain`): z `Kind(c)` złożonego
  (grunt → droga → zabudowa, `cells.go:87–103`): `Allows`, `Solid`, `Graded`, `Pace`,
  `Paces`; z `Way(c)`: rodzaj (`Name`), `Width`, `Links`; `Built(c)` przez wartość (`Floor`,
  `Wall`, `Rail`). Pomijane: `Veil`, `Veils`, `Height`, `Sway`, `Takes`, `SpriteID`, `Fade`,
  `Mix`, nazwa i grupa komórki. Trzymany jako hasz 64-bit porównywalnej struktury rzutu
  (`hash/maphash`, `maphash.Comparable`) na ordinal: `passage []uint64`.
- **Liczniki** (`Cells`, `BoardRead`, `board.go` obok `CellVersion`/`Changes`/`Version`):
  `Passage() uint64` — „counts the changes to what a step over the cells reads; while it
  stays as it was, no cell's passage changed; it never starts over" — i `CellPassage(c)
  uint64` — stempel `Passage` ostatniej zmiany przejezdności komórki, `max(stempel,
  wszystkie)` jak `CellVersion`. `Version`/`CellVersion`/`Changes` bez zmian.
- **Stemplowanie**: `Touch(c)` (wysokości: stok) liczy się zawsze; `Set`/`SetWay`/`SetAll` i
  przebieg efektów (`entities.go:166–203`, po porównaniu znaczników `matters`) przeliczają
  rzut dotkniętej komórki po zastosowaniu zmiany i stemplują tylko, gdy hasz się zmienił;
  `touchAll` (wczytanie, `made`) stempluje wszystkie bezwarunkowo i przelicza hasze. Koszt:
  rzut tylko dla dotkniętych komórek, nigdy na tick.
- **Nawigacja po `Passage`**: `pathfind.Finder.versioned` (`finder.go:85`; `reads.go:100`
  `flatStep`), `spacing.Answers` (`keeping.go:84, 116, 167`), `spacing.Fields` (`fields.go:54–65,
  91`), traffic `terrainSeen` (`system.go:159–160`): asercje `interface{ Version() uint64 }` →
  `interface{ Passage() uint64 }`; `cell.TerrainMap` i każdy typ udający teren w testach dostaje
  `Passage` (`grep -rn 'Version() uint64' plugins/navigation plugins/board`). `BoardRead.Version`
  zostaje dla rysowania (`drawn.go`), reguł i podglądów.
- **Liczniki nawigacji**: `navigation.Counts{Searches, Walks, Fills, Climbs, Builds, Expansions
  uint64}` (typ w pakiecie głównym), `Read.Counts()` (suma po routerach koordynatora i keepingu
  traffic; `Expansions` = wywołania `leveledFrom` i `plainFrom` w `pathfind.Finder`
  (`Expansions()`); `Climbs` i `Builds` 0 do etapu B) i linia `render.Reporter` w panelu
  telemetrii („Routes: searches N, walks N, fills N, expansions N"; wzór: kolizja `stats.go`).

## Pliki

Wolno: `plugins/board/internal/terrain/{cells.go, entities.go}` i ich testy, `plugins/board/
board.go` (dwie metody), `plugins/board/doc.go` (zdanie przy `Version`), `plugins/board/cell/
terrain.go` (`TerrainMap.Passage`), testy board; `plugins/navigation/internal/pathfind/
{finder.go (versioned, liczniki), reads.go}`, `internal/spacing/{keeping.go, fields.go,
path_finder.go}`, `internal/traffic/system.go` (tylko `terrainSeen`), `internal/routing/
coordinator.go` (suma liczników), `plugins/navigation/{rw.go, plugin.go, doc.go, stats.go
(nowy)}`, testy nawigacji. Nie ruszać: planner (`leveledFrom`, `step`, `price`), keepingi poza
kluczem wersji, dotyk i reguły tłumu, dema, zapisy (nic nie zapisujemy; `passage` liczone po
wczytaniu).

## Kroki

1. Rzut i hasz, liczniki w `Cells`, `BoardRead.Passage`/`CellPassage`; testy board:
   `TestCells_ASwayAltersTheVersionNotThePassage` (efekt `Alter` zmieniający tylko `Sway`:
   `Version` i `CellVersion` rosną, `Passage` i `CellPassage` nie), `…AnIceAltersBoth`
   (`Allows`/`Pace`), `…ATouchCountsAsPassage`, `…ASetDifferingInSpriteAloneIsNoPassage`,
   `…ALoadStampsEveryCellsPassage`, `…PassageNeverStartsOver`.
2. Nawigacja po `Passage` (cztery miejsca) i `TerrainMap`; testy: `TestKept_ASwayKeepsTheAnswers`
   (dotknięcie efektem kołysania między dwoma pytaniami: `Searches` bez przyrostu),
   `TestGroup_ASwayKeepsTheField` (`Fills` bez przyrostu), `TestNavigation_AnIceDropsTheAnswers`;
   istniejące `TestKept_*`, `TestGroup_*`, `TestColumn_*`, `TestRoutes_AsPinned` zielone (tam
   plansza zmienia się przez `Set`/`Touch`, przejezdność rośnie jak dziś).
3. `Counts`, `Read.Counts()`, reporter; test `TestNavigation_CountsAreTheRoutersSum`.
4. Sekwencja weryfikacji, `-race` na `./plugins/board/... ./plugins/navigation/...`, SIMD,
   `golines -m 100 -t 4 --no-reformat-tags -l .`; commity na `feat/passage` z prefiksami i
   trailerem `Claude-Session: https://claude.ai/code/session_014GbD5V5FZgxcEYK1zPRzjF` (bez
   `Co-Authored-By`); bez docs/CHANGELOG/BENCHMARKS/go.mod.

## Poza zakresem

Hierarchia, próg, `Reaches` (etap B); wiatr w wycenie żeglugi i lotu; pomiary i sonda (agent
mierzący z briefu `m-passage.md`).

## Raport

Jak w `t4-column.md`: commity; odstępstwa od kontraktu i dlaczego; czego nie zrobiono;
ryzyka (np. rzut pomijający pole, które `step` czyta: lista pól `step` sprawdzona linijka po
linijce w `finder.go:455–468` i `reads.go`); proponowane wpisy do CHANGELOG (temat v0.11.0),
skilli `gram-plugins` (board i navigation) i `gram-perf-navigation`; komendy benchmarków.
