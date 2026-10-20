# 4b + 5 lokalnie: `Mut` w systemach gram i lustro sprite'ów na GPU, zmierzone razem

**Decyzja użytkownika (2026-10-20):** PR goke #55 (wersje per chunk) nie jest scalany, dopóki
konsument (lustro sprite'ów z etapu 5) nie pokaże zysku. Ta część robi oba kroki lokalnie, na
goke z gałęzi `feat/chunk-versions`, i mierzy klatkę z lustrem i bez. Wynik decyduje, czy goke
wydaje v3.6.0.

**Repo, gałąź:** gram, `m/mirror` od `main` `3627cf8f`, worktree `/home/karol/worktrees/gram/mirror`.
W nim `go.work` (nieśledzony; `.gitignore` go nie ignoruje, więc **nigdy `git add -A` ani
`git add .`**, tylko nazwane ścieżki, i `git status` przed commitem) wskazuje lokalne goke:
`/home/karol/worktrees/goke/chunk-versions` (gałąź `feat/chunk-versions`, tylko do odczytu;
błąd w goke zgłosić w raporcie, nie poprawiać). Ceremony i aabbworld z `go.mod`.
**Skille:** `goke` (tu: `Mut[T]`, `Cursor.Ver()`, `ChunkSnapshot.Ver`, `WithMutCheck`), `gram-render`
(sprite'y, `Frame.DrawAt`, staging), `gram-plugins` (world, `world/internal/look`, motion,
collision, navigation, bullet), `gram-probe`.
**Etap w planie:** `doc/plans-perf.md` „Etap 4” (akapit „W gram, po wydaniu”), „Etap 5”, „Etap 0:
wynik” pkt 4–5, „Fala 1: wynik”; kontrakt wgpu w „Etap 1: wynik”.

## Co wiadomo z 4a

- `Mut[T]`: `Slice`/`At` jak `Comp[T]` plus inkrement licznika chunka kursora. **Brudny znaczy
  „ktoś wziął `Mut` dla tego chunka”, nie „bajty się zmieniły”**: system, który bierze `Mut.Slice`
  w każdym chunku, który odwiedza, brudzi wszystkie.
- Reguła konsumenta: klucz `(ArchID, ChunkPtr)`; zmieniona `TableVer` to cała tabela od nowa;
  ta sama i inna `Ver` to ten chunk; zapomnieć wszystko przy `Reset`/`Load`. Dopisanie encji do
  chunka goke liczy sam.
- Zapisy przez `Column`, `OptColumn`, `OptComp` nie są liczone (dopisać `*cur.Ver()++`). Ceremony
  pisze efekty przez `OptColumn`, więc `WithMutCheck` globalnie w testach gram spanikuje:
  włączać go tylko w testach pakietów piszących `Base`, gdzie ceremony nie pisze, albo nazwać w
  raporcie, czego nie dało się sprawdzić.
- Koszt: +2% na przejściu przez `Mut`, +16% na `SeekH` z `Mut.At`.

## Kroki

1. **`Mut` tam, gdzie `Base` jest pisane, i tylko gdy jest pisane.** `grep` wszystkich zapisów do
   `Base` (także przez wskaźnik ze `Seek`), lista w raporcie. Zapis przez `Mut`; system, który
   odwiedza chunk bez zapisu (encja stoi), nie bierze `Mut` dla tego chunka (czyta `Comp`, a
   `Mut.At` albo `Mut.Slice` dopiero przy pierwszym zapisie w chunku). Testy pakietów z
   `WithMutCheck`, gdzie się da. Sekwencja zielona.
2. **Ile chunków brudzi się na tick.** Sonda albo benchmark liczący chunki zapytania po `Base`,
   których `Ver` lub `TableVer` się zmieniły od poprzedniego ticku, jako udział wszystkich:
   scena 5 000 encji z 0%, 10% i 100% w ruchu, ruszający rozrzuceni losowo w jednym
   archetypie i ruszający skupieni w osobnym; dema w kopii: board/topography, motion/expanse,
   motion/collision. To wejście do oceny, nie bramka: krok 3 i tak idzie.
3. **Lustro** (`doc/plans-perf.md` „Etap 5”, „Kształt” i „Kroki” 1–2). Najpierw projekt w
   raporcie pośrednim w kodzie (komentarz kontraktu typu, nie dokument): co trwałe (pudełko,
   kierunek, wysokość), co per klatka (warstwa, flaga, fog z reguł wyglądu), jak batch z trwałymi
   instancjami wchodzi w `Frame.DrawAt` i w sort Composera, jak przycinanie widokiem
   (`view=5%`) działa bez składania wszystkiego. Instancje per chunk w slabach na GPU,
   odświeżane tylko dla brudnych chunków przez pierścień staging; zwalnianie po zniknięciu
   chunka; konwersja `Base` do float32 przy wysyłce. Ścieżka dzisiejsza zostaje i da się ją
   wybrać w tym samym binarnym (opcja światowa albo renderera), żeby mierzyć w jednym procesie.
4. **Pomiar.** Nowy benchmark z tickiem między klatkami (dzisiejszy `Benchmark_World_Draw` robi
   jeden tick i tylko rysuje, więc lustro wygrałoby w nim za darmo): każda iteracja to tick świata
   i klatka, 5 000 encji, 0%, 10% i 100% w ruchu (rozrzuceni), kadry 100%, 25%, 5%; dzisiejsza
   ścieżka i lustro naprzemiennie, z profilem. Dema w kopii (board/topography, motion/collision)
   z `GRAM_FPS_LOG=1`: „drawing” z lustrem i bez. `GRAM_SHOT` z `PROBE_SEED=7`: obrazy identyczne
   (różnica pikseli policzona).
5. **Wariant `Still`, tylko jeśli krok 2 pokaże, że w demach planszy brudzi się większość
   chunków:** znacznik `Still` nadawany, gdy prędkość spada do zera, wykluczany z zapytania ruchu
   (migracja przy zatrzymaniu i ruszeniu), osobny commit, zmierzony jak w kroku 4 (koszt
   migracji z `BENCHMARKS.md` „Markers”). Jeśli nie wchodzi, powiedzieć dlaczego.

## Pliki

Wolno: systemy piszące `Base` w `plugins/{motion,collision,navigation,bullet,world,…}` (tylko
akcesor i to, co z niego wynika), renderer encji świata i `plugins/world/internal/look/*`,
`render/sprites.go`, `render/gpu/*` (staging, nowy typ lustra), `bench/world_test.go`, testy.
Nie wolno: goke, ceremony, aabbworld, typ `Base`, `examples/`, dokumenty i skille. Szew z częścią
2b (gałąź `n/bodies`, równolegle): ona zmienia `plugins/navigation/{bodies,body_keeping,keeping,
navigation_system,bump}.go`; tu w nawigacji tylko zamiana akcesora zapisu `Base`, minimalna.

## Raport

Commity; lista miejsc zapisu `Base`; udział brudnych chunków per scena i demo; projekt lustra
w kilku zdaniach; liczby z kroku 4 (tick+klatka per scena i kadr, ścieżka dzisiejsza i lustro,
obciążenie maszyny), „drawing” w demach, różnice zrzutów; wariant `Still`, jeśli był; co odbiega
od briefu; czego nie zrobiono; **jednoznaczna ocena: czy lustro z wersjami goke daje zysk wart
wydania goke v3.6.0, w jakich scenach i ile**; proponowane wpisy do `CHANGELOG.md`, skilli i
`BENCHMARKS.md`.
