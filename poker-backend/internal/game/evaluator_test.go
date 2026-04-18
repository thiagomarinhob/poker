package game

import (
	"testing"
)

// helpers

func c(rank Rank, suit Suit) Card { return Card{Rank: rank, Suit: suit} }

func assertRank(t *testing.T, name string, cards []Card, want HandRank) {
	t.Helper()
	got := EvaluateHand(cards)
	if got.Rank != want {
		t.Errorf("%s: want %v, got %v", name, want, got.Rank)
	}
}

// --- 5-card hands ---

func TestHighCard(t *testing.T) {
	assertRank(t, "high card", []Card{
		c(Ace, Spades), c(King, Hearts), c(Jack, Diamonds), c(Nine, Clubs), c(Seven, Hearts),
	}, HighCard)
}

func TestOnePair(t *testing.T) {
	assertRank(t, "one pair", []Card{
		c(Ace, Spades), c(Ace, Hearts), c(King, Diamonds), c(Queen, Clubs), c(Jack, Hearts),
	}, OnePair)
}

func TestTwoPair(t *testing.T) {
	assertRank(t, "two pair", []Card{
		c(Ace, Spades), c(Ace, Hearts), c(King, Diamonds), c(King, Clubs), c(Jack, Hearts),
	}, TwoPair)
}

func TestThreeOfAKind(t *testing.T) {
	assertRank(t, "three of a kind", []Card{
		c(Ace, Spades), c(Ace, Hearts), c(Ace, Diamonds), c(King, Clubs), c(Jack, Hearts),
	}, ThreeOfAKind)
}

func TestStraight(t *testing.T) {
	assertRank(t, "straight 9-high", []Card{
		c(Nine, Spades), c(Eight, Hearts), c(Seven, Diamonds), c(Six, Clubs), c(Five, Hearts),
	}, Straight)
}

func TestStraightWheel(t *testing.T) {
	assertRank(t, "wheel A-2-3-4-5", []Card{
		c(Ace, Spades), c(Two, Hearts), c(Three, Diamonds), c(Four, Clubs), c(Five, Hearts),
	}, Straight)
}

func TestFlush(t *testing.T) {
	assertRank(t, "flush", []Card{
		c(Ace, Hearts), c(Ten, Hearts), c(Eight, Hearts), c(Five, Hearts), c(Two, Hearts),
	}, Flush)
}

func TestFullHouse(t *testing.T) {
	assertRank(t, "full house", []Card{
		c(Ace, Spades), c(Ace, Hearts), c(Ace, Diamonds), c(King, Clubs), c(King, Hearts),
	}, FullHouse)
}

func TestFourOfAKind(t *testing.T) {
	assertRank(t, "four of a kind", []Card{
		c(Ace, Spades), c(Ace, Hearts), c(Ace, Diamonds), c(Ace, Clubs), c(King, Hearts),
	}, FourOfAKind)
}

func TestStraightFlush(t *testing.T) {
	assertRank(t, "straight flush", []Card{
		c(Nine, Hearts), c(Eight, Hearts), c(Seven, Hearts), c(Six, Hearts), c(Five, Hearts),
	}, StraightFlush)
}

func TestRoyalFlush(t *testing.T) {
	assertRank(t, "royal flush", []Card{
		c(Ace, Spades), c(King, Spades), c(Queen, Spades), c(Jack, Spades), c(Ten, Spades),
	}, RoyalFlush)
}

// --- Straight edge cases ---

func TestStraightAceHigh(t *testing.T) {
	assertRank(t, "broadway A-K-Q-J-T", []Card{
		c(Ace, Spades), c(King, Hearts), c(Queen, Diamonds), c(Jack, Clubs), c(Ten, Hearts),
	}, Straight)
}

func TestWheelHighCardIsFive(t *testing.T) {
	res := EvaluateHand([]Card{
		c(Ace, Spades), c(Two, Hearts), c(Three, Diamonds), c(Four, Clubs), c(Five, Hearts),
	})
	if res.Rank != Straight {
		t.Fatalf("expected Straight, got %v", res.Rank)
	}
	// After reorder, first card should be Five
	if res.Cards[0].Rank != Five {
		t.Errorf("wheel high card should be Five, got %v", res.Cards[0].Rank)
	}
	// Ace should be last (acting as low)
	if res.Cards[4].Rank != Ace {
		t.Errorf("wheel Ace should be last, got %v", res.Cards[4].Rank)
	}
}

// --- Flush vs Straight ---

