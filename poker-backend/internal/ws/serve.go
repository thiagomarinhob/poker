package ws

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/thiagomarinho/poker-backend/internal/auth"
	"github.com/thiagomarinho/poker-backend/internal/game"
	"github.com/thiagomarinho/poker-backend/internal/user"
	"nhooyr.io/websocket"
)

type Handler struct {
	hub         *Hub
	jwtSecret   string
	userQueries *user.Queries
}

func NewHandler(h *Hub, jwtSecret string, userQueries *user.Queries) *Handler {
	return &Handler{hub: h, jwtSecret: jwtSecret, userQueries: userQueries}
}

func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/ws", h.serveWS)
}

func (h *Handler) serveWS(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "token query param required"})
		return
	}
	sub, _, err := auth.UserFromAccessToken(token, h.jwtSecret)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
		return
	}
	uid, err := uuid.Parse(sub)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid subject"})
		return
	}

	conn, err := websocket.Accept(c.Writer, c.Request, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
		CompressionMode:    websocket.CompressionDisabled,
	})
	if err != nil {
		log.Error().Err(err).Msg("ws: accept failed")
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	cl := newClient(h.hub, uid, conn)
	h.hub.registerClient(cl)
	defer func() {
		h.disconnectRoomSeat(cl.userID)
		h.hub.unregisterClient(cl)
		cl.closeSend()
	}()

	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	go h.writePump(ctx, cl)
	h.readPump(ctx, cl)
}

func (h *Handler) disconnectRoomSeat(uid uuid.UUID) {
	tid, ok := h.hub.currentTable(uid)
	if !ok {
		return
	}
	gr, ok := h.hub.rooms.GameRoom(tid)
	if !ok {
		return
	}
	pid := uid.String()
	_ = sendRoom(gr, game.Disconnect{PlayerID: pid})
}

func (h *Handler) writePump(ctx context.Context, cl *Client) {
	ping := time.NewTicker(25 * time.Second)
	defer ping.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ping.C:
			writeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			_ = cl.conn.Ping(writeCtx)
			cancel()
		case msg, ok := <-cl.send:
			if !ok {
				return
			}
			writeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			err := cl.conn.Write(writeCtx, websocket.MessageText, msg)
			cancel()
			if err != nil {
				return
			}
		}
	}
}

func (h *Handler) readPump(ctx context.Context, cl *Client) {
	for {
		readCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
		_, data, err := cl.conn.Read(readCtx)
		cancel()
		if err != nil {
			return
		}
		var env envelope
		if err := json.Unmarshal(data, &env); err != nil {
			h.hub.sendError(cl.userID, "bad_message", "invalid json envelope")
			continue
		}
		if err := h.handleEnvelope(ctx, cl, env); err != nil {
			h.hub.sendError(cl.userID, "handler", err.Error())
		}
	}
}

func (h *Handler) handleEnvelope(ctx context.Context, cl *Client, env envelope) error {
	switch env.Type {
	case TypePing:
		cl.send <- mustEnvelope(TypePong, map[string]any{"ts": time.Now().UnixMilli()})
		return nil
	case TypeJoinTable:
		return h.handleJoin(ctx, cl, env.Payload)
	case TypeLeaveTable:
		return h.handleLeave(cl, env.Payload)
	case TypeAction:
		return h.handleAction(cl, env.Payload)
	case TypeChat:
		return h.handleChat(cl, env.Payload)
	default:
		return fmt.Errorf("unknown type %q", env.Type)
	}
}

func (h *Handler) handleJoin(_ context.Context, cl *Client, p map[string]any) error {
	tidStr, _ := p["table_id"].(string)
	tid, err := uuid.Parse(tidStr)
	if err != nil {
		return fmt.Errorf("invalid table_id")
	}
	if cur, ok := h.hub.currentTable(cl.userID); ok && cur != tid {
		return fmt.Errorf("already at another table")
	}
	seatF, ok := num(p["seat"])
	if !ok {
		return fmt.Errorf("seat required")
	}
	buyInF, ok := num(p["buy_in"])
	if !ok {
		return fmt.Errorf("buy_in required")
	}
	seat := int(seatF)
	buyIn := int(buyInF)

	gr, ok := h.hub.rooms.GameRoom(tid)
	if !ok {
		return fmt.Errorf("table not found")
	}

	pid := cl.userID.String()
	h.hub.subscribeUserToTable(cl.userID, tid)

	if gr.IsSeated(pid) {
		if err := sendRoom(gr, game.Reconnect{PlayerID: pid}); err != nil {
			h.hub.unsubscribeUser(cl.userID)
			return err
		}
	} else {
		displayEmail := ""
		if h.userQueries != nil {
			if u, err := h.userQueries.GetUserByID(context.Background(), cl.userID); err == nil {
				displayEmail = u.Email
			}
		}
		sd := game.SitDown{
			PlayerID:     pid,
			UserID:       nil,
			Seat:         seat,
			BuyIn:        buyIn,
			SitOut:       false,
			DisplayEmail: displayEmail,
		}
		if err := sendRoom(gr, sd); err != nil {
			h.hub.unsubscribeUser(cl.userID)
			return err
		}
		h.hub.broadcastPlayerJoined(tid, cl.userID, seat, pid)
	}

	st := gr.TableStateViewForPlayer(pid)
	raw, err := json.Marshal(envelope{Type: TypeTableState, Payload: tableStatePayload(tid, st)})
	if err != nil {
		return err
	}
	cl.send <- raw
	return nil
}

