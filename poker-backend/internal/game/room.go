package game

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/google/uuid"
)

const defaultEventBuffer = 128

// RoomOption ajusta a Room (persistência, room_id, etc.).
type RoomOption func(*Room) error

type Command interface {
	isCommand()
}

type SitDown struct {
	PlayerID string
	UserID   *uuid.UUID
	Seat     int
	BuyIn    int
	SitOut   bool
	Resp     chan error
}

func (SitDown) isCommand() {}

type StandUp struct {
	PlayerID string
	Resp     chan error
}

func (StandUp) isCommand() {}

type PlayerAction struct {
	PlayerID string
	Type     ActionType
	Amount   int
	Resp     chan error
}

func (PlayerAction) isCommand() {}

type Disconnect struct {
	PlayerID string
	Resp     chan error
}

func (Disconnect) isCommand() {}

// Reconnect marca o assento como conectado novamente (ex.: WebSocket reconectou).
type Reconnect struct {
	PlayerID string
	Resp     chan error
}

func (Reconnect) isCommand() {}

// InspectRoom consulta estado para operações administrativas (fora do ator da mesa).
type InspectRoom struct {
	Resp chan InspectRoomReply
}

func (InspectRoom) isCommand() {}

type InspectRoomReply struct {
	OccupiedSeats int
	ActiveHand    bool
}

type Event interface {
	isEvent()
}

type HandStarted struct {
	HandID     uuid.UUID
	HandNumber int
	PlayerIDs  []string
	DealerSeat int
}

func (HandStarted) isEvent() {}

type CardsDealt struct {
	HandID   uuid.UUID
	PlayerID string
	Cards    [2]Card
}

func (CardsDealt) isEvent() {}

type ActionRequired struct {
	HandID     uuid.UUID
	PlayerID   string
	ToCall     int
	CanCheck   bool
	MinRaiseTo int
	Timeout    time.Duration
}

func (ActionRequired) isEvent() {}

type HandComplete struct {
	HandID     uuid.UUID
	HandNumber int
	Winners    []string
}

func (HandComplete) isEvent() {}

type roomSeat struct {
	userID    *uuid.UUID
	player    *Player
	sitOut    bool
	connected bool
}

type Room struct {
	MaxSeats    int
	Blinds      Blinds
	TurnTimeout time.Duration

	Commands chan Command
	Out      chan Event

	seats        []*roomSeat
	playerSeat   map[string]int
	seatToHand   map[int]int
	handToSeat   []int
	hand         *Hand
	dealerSeat   int
	handCounter  int
	activeHandID uuid.UUID
	nextActionSeq int32

	roomID string
	userQ  userChipsQuerier
	handQ  handWriter

	persistCh     chan persistJob
	persistCtx    context.Context
	persistCancel context.CancelFunc
	persistWg     sync.WaitGroup

	turnTimer *time.Timer
	turnSeat  int
}

func NewRoom(maxSeats int, blinds Blinds, turnTimeout time.Duration, opts ...RoomOption) (*Room, error) {
	if maxSeats < 2 {
		return nil, fmt.Errorf("room: maxSeats must be >= 2")
	}
	if blinds.Small <= 0 || blinds.Big <= 0 || blinds.Small > blinds.Big {
		return nil, fmt.Errorf("room: invalid blinds %+v", blinds)
	}
	if turnTimeout <= 0 {
		return nil, fmt.Errorf("room: turn timeout must be > 0")
	}

	r := &Room{
		MaxSeats:    maxSeats,
		Blinds:      blinds,
		TurnTimeout: turnTimeout,
		Commands:    make(chan Command, 64),
		Out:         make(chan Event, defaultEventBuffer),
		seats:       make([]*roomSeat, maxSeats),
		playerSeat:  make(map[string]int, maxSeats),
		seatToHand:  make(map[int]int, maxSeats),
		dealerSeat:  -1,
		turnSeat:    -1,
	}
	for _, o := range opts {
		if o == nil {
			continue
		}
		if err := o(r); err != nil {
			return nil, err
		}
	}
	return r, nil
}

