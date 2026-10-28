# Etap 4: kolumna nad polem (szkic do decyzji użytkownika, 2026-10-28)

Gałąź `feat/column` od `main` (po v0.9.0), worktree `/home/karol/worktrees/gram/column`. Plan:
`doc/plans-perf.md` „Kolejka i pole grupy" (etap 4) i „: wynik" etapów 1–3; zasady wspólne:
`doc/briefs/README.md`. Skille: `gram-plugin` (komendy, fakty), `gram-plugins` (navigation),
`gram-perf-navigation` (pole, kolejka), `ceremony` (fakty dla planów). **Bez pomiarów** w części.

## Cel

Marsz grupy gęsiego z porządkiem z planu, nie z listy ani z postrzegania: nikt nie wyprzedza,
czoło czeka na ogon, odstęp trzymany zanim dojdzie do uderzenia. Szerokość czoła wynika z
terenu (front na otwartym, jedno ogniwo w przełęczy): pole, zajętość i podążanie z etapu 2 już
to robią; kolumna dokłada porządek i tempo. Formacje kształtne (linia, klin, „przyjmij
formację") to osobny feature.

## Decyzje do podjęcia przez użytkownika (propozycje)

1. **Rozkaz**: `MoveTo{Column: true}` (pole komendy; domyślnie fałsz: SC2 nie trzyma kolumn)
   zamiast osobnej komendy; rozkaz członka pamięta tryb (`MoveOrder.Column bool`, zapisywany).
2. **Porządek**: koordynator, w przebiegu liczącym partię (już jest), sortuje członków partii po
   `Field.Cost` ich komórek (albo po `Path.Index` z trasy, gdy pola nie ma) raz na Update z
   prośbą albo co k ticków, i pisze na rozkazie członka `AheadOf uid.UID64` (ten tuż przed nim w
   porządku; 0 dla czoła) i `Lag float32` (koszt do celu dzielący go od tego przed nim), a na
   rozkazie czoła `TailLag float32` (ile odstaje ogon). Wiedza planu, nie postrzeganie.
3. **Tempo**: keeping w `Steer`: członek kolumny z `Lag` poniżej odstępu (`(2·half + gap)` w
   jednostkach kosztu, czyli w długościach komórki) ogranicza prędkość chcianą do prędkości
   `AheadOf` (jak `Trail` z etapu 2, bez dotyku; wyszukanie w indeksie ciał); czoło z `TailLag`
   powyżej `k` odstępów (propozycja `k = 3`) staje (`Hold` bez `WaitedOut`), aż ogon nadrobi.
   Śmierć, rozdzielenie, scalenie: sortowanie każdego Update załatwia; nic do łatania.
4. **Fakty** dla planów (umysłów): nowy fakt `InColumn{Lag, Ahead}` (opcjonalnie; etap bez
   niego jest kompletny).
5. **Bez pola** (partia < 4, plansza za duża): kolumna zdegradowana do zwykłego marszu (raz w
   logu świata).

## Pliki (do potwierdzenia)

`internal/order/{commands.go (MoveTo.Column), order.go (Column, AheadOf, Lag, TailLag)}`,
`internal/routing/{coordinator.go (porządek partii), system.go (tryb z komendy)}`,
`internal/spacing/{body_keeping.go, keeping.go} (Steer: czapka po AheadOf; czoło: Hold)`,
`internal/traffic/system.go` (tylko jeśli `Hold` czoła wymaga nowego wejścia), testy pakietu
`navigation` (`column_test.go`). Szwy nie do ruszenia: pole, dotyk, reguły tłumu, planner.

## Testy

Kolumna 10 przez korytarz jednej komórki dochodzi w komplecie, w porządku startu; czoło czeka,
gdy ogon zatrzymany (`halted` z rigu etapu 2); usunięcie środkowego nie zmienia porządku
reszty; dwie grupy w jednej przełęczy; `Navigation_Tick/grouped` w szumie (sortowanie partii to
mikrosekundy). Sonda: 30 górali kolumną (`Column: true`) przez przełęcz: porządek startu
zachowany na szczycie, nikt nie wyprzedza.

## Poza zakresem

Formacje kształtne; HPA*; zmiany pola; `GivingWay` przeżywające `Advance` (zauważone w etapie
3, osobna poprawka).
