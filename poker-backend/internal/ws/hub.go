package ws

import (
	"encoding/json"
	"sync"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/thiagomarinho/poker-backend/internal/game"
)

// PumpFunc liga uma mesa criada ao Hub (consumo de room.Out).
type PumpFunc func(tableID uuid.UUID, room *game.Room)

// GameRoomTable resolve a Room por id de mesa (implementado por table.Registry).
type GameRoomTable interface {
	GameRoom(id uuid.UUID) (*game.Room, bool)
}

// Hub mantém conexões por user_id e assinaturas por mesa.
type Hub struct {
	mu sync.RWMutex

	rooms GameRoomTable

	users        map[uuid.UUID]*Client
	userToTable  map[uuid.UUID]uuid.UUID
	tableToUsers map[uuid.UUID]map[uuid.UUID]struct{}
	pumped       map[uuid.UUID]struct{}
}

func NewHub(rooms GameRoomTable) *Hub {
	return &Hub{
		rooms:        rooms,
		users:        make(map[uuid.UUID]*Client),
		userToTable:  make(map[uuid.UUID]uuid.UUID),
		tableToUsers: make(map[uuid.UUID]map[uuid.UUID]struct{}),
		pumped:       make(map[uuid.UUID]struct{}),
	}
}

// ConnectedUserCount retorna usuários com WebSocket ativo (painel admin).
func (h *Hub) ConnectedUserCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.users)
}

// EvictTable remove assinaturas WS da mesa (ex.: mesa removida pelo admin).
func (h *Hub) EvictTable(tableID uuid.UUID) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if m := h.tableToUsers[tableID]; m != nil {
		for uid := range m {
			delete(h.userToTable, uid)
		}
	}
	delete(h.tableToUsers, tableID)
}

// ClearPump permite ligar um novo room.Out ao hub após troca da Room na mesma mesa.
func (h *Hub) ClearPump(tableID uuid.UUID) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.pumped, tableID)
}

// PumpRoom consome room.Out e distribui eventos (estado personalizado).
func (h *Hub) PumpRoom(tableID uuid.UUID, gr *game.Room) {
	h.mu.Lock()
	if h.pumped == nil {
		h.pumped = make(map[uuid.UUID]struct{})
	}
	if _, ok := h.pumped[tableID]; ok {
		h.mu.Unlock()
		return
	}
	h.pumped[tableID] = struct{}{}
	h.mu.Unlock()

	go func() {
		for {
			ev, ok := <-gr.Out
			if !ok {
				return
			}
			h.dispatchRoomEvent(tableID, gr, ev)
		}
	}()
}

func (h *Hub) dispatchRoomEvent(tableID uuid.UUID, gr *game.Room, ev game.Event) {
	h.broadcastTableStates(tableID, gr)

	switch ev.(type) {
	case game.ActionRequired:
		e := ev.(game.ActionRequired)
		raw := mustEnvelope(TypeActionRequiredS, map[string]any{
			"table_id":     tableID.String(),
			"hand_id":      e.HandID.String(),
			"player_id":    e.PlayerID,
			"to_call":      e.ToCall,
			"can_check":    e.CanCheck,
			"min_raise_to": e.MinRaiseTo,
			"timeout_ms":   e.Timeout.Milliseconds(),
		})
		h.broadcastRawToTable(tableID, raw)
	case game.HandComplete:
		hc := ev.(game.HandComplete)
		raw := mustEnvelope(TypeHandResult, map[string]any{
			"table_id":        tableID.String(),
			"hand_id":         hc.HandID.String(),
			"hand_number":     hc.HandNumber,
			"winners":         hc.Winners,
			"winner_emails":   hc.WinnerEmails,
		})
		h.broadcastRawToTable(tableID, raw)
	default:
	}
}

func (h *Hub) broadcastTableStates(tableID uuid.UUID, gr *game.Room) {
	h.mu.RLock()
	set := h.tableToUsers[tableID]
	ids := make([]uuid.UUID, 0, len(set))
	for u := range set {
		ids = append(ids, u)
	}
	h.mu.RUnlock()

	for _, uid := range ids {
		st := gr.TableStateViewForPlayer(uid.String())
		raw, err := json.Marshal(envelope{Type: TypeTableState, Payload: tableStatePayload(tableID, st)})
		if err != nil {
			log.Error().Err(err).Msg("ws: marshal table state")
			continue
		}
		h.sendToUser(uid, raw)
	}
}