func (r *Room) Run(ctx context.Context) {
	for {
		var turnC <-chan time.Time
		if r.turnTimer != nil {
			turnC = r.turnTimer.C
		}

		select {
		case <-ctx.Done():
			r.stopTurnTimer()
			return
		case <-turnC:
			r.onTurnTimeout()
		case cmd := <-r.Commands:
			r.handleCommand(cmd)
		}
	}
}

func (r *Room) handleCommand(cmd Command) {
	switch c := cmd.(type) {
	case SitDown:
		r.reply(c.Resp, r.onSitDown(c))
	case StandUp:
		r.reply(c.Resp, r.onStandUp(c))
	case PlayerAction:
		r.reply(c.Resp, r.onPlayerAction(c))
	case Disconnect:
		r.reply(c.Resp, r.onDisconnect(c))
	case Reconnect:
		r.reply(c.Resp, r.onReconnect(c))
	case InspectRoom:
		r.onInspectRoom(c)
	}
}

func (r *Room) onSitDown(c SitDown) error {
	if c.PlayerID == "" {
		return fmt.Errorf("sit down: empty player id")
	}
	if c.BuyIn <= 0 {
		return fmt.Errorf("sit down: buy-in must be > 0")
	}
	if c.Seat < 0 || c.Seat >= r.MaxSeats {
		return fmt.Errorf("sit down: seat %d out of range", c.Seat)
	}
	if _, exists := r.playerSeat[c.PlayerID]; exists {
		return fmt.Errorf("sit down: player %s already seated", c.PlayerID)
	}
	if r.seats[c.Seat] != nil {
		return fmt.Errorf("sit down: seat %d occupied", c.Seat)
	}
	if c.UserID != nil && r.userQ == nil {
		return fmt.Errorf("sit down: UserID requer WithPersistence (UserQ)")
	}
	if c.UserID != nil {
		if err := r.applyChipsSync(*c.UserID, -int64(c.BuyIn), "buy_in", nil, nil); err != nil {
			return err
		}
	}

	r.seats[c.Seat] = &roomSeat{
		userID:    c.UserID,
		player:    &Player{ID: c.PlayerID, Stack: c.BuyIn},
		sitOut:    c.SitOut,
		connected: true,
	}
	r.playerSeat[c.PlayerID] = c.Seat
	r.maybeStartHand()
	return nil
}

func (r *Room) onStandUp(c StandUp) error {
	seatIdx, ok := r.playerSeat[c.PlayerID]
	if !ok {
		return fmt.Errorf("stand up: player %s is not seated", c.PlayerID)
	}
	if r.turnSeat == seatIdx {
		r.stopTurnTimer()
	}
	seat := r.seats[seatIdx]
	if r.userQ != nil && seat != nil && seat.userID != nil {
		st := seat.player.Stack
		if st > 0 {
			if err := r.applyChipsSync(*seat.userID, int64(st), "stand_up", nil, nil); err != nil {
				return err
			}
		}
	}

	delete(r.playerSeat, c.PlayerID)
	r.seats[seatIdx] = nil

	if r.hand != nil {
		r.tryForceFoldBySeat(seatIdx)
		r.afterActionOrProgress()
		return nil
	}

	r.maybeStartHand()
	return nil
}

func (r *Room) onDisconnect(c Disconnect) error {
	seatIdx, ok := r.playerSeat[c.PlayerID]
	if !ok {
		return fmt.Errorf("disconnect: player %s is not seated", c.PlayerID)
	}

	seat := r.seats[seatIdx]
	seat.connected = false
	seat.sitOut = true

	if r.turnSeat == seatIdx {
		r.stopTurnTimer()
	}
	if r.hand != nil {
		r.tryForceFoldBySeat(seatIdx)
		r.afterActionOrProgress()
		return nil
	}

	r.maybeStartHand()
	return nil
}

func (r *Room) onInspectRoom(c InspectRoom) {
	n := 0
	for _, s := range r.seats {
		if s != nil {
			n++
		}
	}
	active := r.hand != nil
	if c.Resp != nil {
		select {
		case c.Resp <- InspectRoomReply{OccupiedSeats: n, ActiveHand: active}:
		default:
		}
	}
}

