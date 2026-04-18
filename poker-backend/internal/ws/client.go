package ws

import (
	"sync"

	"github.com/google/uuid"
	"nhooyr.io/websocket"
)

type Client struct {
	hub    *Hub
	userID uuid.UUID
	conn   *websocket.Conn
	send   chan []byte

	closeOnce sync.Once
}

func newClient(h *Hub, uid uuid.UUID, c *websocket.Conn) *Client {
	return &Client{
		hub:    h,
		userID: uid,
		conn:   c,
		send:   make(chan []byte, 64),
	}
}

func (c *Client) closeSend() {
	c.closeOnce.Do(func() {
		close(c.send)
	})
}
