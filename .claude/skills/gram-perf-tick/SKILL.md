---
name: gram-perf-tick
description: Wydajność ticku grama — publikacja i sektory świata, kolumny chunków czytane przez aabbworld jako przebiegi, kolizja i motion, kroki ceremony bez alokacji, pula wątków, float32; niezmienniki i odrzucone pomysły; benchmarki World_*, Collision_*, Effects_Tick, Plans_Tick, Board_*. Czytaj przy zmianie świata, motion, kolizji, efektów, planów albo komponentów encji, gdy tick zwolnił, i przed propozycją zmiany gorącej ścieżki ticku.
---

# gram-perf-tick

Protokół, reguła zysku i pułapki pomiaru: `gram-perf`. Liczby: `BENCHMARKS.md` (`World_*`,
`Collision_*`, „Where the time goes"; fazę szeroką i solver kolizji mierzy `BENCHMARKS.md`
aabbworlda); etapy: `doc/plans-perf.md` (Etap 1, Fale 1–5, „aabbworld v1.17.0").

## Model

Tick to `RunPlan` pluginów i nic więcej; `bench/` buduje Stage jak gra i tyka go bez okna.
Czas idzie na publikację i sektory świata, motion i kolizję (aabbworld), zachowanie (plany,
reguły, efekty, znaczniki) i planszę (grunt, wysokość, stanie).

## Niezmienniki

- Spokojny tick świata, kolizji, terenu, stania i reguł (kroków ceremony) nie alokuje.
- Przestrzeń czyta kolumny chunków goke jako przebiegi (`Run`), zbierane na nowo po `Sync`, i
  przesuwa przebieg jednym wołaniem; nic nie kopiuje `Base` (kopia do `Item` była większością
  profilu).
- `clampStep` bez `math.Hypot` w typowym kroku: inlinuje się tylko bez niego.
- `Simulate` i `RolesOf` wiązane raz, nie na tick; kopia widoków kluczowana wersją gruntu i
  osłony.
- Praca równoległa idzie na pulę gorutyn trzymaną przez całą grę (`internal/parallel`), każdy
  robotnik z własnym `Read(w)` i zapytaniami; żadnych gorutyn na tick.
- Każda liczba świata to float32 (reguła w `CLAUDE.md`): `Base` o połowę mniejsze, kolizja i
  publikacja szybsze; wzrok płaci konwersje na swoich brzegach.
- Supervisor sektorów biegnie raz na tick po zegarze: poziom na obserwatora ze skali
  pokazywanej (`lod.Ladder`, histereza, stopniowanie), tick = minimum; systemy po `lod.Paces`
  liczonych od najdrobniejszego poziomu widzianego przez którykolwiek obserwator (znaczniki
  `Far1..Far5` obok `L0..L6`, 2026-11-04: tam, gdzie widz patrzy najbliżej, czas biegnie pełnym
  krokiem; świat bez obserwatora tyka pełnym krokiem, oszczędność dają agregaty), encje L6 poza
  przestrzenią (`gram-plugins`, wpis `world`). Zoom z góry przez próg znakuje wszystkie żywe
  encje naraz (~200 ns każda, teraz dwa ruchy archetypu na encję): koszt znany, nierozłożony.
- Pętla publikacji (`renderer.publish`, 2026-11-06, `p/publish`): zwykły przebieg po chunku
  (bez pól, nie `Fixed`, banery i liczby wyłączone) jest osobną, ciasną pętlą o takim zbiorze
  zmiennych żywych jak przed płachtami; chunk `Fixed` albo z banerami idzie przez
  `publishStandIns`. Dodanie stanu płachty i banerów do jednej pętli kosztowało +20…28 % na
  każdym rungu `World_Publish` (osiem zbędnych zapisów na stos na encję, choć gałęzie były
  martwe). `seenAny` jest `//go:noinline`: inlinowane do `Publish` dzieliło rejestry z lokalnymi
  `Publish` i przeładowywało slot stosu na każdy widok (~0,7 ns na encję świata). Wspólny
  pomocnik zapisu rekordu kosztuje ~1,8 ns na rekord (odrzucony: rekord pisany inline w obu
  pętlach); zera pisane w kolejności pól struktury łączą się w mniej zapisów; strażnik `progress`
  inlinuje się, gdy odczyt mapy siedzi w `phaseOf`. `figure` ma 88 B (80 przed płachtami):
  samo to kosztuje 3–6 % na gęstych rungach, pokryte powyższym; spakowanie flag wróciłoby do 80.
- Przebieg `standing` planszy pisze `Pace.Share` z `cell.PaceOf` ze stokiem do sąsiada w kierunku
  marszu; jego cena nad reliefem (`Map.Rise` i `exp` na jednostkę na krok) przyjęta jako cena
  strategii (`gram-perf-navigation`).

## Odrzucone

- `Position` na GPU: zostaje autorytatywny na CPU, bo jedenaście systemów czyta go losowo do
  struktur CPU (sektory, zajętość, nawigacja, relief, wzrok, kolizja, renderer); GPU dostaje co
  najwyżej lustro float32.
- GPU do arytmetyki ruchu: to mikrosekundy, a dispatch kosztuje tyle samo.
- Lustro chunków goke na GPU z wersjami per chunk: każdy chunk w widoku brudzony co tick, ~20%
  wolniej; została kopia instancji jednego ruchu.
- Scalony układ siatki rosnącej zasięgiem w aabbworld: +10–30%.
