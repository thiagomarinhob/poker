package table

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/thiagomarinho/poker-backend/internal/ws"
)

type Handler struct {
	reg  *Registry
	pump ws.PumpFunc
}

func NewHandler(reg *Registry, pump ws.PumpFunc) *Handler {
	return &Handler{reg: reg, pump: pump}
}

// RegisterPlayer expõe leitura de mesas para qualquer usuário autenticado (lobby).
func (h *Handler) RegisterPlayer(rg *gin.RouterGroup) {
	rg.GET("/tables", h.List)
}

func (h *Handler) List(c *gin.Context) {
	list := h.reg.List()
	out := make([]gin.H, 0, len(list))
	for _, t := range list {
		out = append(out, gin.H{
			"id":                     t.ID.String(),
			"name":                   t.Name,
			"max_seats":              t.MaxSeats,
			"small_blind":            t.SmallBlind,
			"big_blind":              t.BigBlind,
			"turn_timeout_seconds":   int(t.TurnTimeout.Seconds()),
			"created_at":             t.CreatedAt.UTC().Format(time.RFC3339Nano),
		})
	}
	c.JSON(http.StatusOK, gin.H{"tables": out})
}