func TestFlushBeatsStrength(t *testing.T) {
	flush := HandResult{Rank: Flush}
	straight := HandResult{Rank: Straight}
	if CompareHands(flush, straight) <= 0 {
		t.Error("flush should beat straight")
	}
}

// --- Kicker comparison ---

func TestHighCardKicker(t *testing.T) {
	// A-K-J-9-7 vs A-K-J-9-6: first wins on last kicker
	a := EvaluateHand([]Card{
		c(Ace, Spades), c(King, Hearts), c(Jack, Diamonds), c(Nine, Clubs), c(Seven, Hearts),
	})
	b := EvaluateHand([]Card{
		c(Ace, Hearts), c(King, Diamonds), c(Jack, Clubs), c(Nine, Spades), c(Six, Hearts),
	})
	if CompareHands(a, b) <= 0 {
		t.Error("A-K-J-9-7 should beat A-K-J-9-6")
	}
}

func TestPairKicker(t *testing.T) {
	// AA-K-Q-J vs AA-K-Q-T
	a := EvaluateHand([]Card{
		c(Ace, Spades), c(Ace, Hearts), c(King, Diamonds), c(Queen, Clubs), c(Jack, Hearts),
	})
	b := EvaluateHand([]Card{
		c(Ace, Clubs), c(Ace, Diamonds), c(King, Spades), c(Queen, Hearts), c(Ten, Spades),
	})
	if CompareHands(a, b) <= 0 {
		t.Error("AA-K-Q-J should beat AA-K-Q-T on kicker")
	}
}

func TestTwoPairKicker(t *testing.T) {
	// AA-KK-Q vs AA-KK-J
	a := EvaluateHand([]Card{
		c(Ace, Spades), c(Ace, Hearts), c(King, Diamonds), c(King, Clubs), c(Queen, Hearts),
	})
	b := EvaluateHand([]Card{
		c(Ace, Clubs), c(Ace, Diamonds), c(King, Spades), c(King, Hearts), c(Jack, Spades),
	})
	if CompareHands(a, b) <= 0 {
		t.Error("AA-KK-Q should beat AA-KK-J on kicker")
	}
}

func TestTie(t *testing.T) {
	a := EvaluateHand([]Card{
		c(Ace, Spades), c(King, Hearts), c(Jack, Diamonds), c(Nine, Clubs), c(Seven, Hearts),
	})
	b := EvaluateHand([]Card{
		c(Ace, Hearts), c(King, Diamonds), c(Jack, Clubs), c(Nine, Spades), c(Seven, Clubs),
	})
	if CompareHands(a, b) != 0 {
		t.Error("identical ranks should tie")
	}
}

// --- Two pair with paired board (7 cards) ---

func TestTwoPairPairedBoard7Cards(t *testing.T) {
	// Board: K K Q Q J  — hole cards: A 2
	// Best hand should be K-K-Q-Q-A (two pair, Ace kicker)
	res := EvaluateHand([]Card{
		c(King, Spades), c(King, Hearts), c(Queen, Diamonds), c(Queen, Clubs),
		c(Jack, Hearts), c(Ace, Spades), c(Two, Diamonds),
	})
	if res.Rank != TwoPair {
		t.Fatalf("expected TwoPair, got %v", res.Rank)
	}
	// The two pairs should be Kings and Queens
	if res.Cards[0].Rank != King {
		t.Errorf("first group should be Kings, got %v", res.Cards[0].Rank)
	}
}

// --- 7-card best hand selection ---

func TestBestHandFrom7(t *testing.T) {
	// 7 cards that contain a flush among them
	cards := []Card{
		c(Ace, Hearts), c(King, Hearts), c(Queen, Hearts), c(Jack, Hearts), c(Ten, Hearts),
		c(Two, Spades), c(Three, Clubs),
	}
	res := EvaluateHand(cards)
	if res.Rank != RoyalFlush {
		t.Errorf("expected RoyalFlush from 7-card hand, got %v", res.Rank)
	}
}

func TestStraightFlushVsFlush7Cards(t *testing.T) {
	// Contains both a plain flush and a straight flush — should pick straight flush
	cards := []Card{
		c(Nine, Hearts), c(Eight, Hearts), c(Seven, Hearts), c(Six, Hearts), c(Five, Hearts),
		c(Ace, Hearts), c(Two, Spades),
	}
	res := EvaluateHand(cards)
	if res.Rank != StraightFlush {
		t.Errorf("expected StraightFlush, got %v", res.Rank)
	}
}