func tableStatePayload(tableID uuid.UUID, st game.TableStateView) map[string]any {
	b, _ := json.Marshal(st)
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	if m == nil {
		m = map[string]any{}
	}
	m["table_id"] = tableID.String()
	return m
}

func (h *Hub) broadcastRawToTable(tableID uuid.UUID, raw []byte) {
	h.mu.RLock()
	set := h.tableToUsers[tableID]
	ids := make([]uuid.UUID, 0, len(set))
	for u := range set {
		ids = append(ids, u)
	}
	h.mu.RUnlock()
	for _, uid := range ids {
		h.sendToUser(uid, raw)
	}
}

func (h *Hub) registerClient(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if old := h.users[c.userID]; old != nil {
		old.closeSend()
	}
	h.users[c.userID] = c
}

func (h *Hub) unregisterClient(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if cur, ok := h.users[c.userID]; ok && cur == c {
		delete(h.users, c.userID)
	}
	if tid, ok := h.userToTable[c.userID]; ok {
		delete(h.userToTable, c.userID)
		if m := h.tableToUsers[tid]; m != nil {
			delete(m, c.userID)
			if len(m) == 0 {
				delete(h.tableToUsers, tid)
			}
		}
	}
}

func (h *Hub) subscribeUserToTable(userID, tableID uuid.UUID) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.userToTable[userID] = tableID
	if h.tableToUsers[tableID] == nil {
		h.tableToUsers[tableID] = make(map[uuid.UUID]struct{})
	}
	h.tableToUsers[tableID][userID] = struct{}{}
}

func (h *Hub) unsubscribeUser(userID uuid.UUID) uuid.UUID {
	h.mu.Lock()
	defer h.mu.Unlock()
	tid, ok := h.userToTable[userID]
	if !ok {
		return uuid.UUID{}
	}
	delete(h.userToTable, userID)
	if m := h.tableToUsers[tid]; m != nil {
		delete(m, userID)
		if len(m) == 0 {
			delete(h.tableToUsers, tid)
		}
	}
	return tid
}

func (h *Hub) sendToUser(userID uuid.UUID, raw []byte) bool {
	h.mu.RLock()
	c := h.users[userID]
	h.mu.RUnlock()
	if c == nil {
		return false
	}
	select {
	case c.send <- raw:
		return true
	default:
		return false
	}
}

func (h *Hub) sendError(userID uuid.UUID, code, msg string) {
	raw := mustEnvelope(TypeError, map[string]any{"code": code, "message": msg})
	h.sendToUser(userID, raw)
}

func mustEnvelope(typ string, payload map[string]any) []byte {
	b, err := json.Marshal(envelope{Type: typ, Payload: payload})
	if err != nil {
		return []byte(`{"type":"Error","payload":{"code":"internal","message":"marshal"}}`)
	}
	return b
}

func (h *Hub) broadcastPlayerJoined(tableID, joiner uuid.UUID, seat int, playerID string) {
	raw := mustEnvelope(TypePlayerJoined, map[string]any{
		"table_id":   tableID.String(),
		"player_id":  playerID,
		"seat":       seat,
		"joiner_id":  joiner.String(),
	})
	h.mu.RLock()
	set := h.tableToUsers[tableID]
	ids := make([]uuid.UUID, 0, len(set))
	for u := range set {
		if u != joiner {
			ids = append(ids, u)
		}
	}
	h.mu.RUnlock()
	for _, uid := range ids {
		h.sendToUser(uid, raw)
	}
}

func (h *Hub) broadcastChat(tableID, from uuid.UUID, text string) {
	raw := mustEnvelope(TypeChat, map[string]any{
		"table_id": tableID.String(),
		"from_id":  from.String(),
		"text":     text,
	})
	h.broadcastRawToTable(tableID, raw)
}
