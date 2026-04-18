package admin

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog/log"
	"github.com/thiagomarinho/poker-backend/internal/auditlog"
	"github.com/thiagomarinho/poker-backend/internal/auth"
	"github.com/thiagomarinho/poker-backend/internal/pokerdb"
	"github.com/thiagomarinho/poker-backend/internal/table"
	"github.com/thiagomarinho/poker-backend/internal/user"
	"github.com/thiagomarinho/poker-backend/internal/ws"
)

type Hub interface {
	ConnectedUserCount() int
	EvictTable(tableID uuid.UUID)
	ClearPump(tableID uuid.UUID)
}

type Handler struct {
	users  user.Querier
	hands  pokerdb.Querier
	audit  auditlog.Querier
	reg    *table.Registry
	hub    Hub
	pump   ws.PumpFunc
}

func New(
	users user.Querier,
	hands pokerdb.Querier,
	audit auditlog.Querier,
	reg *table.Registry,
	hub Hub,
	pump ws.PumpFunc,
) *Handler {
	return &Handler{
		users: users,
		hands: hands,
		audit: audit,
		reg:   reg,
		hub:   hub,
		pump:  pump,
	}
}

func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/dashboard", h.Dashboard)
	rg.GET("/tables", h.ListTables)
	rg.POST("/tables", h.CreateTable)
	rg.GET("/tables/:id", h.GetTable)
	rg.PUT("/tables/:id", h.UpdateTable)
	rg.DELETE("/tables/:id", h.DeleteTable)
	rg.GET("/users", h.ListUsers)
	rg.POST("/users/:id/chips", h.AdjustBalance)
	rg.GET("/hands", h.ListHands)
	rg.GET("/hands/:id", h.GetHand)
}

func (h *Handler) actorUUID(c *gin.Context) (uuid.UUID, bool) {
	raw, ok := c.Get(auth.CtxKeyUserID)
	if !ok {
		return uuid.UUID{}, false
	}
	s, _ := raw.(string)
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.UUID{}, false
	}
	return id, true
}

func (h *Handler) writeAudit(c *gin.Context, action string, resourceType, resourceID *string, payload any) {
	actor, ok := h.actorUUID(c)
	if !ok {
		return
	}
	b, err := json.Marshal(payload)
	if err != nil {
		b = []byte(`{}`)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ip := c.ClientIP()
	var ipPtr *string
	if ip != "" {
		ipPtr = &ip
	}
	if _, err := h.audit.InsertAuditLog(ctx, auditlog.InsertAuditLogParams{
		ActorUserID:  actor,
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Payload:      b,
		IpAddress:    ipPtr,
	}); err != nil {
		log.Warn().Err(err).Str("action", action).Msg("admin: audit_log insert failed")
	}
}

func (h *Handler) Dashboard(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 8*time.Second)
	defer cancel()

	now := time.Now().UTC()
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	dayEnd := dayStart.Add(24 * time.Hour)

	handsToday, err := h.hands.CountHandsBetween(ctx, pokerdb.CountHandsBetweenParams{
		CreatedAt:   pgtype.Timestamptz{Time: dayStart, Valid: true},
		CreatedAt_2: pgtype.Timestamptz{Time: dayEnd, Valid: true},
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "dashboard: hands count"})
		return
	}

	tablesRunning := len(h.reg.List())
	activeWS := h.hub.ConnectedUserCount()

	res := gin.H{
		"connected_users_ws": activeWS,
		"tables_running":     tablesRunning,
		"hands_today":        handsToday,
	}
	rt := "dashboard"
	h.writeAudit(c, "admin.dashboard.view", &rt, nil, gin.H{"hands_today": handsToday, "tables_running": tablesRunning})
	c.JSON(http.StatusOK, res)
}

func (h *Handler) ListTables(c *gin.Context) {
	list := h.reg.List()
	out := make([]gin.H, 0, len(list))
	for _, t := range list {
		out = append(out, gin.H{
			"id":                   t.ID.String(),
			"name":                 t.Name,
			"max_seats":            t.MaxSeats,
			"small_blind":          t.SmallBlind,
			"big_blind":            t.BigBlind,
			"turn_timeout_seconds": int(t.TurnTimeout.Seconds()),
			"created_at":           t.CreatedAt.UTC().Format(time.RFC3339Nano),
		})
	}
	rt := "table"
	h.writeAudit(c, "admin.table.list", &rt, nil, gin.H{"count": len(out)})
	c.JSON(http.StatusOK, gin.H{"tables": out})
}

