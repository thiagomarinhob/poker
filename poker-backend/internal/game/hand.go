package game

import (
	"fmt"
	"sort"
)

// Street is the phase of a Texas Hold'em hand.
type Street int

const (
	WaitingStart Street = iota
	Preflop
	Flop
	Turn
	River
	Showdown
	Complete
)

func (s Street) String() string {
	return [...]string{"WaitingStart", "Preflop", "Flop", "Turn", "River", "Showdown", "Complete"}[s]
}

// PlayerStatus tracks a seat's participation in the current hand.
type PlayerStatus int

const (
	StatusActive PlayerStatus = iota
	StatusFolded
	StatusAllIn
)

func (p PlayerStatus) String() string {
	return [...]string{"Active", "Folded", "AllIn"}[p]
}

// ActionType enumerates all legal moves a player can submit.
type ActionType int

const (
	PostBlind ActionType = iota // handled internally by StartHand
	Fold
	Check
	Call
	Bet
	Raise
	AllIn
)

func (a ActionType) String() string {
	return [...]string{"PostBlind", "Fold", "Check", "Call", "Bet", "Raise", "AllIn"}[a]
}

// Action is submitted to ApplyAction.
//
// Amount semantics:
//
//	Bet   — total street-bet amount (becomes new CurrentBet)
//	Raise — raise-to total (new CurrentBet); must be >= CurrentBet + lastRaiseSize
//	Call/AllIn/Check/Fold — ignored (computed automatically)
type Action struct {
	PlayerIndex int
	Type        ActionType
	Amount      int
}

// Player is a seat at the table during a hand.
type Player struct {
	ID        string
	Stack     int
	HoleCards [2]Card
	Status    PlayerStatus
	Position  int // seat index (0-based)

	// per-street accounting
	StreetBet int
	TotalBet  int
	HasActed  bool
}

// Blinds holds the forced bet sizes.
type Blinds struct {
	Small int
	Big   int
}

// SidePot is one pot in a multi-way all-in scenario.
type SidePot struct {
	Amount   int
	Eligible []int // player indices who can win this pot
}

// Hand is the Texas Hold'em state machine.
type Hand struct {
	Players      []*Player
	Board        []Card
	Pot          int
	CurrentBet   int // highest total street-bet so far
	ActionOn     int // index of the player whose turn it is
	DealerButton int
	Blinds       Blinds
	Street       Street

	// populated after Complete
	Winners  []int     // player indexes that split the pot
	SidePots []SidePot // main pot + side pots, populated at showdown

	deck          *Deck
	lastRaiseSize int // minimum raise increment
	sbIndex       int
	bbIndex       int
}

// MinRaiseTo is the minimum total street bet for a valid raise (raise-to).
func (h *Hand) MinRaiseTo() int { return h.CurrentBet + h.lastRaiseSize }

// SBIndex is the hand index of the small blind.
func (h *Hand) SBIndex() int { return h.sbIndex }

// BBIndex is the hand index of the big blind.
func (h *Hand) BBIndex() int { return h.bbIndex }

