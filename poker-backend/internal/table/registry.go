package table

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/thiagomarinho/poker-backend/internal/game"
	"github.com/thiagomarinho/poker-backend/internal/ws"
)

type Runtime struct {
	ID          uuid.UUID
	Name        string
	MaxSeats    int
	SmallBlind  int
	BigBlind    int
	TurnTimeout time.Duration
	Room        *game.Room
	CreatedAt   time.Time

	cancel context.CancelFunc
}

type Registry struct {
	mu             sync.RWMutex
	ctx            context.Context
	tables         map[uuid.UUID]*Runtime
	pump           ws.PumpFunc
	clearPumpHook  func(uuid.UUID)
}

func NewRegistry(parent context.Context) *Registry {
	return &Registry{
		ctx:    parent,
		tables: make(map[uuid.UUID]*Runtime),
	}
}

// SetClearPumpHook é chamado antes de religar room.Out ao hub após UpdateTable.
func (r *Registry) SetClearPumpHook(fn func(uuid.UUID)) {
	r.mu.Lock()
	r.clearPumpHook = fn
	r.mu.Unlock()
}

func (r *Registry) GameRoom(id uuid.UUID) (*game.Room, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tables[id]
	if !ok {
		return nil, false
	}
	return t.Room, true
}

func (r *Registry) CreateTable(name string, maxSeats, smallBlind, bigBlind int, turnTimeout time.Duration, pump ws.PumpFunc) (*Runtime, error) {
	if name == "" {
		return nil, fmt.Errorf("table: name required")
	}
	if maxSeats < 2 || maxSeats > 10 {
		return nil, fmt.Errorf("table: max_seats must be 2..10")
	}
	if smallBlind <= 0 || bigBlind <= 0 || smallBlind > bigBlind {
		return nil, fmt.Errorf("table: invalid blinds")
	}
	if turnTimeout <= 0 {
		return nil, fmt.Errorf("table: turn_timeout must be > 0")
	}

	room, err := game.NewRoom(maxSeats, game.Blinds{Small: smallBlind, Big: bigBlind}, turnTimeout)
	if err != nil {
		return nil, err
	}

	id := uuid.New()
	roomCtx, cancel := context.WithCancel(r.ctx)
	go room.Run(roomCtx)

	rt := &Runtime{
		ID:          id,
		Name:        name,
		MaxSeats:    maxSeats,
		SmallBlind:  smallBlind,
		BigBlind:    bigBlind,
		TurnTimeout: turnTimeout,
		Room:        room,
		cancel:      cancel,
		CreatedAt:   time.Now(),
	}

	r.mu.Lock()
	if pump != nil {
		r.pump = pump
	}
	pFn := r.pump
	r.tables[id] = rt
	r.mu.Unlock()

	if pFn != nil {
		pFn(id, room)
	}
	return rt, nil
}

func (r *Registry) Get(id uuid.UUID) (*Runtime, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tables[id]
	if !ok {
		return nil, false
	}
	cp := *t
	cp.Room = nil
	cp.cancel = nil
	return &cp, true
}

func (r *Registry) inspect(ctx context.Context, id uuid.UUID) (game.InspectRoomReply, error) {
	r.mu.RLock()
	rt, ok := r.tables[id]
	r.mu.RUnlock()
	if !ok {
		return game.InspectRoomReply{}, fmt.Errorf("table not found")
	}
	return rt.Room.RequestInspect(ctx)
}

// UpdateTable recria a Room quando não há jogadores nem mão ativa.
func (r *Registry) UpdateTable(ctx context.Context, id uuid.UUID, name string, maxSeats, smallBlind, bigBlind int, turnTimeout time.Duration) error {
	if name == "" {
		return fmt.Errorf("table: name required")
	}
	if maxSeats < 2 || maxSeats > 10 {
		return fmt.Errorf("table: max_seats must be 2..10")
	}
	if smallBlind <= 0 || bigBlind <= 0 || smallBlind > bigBlind {
		return fmt.Errorf("table: invalid blinds")
	}
	if turnTimeout <= 0 {
		return fmt.Errorf("table: turn_timeout must be > 0")
	}

	s1, err := r.inspect(ctx, id)
	if err != nil {
		return err
	}
	if s1.OccupiedSeats > 0 || s1.ActiveHand {
		return fmt.Errorf("table: cannot update while players seated or hand active")
	}

	room, err := game.NewRoom(maxSeats, game.Blinds{Small: smallBlind, Big: bigBlind}, turnTimeout)
	if err != nil {
		return err
	}
	roomCtx, cancel := context.WithCancel(r.ctx)

	s2, err := r.inspect(ctx, id)
	if err != nil {
		cancel()
		return err
	}
	if s2.OccupiedSeats > 0 || s2.ActiveHand {
		cancel()
		return fmt.Errorf("table: state changed, retry")
	}

	r.mu.Lock()
	rt, ok := r.tables[id]
	if !ok {
		r.mu.Unlock()
		cancel()
		return fmt.Errorf("table not found")
	}
	oldCancel := rt.cancel
	oldRoom := rt.Room
	pFn := r.pump
	clearHook := r.clearPumpHook
	rt.Room = room
	rt.cancel = cancel
	rt.Name = name
	rt.MaxSeats = maxSeats
	rt.SmallBlind = smallBlind
	rt.BigBlind = bigBlind
	rt.TurnTimeout = turnTimeout
	r.mu.Unlock()

	if oldCancel != nil {
		oldCancel()
	}
	_ = oldRoom
	go room.Run(roomCtx)
	if clearHook != nil {
		clearHook(id)
	}
	if pFn != nil {
		pFn(id, room)
	}
	return nil
}

// DeleteTable remove a mesa quando vazia e sem mão ativa.
func (r *Registry) DeleteTable(ctx context.Context, id uuid.UUID) error {
	s1, err := r.inspect(ctx, id)
	if err != nil {
		return err
	}
	if s1.OccupiedSeats > 0 || s1.ActiveHand {
		return fmt.Errorf("table: cannot delete while players seated or hand active")
	}
	s2, err := r.inspect(ctx, id)
	if err != nil {
		return err
	}
	if s2.OccupiedSeats > 0 || s2.ActiveHand {
		return fmt.Errorf("table: state changed, retry")
	}

	r.mu.Lock()
	rt, ok := r.tables[id]
	if !ok {
		r.mu.Unlock()
		return fmt.Errorf("table not found")
	}
	delete(r.tables, id)
	r.mu.Unlock()

	if rt.cancel != nil {
		rt.cancel()
	}
	return nil
}

func (r *Registry) List() []Runtime {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Runtime, 0, len(r.tables))
	for _, t := range r.tables {
		out = append(out, Runtime{
			ID:          t.ID,
			Name:        t.Name,
			MaxSeats:    t.MaxSeats,
			SmallBlind:  t.SmallBlind,
			BigBlind:    t.BigBlind,
			TurnTimeout: t.TurnTimeout,
			Room:        nil,
			CreatedAt:   t.CreatedAt,
		})
	}
	return out
}

func (r *Registry) Shutdown() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, t := range r.tables {
		if t.cancel != nil {
			t.cancel()
		}
	}
}