func (h *Handler) CreateTable(c *gin.Context) {
	var body table.CreateTableBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if err := body.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	rt, err := h.reg.CreateTable(body.Name, body.MaxSeats, body.SmallBlind, body.BigBlind, body.TurnDuration(), h.pump)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	idStr := rt.ID.String()
	rtype := "table"
	h.writeAudit(c, "admin.table.create", &rtype, &idStr, body)
	c.JSON(http.StatusCreated, gin.H{
		"id":                   idStr,
		"name":                 rt.Name,
		"max_seats":            rt.MaxSeats,
		"small_blind":          rt.SmallBlind,
		"big_blind":            rt.BigBlind,
		"turn_timeout_seconds": body.TurnTimeoutSeconds,
	})
}

func (h *Handler) GetTable(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid table id"})
		return
	}
	rt, ok := h.reg.Get(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "table not found"})
		return
	}
	idStr := id.String()
	rtype := "table"
	h.writeAudit(c, "admin.table.get", &rtype, &idStr, nil)
	c.JSON(http.StatusOK, gin.H{
		"id":                   rt.ID.String(),
		"name":                 rt.Name,
		"max_seats":            rt.MaxSeats,
		"small_blind":          rt.SmallBlind,
		"big_blind":            rt.BigBlind,
		"turn_timeout_seconds": int(rt.TurnTimeout.Seconds()),
		"created_at":           rt.CreatedAt.UTC().Format(time.RFC3339Nano),
	})
}

func (h *Handler) UpdateTable(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid table id"})
		return
	}
	var body table.CreateTableBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if err := body.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 12*time.Second)
	defer cancel()
	if err := h.reg.UpdateTable(ctx, id, body.Name, body.MaxSeats, body.SmallBlind, body.BigBlind, body.TurnDuration()); err != nil {
		if err.Error() == "table not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	idStr := id.String()
	rtype := "table"
	h.writeAudit(c, "admin.table.update", &rtype, &idStr, body)
	c.JSON(http.StatusOK, gin.H{"id": idStr, "updated": true})
}

func (h *Handler) DeleteTable(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid table id"})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 12*time.Second)
	defer cancel()
	if err := h.reg.DeleteTable(ctx, id); err != nil {
		if err.Error() == "table not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	h.hub.EvictTable(id)
	h.hub.ClearPump(id)
	idStr := id.String()
	rtype := "table"
	h.writeAudit(c, "admin.table.delete", &rtype, &idStr, nil)
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

func parseLimitOffset(c *gin.Context, defLimit, maxLimit int) (limit, offset int) {
	limit = defLimit
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	if limit > maxLimit {
		limit = maxLimit
	}
	if v := c.Query("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}
	return limit, offset
}

func (h *Handler) ListUsers(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	limit, offset := parseLimitOffset(c, 50, 200)
	q := c.Query("q")
	var out []gin.H
	if q != "" {
		pat := "%" + q + "%"
		list, err := h.users.SearchUsersForAdmin(ctx, user.SearchUsersForAdminParams{
			Email:  pat,
			Limit:  int32(limit),
			Offset: int32(offset),
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "list users failed"})
			return
		}
		out = make([]gin.H, 0, len(list))
		for _, u := range list {
			out = append(out, userPublicJSON(u.ID, u.Email, u.Role, u.ChipsBalance, u.CreatedAt, u.UpdatedAt))
		}
	} else {
		list, err := h.users.ListUsersForAdmin(ctx, user.ListUsersForAdminParams{
			Limit:  int32(limit),
			Offset: int32(offset),
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "list users failed"})
			return
		}
		out = make([]gin.H, 0, len(list))
		for _, u := range list {
			out = append(out, userPublicJSON(u.ID, u.Email, u.Role, u.ChipsBalance, u.CreatedAt, u.UpdatedAt))
		}
	}
	rt := "user"
	h.writeAudit(c, "admin.user.list", &rt, nil, gin.H{"limit": limit, "offset": offset, "q": q})
	c.JSON(http.StatusOK, gin.H{"users": out})
}

func userPublicJSON(id uuid.UUID, email, role string, chips int64, created, updated pgtype.Timestamptz) gin.H {
	return gin.H{
		"id":            id.String(),
		"email":         email,
		"role":          role,
		"chips_balance": chips,
		"created_at":    timestamptzRFC(created),
		"updated_at":    timestamptzRFC(updated),
	}
}

func timestamptzRFC(t pgtype.Timestamptz) string {
	if !t.Valid {
		return ""
	}
	return t.Time.UTC().Format(time.RFC3339Nano)
}

type adjustChipsBody struct {
	Delta  int64  `json:"delta"`
	Reason string `json:"reason"`
}

func (h *Handler) AdjustBalance(c *gin.Context) {
	uid, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}
	var body adjustChipsBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if body.Reason == "" || len(body.Reason) > 500 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "reason required, max 500 chars"})
		return
	}
	if body.Delta == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "delta must be non-zero"})
		return
	}
	actor, ok := h.actorUUID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid actor"})
		return
	}
	meta := map[string]any{
		"reason":    body.Reason,
		"admin_id":  actor.String(),
	}
	metaBytes, err := json.Marshal(meta)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "metadata"})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	u, err := h.users.ApplyChipsDelta(ctx, user.ApplyChipsDeltaParams{
		ID:       uid,
		Amount:   body.Delta,
		Type:     "admin_adjust",
		HandID:   pgtype.UUID{Valid: false},
		Metadata: metaBytes,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "insufficient balance or user not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "adjust balance failed"})
		return
	}
	idStr := uid.String()
	rtype := "user"
	h.writeAudit(c, "admin.user.balance_adjust", &rtype, &idStr, gin.H{
		"delta":           body.Delta,
		"reason":          body.Reason,
		"chips_balance_new": u.ChipsBalance,
	})
	c.JSON(http.StatusOK, gin.H{
		"id":             u.ID.String(),
		"chips_balance": u.ChipsBalance,
	})
}

