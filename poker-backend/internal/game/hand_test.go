package game

import (
	"testing"
)

func mkPlayer(id string, stack int) *Player {
	return &Player{ID: id, Stack: stack}
}

// TestFoldPreflop: UTG and SB fold, BB wins the dead money.
func TestFoldPreflop(t *testing.T) {
	players := []*Player{
		mkPlayer("Alice", 1000),   // seat 0 — dealer / UTG in 3-handed
		mkPlayer("Bob", 1000),     // seat 1 — SB
		mkPlayer("Charlie", 1000), // seat 2 — BB
	}
	blinds := Blinds{Small: 10, Big: 20}
	hand, err := StartHand(players, blinds, 0)
	if err != nil {
		t.Fatal(err)
	}

	// dealer=0, SB=1 (posts 10), BB=2 (posts 20)
	// preflop action order: seat 0 (UTG), seat 1 (SB), seat 2 (BB option)
	assertActionOn(t, hand, 0)

	mustApply(t, hand, Action{PlayerIndex: 0, Type: Fold})
	mustApply(t, hand, Action{PlayerIndex: 1, Type: Fold})

	if hand.Street != Complete {
		t.Fatalf("expected Complete, got %s", hand.Street)
	}
	if hand.Winners[0] != 2 {
		t.Errorf("expected BB (index 2) to win, got %v", hand.Winners)
	}
	// BB posted 20, wins the pot of 30 → net stack = 1000-20+30 = 1010
	if players[2].Stack != 1010 {
		t.Errorf("BB stack: want 1010, got %d", players[2].Stack)
	}
	assertChipConservation(t, players, 3000)
}

// TestAllInHeadsUp: both players go all-in preflop; board runs out automatically.
func TestAllInHeadsUp(t *testing.T) {
	players := []*Player{
		mkPlayer("Alice", 500), // seat 0 — dealer / SB (heads-up)
		mkPlayer("Bob", 500),   // seat 1 — BB
	}
	blinds := Blinds{Small: 25, Big: 50}
	hand, err := StartHand(players, blinds, 0)
	if err != nil {
		t.Fatal(err)
	}
	// heads-up: dealer=SB=0, BB=1; preflop action on SB(0)
	assertActionOn(t, hand, 0)

	mustApply(t, hand, Action{PlayerIndex: 0, Type: AllIn})
	// Alice went all-in for 500 (raised); now Bob must respond
	assertActionOn(t, hand, 1)

	mustApply(t, hand, Action{PlayerIndex: 1, Type: AllIn})

	if hand.Street != Complete {
		t.Fatalf("expected Complete after both all-in, got %s", hand.Street)
	}
	if len(hand.Board) != 5 {
		t.Errorf("expected 5 board cards, got %d", len(hand.Board))
	}
	if len(hand.Winners) == 0 {
		t.Error("expected at least one winner")
	}
	assertChipConservation(t, players, 1000)
}

// TestShowdownMultiplePlayers: 3-handed check-down through all streets.
func TestShowdownMultiplePlayers(t *testing.T) {
	players := []*Player{
		mkPlayer("Alice", 1000),
		mkPlayer("Bob", 1000),
		mkPlayer("Charlie", 1000),
	}
	blinds := Blinds{Small: 10, Big: 20}
	hand, err := StartHand(players, blinds, 0)
	if err != nil {
		t.Fatal(err)
	}

	// Preflop: seat 0 (UTG) calls, seat 1 (SB) calls, seat 2 (BB) checks option
	mustApply(t, hand, Action{PlayerIndex: 0, Type: Call})
	mustApply(t, hand, Action{PlayerIndex: 1, Type: Call})
	mustApply(t, hand, Action{PlayerIndex: 2, Type: Check})

	assertStreet(t, hand, Flop)
	if len(hand.Board) != 3 {
		t.Fatalf("flop: want 3 board cards, got %d", len(hand.Board))
	}

	// Postflop order: SB(1), BB(2), dealer(0)
	checkAround(t, hand, []int{1, 2, 0})
	assertStreet(t, hand, Turn)
	if len(hand.Board) != 4 {
		t.Fatalf("turn: want 4 board cards, got %d", len(hand.Board))
	}

	checkAround(t, hand, []int{1, 2, 0})
	assertStreet(t, hand, River)
	if len(hand.Board) != 5 {
		t.Fatalf("river: want 5 board cards, got %d", len(hand.Board))
	}

	checkAround(t, hand, []int{1, 2, 0})
	assertStreet(t, hand, Complete)

	if len(hand.Winners) == 0 {
		t.Error("expected at least one winner")
	}
	assertChipConservation(t, players, 3000)
}