// RequestInspect envia InspectRoom ao loop da Room (thread-safe).
func (r *Room) RequestInspect(ctx context.Context) (InspectRoomReply, error) {
	resp := make(chan InspectRoomReply, 1)
	cmd := InspectRoom{Resp: resp}
	select {
	case r.Commands <- cmd:
	case <-ctx.Done():
		return InspectRoomReply{}, ctx.Err()
	}
	select {
	case out := <-resp:
		return out, nil
	case <-ctx.Done():
		return InspectRoomReply{}, ctx.Err()
	}
}

func (r *Room) onReconnect(c Reconnect) error {
	seatIdx, ok := r.playerSeat[c.PlayerID]
	if !ok {
		return fmt.Errorf("reconnect: player %s is not seated", c.PlayerID)
	}
	seat := r.seats[seatIdx]
	if seat == nil {
		return fmt.Errorf("reconnect: empty seat for player %s", c.PlayerID)
	}
	seat.connected = true
	seat.sitOut = false
	if r.hand == nil {
		r.maybeStartHand()
	}
	return nil
}

func (r *Room) onPlayerAction(c PlayerAction) error {
	if r.hand == nil || r.hand.Street == Complete {
		return fmt.Errorf("player action: no active hand")
	}

	seatIdx, ok := r.playerSeat[c.PlayerID]
	if !ok {
		return fmt.Errorf("player action: player %s is not seated", c.PlayerID)
	}
	handIdx, ok := r.seatToHand[seatIdx]
	if !ok {
		return fmt.Errorf("player action: player %s not in active hand", c.PlayerID)
	}
	if handIdx != r.hand.ActionOn {
		return fmt.Errorf("player action: not player %s turn", c.PlayerID)
	}

	r.stopTurnTimer()
	hid := r.activeHandID
	str := r.hand.Street.String()
	var amt *int64
	if c.Type == Bet || c.Type == Raise {
		v := int64(c.Amount)
		amt = &v
	}
	err := r.hand.ApplyAction(Action{PlayerIndex: handIdx, Type: c.Type, Amount: c.Amount})
	if err != nil {
		return err
	}
	r.persistHandAction(hid, seatIdx, handIdx, c.Type, amt, str, false)
	r.afterActionOrProgress()
	return nil
}

func (r *Room) onTurnTimeout() {
	if r.hand == nil || r.hand.Street == Complete {
		r.stopTurnTimer()
		return
	}
	if r.turnSeat < 0 {
		return
	}

	handIdx, ok := r.seatToHand[r.turnSeat]
	if !ok || handIdx != r.hand.ActionOn {
		r.stopTurnTimer()
		return
	}

	r.stopTurnTimer()
	hid := r.activeHandID
	str := r.hand.Street.String()
	_ = r.hand.ApplyAction(Action{PlayerIndex: handIdx, Type: Fold})
	r.persistHandAction(hid, r.turnSeat, handIdx, Fold, nil, str, true)
	r.afterActionOrProgress()
}

func (r *Room) afterActionOrProgress() {
	if r.hand == nil {
		return
	}
	if r.hand.Street == Complete {
		hnd := r.hand
		hid := r.activeHandID
		winners := make([]string, 0, len(r.hand.Winners))
		for _, idx := range r.hand.Winners {
			seatIdx := r.handToSeat[idx]
			if seat := r.seats[seatIdx]; seat != nil {
				winners = append(winners, seat.player.ID)
			}
		}
		r.emit(HandComplete{
			HandID:     hid,
			HandNumber: r.handCounter,
			Winners:    winners,
		})
		r.persistHandComplete(hnd, hid)
		r.stopTurnTimer()
		r.hand = nil
		r.activeHandID = uuid.UUID{}
		r.nextActionSeq = 0
		r.seatToHand = make(map[int]int, r.MaxSeats)
		r.handToSeat = nil
		r.turnSeat = -1
		r.maybeStartHand()
		return
	}

	r.emitActionRequired()
}