func (h *Handler) ListHands(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	limit, offset := parseLimitOffset(c, 50, 200)
	list, err := h.hands.ListHandsForAdmin(ctx, pokerdb.ListHandsForAdminParams{
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "list hands failed"})
		return
	}
	out := make([]gin.H, 0, len(list))
	for _, hn := range list {
		out = append(out, handToJSON(hn))
	}
	rt := "hand"
	h.writeAudit(c, "admin.hand.list", &rt, nil, gin.H{"limit": limit, "offset": offset})
	c.JSON(http.StatusOK, gin.H{"hands": out})
}

func handToJSON(h pokerdb.Hand) gin.H {
	m := gin.H{
		"id":           h.ID.String(),
		"room_id":      h.RoomID,
		"hand_number":  h.HandNumber,
		"dealer_seat":  h.DealerSeat,
		"small_blind":  h.SmallBlind,
		"big_blind":    h.BigBlind,
		"status":       h.Status,
		"created_at":   timestamptzRFC(h.CreatedAt),
		"completed_at": timestamptzRFC(h.CompletedAt),
	}
	if h.PotTotal != nil {
		m["pot_total"] = *h.PotTotal
	}
	return m
}

func (h *Handler) GetHand(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid hand id"})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	hand, err := h.hands.GetHandByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "hand not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "get hand failed"})
		return
	}
	acts, err := h.hands.ListHandActionsByHand(ctx, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "list actions failed"})
		return
	}
	aj := make([]gin.H, 0, len(acts))
	for _, a := range acts {
		aj = append(aj, handActionToJSON(a))
	}
	idStr := id.String()
	rtype := "hand"
	h.writeAudit(c, "admin.hand.get", &rtype, &idStr, gin.H{"actions_count": len(aj)})
	c.JSON(http.StatusOK, gin.H{
		"hand":    handToJSON(hand),
		"actions": aj,
	})
}

func handActionToJSON(a pokerdb.HandAction) gin.H {
	m := gin.H{
		"id":          a.ID,
		"hand_id":     a.HandID.String(),
		"action_seq":  a.ActionSeq,
		"action_type": a.ActionType,
		"street":      a.Street,
		"is_timeout":  a.IsTimeout,
		"created_at":  timestamptzRFC(a.CreatedAt),
	}
	if a.TableSeat != nil {
		m["table_seat"] = *a.TableSeat
	}
	if a.HandPlayerIndex != nil {
		m["hand_player_index"] = *a.HandPlayerIndex
	}
	if a.UserID.Valid {
		m["user_id"] = uuid.UUID(a.UserID.Bytes).String()
	}
	if a.Amount != nil {
		m["amount"] = *a.Amount
	}
	return m
}