// TestCheckDownHeadsUp: heads-up hand reaches showdown via check-down.
func TestCheckDownHeadsUp(t *testing.T) {
	players := []*Player{
		mkPlayer("Alice", 200),
		mkPlayer("Bob", 200),
	}
	blinds := Blinds{Small: 10, Big: 20}
	hand, err := StartHand(players, blinds, 0)
	if err != nil {
		t.Fatal(err)
	}
	// heads-up: dealer/SB=0, BB=1; preflop action on SB(0)
	mustApply(t, hand, Action{PlayerIndex: 0, Type: Call})
	mustApply(t, hand, Action{PlayerIndex: 1, Type: Check})

	// Postflop: BB(1) acts first, then dealer(0)
	checkAround(t, hand, []int{1, 0}) // flop
	checkAround(t, hand, []int{1, 0}) // turn
	checkAround(t, hand, []int{1, 0}) // river

	assertStreet(t, hand, Complete)
	if len(hand.Board) != 5 {
		t.Errorf("want 5 board cards, got %d", len(hand.Board))
	}
	assertChipConservation(t, players, 400)
}

// TestRaiseAndFold: preflop raise causes others to fold.
func TestRaiseAndFold(t *testing.T) {
	players := []*Player{
		mkPlayer("Alice", 1000),
		mkPlayer("Bob", 1000),
		mkPlayer("Charlie", 1000),
	}
	blinds := Blinds{Small: 10, Big: 20}
	hand, err := StartHand(players, blinds, 0)
	if err != nil {
		t.Fatal(err)
	}
	// UTG (0) raises to 60
	mustApply(t, hand, Action{PlayerIndex: 0, Type: Raise, Amount: 60})
	// SB (1) folds
	mustApply(t, hand, Action{PlayerIndex: 1, Type: Fold})
	// BB (2) folds
	mustApply(t, hand, Action{PlayerIndex: 2, Type: Fold})

	assertStreet(t, hand, Complete)
	if hand.Winners[0] != 0 {
		t.Errorf("expected UTG (0) to win, got %v", hand.Winners)
	}
	assertChipConservation(t, players, 3000)
}

// TestBetRaiseCallShowdown: postflop bet and raise resolved to showdown.
func TestBetRaiseCallShowdown(t *testing.T) {
	players := []*Player{
		mkPlayer("Alice", 1000),
		mkPlayer("Bob", 1000),
	}
	blinds := Blinds{Small: 10, Big: 20}
	hand, err := StartHand(players, blinds, 0)
	if err != nil {
		t.Fatal(err)
	}
	// Preflop: SB(0) calls, BB(1) checks
	mustApply(t, hand, Action{PlayerIndex: 0, Type: Call})
	mustApply(t, hand, Action{PlayerIndex: 1, Type: Check})
	assertStreet(t, hand, Flop)

	// Flop: BB(1) bets 40, SB(0) raises to 100, BB calls
	mustApply(t, hand, Action{PlayerIndex: 1, Type: Bet, Amount: 40})
	mustApply(t, hand, Action{PlayerIndex: 0, Type: Raise, Amount: 100})
	mustApply(t, hand, Action{PlayerIndex: 1, Type: Call})
	assertStreet(t, hand, Turn)

	checkAround(t, hand, []int{1, 0}) // turn check-down
	checkAround(t, hand, []int{1, 0}) // river check-down

	assertStreet(t, hand, Complete)
	assertChipConservation(t, players, 2000)
}

