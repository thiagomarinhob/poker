package game

import (
	"github.com/google/uuid"
)

// TableStateView é a visão serializável da mesa para WebSocket; cartas de buraco só do viewer.
type TableStateView struct {
	HandID        uuid.UUID            `json:"hand_id,omitempty"`
	HandNumber    int                  `json:"hand_number,omitempty"`
	DealerSeat    int                  `json:"dealer_seat,omitempty"`
	Street        string               `json:"street"`
	Board         []string             `json:"board"`
	Pot           int                  `json:"pot"`
	Seats         []TableSeatView      `json:"seats"`
	YourCards     []string             `json:"your_cards,omitempty"`
	YourPlayerID  string               `json:"your_player_id,omitempty"`
	ActiveHandID  uuid.UUID            `json:"active_hand_id,omitempty"`
	CurrentBet    int                  `json:"current_bet,omitempty"`
	MinRaiseTo    int                  `json:"min_raise_to,omitempty"`
	ActionOnIndex *int                 `json:"action_on_index,omitempty"`
}

// TableSeatView representa um assento na mesa (público + cartas só se for o viewer no assento).
type TableSeatView struct {
	Index      int     `json:"index"`
	PlayerID   string  `json:"player_id,omitempty"`
	Stack      int     `json:"stack"`
	StreetBet  int     `json:"street_bet"`
	TotalBet   int     `json:"total_bet"`
	Status     string  `json:"status"`
	InHand     bool    `json:"in_hand"`
	HoleCards  []string `json:"hole_cards,omitempty"`
}

// TableStateViewForPlayer gera o estado visível para um viewer (PlayerID = identidade na mesa).
func (r *Room) TableStateViewForPlayer(viewerPlayerID string) TableStateView {
	v := TableStateView{
		Street:       "Waiting",
		YourPlayerID: viewerPlayerID,
		YourCards:    nil,
	}
	if r.hand == nil {
		v.Seats = r.seatViewsLobby()
		return v
	}
	v.HandID = r.activeHandID
	v.HandNumber = r.handCounter
	v.Street = r.hand.Street.String()
	v.Pot = r.hand.Pot
	v.CurrentBet = r.hand.CurrentBet
	v.MinRaiseTo = r.hand.MinRaiseTo()
	board := make([]string, len(r.hand.Board))
	for i, c := range r.hand.Board {
		board[i] = c.String()
	}
	v.Board = board
	ai := r.hand.ActionOn
	v.ActionOnIndex = &ai

	v.Seats = r.seatViewsForHand(viewerPlayerID)
	_, inHand, cards := r.viewerHandInfo(viewerPlayerID)
	if inHand && len(cards) == 2 {
		v.YourCards = []string{cards[0].String(), cards[1].String()}
	}
	v.ActiveHandID = r.activeHandID
	v.DealerSeat = r.dealerAtPhysicalSeat()
	return v
}

func (r *Room) seatViewsLobby() []TableSeatView {
	out := make([]TableSeatView, r.MaxSeats)
	for i := 0; i < r.MaxSeats; i++ {
		seat := r.seats[i]
		sv := TableSeatView{Index: i, InHand: false, Status: "Idle"}
		if seat != nil && seat.player != nil {
			sv.PlayerID = seat.player.ID
			sv.Stack = seat.player.Stack
		}
		out[i] = sv
	}
	return out
}

func (r *Room) seatViewsForHand(viewerPlayerID string) []TableSeatView {
	out := make([]TableSeatView, r.MaxSeats)
	if r.hand == nil {
		return r.seatViewsLobby()
	}
	viewerHIdx := -1
	if idx, hok := r.seatToHand[physicalSeatForPlayer(r, viewerPlayerID)]; hok {
		viewerHIdx = idx
	}
	for i := 0; i < r.MaxSeats; i++ {
		seat := r.seats[i]
		sv := TableSeatView{Index: i, InHand: false}
		if seat == nil || seat.player == nil {
			out[i] = sv
			continue
		}
		sv.PlayerID = seat.player.ID
		sv.Stack = seat.player.Stack
		if hi, ok := r.seatToHand[i]; ok {
			hp := r.hand.Players[hi]
			sv.InHand = true
			sv.StreetBet = hp.StreetBet
			sv.TotalBet = hp.TotalBet
			sv.Status = hp.Status.String()
			if hi == viewerHIdx {
				sv.HoleCards = []string{hp.HoleCards[0].String(), hp.HoleCards[1].String()}
			} else {
				if r.hand.Street == Showdown {
					sv.HoleCards = []string{hp.HoleCards[0].String(), hp.HoleCards[1].String()}
				}
			}
		} else {
			sv.InHand = false
			sv.Status = "Waiting"
		}
		out[i] = sv
	}
	return out
}

func physicalSeatForPlayer(r *Room, playerID string) int {
	if s, ok := r.playerSeat[playerID]; ok {
		return s
	}
	return -1
}

func (r *Room) viewerHandInfo(playerID string) (handIdx int, inHand bool, cards [2]Card) {
	seat := physicalSeatForPlayer(r, playerID)
	if seat < 0 || r.hand == nil {
		return -1, false, [2]Card{}
	}
	hi, ok := r.seatToHand[seat]
	if !ok {
		return -1, false, [2]Card{}
	}
	return hi, true, r.hand.Players[hi].HoleCards
}

func (r *Room) dealerAtPhysicalSeat() int {
	if r.dealerSeat < 0 {
		return -1
	}
	return r.dealerSeat
}
