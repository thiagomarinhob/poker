// Command simulate runs random-bot Texas Hold'em for stress testing and
// invariant checks: chip conservation, non-negative stacks, pot accounting,
// and showdown strength vs side-pot winners.
package main

import (
	"flag"
	"fmt"
	"math/rand/v2"
	"os"
	"slices"
	"time"

	"github.com/thiagomarinho/poker-backend/internal/game"
)

const (
	numSeats         = 6
	handsIter        = 100
	startStack       = 100_000
	minBetMultiplier = 1000
)

var (
	iters   = flag.Int("iters", 1, "número de iterações (cada uma: handsIter mãos)")
	verbose = flag.Bool("v", false, "imprimir cada ação (ligado de graça com -iters=1; fora disso, use -v para stress)")
	quiet   = flag.Bool("q", false, "não imprime ações/cabeçalhos; só a linha final de OK (ou mudo total com -q)")
)

func main() {
	flag.Parse()
	seed1 := uint64(time.Now().UnixNano())
	seed2 := seed1 ^ 0xA5A5_5A5A_5A5A_5A5A
	rng := rand.New(rand.NewPCG(seed1, seed2))

	verboseOn := false
	if *quiet {
		verboseOn = false
	} else if *iters == 1 {
		verboseOn = true
	} else {
		verboseOn = *verbose
	}

	for iter := 0; iter < *iters; iter++ {
		if !*quiet && *iters > 1 {
			fmt.Printf("iteração %d / %d\n", iter+1, *iters)
		}
		dealer := 0
		tableTotal := numSeats * startStack * minBetMultiplier

		for hi := 0; hi < handsIter; hi++ {
			players, blinds := newTable(tableTotal)
			h, err := game.StartHand(players, blinds, dealer)
			if err != nil {
				panic(err)
			}
			initial, err := tableChips(h)
			if err != nil {
				panic(err)
			}
			if verboseOn {
				fmt.Printf("\n--- mão %d (iter %d) dealer %d, blinds %d/%d ---\n",
					hi+1, iter+1, dealer, blinds.Small, blinds.Big)
			}

			for h.Street != game.Complete {
				if err := checkMidHandInvariants(h, initial); err != nil {
					panic(err)
				}
				act, err := randomLegalAction(rng, h)
				if err != nil {
					panic(err)
				}
				if verboseOn {
					p := h.Players[act.PlayerIndex]
					extra := ""
					if act.Type == game.Bet || act.Type == game.Raise {
						extra = fmt.Sprintf(" amount=%d", act.Amount)
					}
					fmt.Printf("  [%s] %s%s  (rua: %s, pote: %d)\n", p.ID, act.Type, extra, h.Street, h.Pot)
				}
				if err := h.ApplyAction(act); err != nil {
					panic(err)
				}
			}

			if err := checkShowdownWinners(h); err != nil {
				panic(err)
			}
			final, err := tableChips(h)
			if err != nil {
				panic(err)
			}
			if final != initial {
				panic(fmt.Sprintf("conservação: fim mão %d: total %d != início %d", hi+1, final, initial))
			}
			for _, p := range players {
				if p.Stack < 0 {
					panic(fmt.Sprintf("stack negativo: %s = %d", p.ID, p.Stack))
				}
			}
			dealer = (dealer + 1) % numSeats
		}
	}
	if *quiet {
		fmt.Println("OK")
	} else {
		fmt.Printf("OK: %d iteração(ões) × %d mãos; motor consistente com os invariantes.\n", *iters, handsIter)
	}
	os.Exit(0)
}

func newTable(totalChips int) ([]*game.Player, game.Blinds) {
	per := totalChips / numSeats
	players := make([]*game.Player, numSeats)
	for i := 0; i < numSeats; i++ {
		players[i] = &game.Player{ID: fmt.Sprintf("B%d", i+1), Stack: per}
	}
	bb := per / 200
	if bb < 2 {
		bb = 2
	}
	small := bb / 2
	if small < 1 {
		small = 1
	}
	if small > bb {
		small = bb
	}
	return players, game.Blinds{Small: small, Big: bb}
}

func tableChips(h *game.Hand) (int, error) {
	sum := 0
	for _, p := range h.Players {
		if p.Stack < 0 {
			return 0, fmt.Errorf("stack negativo: %s", p.ID)
		}
		sum += p.Stack
	}
	return sum + h.Pot, nil
}

// Enquanto a mão não acabou: soma(stacks)+pote é constante; pote = soma(TotalBet).
func checkMidHandInvariants(h *game.Hand, wantTotal int) error {
	if h.Street == game.Complete {
		return nil
	}
	total, inPot := 0, 0
	for _, p := range h.Players {
		if p.Stack < 0 {
			return fmt.Errorf("stack negativo: %s", p.ID)
		}
		total += p.Stack
		inPot += p.TotalBet
	}
	if h.Pot != inPot {
		return fmt.Errorf("pote %d != soma das apostas (TotalBet) %d (rua %s)", h.Pot, inPot, h.Street)
	}
	total += h.Pot
	if total != wantTotal {
		return fmt.Errorf("fichas na mesa %d != constante %d (rua %s)", total, wantTotal, h.Street)
	}
	return nil
}