// TestValidationErrors verifies illegal actions are rejected.
func TestValidationErrors(t *testing.T) {
	players := []*Player{
		mkPlayer("Alice", 1000),
		mkPlayer("Bob", 1000),
	}
	blinds := Blinds{Small: 10, Big: 20}
	hand, err := StartHand(players, blinds, 0)
	if err != nil {
		t.Fatal(err)
	}
	// heads-up: ActionOn=0 (SB), CurrentBet=20, P0.StreetBet=10

	// Wrong player
	if err := hand.ApplyAction(Action{PlayerIndex: 1, Type: Check}); err == nil {
		t.Error("expected error: wrong player acting out of turn")
	}
	// Check when owing money
	if err := hand.ApplyAction(Action{PlayerIndex: 0, Type: Check}); err == nil {
		t.Error("expected error: check when current bet > street bet")
	}
	// Raise below minimum (min raise to = 20+20=40)
	if err := hand.ApplyAction(Action{PlayerIndex: 0, Type: Raise, Amount: 25}); err == nil {
		t.Error("expected error: raise below minimum")
	}
	// Hand is still in Preflop — confirm it wasn't corrupted
	assertStreet(t, hand, Preflop)
}

// --- helpers ---

func mustApply(t *testing.T, h *Hand, a Action) {
	t.Helper()
	if err := h.ApplyAction(a); err != nil {
		t.Fatalf("ApplyAction(%v): %v", a, err)
	}
}

func assertActionOn(t *testing.T, h *Hand, want int) {
	t.Helper()
	if h.ActionOn != want {
		t.Fatalf("ActionOn: want %d, got %d", want, h.ActionOn)
	}
}

func assertStreet(t *testing.T, h *Hand, want Street) {
	t.Helper()
	if h.Street != want {
		t.Fatalf("Street: want %s, got %s", want, h.Street)
	}
}

func assertChipConservation(t *testing.T, players []*Player, total int) {
	t.Helper()
	got := 0
	for _, p := range players {
		got += p.Stack
	}
	if got != total {
		t.Errorf("chip conservation: want %d, got %d", total, got)
	}
}

// checkAround checks in the given player order.
func checkAround(t *testing.T, h *Hand, order []int) {
	t.Helper()
	for _, idx := range order {
		mustApply(t, h, Action{PlayerIndex: idx, Type: Check})
	}
}

// --- side-pot unit tests ---

// TestBuildSidePotsThreeLevels verifies pot structure for three all-in players
// with distinct stacks 100/200/300.
func TestBuildSidePotsThreeLevels(t *testing.T) {
	players := []*Player{
		{TotalBet: 100, Status: StatusAllIn}, // index 0
		{TotalBet: 200, Status: StatusAllIn}, // index 1
		{TotalBet: 300, Status: StatusAllIn}, // index 2
	}
	pots := buildSidePots(players)
	if len(pots) != 3 {
		t.Fatalf("want 3 pots, got %d: %+v", len(pots), pots)
	}

	// Main pot: 100 * 3 = 300, all eligible.
	assertPot(t, pots[0], 300, []int{0, 1, 2})
	// Side pot 1: 100 * 2 = 200, indices 1 and 2 eligible.
	assertPot(t, pots[1], 200, []int{1, 2})
	// Side pot 2: 100 * 1 = 100, only index 2 eligible.
	assertPot(t, pots[2], 100, []int{2})
}

