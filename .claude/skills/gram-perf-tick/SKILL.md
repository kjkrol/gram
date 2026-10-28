---
name: gram-perf-tick
description: Wydajność ticku grama — publikacja i sektory świata, kolumny chunków czytane przez aabbworld jako przebiegi, kolizja i motion, kroki ceremony bez alokacji, pula wątków, float32; co każda optymalizacja odpowiedziała, na jakim niezmienniku stoi i ile dała; benchmarki World_*, Collision_*, Effects_Tick, Plans_Tick, Board_*; co próbowano i porzucono. Czytaj przy zmianie świata, motion, kolizji, efektów, planów albo komponentów encji, gdy tick zwolnił, i przed propozycją zmiany gorącej ścieżki ticku.
---

# gram-perf-tick

Protokół pomiaru i pułapki są w `gram-perf`; tu mapa tego, co w ticku zrobiono i na czym to
stoi. Liczby: `BENCHMARKS.md` (sekcje `World_*`, `Collision_*`, „Where the time goes"),
historie: `doc/plans-perf.md` (Etap 1, Fale 1–5, „aabbworld v1.17.0: float32").

## Gdzie idzie czas

Tick (`bench/`, pakiet `bench_test`: Stage zbudowany jak w grze, tykany bez okna): publikacja i
sektory świata (`World_Tick`, `World_Publish`, `World_PositionScan`, `Sectors_Expanse`; tick przy
5 000 encji 37 µs, kiedyś 259), motion i kolizja (`Collision_Tick`, `Collision_Swept`: faza
szeroka i solver aabbworlda, jego `BENCHMARKS.md` „Key takeaways"), zachowanie (`Plans_Tick`,
`Rules_Moving`, `Effects_Tick`, `Marker_*`), plansza i to, co na niej stoi (`Board_GroundAt`,
`Board_Altitude`, `Gravity_Standing`, `Board_Terrain`, `Bullet_Flight`, `Climate_Step`;
benchmarki komponowania planszy odeszły 2026-10-27: komponowanie to od pluginu światła samo
planowanie, ~0,1 µs na klatkę).

## Co zrobiono i na czym stoi

- **Etap 1** (2026-10-19): tick bez `math.Hypot` (`clampStep` inlinuje się tylko bez niego),
  `Simulate` i `RolesOf` wiązane raz, kopia widoków kluczowana wersją gruntu i osłony. Tick
  świata 5 000: 215 → 169 µs, 1 → 0 alokacji; `Views_Copy` 925 µs → 16 ns. Niezmiennik:
  spokojny tick świata i kolizji nie alokuje.
- **Fale 1–5** (2026-10-20…22): aabbworld czyta kolumny chunków goke jako `Run`y zamiast kopii
  `Item` (`World_Tick` 1 000/5 000: 23,3/110,9 → 13,9/58,0 µs; kopia była 52% profilu) i
  przesuwa przebieg jednym wołaniem (−24…26%); indeks budowany przed partią wzroku
  (`SpaceRead.Ready`: bez tego `Vision_Scan` 500 +78%); pula wątków (`Vision_Scan` 9 → 0
  alokacji); kroki ceremony bez alokacji (open 1 000: 2 599 → 90 na tick); uniformy pakowane
  raz, blity bez bufora. Niezmiennik: przestrzeń czyta kolumny chunków, przebudowane po `Sync`;
  nic nie kopiuje `Base`. Reguła przyjęcia od fali 3: mediana ≥ 3% lepsza w reżimie, w który
  zmiana celuje, serie obu stron nienakładające się, żaden reżim gorszy; mniej alokacji przy
  czasie nie gorszym to też zysk (użytkownik, 2026-10-21).
- **float32** (2026-10-25, aabbworld v1.17.0/v1.18.0): każda liczba świata float32
  (`world.Base` 104 → 52 B): `World_Tick` 5 000 43,6 → 37,3 µs, kolizja 4–14% mniej,
  publikacja 7–12% mniej, wzrok 0–5% więcej (konwersje na jego brzegach). Niezmiennik: reguła
  float32 w `CLAUDE.md`, tolerancje przez `f32.Eps`.
- **Sektory i pasma**: supervisor raz na tick po zegarze, systemy w pasmach po polityce
  (`sector.Policy`), encje `Off` poza przestrzenią; `Sectors_Expanse` i `World_Tick` to ich
  miara (`gram-plugins`, wpis `world`).
- **Tempo z udziałów** (2026-10-28, „Udziały tempa"): przebieg `standing` boarda pisze
  `Pace.Share` z `cell.PaceOf(Mobility, Step)` ze stokiem do sąsiada w kierunku marszu
  (`Board_Altitude`, `Gravity_Standing` nad szachownicą wysokości to jego miara); szczegóły w
  `gram-perf-navigation`.

## Próbowano i porzucono

Lustro chunków goke na GPU z wersjami per chunk (każdy chunk w widoku brudzony co tick: ~20%
wolniej; została tylko kopia instancji jednego ruchu), scalony układ siatki rosnącej zasięgiem w
aabbworld (+10–30%), GPU do arytmetyki motion (dispatch kosztuje tyle samo), `Position` poza
CPU. Nie proponować bez nowego powodu.