// StartHand initialises and deals a new hand.
// players must have len >= 2; dealerPos is the seat index of the dealer button.
func StartHand(players []*Player, blinds Blinds, dealerPos int) (*Hand, error) {
	n := len(players)
	if n < 2 {
		return nil, fmt.Errorf("StartHand: need at least 2 players, got %d", n)
	}
	if blinds.Small <= 0 || blinds.Big <= 0 || blinds.Small > blinds.Big {
		return nil, fmt.Errorf("StartHand: invalid blinds %+v", blinds)
	}
	if dealerPos < 0 || dealerPos >= n {
		return nil, fmt.Errorf("StartHand: dealerPos %d out of range", dealerPos)
	}
	for _, p := range players {
		if p.Stack <= 0 {
			return nil, fmt.Errorf("StartHand: player %s has no chips", p.ID)
		}
	}

	for i, p := range players {
		p.Position = i
		p.Status = StatusActive
		p.StreetBet = 0
		p.TotalBet = 0
		p.HasActed = false
		p.HoleCards = [2]Card{}
	}

	deck := NewDeck()
	if err := deck.Shuffle(); err != nil {
		return nil, fmt.Errorf("StartHand: shuffle: %w", err)
	}
	for _, p := range players {
		cards, err := deck.Draw(2)
		if err != nil {
			return nil, fmt.Errorf("StartHand: deal: %w", err)
		}
		p.HoleCards = [2]Card{cards[0], cards[1]}
	}

	// Heads-up: dealer == SB and acts first preflop.
	sbIndex := (dealerPos + 1) % n
	bbIndex := (dealerPos + 2) % n
	if n == 2 {
		sbIndex = dealerPos
		bbIndex = (dealerPos + 1) % n
	}

	h := &Hand{
		Players:       players,
		Board:         make([]Card, 0, 5),
		DealerButton:  dealerPos,
		Blinds:        blinds,
		Street:        Preflop,
		deck:          deck,
		lastRaiseSize: blinds.Big,
		sbIndex:       sbIndex,
		bbIndex:       bbIndex,
	}

	h.postBlind(sbIndex, blinds.Small)
	h.postBlind(bbIndex, blinds.Big)
	h.CurrentBet = blinds.Big

	// Preflop: first to act is left of BB (UTG), or dealer in heads-up.
	h.ActionOn = h.firstActiveFrom(bbIndex)
	return h, nil
}

// postBlind deducts the blind from a player's stack, handling short stacks.
func (h *Hand) postBlind(idx, amount int) {
	p := h.Players[idx]
	chips := amount
	if p.Stack < chips {
		chips = p.Stack
	}
	p.Stack -= chips
	p.StreetBet += chips
	p.TotalBet += chips
	h.Pot += chips
	if p.Stack == 0 {
		p.Status = StatusAllIn
	}
}

// ApplyAction validates and applies one player action, advancing the state machine.
func (h *Hand) ApplyAction(a Action) error {
	if h.Street == WaitingStart || h.Street == Complete || h.Street == Showdown {
		return fmt.Errorf("ApplyAction: no action allowed in street %s", h.Street)
	}
	if a.PlayerIndex < 0 || a.PlayerIndex >= len(h.Players) {
		return fmt.Errorf("ApplyAction: player index %d out of range", a.PlayerIndex)
	}
	if a.PlayerIndex != h.ActionOn {
		return fmt.Errorf("ApplyAction: not player %d's turn (action on %d)", a.PlayerIndex, h.ActionOn)
	}

	p := h.Players[a.PlayerIndex]
	if p.Status != StatusActive {
		return fmt.Errorf("ApplyAction: player %d is not active (status %d)", a.PlayerIndex, p.Status)
	}

	switch a.Type {
	case Fold:
		p.Status = StatusFolded

	case Check:
		if p.StreetBet != h.CurrentBet {
			return fmt.Errorf("ApplyAction: cannot check — current bet %d, player has %d in", h.CurrentBet, p.StreetBet)
		}
		p.HasActed = true

	case Call:
		toCall := h.CurrentBet - p.StreetBet
		if toCall <= 0 {
			return fmt.Errorf("ApplyAction: nothing to call")
		}
		chips := toCall
		if p.Stack < chips {
			chips = p.Stack // call all-in (simplified, no side-pot)
		}
		p.Stack -= chips
		p.StreetBet += chips
		p.TotalBet += chips
		h.Pot += chips
		if p.Stack == 0 {
			p.Status = StatusAllIn
		}
		p.HasActed = true

	case Bet:
		if h.CurrentBet != 0 {
			return fmt.Errorf("ApplyAction: cannot bet — a bet of %d already exists (use Raise)", h.CurrentBet)
		}
		if a.Amount < h.Blinds.Big {
			return fmt.Errorf("ApplyAction: bet %d below minimum %d", a.Amount, h.Blinds.Big)
		}
		if a.Amount > p.Stack+p.StreetBet {
			return fmt.Errorf("ApplyAction: bet %d exceeds available %d", a.Amount, p.Stack+p.StreetBet)
		}
		chips := a.Amount - p.StreetBet
		p.Stack -= chips
		p.StreetBet = a.Amount
		p.TotalBet += chips
		h.Pot += chips
		h.lastRaiseSize = a.Amount
		h.CurrentBet = a.Amount
		if p.Stack == 0 {
			p.Status = StatusAllIn
		}
		h.resetHasActed(a.PlayerIndex)
		p.HasActed = true

	case Raise:
		minTo := h.CurrentBet + h.lastRaiseSize
		if a.Amount < minTo {
			return fmt.Errorf("ApplyAction: raise to %d below minimum %d", a.Amount, minTo)
		}
		if a.Amount > p.Stack+p.StreetBet {
			return fmt.Errorf("ApplyAction: raise to %d exceeds available %d", a.Amount, p.Stack+p.StreetBet)
		}
		chips := a.Amount - p.StreetBet
		h.lastRaiseSize = a.Amount - h.CurrentBet
		h.CurrentBet = a.Amount
		p.Stack -= chips
		p.StreetBet = a.Amount
		p.TotalBet += chips
		h.Pot += chips
		if p.Stack == 0 {
			p.Status = StatusAllIn
		}
		h.resetHasActed(a.PlayerIndex)
		p.HasActed = true

	case AllIn:
		chips := p.Stack
		if chips == 0 {
			return fmt.Errorf("ApplyAction: player %d already all-in", a.PlayerIndex)
		}
		newTotal := p.StreetBet + chips
		p.Stack = 0
		p.StreetBet = newTotal
		p.TotalBet += chips
		h.Pot += chips
		// Only counts as a raise if it exceeds the current bet.
		if newTotal > h.CurrentBet {
			h.lastRaiseSize = newTotal - h.CurrentBet
			h.CurrentBet = newTotal
			h.resetHasActed(a.PlayerIndex)
		}
		p.Status = StatusAllIn
		p.HasActed = true

	default:
		return fmt.Errorf("ApplyAction: unknown action type %d", a.Type)
	}

	// If only one player hasn't folded, award pot immediately.
	if h.countNotFolded() == 1 {
		h.awardPotToLastStanding()
		h.Street = Complete
		return nil
	}

	next := h.nextActingPlayer(h.ActionOn)
	if next == -1 {
		return h.advanceStreet()
	}
	h.ActionOn = next
	return nil
}