// TestBuildSidePotsWithFold verifies that a folded player contributes to pot
// size but is excluded from Eligible.
func TestBuildSidePotsWithFold(t *testing.T) {
	players := []*Player{
		{TotalBet: 100, Status: StatusFolded}, // index 0 — folded
		{TotalBet: 100, Status: StatusAllIn},  // index 1
		{TotalBet: 200, Status: StatusActive}, // index 2
	}
	pots := buildSidePots(players)
	if len(pots) != 2 {
		t.Fatalf("want 2 pots, got %d: %+v", len(pots), pots)
	}
	// Main pot: 100 * 3 = 300, but index 0 folded → eligible: 1, 2.
	assertPot(t, pots[0], 300, []int{1, 2})
	// Side pot: 100 * 1 = 100, only index 2 contributed and is not folded.
	assertPot(t, pots[1], 100, []int{2})
}

// TestBuildSidePotsSingleLevel verifies no split when all bets are equal.
func TestBuildSidePotsSingleLevel(t *testing.T) {
	players := []*Player{
		{TotalBet: 100, Status: StatusActive},
		{TotalBet: 100, Status: StatusActive},
		{TotalBet: 100, Status: StatusActive},
	}
	pots := buildSidePots(players)
	if len(pots) != 1 {
		t.Fatalf("want 1 pot, got %d", len(pots))
	}
	assertPot(t, pots[0], 300, []int{0, 1, 2})
}

// --- showdown distribution tests (direct Hand construction) ---

// card is a shorthand constructor for tests.
func card(r Rank, s Suit) Card { return Card{Rank: r, Suit: s} }

// rainbowBoard returns a 5-card board with no flush or straight possibilities
// that would affect the outcome of pocket-pair comparisons.
// 2♦ 3♣ 5♠ 7♥ 9♣ — no pair, no flush, no straight.
func rainbowBoard() []Card {
	return []Card{
		card(Two, Diamonds), card(Three, Clubs),
		card(Five, Spades), card(Seven, Hearts),
		card(Nine, Clubs),
	}
}

// showdownHand constructs a Hand at the showdown stage for direct distribution
// testing, bypassing StartHand.
func showdownHand(players []*Player, board []Card) *Hand {
	total := 0
	for _, p := range players {
		total += p.TotalBet
	}
	return &Hand{
		Players: players,
		Board:   board,
		Pot:     total,
		Street:  River,
	}
}

// TestSidePotSmallestStackWins: Alice (100, AA) beats Bob (200, KK) and
// Charlie (300, QQ). Alice wins only the main pot; Bob wins side pot 1;
// Charlie auto-wins side pot 2.
func TestSidePotSmallestStackWins(t *testing.T) {
	players := []*Player{
		{ID: "Alice", Stack: 0, Status: StatusAllIn, TotalBet: 100,
			HoleCards: [2]Card{card(Ace, Spades), card(Ace, Hearts)}},
		{ID: "Bob", Stack: 0, Status: StatusAllIn, TotalBet: 200,
			HoleCards: [2]Card{card(King, Clubs), card(King, Hearts)}},
		{ID: "Charlie", Stack: 0, Status: StatusAllIn, TotalBet: 300,
			HoleCards: [2]Card{card(Queen, Clubs), card(Queen, Hearts)}},
	}
	h := showdownHand(players, rainbowBoard())
	h.resolveShowdown()

	if h.Street != Complete {
		t.Fatalf("expected Complete, got %s", h.Street)
	}
	// Main pot 300 → Alice; side pot 1 (200) → Bob; side pot 2 (100) → Charlie.
	if players[0].Stack != 300 {
		t.Errorf("Alice stack: want 300, got %d", players[0].Stack)
	}
	if players[1].Stack != 200 {
		t.Errorf("Bob stack: want 200, got %d", players[1].Stack)
	}
	if players[2].Stack != 100 {
		t.Errorf("Charlie stack: want 100, got %d", players[2].Stack)
	}
	assertChipConservation(t, players, 600)
	if len(h.SidePots) != 3 {
		t.Errorf("want 3 side pots, got %d", len(h.SidePots))
	}
}