func (h *Handler) handleLeave(cl *Client, p map[string]any) error {
	tidStr, _ := p["table_id"].(string)
	want, err := uuid.Parse(tidStr)
	if err != nil {
		return fmt.Errorf("invalid table_id")
	}
	cur, ok := h.hub.currentTable(cl.userID)
	if !ok || cur != want {
		return fmt.Errorf("not at this table")
	}
	gr, ok := h.hub.rooms.GameRoom(want)
	if !ok {
		h.hub.unsubscribeUser(cl.userID)
		return nil
	}
	pid := cl.userID.String()
	_ = sendRoom(gr, game.StandUp{PlayerID: pid})
	h.hub.unsubscribeUser(cl.userID)
	return nil
}

func (h *Handler) handleAction(cl *Client, p map[string]any) error {
	tidStr, _ := p["table_id"].(string)
	tid, err := uuid.Parse(tidStr)
	if err != nil {
		return fmt.Errorf("invalid table_id")
	}
	cur, ok := h.hub.currentTable(cl.userID)
	if !ok || cur != tid {
		return fmt.Errorf("not at this table")
	}
	gr, ok := h.hub.rooms.GameRoom(tid)
	if !ok {
		return fmt.Errorf("table not found")
	}
	at, err := parseActionType(p["action"])
	if err != nil {
		return err
	}
	amt := 0
	if v, ok := num(p["amount"]); ok {
		amt = int(v)
	}
	pid := cl.userID.String()
	return sendRoom(gr, game.PlayerAction{PlayerID: pid, Type: at, Amount: amt})
}

func (h *Handler) handleChat(cl *Client, p map[string]any) error {
	tidStr, _ := p["table_id"].(string)
	tid, err := uuid.Parse(tidStr)
	if err != nil {
		return fmt.Errorf("invalid table_id")
	}
	cur, ok := h.hub.currentTable(cl.userID)
	if !ok || cur != tid {
		return fmt.Errorf("not at this table")
	}
	text, _ := p["text"].(string)
	if text == "" {
		return fmt.Errorf("empty text")
	}
	h.hub.broadcastChat(tid, cl.userID, text)
	return nil
}

func (h *Hub) currentTable(userID uuid.UUID) (uuid.UUID, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	t, ok := h.userToTable[userID]
	return t, ok
}

func sendRoom(gr *game.Room, cmd any) error {
	ch := make(chan error, 1)
	switch c := cmd.(type) {
	case game.SitDown:
		c.Resp = ch
		gr.Commands <- c
	case game.StandUp:
		c.Resp = ch
		gr.Commands <- c
	case game.PlayerAction:
		c.Resp = ch
		gr.Commands <- c
	case game.Disconnect:
		c.Resp = ch
		gr.Commands <- c
	case game.Reconnect:
		c.Resp = ch
		gr.Commands <- c
	default:
		return fmt.Errorf("unsupported command")
	}
	select {
	case err := <-ch:
		return err
	case <-time.After(5 * time.Second):
		return fmt.Errorf("room timeout")
	}
}

func num(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case int:
		return float64(x), true
	case int64:
		return float64(x), true
	case json.Number:
		f, err := x.Float64()
		return f, err == nil
	case string:
		f, err := strconv.ParseFloat(x, 64)
		return f, err == nil
	default:
		return 0, false
	}
}

func parseActionType(v any) (game.ActionType, error) {
	s, _ := v.(string)
	switch s {
	case "Fold":
		return game.Fold, nil
	case "Check":
		return game.Check, nil
	case "Call":
		return game.Call, nil
	case "Bet":
		return game.Bet, nil
	case "Raise":
		return game.Raise, nil
	case "AllIn":
		return game.AllIn, nil
	default:
		return 0, fmt.Errorf("unknown action %q", s)
	}
}