// resetHasActed clears HasActed for every active player except the aggressor,
// forcing them to respond to the new bet/raise.
func (h *Hand) resetHasActed(aggressorIdx int) {
	for i, p := range h.Players {
		if i != aggressorIdx && p.Status == StatusActive {
			p.HasActed = false
		}
	}
}

// nextActingPlayer returns the index of the next StatusActive player clockwise
// from `from` who still needs to act, or -1 if the betting round is over.
func (h *Hand) nextActingPlayer(from int) int {
	n := len(h.Players)
	for i := 1; i <= n; i++ {
		idx := (from + i) % n
		p := h.Players[idx]
		if p.Status == StatusActive && (!p.HasActed || p.StreetBet < h.CurrentBet) {
			return idx
		}
	}
	return -1
}

// firstActiveFrom returns the first StatusActive player clockwise after `from`.
func (h *Hand) firstActiveFrom(from int) int {
	n := len(h.Players)
	for i := 1; i <= n; i++ {
		idx := (from + i) % n
		if h.Players[idx].Status == StatusActive {
			return idx
		}
	}
	return -1
}

// firstActingPlayerPostflop returns the first player who can act postflop
// (StatusActive, not all-in), searching clockwise from the dealer button.
// Returns -1 if all remaining players are all-in.
func (h *Hand) firstActingPlayerPostflop() int {
	n := len(h.Players)
	for i := 1; i <= n; i++ {
		idx := (h.DealerButton + i) % n
		if h.Players[idx].Status == StatusActive {
			return idx
		}
	}
	return -1
}

func (h *Hand) countNotFolded() int {
	count := 0
	for _, p := range h.Players {
		if p.Status != StatusFolded {
			count++
		}
	}
	return count
}

func (h *Hand) awardPotToLastStanding() {
	for i, p := range h.Players {
		if p.Status != StatusFolded {
			p.Stack += h.Pot
			h.Pot = 0
			h.Winners = []int{i}
			return
		}
	}
}

