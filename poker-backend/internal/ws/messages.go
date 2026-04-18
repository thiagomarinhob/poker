package ws

// Mensagens JSON: {"type":"...","payload":{...}}

const (
	TypeJoinTable  = "JoinTable"
	TypeLeaveTable = "LeaveTable"
	TypeAction     = "Action"
	TypeChat       = "Chat" // cliente→servidor e eco servidor→cliente
	TypePing       = "Ping"
)

const (
	TypeTableState      = "TableState"
	TypePlayerJoined    = "PlayerJoined"
	TypeActionRequiredS = "ActionRequired"
	TypeHandResult      = "HandResult"
	TypeError           = "Error"
	TypePong            = "Pong"
)

type envelope struct {
	Type    string         `json:"type"`
	Payload map[string]any `json:"payload"`
}