// TestSidePotLargestStackWins: Charlie (300, AA) wins all three pots.
func TestSidePotLargestStackWins(t *testing.T) {
	players := []*Player{
		{ID: "Alice", Stack: 0, Status: StatusAllIn, TotalBet: 100,
			HoleCards: [2]Card{card(Queen, Clubs), card(Queen, Hearts)}},
		{ID: "Bob", Stack: 0, Status: StatusAllIn, TotalBet: 200,
			HoleCards: [2]Card{card(King, Clubs), card(King, Hearts)}},
		{ID: "Charlie", Stack: 0, Status: StatusAllIn, TotalBet: 300,
			HoleCards: [2]Card{card(Ace, Spades), card(Ace, Hearts)}},
	}
	h := showdownHand(players, rainbowBoard())
	h.resolveShowdown()

	// Charlie wins all 600.
	if players[2].Stack != 600 {
		t.Errorf("Charlie stack: want 600, got %d", players[2].Stack)
	}
	if players[0].Stack != 0 {
		t.Errorf("Alice stack: want 0, got %d", players[0].Stack)
	}
	if players[1].Stack != 0 {
		t.Errorf("Bob stack: want 0, got %d", players[1].Stack)
	}
	assertChipConservation(t, players, 600)
	if len(h.Winners) != 1 || h.Winners[0] != 2 {
		t.Errorf("winners: want [2], got %v", h.Winners)
	}
}

// TestSidePotSplitInSidePot: Alice (100, AA) wins main pot; Bob and Charlie
// both hold KK and split side pot 1; Charlie auto-wins side pot 2.
func TestSidePotSplitInSidePot(t *testing.T) {
	players := []*Player{
		{ID: "Alice", Stack: 0, Status: StatusAllIn, TotalBet: 100,
			HoleCards: [2]Card{card(Ace, Spades), card(Ace, Hearts)}},
		{ID: "Bob", Stack: 0, Status: StatusAllIn, TotalBet: 200,
			HoleCards: [2]Card{card(King, Clubs), card(King, Hearts)}},
		{ID: "Charlie", Stack: 0, Status: StatusAllIn, TotalBet: 300,
			HoleCards: [2]Card{card(King, Spades), card(King, Diamonds)}},
	}
	h := showdownHand(players, rainbowBoard())
	h.resolveShowdown()

	// Main pot 300 → Alice; side pot 1 (200) split → Bob 100, Charlie 100;
	// side pot 2 (100) → Charlie.
	if players[0].Stack != 300 {
		t.Errorf("Alice stack: want 300, got %d", players[0].Stack)
	}
	if players[1].Stack != 100 {
		t.Errorf("Bob stack: want 100, got %d", players[1].Stack)
	}
	if players[2].Stack != 200 {
		t.Errorf("Charlie stack: want 200, got %d", players[2].Stack)
	}
	assertChipConservation(t, players, 600)
}

// TestSidePotWithActivePlayer: Alice (100) goes all-in, Bob (200) and Charlie
// (200) match and are active. Board runs out. Main pot has all three eligible;
// side pot between Bob and Charlie.
func TestSidePotWithActivePlayer(t *testing.T) {
	// Construct directly: Alice all-in 100, Bob and Charlie active at 200 each.
	// Board: Bob has AA, Charlie has KK → Bob wins side pot, either may win main.
	players := []*Player{
		{ID: "Alice", Stack: 0, Status: StatusAllIn, TotalBet: 100,
			HoleCards: [2]Card{card(Queen, Clubs), card(Queen, Hearts)}},
		{ID: "Bob", Stack: 0, Status: StatusActive, TotalBet: 200,
			HoleCards: [2]Card{card(Ace, Spades), card(Ace, Hearts)}},
		{ID: "Charlie", Stack: 0, Status: StatusActive, TotalBet: 200,
			HoleCards: [2]Card{card(King, Clubs), card(King, Hearts)}},
	}
	h := showdownHand(players, rainbowBoard())
	h.resolveShowdown()

	// Main pot (300): QQ vs AA vs KK → Bob (AA) wins → 300.
	// Side pot (200): AA vs KK → Bob wins → 200.
	if players[1].Stack != 500 {
		t.Errorf("Bob stack: want 500, got %d", players[1].Stack)
	}
	if players[0].Stack != 0 {
		t.Errorf("Alice stack: want 0, got %d", players[0].Stack)
	}
	if players[2].Stack != 0 {
		t.Errorf("Charlie stack: want 0, got %d", players[2].Stack)
	}
	assertChipConservation(t, players, 500)
}

