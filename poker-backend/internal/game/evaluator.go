package game

import "sort"

type HandRank int

const (
	HighCard HandRank = iota
	OnePair
	TwoPair
	ThreeOfAKind
	Straight
	Flush
	FullHouse
	FourOfAKind
	StraightFlush
	RoyalFlush
)

// HandResult holds the rank and the 5 cards that form it (highest first),
// enabling kicker comparison via CompareHands.
type HandResult struct {
	Rank  HandRank
	Cards [5]Card // best 5-card combo, ordered for tiebreak
}

// EvaluateHand returns the best 5-card HandResult from 5–7 cards.
func EvaluateHand(cards []Card) HandResult {
	if len(cards) == 5 {
		return evaluate5(cards)
	}

	best := HandResult{Rank: -1}
	for _, combo := range combinations5(cards) {
		r := evaluate5(combo[:])
		if compareResult(r, best) > 0 {
			best = r
		}
	}
	return best
}

// CompareHands returns positive if a > b, negative if a < b, 0 if tie.
func CompareHands(a, b HandResult) int {
	return compareResult(a, b)
}

// --- internal ---

func evaluate5(cards []Card) HandResult {
	sorted := make([]Card, 5)
	copy(sorted, cards)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Rank > sorted[j].Rank
	})

	isFlush := checkFlush(sorted)
	straightHigh, isStraight := checkStraight(sorted)

	switch {
	case isFlush && isStraight && straightHigh == Ace:
		return HandResult{Rank: RoyalFlush, Cards: arr5(sorted)}

	case isFlush && isStraight:
		ordered := straightOrder(sorted, straightHigh)
		return HandResult{Rank: StraightFlush, Cards: arr5(ordered)}

	case isFlush:
		return HandResult{Rank: Flush, Cards: arr5(sorted)}

	case isStraight:
		ordered := straightOrder(sorted, straightHigh)
		return HandResult{Rank: Straight, Cards: arr5(ordered)}
	}

	groups := groupByRank(sorted)

	switch {
	case groups[0].count == 4:
		return HandResult{Rank: FourOfAKind, Cards: arr5(groups.toCards())}
	case groups[0].count == 3 && groups[1].count == 2:
		return HandResult{Rank: FullHouse, Cards: arr5(groups.toCards())}
	case groups[0].count == 3:
		return HandResult{Rank: ThreeOfAKind, Cards: arr5(groups.toCards())}
	case groups[0].count == 2 && groups[1].count == 2:
		return HandResult{Rank: TwoPair, Cards: arr5(groups.toCards())}
	case groups[0].count == 2:
		return HandResult{Rank: OnePair, Cards: arr5(groups.toCards())}
	default:
		return HandResult{Rank: HighCard, Cards: arr5(groups.toCards())}
	}
}

func checkFlush(sorted []Card) bool {
	s := sorted[0].Suit
	for _, c := range sorted[1:] {
		if c.Suit != s {
			return false
		}
	}
	return true
}

// checkStraight returns the high card rank and whether it is a straight.
// Handles the wheel (A-2-3-4-5) where Ace acts as low.
func checkStraight(sorted []Card) (Rank, bool) {
	ranks := [5]Rank{sorted[0].Rank, sorted[1].Rank, sorted[2].Rank, sorted[3].Rank, sorted[4].Rank}

	// Normal straight
	if ranks[0]-ranks[4] == 4 && uniqueRanks(ranks) {
		return ranks[0], true
	}

	// Wheel: A-2-3-4-5 (sorted as A,5,4,3,2)
	if ranks[0] == Ace && ranks[1] == Five && ranks[2] == Four &&
		ranks[3] == Three && ranks[4] == Two {
		return Five, true
	}

	return 0, false
}

func uniqueRanks(ranks [5]Rank) bool {
	for i := 1; i < 5; i++ {
		if ranks[i] == ranks[i-1] {
			return false
		}
	}
	return true
}

// straightOrder reorders the 5 cards so that card[0] is the high card.
// For the wheel the Ace moves to position 4.
func straightOrder(sorted []Card, high Rank) []Card {
	if high == Five {
		// move Ace to end: sorted[0] is Ace, [1..4] are 5,4,3,2
		return []Card{sorted[1], sorted[2], sorted[3], sorted[4], sorted[0]}
	}
	return sorted
}

// rankGroup is used to sort cards by group count then by rank.
type rankGroup struct {
	rank  Rank
	count int
	cards []Card
}

type rankGroups []rankGroup

func groupByRank(sorted []Card) rankGroups {
	m := make(map[Rank][]Card)
	order := []Rank{}
	for _, c := range sorted {
		if _, ok := m[c.Rank]; !ok {
			order = append(order, c.Rank)
		}
		m[c.Rank] = append(m[c.Rank], c)
	}

	groups := make(rankGroups, 0, len(order))
	for _, r := range order {
		groups = append(groups, rankGroup{rank: r, count: len(m[r]), cards: m[r]})
	}

	// Sort: higher count first, then higher rank for ties
	sort.Slice(groups, func(i, j int) bool {
		if groups[i].count != groups[j].count {
			return groups[i].count > groups[j].count
		}
		return groups[i].rank > groups[j].rank
	})
	return groups
}

// toCards returns all cards flattened in group order (for tiebreak).
func (gs rankGroups) toCards() []Card {
	out := make([]Card, 0, 5)
	for _, g := range gs {
		out = append(out, g.cards...)
	}
	return out
}

func arr5(cards []Card) [5]Card {
	var a [5]Card
	copy(a[:], cards)
	return a
}

func combinations5(cards []Card) [][5]Card {
	n := len(cards)
	var out [][5]Card
	for i := 0; i < n-4; i++ {
		for j := i + 1; j < n-3; j++ {
			for k := j + 1; k < n-2; k++ {
				for l := k + 1; l < n-1; l++ {
					for m := l + 1; m < n; m++ {
						out = append(out, [5]Card{cards[i], cards[j], cards[k], cards[l], cards[m]})
					}
				}
			}
		}
	}
	return out
}

func compareResult(a, b HandResult) int {
	if a.Rank != b.Rank {
		if a.Rank > b.Rank {
			return 1
		}
		return -1
	}
	for i := 0; i < 5; i++ {
		if a.Cards[i].Rank != b.Cards[i].Rank {
			if a.Cards[i].Rank > b.Cards[i].Rank {
				return 1
			}
			return -1
		}
	}
	return 0
}