func (r *Room) maybeStartHand() {
	if r.hand != nil {
		return
	}

	eligible := r.eligibleSeats()
	if len(eligible) < 2 {
		return
	}

	dealerSeat := r.nextDealerSeat(eligible)
	players := make([]*Player, 0, len(eligible))
	handToSeat := make([]int, 0, len(eligible))
	seatToHand := make(map[int]int, len(eligible))
	dealerPos := -1

	for idx, seatNum := range eligible {
		players = append(players, r.seats[seatNum].player)
		handToSeat = append(handToSeat, seatNum)
		seatToHand[seatNum] = idx
		if seatNum == dealerSeat {
			dealerPos = idx
		}
	}
	if dealerPos < 0 {
		return
	}

	hand, err := StartHand(players, r.Blinds, dealerPos)
	if err != nil {
		return
	}

	r.handCounter++
	handID := uuid.New()
	r.hand = hand
	r.activeHandID = handID
	r.handToSeat = handToSeat
	r.seatToHand = seatToHand
	r.dealerSeat = dealerSeat

	playerIDs := make([]string, len(players))
	for i, p := range players {
		playerIDs[i] = p.ID
	}

	r.emit(HandStarted{
		HandID:     handID,
		HandNumber: r.handCounter,
		PlayerIDs:  playerIDs,
		DealerSeat: dealerSeat,
	})
	for i, p := range r.hand.Players {
		_ = i
		r.emit(CardsDealt{HandID: handID, PlayerID: p.ID, Cards: p.HoleCards})
	}
	r.emitActionRequired()
	r.persistHandAndBlindsStart(r.hand, handID)
}

func (r *Room) emitActionRequired() {
	if r.hand == nil || r.hand.Street == Complete {
		return
	}
	actionSeat := r.handToSeat[r.hand.ActionOn]
	r.turnSeat = actionSeat

	player := r.seats[actionSeat].player
	toCall := r.hand.CurrentBet - r.hand.Players[r.hand.ActionOn].StreetBet
	if toCall < 0 {
		toCall = 0
	}

	r.emit(ActionRequired{
		HandID:     r.activeHandID,
		PlayerID:   player.ID,
		ToCall:     toCall,
		CanCheck:   toCall == 0,
		MinRaiseTo: r.hand.MinRaiseTo(),
		Timeout:    r.TurnTimeout,
	})

	r.stopTurnTimer()
	r.turnTimer = time.NewTimer(r.TurnTimeout)
}

func (r *Room) tryForceFoldBySeat(seatIdx int) {
	if r.hand == nil || r.hand.Street == Complete {
		return
	}

	handIdx, ok := r.seatToHand[seatIdx]
	if !ok {
		return
	}
	if handIdx != r.hand.ActionOn {
		return
	}
	_ = r.hand.ApplyAction(Action{PlayerIndex: handIdx, Type: Fold})
}

func (r *Room) eligibleSeats() []int {
	eligible := make([]int, 0, r.MaxSeats)
	for seatIdx, seat := range r.seats {
		if seat == nil || seat.sitOut || !seat.connected {
			continue
		}
		if seat.player.Stack <= 0 {
			continue
		}
		eligible = append(eligible, seatIdx)
	}
	return eligible
}

func (r *Room) nextDealerSeat(eligible []int) int {
	slices.Sort(eligible)
	if r.dealerSeat < 0 {
		return eligible[0]
	}
	for _, seat := range eligible {
		if seat > r.dealerSeat {
			return seat
		}
	}
	return eligible[0]
}

func (r *Room) stopTurnTimer() {
	if r.turnTimer == nil {
		return
	}
	if !r.turnTimer.Stop() {
		select {
		case <-r.turnTimer.C:
		default:
		}
	}
	r.turnTimer = nil
}

// ActiveHandID retorna o id da mão ativa (zero se não houver).
func (r *Room) ActiveHandID() uuid.UUID {
	return r.activeHandID
}

// IsSeated indica se o player já tem assento.
func (r *Room) IsSeated(playerID string) bool {
	_, ok := r.playerSeat[playerID]
	return ok
}

func (r *Room) emit(event Event) {
	r.Out <- event
}

func (r *Room) reply(ch chan error, err error) {
	if ch == nil {
		return
	}
	ch <- err
}