func bestTiedIndices(h *game.Hand, eligible []int) []int {
	if len(eligible) == 0 {
		return nil
	}
	best := game.EvaluateHand(sevenFor(h, eligible[0]))
	var out []int
	for _, idx := range eligible {
		r := game.EvaluateHand(sevenFor(h, idx))
		cmp := game.CompareHands(r, best)
		if cmp > 0 {
			best = r
			out = []int{idx}
		} else if cmp == 0 {
			out = append(out, idx)
		}
	}
	return out
}

func sevenFor(h *game.Hand, idx int) []game.Card {
	p := h.Players[idx]
	out := make([]game.Card, 0, 7)
	out = append(out, p.HoleCards[:]...)
	out = append(out, h.Board...)
	return out
}

func checkShowdownWinners(h *game.Hand) error {
	if len(h.SidePots) == 0 {
		if len(h.Winners) != 1 {
			return fmt.Errorf("fim com pote a um jogador: esperado 1 vencedor, tem %v", h.Winners)
		}
		return nil
	}
	if len(h.Board) != 5 {
		return fmt.Errorf("showdown com %d cartas comunitárias (esperado 5)", len(h.Board))
	}
	seen := make(map[int]struct{})
	for _, sp := range h.SidePots {
		if sp.Amount < 0 {
			return fmt.Errorf("pote com valor negativo: %d", sp.Amount)
		}
		if len(sp.Eligible) == 0 {
			return fmt.Errorf("side pot sem elegíveis")
		}
		best := bestTiedIndices(h, sp.Eligible)
		if len(best) == 0 {
			return fmt.Errorf("sem melhor mão entre elegíveis %v", sp.Eligible)
		}
		for _, b := range best {
			if !slices.Contains(h.Winners, b) {
				return fmt.Errorf("jogador %d é melhor entre elegíveis do pote mas não está em Winners %v", b, h.Winners)
			}
			seen[b] = struct{}{}
		}
	}
	uniq := make([]int, 0, len(seen))
	for w := range seen {
		uniq = append(uniq, w)
	}
	slices.Sort(uniq)
	wm := append([]int(nil), h.Winners...)
	slices.Sort(wm)
	if !slices.Equal(uniq, wm) {
		return fmt.Errorf("vencedores por pote (união) %v != Winners %v", uniq, wm)
	}
	return nil
}

func randomLegalAction(r *rand.Rand, h *game.Hand) (game.Action, error) {
	i := h.ActionOn
	p := h.Players[i]
	if p.Status != game.StatusActive {
		return game.Action{}, fmt.Errorf("ação fora de jogador ativo %d", i)
	}
	var cands []game.Action
	add := func(a game.Action) { cands = append(cands, a) }

	add(game.Action{PlayerIndex: i, Type: game.Fold})
	if p.StreetBet == h.CurrentBet {
		add(game.Action{PlayerIndex: i, Type: game.Check})
	}
	if h.CurrentBet > p.StreetBet {
		add(game.Action{PlayerIndex: i, Type: game.Call})
	}
	if p.Stack > 0 {
		add(game.Action{PlayerIndex: i, Type: game.AllIn})
	}
	maxTotal := p.Stack + p.StreetBet
	if h.CurrentBet == 0 {
		for _, a := range pickBetSizes(r, h.Blinds.Big, maxTotal, 20) {
			if a >= h.Blinds.Big && a <= maxTotal {
				add(game.Action{PlayerIndex: i, Type: game.Bet, Amount: a})
			}
		}
	} else {
		minTo := h.MinRaiseTo()
		for _, t := range pickBetSizes(r, minTo, maxTotal, 20) {
			if t >= minTo && t <= maxTotal {
				add(game.Action{PlayerIndex: i, Type: game.Raise, Amount: t})
			}
		}
	}
	if len(cands) == 0 {
		return game.Action{}, fmt.Errorf("nenhuma ação legal")
	}
	return cands[r.IntN(len(cands))], nil
}

// pickBetSizes gera tamanhos discretos entre lo e hi inclusive (máx. n valores únicos).
func pickBetSizes(r *rand.Rand, lo, hi, n int) []int {
	if lo > hi {
		return nil
	}
	if lo == hi {
		return []int{lo}
	}
	seen := make(map[int]struct{})
	out := []int{lo, hi}
	seen[lo], seen[hi] = struct{}{}, struct{}{}
	span := hi - lo
	for len(seen) < n && len(seen) < span+1 {
		// v2: IntN: func (n int) int in *Rand
		v := lo + r.IntN(span+1)
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}