// advanceStreet closes the current betting round and moves to the next street,
// dealing community cards as appropriate. If all remaining players are all-in,
// it runs out the board automatically until showdown.
func (h *Hand) advanceStreet() error {
	// Reset per-street tracking for the new betting round.
	for _, p := range h.Players {
		p.StreetBet = 0
		p.HasActed = false
	}
	h.CurrentBet = 0
	h.lastRaiseSize = h.Blinds.Big

	switch h.Street {
	case Preflop:
		cards, err := h.deck.Draw(3)
		if err != nil {
			return fmt.Errorf("advanceStreet flop: %w", err)
		}
		h.Board = append(h.Board, cards...)
		h.Street = Flop

	case Flop:
		cards, err := h.deck.Draw(1)
		if err != nil {
			return fmt.Errorf("advanceStreet turn: %w", err)
		}
		h.Board = append(h.Board, cards...)
		h.Street = Turn

	case Turn:
		cards, err := h.deck.Draw(1)
		if err != nil {
			return fmt.Errorf("advanceStreet river: %w", err)
		}
		h.Board = append(h.Board, cards...)
		h.Street = River

	case River:
		h.resolveShowdown()
		return nil

	default:
		return fmt.Errorf("advanceStreet: unexpected street %s", h.Street)
	}

	// If no active (non-all-in) player can bet, run the board out automatically.
	next := h.firstActingPlayerPostflop()
	if next == -1 {
		return h.advanceStreet()
	}
	h.ActionOn = next
	return nil
}

// buildSidePots splits the wagered chips into a main pot and zero or more side
// pots based on each player's total contribution (TotalBet). Folded players
// contribute chips but are excluded from Eligible.
func buildSidePots(players []*Player) []SidePot {
	levelSet := make(map[int]struct{})
	for _, p := range players {
		if p.TotalBet > 0 {
			levelSet[p.TotalBet] = struct{}{}
		}
	}
	levels := make([]int, 0, len(levelSet))
	for l := range levelSet {
		levels = append(levels, l)
	}
	sort.Ints(levels)

	var pots []SidePot
	prev := 0
	for _, level := range levels {
		perPlayer := level - prev

		// Count all contributors (including folded) at this level.
		count := 0
		for _, p := range players {
			if p.TotalBet >= level {
				count++
			}
		}

		// Only non-folded contributors may win the pot.
		var eligible []int
		for i, p := range players {
			if p.Status != StatusFolded && p.TotalBet >= level {
				eligible = append(eligible, i)
			}
		}

		if count > 0 {
			pots = append(pots, SidePot{
				Amount:   perPlayer * count,
				Eligible: eligible,
			})
		}
		prev = level
	}
	return pots
}

// resolveShowdown evaluates hands, distributes each pot to its winner(s),
// and sets Street = Complete.
func (h *Hand) resolveShowdown() {
	pots := buildSidePots(h.Players)
	h.SidePots = pots

	winnerSet := make(map[int]struct{})
	for _, pot := range pots {
		winners := h.bestHandAmong(pot.Eligible)
		share := pot.Amount / len(winners)
		remainder := pot.Amount - share*len(winners)
		for _, idx := range winners {
			h.Players[idx].Stack += share
			winnerSet[idx] = struct{}{}
		}
		// Odd chip to the first (lowest-index) winner.
		if remainder > 0 {
			h.Players[winners[0]].Stack += remainder
		}
	}

	h.Winners = make([]int, 0, len(winnerSet))
	for idx := range winnerSet {
		h.Winners = append(h.Winners, idx)
	}
	sort.Ints(h.Winners)
	h.Pot = 0
	h.Street = Complete
}

// bestHandAmong returns the player indices (from eligible) that share the
// best hand at showdown.
func (h *Hand) bestHandAmong(eligible []int) []int {
	type entry struct {
		idx  int
		hand HandResult
	}
	results := make([]entry, 0, len(eligible))
	for _, idx := range eligible {
		p := h.Players[idx]
		all := make([]Card, 0, 7)
		all = append(all, p.HoleCards[:]...)
		all = append(all, h.Board...)
		results = append(results, entry{idx, EvaluateHand(all)})
	}

	best := results[0].hand
	for _, r := range results[1:] {
		if CompareHands(r.hand, best) > 0 {
			best = r.hand
		}
	}

	var winners []int
	for _, r := range results {
		if CompareHands(r.hand, best) == 0 {
			winners = append(winners, r.idx)
		}
	}
	return winners
}
