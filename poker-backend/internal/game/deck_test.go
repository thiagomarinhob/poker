package game

import (
	"testing"
)

func TestNewDeck_52Cards(t *testing.T) {
	d := NewDeck()
	if d.Remaining() != 52 {
		t.Fatalf("expected 52 cards, got %d", d.Remaining())
	}
}

func TestNewDeck_UniqueCards(t *testing.T) {
	d := NewDeck()
	cards, err := d.Draw(52)
	if err != nil {
		t.Fatal(err)
	}
	seen := make(map[Card]bool)
	for _, c := range cards {
		if seen[c] {
			t.Fatalf("duplicate card: %v", c)
		}
		seen[c] = true
	}
}

func TestShuffle_NoRepeats(t *testing.T) {
	d := NewDeck()
	if err := d.Shuffle(); err != nil {
		t.Fatal(err)
	}
	if d.Remaining() != 52 {
		t.Fatalf("shuffle changed card count: %d", d.Remaining())
	}
	cards, err := d.Draw(52)
	if err != nil {
		t.Fatal(err)
	}
	seen := make(map[Card]bool)
	for _, c := range cards {
		if seen[c] {
			t.Fatalf("duplicate card after shuffle: %v", c)
		}
		seen[c] = true
	}
}

func TestShuffle_IsRandom(t *testing.T) {
	// Two shuffles of two decks should almost never produce identical order.
	d1 := NewDeck()
	d2 := NewDeck()
	if err := d1.Shuffle(); err != nil {
		t.Fatal(err)
	}
	if err := d2.Shuffle(); err != nil {
		t.Fatal(err)
	}
	c1, _ := d1.Draw(52)
	c2, _ := d2.Draw(52)
	same := 0
	for i := range c1 {
		if c1[i] == c2[i] {
			same++
		}
	}
	// Probability of all 52 matching is astronomically small.
	if same == 52 {
		t.Fatal("two shuffles produced identical order — shuffle is not random")
	}
}

func TestDraw_ReducesRemaining(t *testing.T) {
	d := NewDeck()
	if _, err := d.Draw(5); err != nil {
		t.Fatal(err)
	}
	if d.Remaining() != 47 {
		t.Fatalf("expected 47, got %d", d.Remaining())
	}
}

func TestDraw_TooMany(t *testing.T) {
	d := NewDeck()
	if _, err := d.Draw(53); err == nil {
		t.Fatal("expected error drawing more cards than available")
	}
}