// TestSidePotThreeWayAllInIntegration: full hand through StartHand — three
// players go all-in with distinct stacks. Verifies pot structure and chip
// conservation end-to-end.
func TestSidePotThreeWayAllInIntegration(t *testing.T) {
	// dealer=0, SB=1 (posts 5), BB=2 (posts 10)
	// UTG(0) AllIn→100, SB(1) AllIn→200, BB(2) AllIn→300
	players := []*Player{
		mkPlayer("Alice", 100),
		mkPlayer("Bob", 200),
		mkPlayer("Charlie", 300),
	}
	hand, err := StartHand(players, Blinds{Small: 5, Big: 10}, 0)
	if err != nil {
		t.Fatal(err)
	}

	// Alice (UTG) goes all-in.
	mustApply(t, hand, Action{PlayerIndex: 0, Type: AllIn})
	// Bob (SB) goes all-in.
	mustApply(t, hand, Action{PlayerIndex: 1, Type: AllIn})
	// Charlie (BB) goes all-in.
	mustApply(t, hand, Action{PlayerIndex: 2, Type: AllIn})

	if hand.Street != Complete {
		t.Fatalf("expected Complete after three-way all-in, got %s", hand.Street)
	}
	if len(hand.Board) != 5 {
		t.Errorf("expected 5 board cards, got %d", len(hand.Board))
	}
	if len(hand.SidePots) != 3 {
		t.Errorf("want 3 side pots, got %d: %+v", len(hand.SidePots), hand.SidePots)
	}
	// Main pot: 100*3=300.
	if hand.SidePots[0].Amount != 300 {
		t.Errorf("main pot: want 300, got %d", hand.SidePots[0].Amount)
	}
	if len(hand.SidePots[0].Eligible) != 3 {
		t.Errorf("main pot eligible: want 3, got %d", len(hand.SidePots[0].Eligible))
	}
	// Side pot 1: 100*2=200.
	if hand.SidePots[1].Amount != 200 {
		t.Errorf("side pot 1: want 200, got %d", hand.SidePots[1].Amount)
	}
	if len(hand.SidePots[1].Eligible) != 2 {
		t.Errorf("side pot 1 eligible: want 2, got %d", len(hand.SidePots[1].Eligible))
	}
	// Side pot 2: 100*1=100.
	if hand.SidePots[2].Amount != 100 {
		t.Errorf("side pot 2: want 100, got %d", hand.SidePots[2].Amount)
	}
	if len(hand.SidePots[2].Eligible) != 1 {
		t.Errorf("side pot 2 eligible: want 1, got %d", len(hand.SidePots[2].Eligible))
	}
	// Short-stack can win at most 3×100 = 300.
	if players[0].Stack > 300 {
		t.Errorf("Alice (short stack) won more than main pot: stack=%d", players[0].Stack)
	}
	assertChipConservation(t, players, 600)
}

// --- side-pot helpers ---

func assertPot(t *testing.T, pot SidePot, wantAmount int, wantEligible []int) {
	t.Helper()
	if pot.Amount != wantAmount {
		t.Errorf("pot amount: want %d, got %d", wantAmount, pot.Amount)
	}
	if len(pot.Eligible) != len(wantEligible) {
		t.Errorf("pot eligible len: want %v, got %v", wantEligible, pot.Eligible)
		return
	}
	for i, idx := range wantEligible {
		if pot.Eligible[i] != idx {
			t.Errorf("pot.Eligible[%d]: want %d, got %d", i, idx, pot.Eligible[i])
		}
	}
}
