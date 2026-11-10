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

- Rzecz stojąca na stałe (`Fixed`) ma kosztować zero na krok (2026-11-08, `f/still`): wysokość
  części liczona przy przyjściu i przy zmianie wersji reliefu (`relief/altitude.go`), minimapa
  chodzi po rodzajach ze znakiem, nie pyta przestrzeni o każdą encję. Warstwa nieruchoma
  (`f/space`, 2026-11-08; aabbworld v1.21.0 `Space.RebuildStill`, komórki rzadkie): indekser
  świata oddaje chunki `Fixed` warstwie nieruchomej raz (`Include[Fixed]`), ruchome
  (`Exclude[Fixed]`) przebudowuje jak dotąd, motion nie krokuje `Fixed`, kolizja pomija chunki
  `Fixed` w `rebuild` i trzyma ich caps stałe (`Write.Restill()` po przyjściu kolidera `Fixed` i
  po zdjęciu `Collider`); 30 tys. części: krok 157 → 26 µs. Pułapka: caps pudła nieruchomego nie
  wolno zmienić w miejscu między wywołaniami; każda taka zmiana to `Restill`.
  Pogoda chodzi po komórkach planszy (2 ms/krok przy 640×320, niezależnie od encji). Pułapka:
  tick robi 60 kroków/s obok klatki, najwyżej 5 na klatkę; krok ponad ~15 ms zbija klatkarz
  spiralnie do 11–13 FPS, a adapter budżetu przesuwa tylko rysowanie, nie poziom ticku.
- Profil Madery na L4 po rozruchu (2026-11-08, sonda `scratchpad/prof`): tick 3,8 ms, z czego ~1,4
  ms to pogoda (`weathering.walk`: reguły `Overhead` gry na komórkę raz na sekundę gry, 205 tys.
  komórek; 70 % tego to próbkowanie kopuły chmur na komórkę, `domeLevel.at`: cztery `cellAt` z
  dzieleniami całkowitymi); przez pierwsze ~25 s hierarchia HPA* buduje się w budżecie `buildBudget`
  75 000 kroków na tick (~3 ms na tick, tick 5,4 ms; większy budżet to krótszy rozruch i wyższy
  tick, ta sama praca na każdej maszynie). Pułapka: `Finder.Reads` czyściło `flats` (krok gruntu na
  komórkę, stemplowany przejezdnością i domeną) przy każdym `Read` routera, czyli co tick podczas
  budowy i przy każdym pytaniu: 0,9 ms zerowania; czyści tylko przy innym terenie, wycenie lub
  poziomach (`same`, przez `reflect`, bo interfejsy nieporównywalne panikują na `==`).
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
- Suche rozwinięcie (`preview.go`, `d/preview`, 2026-11-06): sektor czekający na fotografię pod
  gołym rozciągiem pola dostaje podgląd, najwyżej `perFrame` na krok: `Unfold` idzie do reguł
  jak przy rozwinięciu, `SpawnPart` pod podglądem zapisuje migawkę zamiast spawnu, podgląd żyje
  dwa kroki. `World_Preview` (bench/): podgląd i publikacja sektora z 256 częściami, alokacje
  tylko migawek. Koszt leży w regułach gry, nie w świecie: w demie Madery 512 reguł lasu i 1024
  osady idą raz na sektor na bieg (potem kafle). Do 2026-11-09 kamera ortogonalna na L2
  trzymała wszystkie pola w zasięgu `Sight` (skala stała w kadrze: ~30 tys. drzew), a
  perspektywiczna koło wokół oka (punkt za okiem liczy się jak przed nim: 150–200 sektorów fine
  przy 25 pokazanych, 20–25 tys. drzew); od tego dnia sektor poza kadrem (plus pół sektora) nie
  schodzi poniżej L4 poza sektorem punktu widzenia i ośmioma wokół, więc tick L2 nad wyspą to
  lasy w kadrze i pierścień sektorów wokół nich (stopniowanie o jeden poziom).
- Budżet rozwijania (`wholes.go`, `t/budget`, 2026-11-07): `open` rozwija najwyżej
  `Ladder.Unfolds` odcinków na krok (zero = 16), najbliższe punktom widzenia kamer
  (`sectors.cams`) pierwsze, krótszą drogą przez zawijaną oś; `nearest()` liczy się tylko, gdy
  kandydatów jest więcej niż budżet, bez alokacji na ciepło; odcinek czekający nie dostaje encji
  odcinka, `Unfolded` ani `Unfold`, migawka zostaje; sektor wracający na L4 przed kolejką nie
  rozwija się. Zoom L4→L3 nad Maderą: 31 376 części w 17 krokach po ~1800 zamiast jednego.
  Pułapka: najdłuższy krok zoomu (t248, ~170–250 ms w obu drzewach) to nawigacja, nie rozwinięcie:
  jednostki schodzące z grafu abstrakcyjnego pytają o trasy naraz (`traffic.walkOn` →
  `routing.answer` → hpastar).
- Piętra w publikacji (`t/tiers`): tabela poziomów komórek pięter na widok i `Grid.Fit` na rzecz
  dłuższą niż `Small` albo kawałek uchwytu; `World_Seams` (bench/, 1000 rzeczy `Fixed` na szwach
  L3|L4, 200 z nich kawałkami 50 uchwytów): `Fit` to ~27 % publikacji na długich rzeczach (kilka
  dzieleń całkowitych na próbowane piętro); `Fit` w O(1) z pierwszej i ostatniej kolumny i wiersza
  sektorów pudła to dźwignia. `World_Publish` bez płacht nietknięty (dwie kolumny opcjonalne).
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
