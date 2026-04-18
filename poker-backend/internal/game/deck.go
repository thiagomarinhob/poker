package game

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
)

type Suit int8

const (
	Clubs    Suit = iota // 0
	Diamonds             // 1
	Hearts               // 2
	Spades               // 3
)

func (s Suit) String() string {
	return [...]string{"c", "d", "h", "s"}[s]
}

type Rank int8

const (
	Two   Rank = 2
	Three Rank = 3
	Four  Rank = 4
	Five  Rank = 5
	Six   Rank = 6
	Seven Rank = 7
	Eight Rank = 8
	Nine  Rank = 9
	Ten   Rank = 10
	Jack  Rank = 11
	Queen Rank = 12
	King  Rank = 13
	Ace   Rank = 14
)

func (r Rank) String() string {
	switch r {
	case Ten:
		return "T"
	case Jack:
		return "J"
	case Queen:
		return "Q"
	case King:
		return "K"
	case Ace:
		return "A"
	default:
		return fmt.Sprintf("%d", int(r))
	}
}

type Card struct {
	Rank Rank
	Suit Suit
}

func (c Card) String() string {
	return c.Rank.String() + c.Suit.String()
}

type Deck struct {
	cards []Card
}

func NewDeck() *Deck {
	cards := make([]Card, 0, 52)
	for _, s := range []Suit{Clubs, Diamonds, Hearts, Spades} {
		for r := Two; r <= Ace; r++ {
			cards = append(cards, Card{Rank: r, Suit: s})
		}
	}
	return &Deck{cards: cards}
}

// Shuffle uses Fisher-Yates with crypto/rand for true randomness.
func (d *Deck) Shuffle() error {
	n := len(d.cards)
	for i := n - 1; i > 0; i-- {
		j, err := cryptoRandInt(i + 1)
		if err != nil {
			return err
		}
		d.cards[i], d.cards[j] = d.cards[j], d.cards[i]
	}
	return nil
}

func (d *Deck) Draw(n int) ([]Card, error) {
	if n > len(d.cards) {
		return nil, fmt.Errorf("draw %d: only %d cards remaining", n, len(d.cards))
	}
	drawn := make([]Card, n)
	copy(drawn, d.cards[:n])
	d.cards = d.cards[n:]
	return drawn, nil
}

func (d *Deck) Remaining() int { return len(d.cards) }

// cryptoRandInt returns a random int in [0, max) using crypto/rand.
func cryptoRandInt(max int) (int, error) {
	if max <= 0 {
		return 0, nil
	}
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return 0, err
	}
	return int(binary.BigEndian.Uint64(b[:]) % uint64(max)), nil
}
